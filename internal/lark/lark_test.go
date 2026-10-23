package lark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	codexmodel "agentor/internal/codex/model"
	larkmodel "agentor/internal/lark/model"
	"agentor/internal/tasks"
	tasksmodel "agentor/internal/tasks/model"
	"agentor/internal/workspace"

	larksdk "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// TestConnectionWithoutEnableSwitch 验证连接不再依赖启用开关
func TestConnectionWithoutEnableSwitch(t *testing.T) {
	s, err := NewService(t.TempDir(), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.ServiceShutdown() })

	view, err := s.SaveSettings(larkmodel.SettingsInput{Settings: larkmodel.Settings{Region: "feishu"}})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status.Phase != "unconfigured" {
		t.Fatal("缺少凭据时应提示待配置")
	}

	s.Disconnect()
	if s.GetState().Status.Phase != "disconnected" {
		t.Fatal("断开后应显示已断开")
	}
}

// TestNormalizeOrderAndAuthorization 验证富文本顺序和机器人身份授权
func TestNormalizeOrderAndAuthorization(t *testing.T) {
	content := `{"zh_cn":{"title":"Task","content":[[{"tag":"text","text":"first"},{"tag":"img","image_key":"img-1"},{"tag":"text","text":"last"}]]}}`
	raw := map[string]any{"event": map[string]any{"sender": map[string]any{"sender_type": "user", "sender_id": map[string]string{"open_id": "test-user"}}, "message": map[string]any{"message_id": "message", "chat_id": "group", "chat_type": "group", "message_type": "post", "content": content, "mentions": []any{map[string]any{"id": map[string]string{"open_id": "test-bot"}, "key": "@_user_1"}}}}}
	var event larkim.P2MessageReceiveV1
	json.Unmarshal([]byte(jsonText(raw)), &event)
	message, err := normalize(&event, "test-app", "test-bot")
	if err != nil {
		t.Fatal(err)
	}
	if message.Text != "Task\nfirst[图片 image-1.png]last" || len(message.Resources) != 1 || message.Source.ChatType != "group" {
		t.Fatal(message)
	}
	config := larkmodel.Settings{AllowedUsers: []larkmodel.Identity{{ID: "test-user"}}, AllowedChats: []larkmodel.Identity{{ID: "group"}}}
	if !authorized(config, "test-bot", message) {
		t.Fatal("authorized @bot rejected")
	}
	message.Mentions = []string{"other-bot"}
	if authorized(config, "test-bot", message) {
		t.Fatal("mentioning another bot was accepted")
	}
	message.ChatType = "p2p"
	message.SenderType = "bot"
	if authorized(config, "test-bot", message) {
		t.Fatal("bot sender accepted")
	}
}

// TestUnauthorizedMediaAndDuplicatePairingDoNotSubmitTasks 验证未授权资源不下载且配对消息不重复入队
func TestUnauthorizedMediaAndDuplicatePairingDoNotSubmitTasks(t *testing.T) {
	var requests atomic.Int32
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"code":0}`)
	})
	message := larkmodel.Message{Source: tasksmodel.Source{Type: "lark", AppID: "test-app", SenderID: "stranger", ChatID: "chat", MessageID: "blocked", ChatType: "p2p"}, ChatType: "p2p", SenderType: "user", Text: "image", Resources: []larkmodel.Resource{{Key: "img", Kind: "image", Name: "image.png"}}}
	if err := s.receive(s.ctx, 1, s.client, message); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 0 || len(s.GetState().Incoming) != 0 {
		t.Fatal("unauthorized media was downloaded or persisted")
	}
	pairing, err := s.StartPairing("test-user")
	if err != nil {
		t.Fatal(err)
	}
	message.Source.SenderID, message.Source.MessageID = "test-user", "pair-message"
	message.Text = "/pair " + pairing.Code
	message.Resources = nil
	if err = s.receive(s.ctx, 1, s.client, message); err != nil {
		t.Fatal(err)
	}
	if err = s.receive(s.ctx, 1, s.client, message); err != nil {
		t.Fatal(err)
	}
	wait(t, func() bool {
		view := s.GetState()
		return len(view.Incoming) == 1 && view.Incoming[0].Status == "control"
	})
	if view := s.GetState(); view.Binding.ChatID != "chat" || view.Binding.SenderID != "test-user" || len(s.tasks.GetState().Requests) != 0 {
		t.Fatal(view)
	}
	if err = s.pair(message.Source, pairing.Code); err == nil {
		t.Fatal("pairing code reused")
	}
	raw, _ := json.Marshal(s.GetState())
	if strings.Contains(string(raw), "local-test-secret") || strings.Contains(string(raw), "local-test-token") {
		t.Fatal("credential leaked in public DTO")
	}
}

// TestDeliveryRecordsSentFailedUnknownAndRetriesOnlyFailure 验证明确成功失败与未知结果分别记录
func TestDeliveryRecordsSentFailedUnknownAndRetriesOnlyFailure(t *testing.T) {
	var calls atomic.Int32
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch {
		case strings.HasSuffix(r.URL.Path, "/ok/reply"):
			fmt.Fprint(w, `{"code":0,"data":{"message_id":"sent-id"}}`)
		case strings.HasSuffix(r.URL.Path, "/denied/reply"):
			fmt.Fprint(w, `{"code":230001,"msg":"permission denied"}`)
		default:
			connection, _, _ := w.(http.Hijacker).Hijack()
			connection.Close()
		}
	})
	s.tasks.SetNotificationHandler(s.OnNotification)
	for _, messageID := range []string{"ok", "denied", "unknown"} {
		id := "notification-" + messageID
		if err := s.tasks.RecordNotification(tasksmodel.Notification{ID: id, Body: "test", Source: tasksmodel.Source{Type: "lark", AppID: "test-app", SenderID: "test-user", ChatID: "chat", MessageID: messageID, ChatType: "p2p"}}); err != nil {
			t.Fatal(err)
		}
	}
	wait(t, func() bool {
		for _, notification := range s.tasks.GetState().Notifications {
			if len(notification.Parts) != 2 {
				return false
			}
			part := notification.Parts[1]
			if part.AttemptedAt == "" || part.Error == "发送中；未收到明确响应前请勿重发" {
				return false
			}
		}
		return len(s.tasks.GetState().Notifications) == 3
	})
	parts := map[string]tasksmodel.NotificationPart{}
	wait(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()

		return len(s.sending) == 0
	})
	for _, notification := range s.tasks.GetState().Notifications {
		parts[notification.ID] = notification.Parts[1]
	}
	if parts["notification-ok"].Status != "sent" || parts["notification-denied"].Status != "failed" || parts["notification-unknown"].Status != "unknown" {
		t.Fatal(parts)
	}
	if err := s.RetryNotificationPart("notification-unknown", parts["notification-unknown"].ID); err == nil {
		t.Fatal("unknown delivery retried")
	}
	if err := s.IgnoreNotificationPart("notification-unknown", parts["notification-unknown"].ID, "checked manually"); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	if err := s.RetryNotificationPart("notification-denied", parts["notification-denied"].ID); err != nil {
		t.Fatal(err)
	}
	wait(t, func() bool { return calls.Load() > before })
	wait(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()

		return len(s.sending) == 0
	})
	if err := s.IgnoreNotificationPart("notification-ok", parts["notification-ok"].ID, "ignore"); err == nil {
		t.Fatal("sent item ignored")
	}
}

// TestApprovalRouteChecksIdentityAndExpiredRuntime 验证审批身份归属与上游失效检查
func TestApprovalRouteChecksIdentityAndExpiredRuntime(t *testing.T) {
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected outbound request") })
	s.state.Approvals = []larkmodel.ApprovalRoute{{Key: "route", ApprovalID: "42", Generation: 1, Status: "pending", Source: tasksmodel.Source{Type: "lark", AppID: "test-app", SenderID: "test-user", ChatID: "chat", ChatType: "p2p"}}}
	wrong := tasksmodel.Source{Type: "lark", AppID: "test-app", SenderID: "other", ChatID: "chat"}
	if err := s.respondRoute("route", wrong, "accept", "/approve"); err == nil {
		t.Fatal("wrong recipient accepted")
	}
	wrong.SenderID, wrong.ChatID = "test-user", "other-chat"
	if err := s.respondRoute("route", wrong, "accept", "/approve"); err == nil {
		t.Fatal("wrong chat accepted")
	}
	wrong.ChatID = "chat"
	if err := s.respondRoute("route", wrong, "accept", "/approve"); err == nil {
		t.Fatal("expired runtime approval accepted")
	}
	if len(s.tasks.GetState().Requests) != 0 {
		t.Fatal("approval response entered controller queue")
	}
	card := approvalCard(codexmodel.Approval{ID: "42", Method: "item/permissions/requestApproval", PermissionsJSON: `{"filesystem":{"write":["/tmp/test"]}}`}, "route")
	if card["schema"] != "2.0" {
		t.Fatal("card schema")
	}
	body := approvalBody(codexmodel.Approval{Changes: []codexmodel.FileChange{{Path: "/tmp/check.txt", Kind: codexmodel.FileChangeKind{Type: "add"}, Diff: "CHECK"}}})
	if !strings.Contains(body, "/tmp/check.txt") || !strings.Contains(body, "CHECK") {
		t.Fatal("远程卡片缺少文件修改内容")
	}
}

// TestResourceDownloadKeepsPartialFailure 验证下载成功和失败的资源分别归属原请求
func TestResourceDownloadKeepsPartialFailure(t *testing.T) {
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/resources/file-ok"):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="note.md"`)
			fmt.Fprint(w, "RESOURCE_CHECK")
		case strings.HasSuffix(r.URL.Path, "/resources/file-failed"):
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"code":230001,"msg":"permission denied"}`)
		default:
			fmt.Fprint(w, `{"code":0,"data":{"reaction_id":"busy"}}`)
		}
	})
	message := larkmodel.Message{Source: tasksmodel.Source{Type: "lark", AppID: "test-app", SenderID: "test-user", ChatID: "chat", ChatType: "p2p", MessageID: "resource-message"}, Text: "读取附件", Resources: []larkmodel.Resource{{Key: "file-ok", Kind: "file", Name: "note.md"}, {Key: "file-failed", Kind: "file", Name: "failed.md"}}}
	s.processMessage(s.ctx, 1, s.client, message, "resource-request", "")
	items := s.files.GetRequestAttachments("resource-request")
	if len(items) != 2 || items[0].Status != "ready" || items[1].Status != "failed" || !strings.Contains(items[1].Error, "permission denied") {
		t.Fatal(items)
	}
}

// fixture 建立使用官方 SDK 和本地接口的测试环境
func fixture(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server) {
	t.Helper()
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	files, err := artifact.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	runtime := codex.NewService(data, files)
	executions, err := tasks.NewService(data, runtime, work, files)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService(data.GetInfo().DataDirectory, executions, work, files, runtime, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "tenant_access_token") {
			fmt.Fprint(w, `{"code":0,"tenant_access_token":"local-test-token","expire":7200}`)
			return
		}
		handler(w, r)
	}))
	s.config = credentials{Settings: larkmodel.Settings{AppID: "test-app", Region: "feishu", AllowedUsers: []larkmodel.Identity{{ID: "test-user"}}}, AppSecret: "local-test-secret"}
	s.status = larkmodel.Status{Phase: "ready", BotOpenID: "test-bot", Generation: 1}
	s.client = larksdk.NewClient("test-app", "local-test-secret", larksdk.WithOpenBaseUrl(server.URL), larksdk.WithHttpClient(server.Client()), larksdk.WithLogger(quietLogger{}))
	s.connectionCtx = s.ctx
	t.Cleanup(func() {
		s.ServiceShutdown()
		executions.ServiceShutdown()
		runtime.ServiceShutdown()
		server.Close()
	})
	return s, server
}

// wait 等待异步状态达到测试条件
func wait(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out")
}

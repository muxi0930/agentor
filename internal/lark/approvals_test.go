package lark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	codexmodel "agentor/internal/codex/model"
	larkmodel "agentor/internal/lark/model"
	tasksmodel "agentor/internal/tasks/model"
)

// TestUpdateApprovalCardPreservesOriginalContent 验证结果更新保留原正文并移除审批按钮
func TestUpdateApprovalCardPreservesOriginalContent(t *testing.T) {
	approvals := []codexmodel.Approval{
		{Method: "item/commandExecution/requestApproval", Reason: "需要读取后端会话", Command: "git status", Cwd: "/tmp/project", Details: "仅检查工作区"},
		{Method: "item/fileChange/requestApproval", Reason: "修复配置", GrantRoot: "/tmp/project", Changes: []codexmodel.FileChange{{Path: "/tmp/project/config", Kind: codexmodel.FileChangeKind{Type: "add"}, Diff: "+enabled=true"}}},
		{Method: "item/permissions/requestApproval", Reason: "读取所需资源", PermissionsJSON: `{"network":{"enabled":true}}`},
		{Method: "item/tool/requestUserInput", Questions: []codexmodel.Question{{ID: "choice", Header: "执行方式", Question: "选择检查范围", Options: []codexmodel.QuestionOption{{Label: "本地", Description: "仅检查本地"}}}}},
		{Method: "mcpServer/elicitation/request", Message: "确认资源访问", URL: "https://example.invalid/confirm", SchemaJSON: `{"type":"object"}`},
		{Method: "item/commandExecution/requestApproval", Reason: "长正文保持原截断规则", Command: strings.Repeat("检查内容", 1600)},
	}
	for _, approval := range approvals {
		for _, status := range []string{"resolved", "expired"} {
			t.Run(approval.Method+"/"+status, func(t *testing.T) {
				contents := make(chan string, 1)
				s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPatch || !strings.HasSuffix(r.URL.Path, "/messages/card-message") {
						t.Errorf("非预期更新请求：%s %s", r.Method, r.URL.Path)
					}
					var body struct {
						Content string `json:"content"` // 更新卡片的完整内容
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					contents <- body.Content
					fmt.Fprint(w, `{"code":0}`)
				})
				approval.ID, approval.ThreadID, approval.TurnID = "12", "thread", "turn"
				approval.DecisionOptions = []codexmodel.DecisionOption{{Value: "accept", Label: "允许"}, {Value: "decline", Label: "拒绝"}}
				initial := approvalCard(approval, "route")
				original := initial["body"].(map[string]any)["elements"].([]any)[0].(map[string]string)["content"]
				notification := tasksmodel.Notification{ID: "approval-notification", Body: original}
				if err := s.tasks.RecordNotification(notification); err != nil {
					t.Fatal(err)
				}
				route := larkmodel.ApprovalRoute{Key: "route", ApprovalID: "12", NotificationID: notification.ID, MessageID: "card-message", Source: tasksmodel.Source{Type: "lark", AppID: "test-app", ChatID: "chat", SenderID: "test-user", ChatType: "p2p"}}
				s.updateApprovalCard(route, status)
				select {
				case content := <-contents:
					var updated struct {
						Schema string `json:"schema"` // 卡片协议版本
						Body   struct {
							Elements []struct {
								Tag     string `json:"tag"`     // 元素类型
								Content string `json:"content"` // 正文内容
							} `json:"elements"` // 卡片正文元素
						} `json:"body"` // 卡片正文
					}
					if err := json.Unmarshal([]byte(content), &updated); err != nil {
						t.Fatal(err)
					}
					if updated.Schema != "2.0" || len(updated.Body.Elements) != 1 || updated.Body.Elements[0].Tag != "markdown" {
						t.Fatalf("结果卡片结构错误或仍含操作按钮：%s", content)
					}
					if got := updated.Body.Elements[0].Content; got != original+"\n\n状态："+status {
						t.Fatalf("原审批内容丢失或结果错误：\n%s", got)
					}
				default:
					t.Fatal("未更新审批卡片")
				}
			})
		}
	}
}

// TestApprovalResolvedBeforeDeliveryPreservesContent 验证审批先完成时发送后的补充更新保留正文
func TestApprovalResolvedBeforeDeliveryPreservesContent(t *testing.T) {
	contents := make(chan string, 1)
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/original-message/reply"):
			fmt.Fprint(w, `{"code":0,"data":{"message_id":"late-card"}}`)
		case r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/messages/late-card"):
			var body struct {
				Content string `json:"content"` // 更新卡片的完整内容
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			contents <- body.Content
			fmt.Fprint(w, `{"code":0}`)
		default:
			t.Errorf("非预期请求：%s %s", r.Method, r.URL.Path)
			fmt.Fprint(w, `{"code":1}`)
		}
	})
	approval := codexmodel.Approval{ID: "12", ThreadID: "thread", TurnID: "turn", Reason: "检查待审批操作", Command: "git status", DecisionOptions: []codexmodel.DecisionOption{{Value: "accept", Label: "允许"}}}
	source := tasksmodel.Source{Type: "lark", AppID: "test-app", SenderID: "test-user", ChatID: "chat", ChatType: "p2p", MessageID: "original-message"}
	part := tasksmodel.NotificationPart{ID: "approval-part", Kind: "approval", Channel: "lark", Status: "pending", PayloadJSON: jsonText(approvalCard(approval, "route")), Content: approvalBody(approval)}
	notification := tasksmodel.Notification{ID: "late-notification", Body: part.Content, Source: source, Parts: []tasksmodel.NotificationPart{part}}
	if err := s.tasks.RecordNotification(notification); err != nil {
		t.Fatal(err)
	}
	s.state.Approvals = []larkmodel.ApprovalRoute{{Key: "route", ApprovalID: approval.ID, Source: source, NotificationID: notification.ID, Status: "resolved"}}
	s.deliver(notification, part)
	wait(t, func() bool { return len(contents) > 0 })
	var updated struct {
		Body struct {
			Elements []struct {
				Tag     string `json:"tag"`     // 元素类型
				Content string `json:"content"` // 正文内容
			} `json:"elements"` // 卡片正文元素
		} `json:"body"` // 卡片正文
	}
	if err := json.Unmarshal([]byte(<-contents), &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Body.Elements) != 1 || updated.Body.Elements[0].Tag != "markdown" || updated.Body.Elements[0].Content != notification.Body+"\n\n状态：resolved" {
		t.Fatalf("延迟发送后的结果卡片丢失正文或仍含操作按钮：%+v", updated)
	}
}

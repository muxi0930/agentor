package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentor/internal/appdata"
	"agentor/internal/codex"
	codexmodel "agentor/internal/codex/model"
	larkmodel "agentor/internal/lark/model"
	tasksmodel "agentor/internal/tasks/model"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

// TestMain 仅在模拟子进程中以测试程序代替真实 Codex，其他情况正常运行测试
func TestMain(m *testing.M) {
	if output := os.Getenv("AGENTOR_TEST_ELICITATION_OUTPUT"); output != "" && len(os.Args) == 2 {
		switch os.Args[1] {
		case "--version":
			fmt.Println("codex-local-elicitation-test")
			os.Exit(0)
		case "app-server":
			decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
			for {
				var message struct {
					ID     json.RawMessage `json:"id"`     // 协议请求标识
					Method string          `json:"method"` // 协议请求方法
					Result json.RawMessage `json:"result"` // 客户端回传结果
				}
				if decoder.Decode(&message) != nil {
					os.Exit(0)
				}
				switch message.Method {
				case "initialize":
					encoder.Encode(map[string]any{"id": message.ID, "result": map[string]string{"userAgent": "local-test"}})
				case "initialized":
					encoder.Encode(map[string]any{"id": 4, "method": "mcpServer/elicitation/request", "params": map[string]any{"threadId": "thread", "turnId": "turn", "serverName": "computer-use", "mode": os.Getenv("AGENTOR_TEST_ELICITATION_MODE"), "message": `Allow Computer Use to use "agentor"?`, "requestedSchema": json.RawMessage(os.Getenv("AGENTOR_TEST_ELICITATION_SCHEMA"))}})
				default:
					if string(message.ID) == "4" && len(message.Result) > 0 {
						if err := os.WriteFile(output, message.Result, 0600); err != nil {
							os.Exit(1)
						}
					}
				}
			}
		}
	}
	os.Exit(m.Run())
}

// TestElicitationCardQuickAccept 验证仅明确的空对象表单提供快捷同意
func TestElicitationCardQuickAccept(t *testing.T) {
	for _, test := range []struct {
		name   string // 场景名称
		mode   string // 问答模式
		schema string // 原始表单定义
		accept bool   // 是否提供快捷同意
	}{
		{"computer-use", "form", `{"type":"object","properties":{}}`, true},
		{"extended-form", "openai/form", `{"type":"object","properties":{},"required":[],"title":"授权","description":"确认访问"}`, true},
		{"required-field", "form", `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`, false},
		{"optional-field", "form", `{"type":"object","properties":{"name":{"type":"string"}}}`, false},
		{"unsupported-field", "form", `{"type":"object","properties":{"nested":{"type":"object"}}}`, false},
		{"required-without-property", "form", `{"type":"object","properties":{},"required":["name"]}`, false},
		{"composed-schema", "openai/form", `{"type":"object","properties":{},"allOf":[{"required":["name"]}]}`, false},
		{"minimum-properties", "form", `{"type":"object","properties":{},"minProperties":1}`, false},
		{"missing-properties", "form", `{"type":"object"}`, false},
		{"null-properties", "form", `{"type":"object","properties":null}`, false},
		{"invalid-schema", "form", `{`, false},
		{"missing-schema", "form", ``, false},
		{"url", "url", `{"type":"object","properties":{}}`, false},
		{"verification", "openai/userVerification", `{"type":"object","properties":{}}`, false},
		{"unknown-mode", "unknown", `{"type":"object","properties":{}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			approval := codexmodel.Approval{ID: "4", Method: "mcpServer/elicitation/request", Mode: test.mode, Message: `Allow Computer Use to use "agentor"?`, SchemaJSON: test.schema}
			initial := approvalCard(approval, "route")
			elements := initial["body"].(map[string]any)["elements"].([]any)
			decisions := map[string]string{}
			columns := elements[1].(map[string]any)["columns"].([]any)
			for _, column := range columns {
				button := column.(map[string]any)["elements"].([]any)[0].(map[string]any)
				behavior := button["behaviors"].([]any)[0].(map[string]any)
				value := behavior["value"].(map[string]string)
				if value["routeKey"] != "route" || behavior["type"] != "callback" {
					t.Fatalf("未使用原审批回调：%+v", behavior)
				}
				decisions[value["decision"]] = button["text"].(map[string]string)["content"]
			}
			if (decisions["accept"] == "同意") != test.accept || decisions["decline"] != "拒绝" || decisions["cancel"] != "取消" {
				t.Fatalf("审批按钮错误：%+v，快捷同意应为 %v", decisions, test.accept)
			}
			body := elements[0].(map[string]string)["content"]
			if !strings.Contains(body, approval.Message) || !strings.Contains(body, test.schema) {
				t.Fatal("卡片丢失审批原内容")
			}
			if test.accept && strings.Contains(body, "/elicitation") {
				t.Fatal("空表单仍要求手写 JSON")
			}
		})
	}
}

// TestElicitationCardCallback 验证卡片回调经真实路由与运行时向模拟上游提交正确结果
func TestElicitationCardCallback(t *testing.T) {
	for _, test := range []struct {
		name     string // 场景名称
		mode     string // 问答模式
		schema   string // 原始表单定义
		decision string // 用户点击选项
		content  string // 预期答案内容
		manual   bool   // 是否需拒绝快捷接受后手动填写
	}{
		{"accept", "form", `{"type":"object","properties":{}}`, "accept", `{}`, false},
		{"decline", "form", `{"type":"object","properties":{}}`, "decline", `null`, false},
		{"cancel", "form", `{"type":"object","properties":{}}`, "cancel", `null`, false},
		{"extended-accept", "openai/form", `{"type":"object","properties":{}}`, "accept", `{}`, false},
		{"required-input", "form", `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`, "accept", `{"name":"local"}`, true},
		{"unsupported-input", "openai/form", `{"type":"object","properties":{"nested":{"type":"object"}}}`, "accept", `{"nested":{}}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("不应发送飞书消息") })
			output := filepath.Join(t.TempDir(), "response.json")
			t.Setenv("AGENTOR_TEST_ELICITATION_OUTPUT", output)
			t.Setenv("AGENTOR_TEST_ELICITATION_MODE", test.mode)
			t.Setenv("AGENTOR_TEST_ELICITATION_SCHEMA", test.schema)
			data, err := appdata.NewService("test")
			if err != nil {
				t.Fatal(err)
			}
			if err := data.SetCodexHome("custom", t.TempDir()); err != nil {
				t.Fatal(err)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := data.SetCodexPath(binary); err != nil {
				t.Fatal(err)
			}
			s.runtime = codex.NewService(data, nil)
			t.Cleanup(func() { s.runtime.ServiceShutdown() })
			status, err := s.runtime.Connect()
			if err != nil {
				t.Fatal(err)
			}
			wait(t, func() bool { return len(s.runtime.GetApprovals()) == 1 })
			approval := s.runtime.GetApprovals()[0]
			source := tasksmodel.Source{Type: "lark", AppID: "test-app", ChatID: "chat", SenderID: "test-user", ChatType: "p2p", MessageID: "original"}
			s.state.Approvals = []larkmodel.ApprovalRoute{{Key: "route", ApprovalID: approval.ID, Generation: status.Generation, Source: source, Status: "pending"}}
			if _, err := os.Stat(output); !os.IsNotExist(err) || approval.Status != "pending" {
				t.Fatal("用户点击前不应自动回复")
			}
			event := &callback.CardActionTriggerEvent{Event: &callback.CardActionTriggerRequest{Operator: &callback.Operator{OpenID: "test-user"}, Context: &callback.Context{OpenChatID: "chat"}, Action: &callback.CallBackAction{Value: map[string]any{"routeKey": "route", "decision": test.decision}}}}
			response, err := s.cardAction(context.Background(), 1, event)
			if err != nil || response.Card != nil {
				t.Fatalf("回调异常或覆盖了原审批内容：%+v %v", response, err)
			}
			if test.manual {
				if response.Toast.Type != "error" || s.runtime.GetApprovals()[0].Status != "pending" {
					t.Fatal("有字段的问答被快捷同意")
				}
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatal("拒绝快捷接受时不应向上游写入")
				}
				if err := s.respondRoute("route", source, test.content, "/elicitation"); err != nil {
					t.Fatal(err)
				}
			} else if response.Toast.Type != "success" {
				t.Fatalf("快捷响应失败：%+v", response.Toast)
			}
			wait(t, func() bool { _, err := os.Stat(output); return err == nil })
			raw, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Action  string          `json:"action"`  // MCP 响应动作
				Content json.RawMessage `json:"content"` // MCP 答案内容
			}
			if err := json.Unmarshal(raw, &result); err != nil || result.Action != test.decision || string(result.Content) != test.content {
				t.Fatalf("上游响应错误：%s %v", raw, err)
			}
			response, err = s.cardAction(context.Background(), 1, event)
			if err != nil || response.Toast.Type != "error" || response.Card != nil {
				t.Fatal("重复回调未拒绝或覆盖了原审批内容")
			}
			if len(s.tasks.GetState().Requests) != 0 {
				t.Fatal("审批响应进入了任务队列")
			}
		})
	}
}

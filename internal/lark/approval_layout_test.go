package lark

import (
	"encoding/json"
	"reflect"
	"testing"

	"agentor/internal/codex"
)

// TestApprovalCardHorizontalLayout 验证各审批路径的按钮同排且顺序、正文和回调保持不变
func TestApprovalCardHorizontalLayout(t *testing.T) {
	options := []codex.DecisionOption{{Value: "accept", Label: "允许"}, {Value: "acceptForSession", Label: "会话内允许"}, {Value: "decline", Label: "拒绝"}, {Value: "cancel", Label: "取消"}}
	for _, test := range []struct {
		name     string                 // 场景名称
		approval codex.Approval         // 原审批请求
		options  []codex.DecisionOption // 预期按钮顺序
	}{
		{"command", codex.Approval{Method: "item/commandExecution/requestApproval", DecisionOptions: options}, options},
		{"file", codex.Approval{Method: "item/fileChange/requestApproval", DecisionOptions: options[:1]}, options[:1]},
		{"permissions", codex.Approval{Method: "item/permissions/requestApproval"}, []codex.DecisionOption{{Value: "accept", Label: "允许本轮"}, {Value: "decline", Label: "拒绝"}}},
		{"elicitation", codex.Approval{Method: "mcpServer/elicitation/request"}, []codex.DecisionOption{{Value: "decline", Label: "拒绝"}, {Value: "cancel", Label: "取消"}}},
		{"computer-use", codex.Approval{Method: "mcpServer/elicitation/request", Mode: "form", SchemaJSON: `{"type":"object","properties":{}}`}, []codex.DecisionOption{{Value: "accept", Label: "同意"}, {Value: "decline", Label: "拒绝"}, {Value: "cancel", Label: "取消"}}},
		{"user-input", codex.Approval{Method: "item/tool/requestUserInput", DecisionOptions: options}, nil},
		{"no-options", codex.Approval{}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			initial := approvalCard(test.approval, "route")
			if _, err := json.Marshal(initial); err != nil {
				t.Fatal(err)
			}
			elements := initial["body"].(map[string]any)["elements"].([]any)
			if initial["schema"] != "2.0" || !reflect.DeepEqual(elements[0], map[string]string{"tag": "markdown", "content": approvalBody(test.approval)}) {
				t.Fatal("卡片版本或审批正文改变")
			}
			if len(test.options) == 0 {
				if len(elements) != 1 {
					t.Fatal("无操作时仍生成按钮容器")
				}
				return
			}
			if len(elements) != 2 {
				t.Fatalf("按钮未集中在同一容器：%+v", elements)
			}
			row := elements[1].(map[string]any)
			if row["tag"] != "column_set" || row["flex_mode"] != "none" {
				t.Fatalf("未使用不换行的分栏布局：%+v", row)
			}
			columns := row["columns"].([]any)
			if len(columns) != len(test.options) {
				t.Fatalf("按钮数量改变：%+v", columns)
			}
			for i, option := range test.options {
				button := map[string]any{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": option.Label}, "type": "default", "behaviors": []any{map[string]any{"type": "callback", "value": map[string]string{"routeKey": "route", "decision": option.Value}}}}
				expected := map[string]any{"tag": "column", "width": "auto", "elements": []any{button}}
				if !reflect.DeepEqual(columns[i], expected) {
					t.Fatalf("第 %d 列的按钮内容、状态或回调改变：%+v", i, columns[i])
				}
			}
		})
	}
}

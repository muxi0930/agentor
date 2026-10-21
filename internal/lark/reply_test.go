package lark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"agentor/internal/codex"
	"agentor/internal/tasks"
)

// TestReplyFooterUsesLastContext 验证脚注使用最近上下文而非累计令牌，并保留未知状态
func TestReplyFooterUsesLastContext(t *testing.T) {
	window := int64(100000)
	usage := &codex.TokenUsage{Last: codex.TokenBreakdown{TotalTokens: 40000}, Total: codex.TokenBreakdown{TotalTokens: 800000}, ModelContextWindow: &window}
	if got := formatReplyFooter("gpt-6-luna", "high", usage); got != "gpt-6-luna · high · ctx 40%" {
		t.Fatal(got)
	}
	if got := formatReplyFooter("", "", nil); got != "未知模型 · 未知强度 · ctx 未知" {
		t.Fatal(got)
	}
	window = 0
	if got := formatReplyFooter("model", "low", usage); got != "model · low · ctx 未知" {
		t.Fatal(got)
	}
}

// TestReplyMarkdownSplitting 验证短回复保持原文，长代码与表格分卡后仍可独立渲染
func TestReplyMarkdownSplitting(t *testing.T) {
	short := "正文 [链接](https://example.com)\n\n```go\nfmt.Println(1)\n```\n\n| A | B |\n| --- | --- |\n| 1 | 2 |"
	if parts := splitReplyMarkdown(short); len(parts) != 1 || parts[0] != short {
		t.Fatal(parts)
	}
	for _, fence := range []string{"```go", "~~~go", "````go"} {
		line := "中文代码(\"value\")\n"
		closing := strings.TrimSuffix(fence, "go")
		parts := splitReplyMarkdown(fence + "\n" + strings.Repeat(line, 1600) + closing)
		if len(parts) < 2 {
			t.Fatal("代码未分卡")
		}
		count := 0
		for _, part := range parts {
			if !strings.HasPrefix(part, fence+"\n") || !strings.HasSuffix(strings.TrimSpace(part), closing) || len(jsonText(part)) > 10000 || !utf8.ValidString(part) {
				t.Fatalf("代码卡片不完整：%q", part)
			}
			count += strings.Count(part, line)
		}
		if count != 1600 {
			t.Fatalf("代码行数：%d", count)
		}
	}
	header := "| A | B |\n| --- | --- |\n"
	row := "| 一 | 二 |\n"
	parts := splitReplyMarkdown(header + strings.Repeat(row, 1800))
	count := 0
	for _, part := range parts {
		if !strings.HasPrefix(part, header) || len(jsonText(part)) > 10000 {
			t.Fatal("表头或预算不正确")
		}
		count += strings.Count(part, row)
	}
	if len(parts) < 2 || count != 1800 {
		t.Fatal(len(parts), count)
	}
	parts = splitReplyMarkdown("表格说明\n" + header + strings.Repeat(row, 1800) + "表格结束\n")
	count = 0
	for _, part := range parts {
		if strings.Contains(part, row) && !strings.HasPrefix(part, header) {
			t.Fatal("紧邻正文的表格丢失表头")
		}
		count += strings.Count(part, row)
	}
	if count != 1800 || !strings.Contains(strings.Join(parts, ""), "表格结束") {
		t.Fatal("表格或正文丢失")
	}
}

// TestNotificationSendsMarkdownCard 验证通知真实发送请求使用卡片且脚注为末尾小字号右对齐
func TestNotificationSendsMarkdownCard(t *testing.T) {
	messages := make(chan map[string]any, 1)
	s, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		messages <- payload
		fmt.Fprint(w, `{"code":0,"data":{"message_id":"reply-id"}}`)
	})
	body := "**结果**\n\n```go\nfmt.Println(1)\n```"
	if err := s.tasks.RecordNotification(tasks.Notification{ID: "markdown-card", Body: body, Source: tasks.Source{Type: "lark", AppID: "test-app", SenderID: "test-user", ChatID: "chat", MessageID: "original", ChatType: "p2p"}}); err != nil {
		t.Fatal(err)
	}
	notification, _ := s.tasks.GetNotification("markdown-card")
	s.OnNotification(notification)
	wait(t, func() bool { return len(messages) == 1 })
	payload := <-messages
	if payload["msg_type"] != "interactive" {
		t.Fatal(payload)
	}
	var card struct {
		Schema string `json:"schema"` // 卡片版本
		Body   struct {
			Elements []struct {
				Tag     string            `json:"tag"`     // 组件类型
				Content string            `json:"content"` // Markdown 正文
				Text    map[string]string `json:"text"`    // 脚注样式
			} `json:"elements"` // 顺序排列的组件
		} `json:"body"` // 卡片内容
	}
	if err := json.Unmarshal([]byte(payload["content"].(string)), &card); err != nil {
		t.Fatal(err)
	}
	if card.Schema != "2.0" || len(card.Body.Elements) != 2 || card.Body.Elements[0].Content != body {
		t.Fatal(card)
	}
	footer := card.Body.Elements[1].Text
	if footer["text_align"] != "right" || footer["text_size"] != "notation" || footer["content"] != "未知模型 · 未知强度 · ctx 未知" {
		t.Fatal(footer)
	}
}

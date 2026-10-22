package codex

import (
	"encoding/json"
	"io"
	"testing"
)

// TestTurnServiceTier 验证快速模式开关和托管请求分别发送本轮速度档位
func TestTurnServiceTier(t *testing.T) {
	s := accountFixture(t)
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
	s.client, s.status.Phase = c, "ready"
	s.opened["thread"] = true
	defer func() { s.client = nil }()
	requests := make(chan map[string]any, 4)
	go func() {
		decoder := json.NewDecoder(reader)
		for {
			var message packet
			if decoder.Decode(&message) != nil {
				return
			}
			var params map[string]any
			json.Unmarshal(message.Params, &params)
			requests <- params
			c.receive(packet{ID: message.ID, Result: json.RawMessage(`{"turn":{"id":"turn","status":"completed"}}`)})
		}
	}()

	for _, enabled := range []bool{false, true, false} {
		if _, err := s.data.SetFastMode(enabled); err != nil {
			t.Fatal(err)
		}
		if _, err := s.StartTurn("thread", "检查速度档位", "model", "high", "message", nil); err != nil {
			t.Fatal(err)
		}
		params := <-requests
		want := "default"
		if enabled {
			want = "fast"
		}
		if params["serviceTierForTurn"] != want || params["model"] != "model" || params["effort"] != "high" {
			t.Fatalf("快速模式改变了模型、强度或发送了错误档位：%+v", params)
		}
		if _, exists := params["serviceTier"]; exists {
			t.Fatal("本轮速度选择不应覆盖会话的默认速度")
		}
		// 模拟上一轮完成，允许同一会话继续验证开关切换
		s.mu.Lock()
		delete(s.active, "thread")
		delete(s.activeText, "thread")
		s.mu.Unlock()
	}
	if _, err := s.StartManagedTurn("thread", "排队请求", "model", "high", "read-only", "", "queued", nil, "fast"); err != nil {
		t.Fatal(err)
	}
	if params := <-requests; params["serviceTierForTurn"] != "fast" {
		t.Fatal("托管请求被当前已关闭的快速模式覆盖")
	}
}

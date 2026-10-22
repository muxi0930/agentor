package codex

import (
	"encoding/json"
	"io"
	"reflect"
	"sync/atomic"
	"testing"
)

// TestOccupiedThreadCanReadAndResume 验证占用会话可读、禁止发送并在释放后恢复
func TestOccupiedThreadCanReadAndResume(t *testing.T) {
	s := accountFixture(t)
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
	s.client, s.status.Phase = c, "ready"
	defer func() { s.client = nil }()
	var locked atomic.Bool
	locked.Store(true)
	requests := make(chan string, 20)
	go func() {
		decoder := json.NewDecoder(reader)
		for {
			var request packet
			if decoder.Decode(&request) != nil {
				return
			}
			requests <- request.Method
			response := packet{ID: request.ID}
			switch request.Method {
			case "thread/resume":
				if locked.Load() {
					response.Error = &rpcError{Code: -32600, Message: "thread occupied already has an active writer"}
				} else {
					response.Result = json.RawMessage(`{"thread":{"id":"occupied","cwd":"/tmp"}}`)
				}
			case "thread/read":
				response.Result = json.RawMessage(`{"thread":{"id":"occupied","cwd":"/tmp"}}`)
			case "thread/turns/list":
				response.Result = json.RawMessage(`{"data":[{"id":"history","status":"completed","items":[{"id":"message","type":"agentMessage","text":"可读历史"}]}],"nextCursor":"older"}`)
			case "turn/start":
				response.Result = json.RawMessage(`{"turn":{"id":"continued","status":"inProgress"}}`)
			case "thread/unsubscribe":
				response.Result = json.RawMessage(`{"status":"notSubscribed"}`)
			}
			c.receive(response)
		}
	}()
	conversation, err := s.OpenThread("occupied")
	if err != nil {
		t.Fatalf("占用时应能打开历史: %v", err)
	}
	if !conversation.ReadOnly || s.opened["occupied"] || len(conversation.Turns.Data) != 1 || conversation.Turns.NextCursor == nil {
		t.Fatalf("占用会话状态或历史错误: %+v", conversation)
	}
	if conversation.Turns.Data[0].Items[0].Text != "可读历史" || *conversation.Turns.NextCursor != "older" {
		t.Fatalf("历史内容或分页错误: %+v", conversation.Turns)
	}
	if _, err := s.StartTurn("occupied", "禁止发送", "", "", "client", nil); err == nil {
		t.Fatal("占用期间发送未被阻止")
	}
	locked.Store(false)
	conversation, err = s.OpenThread("occupied")
	if err != nil || !s.opened["occupied"] {
		t.Fatalf("释放后未恢复: %v", err)
	}
	if conversation.ReadOnly {
		t.Fatal("释放后仍为只读")
	}
	if _, err := s.StartTurn("occupied", "继续", "", "", "client", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenThread("occupied"); err != nil {
		t.Fatal(err)
	}
	close(requests)
	var methods []string
	for method := range requests {
		methods = append(methods, method)
	}
	want := []string{"thread/resume", "thread/read", "thread/turns/list", "thread/unsubscribe", "thread/resume", "thread/resume", "thread/turns/list", "turn/start", "thread/read", "thread/turns/list"}
	if !reflect.DeepEqual(methods, want) {
		t.Fatalf("请求链路错误: %v", methods)
	}
}

// TestOpenThreadPreservesErrors 验证恢复、读取与取消只读订阅的错误不会被吞掉
func TestOpenThreadPreservesErrors(t *testing.T) {
	for _, message := range []string{"thread missing", "thread occupied already has an active writer", "unsubscribe failed"} {
		t.Run(message, func(t *testing.T) {
			s := accountFixture(t)
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
			s.client, s.status.Phase = c, "ready"
			defer func() { s.client = nil }()
			go func() {
				decoder := json.NewDecoder(reader)
				for {
					var request packet
					if decoder.Decode(&request) != nil {
						return
					}
					if message == "unsubscribe failed" {
						response := packet{ID: request.ID, Result: json.RawMessage(`{}`)}
						switch request.Method {
						case "thread/resume":
							response.Error = &rpcError{Code: -32600, Message: "thread occupied already has an active writer"}
						case "thread/unsubscribe":
							response.Error = &rpcError{Code: -32600, Message: message}
						}
						c.receive(response)
						continue
					}
					failure := message
					if request.Method != "thread/resume" {
						failure = "history unavailable"
					}
					c.receive(packet{ID: request.ID, Error: &rpcError{Code: -32600, Message: failure}})
				}
			}()
			_, err := s.OpenThread("occupied")
			want := "thread/resume: thread missing"
			if message != "thread missing" {
				want = "thread/read: history unavailable"
			}
			if message == "unsubscribe failed" {
				want = "thread/unsubscribe: unsubscribe failed"
			}
			if err == nil || err.Error() != want || s.opened["occupied"] {
				t.Fatalf("读取错误或恢复状态不符: %v", err)
			}
		})
	}
}

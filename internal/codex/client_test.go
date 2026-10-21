package codex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// TestConcurrentRequestsMatchOutOfOrderResponses 验证并发请求按标识接收乱序响应
func TestConcurrentRequestsMatchOutOfOrderResponses(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	serverRequest := make(chan packet, 1)
	c := &client{stdin: writer, done: make(chan struct{}), pending: make(map[string]chan packet), onMessage: func(message packet) {
		serverRequest <- message
	}}
	go func() {
		decoder := json.NewDecoder(reader)
		var first, second packet
		if decoder.Decode(&first) != nil || decoder.Decode(&second) != nil {
			return
		}

		c.receive(packet{ID: first.ID, Method: "item/commandExecution/requestApproval", Params: json.RawMessage(`{"threadId":"other-thread"}`)})
		c.receive(packet{ID: second.ID, Result: json.RawMessage(`"` + second.Method + `"`)})
		c.receive(packet{ID: first.ID, Result: json.RawMessage(`"` + first.Method + `"`)})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var group sync.WaitGroup
	for _, method := range []string{"thread/read", "model/list"} {
		group.Add(1)
		go func() {
			defer group.Done()

			var result string
			if err := c.request(ctx, method, map[string]any{}, &result); err != nil {
				t.Error(err)
				return
			}
			if result != method {
				t.Errorf("%s received %q", method, result)
			}
		}()
	}

	group.Wait()
	select {
	case request := <-serverRequest:
		if request.Method != "item/commandExecution/requestApproval" {
			t.Fatal(request.Method)
		}
	default:
		t.Fatal("server request was consumed as a response")
	}
}

// TestDisconnectEndsPendingRequest 验证连接断开会结束等待中的请求
func TestDisconnectEndsPendingRequest(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	c := &client{stdin: writer, done: make(chan struct{}), pending: make(map[string]chan packet)}
	go func() {
		var request packet
		json.NewDecoder(reader).Decode(&request)
		c.finish(errors.New("connection lost"))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := c.request(ctx, "turn/start", map[string]any{}, nil); err == nil || err.Error() != "connection lost" {
		t.Fatalf("expected disconnect, got %v", err)
	}
}

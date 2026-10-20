package codex

import (
	"encoding/json"
	"io"
	"testing"
)

// TestEnsureOpenSkipsHistoryForNewThreads 验证空新线程不读历史且旧线程只恢复一次
func TestEnsureOpenSkipsHistoryForNewThreads(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()
	s.opened["new"] = true
	if err := s.EnsureOpen("new"); err != nil {
		t.Fatal(err)
	}

	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	c := &client{stdin: writer, done: make(chan struct{}), pending: map[string]chan packet{}}
	s.client, s.status.Phase = c, "ready"
	request := make(chan packet, 1)
	go func() {
		var message packet
		if json.NewDecoder(reader).Decode(&message) == nil {
			request <- message
			c.receive(packet{ID: message.ID, Result: json.RawMessage(`{"thread":{"id":"old"}}`)})
		}
	}()
	if err := s.EnsureOpen("old"); err != nil {
		t.Fatal(err)
	}
	message := <-request
	if message.Method != "thread/resume" || !s.opened["old"] {
		t.Fatal(message)
	}
	if err := s.EnsureOpen("old"); err != nil {
		t.Fatal(err)
	}
}

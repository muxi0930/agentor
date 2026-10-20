package codex

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

type bufferWriter struct{ bytes.Buffer }

func (*bufferWriter) Close() error { return nil }

func TestApprovalCanBeAnsweredOnlyOnce(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()
	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.approvals["7"] = Approval{ID: "7", Generation: 1, Status: "pending", Method: "item/commandExecution/requestApproval", AvailableDecisions: []string{"accept", "decline"}}
	var successful atomic.Int32
	var group sync.WaitGroup
	for range 2 {
		group.Go(func() {
			if s.RespondApproval("7", "accept") == nil {
				successful.Add(1)
			}
		})
	}
	group.Wait()
	if successful.Load() != 1 {
		t.Fatalf("got %d successful responses", successful.Load())
	}
	decoder := json.NewDecoder(&writer.Buffer)
	var response packet
	if err := decoder.Decode(&response); err != nil {
		t.Fatal(err)
	}
	if string(response.ID) != "7" {
		t.Fatalf("wrong request: %s", response.ID)
	}
	if err := decoder.Decode(&response); err != io.EOF {
		t.Fatalf("unexpected second response: %v", err)
	}
}

func TestCompletingOneTurnExpiresOnlyItsApproval(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()
	s.status.Generation = 3
	s.approvals["1"] = Approval{ID: "1", Generation: 3, Status: "pending", ThreadID: "thread-a", TurnID: "turn-a"}
	s.approvals["2"] = Approval{ID: "2", Generation: 3, Status: "pending", ThreadID: "thread-b", TurnID: "turn-b"}
	s.trackNotification(3, packet{Method: "turn/completed", Params: json.RawMessage(`{"threadId":"thread-a","turn":{"id":"turn-a","status":"completed","items":[]}}`)})
	if s.approvals["1"].Status != "expired" || s.approvals["2"].Status != "pending" {
		t.Fatal(s.approvals)
	}
	if err := s.RespondApproval("1", "accept"); err == nil {
		t.Fatal("expired approval accepted")
	}
}

func TestReasoningHistoryDoesNotDecodeAsUserInput(t *testing.T) {
	var item Item
	if err := json.Unmarshal([]byte(`{"id":"r1","type":"reasoning","content":["detail"],"summary":["summary"]}`), &item); err != nil {
		t.Fatal(err)
	}
	if len(item.ReasoningContent) != 1 || item.ReasoningContent[0] != "detail" {
		t.Fatal(item)
	}
}

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

func TestPermissionResponseCannotBroadenRequest(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()
	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.handleServerRequest(1, packet{ID: json.RawMessage(`"permission"`), Method: "item/permissions/requestApproval", Params: json.RawMessage(`{"threadId":"t","turnId":"r","permissions":{"network":{"enabled":true}}}`)})
	if err := s.RespondPermissions(`"permission"`, true, "session"); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Result struct {
			Permissions struct {
				Network struct {
					Enabled bool `json:"enabled"`
				} `json:"network"`
			} `json:"permissions"`
			Scope string `json:"scope"`
		} `json:"result"`
	}
	if err := json.Unmarshal(writer.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Result.Permissions.Network.Enabled || result.Result.Scope != "session" {
		t.Fatal(writer.String())
	}
}

func TestMcpElicitationPreservesAnswerTypesAndRejectsInvalidAnswer(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()
	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.handleServerRequest(1, packet{ID: json.RawMessage(`8`), Method: "mcpServer/elicitation/request", Params: json.RawMessage(`{"threadId":"t","serverName":"test","mode":"form","message":"Answer","requestedSchema":{"type":"object","required":["count","confirm"],"properties":{"count":{"type":"integer","minimum":1},"confirm":{"type":"boolean"}}}}`)})
	if err := s.AnswerElicitation("8", "accept", `{"count":0,"confirm":true}`); err == nil {
		t.Fatal("invalid number accepted")
	}
	if s.GetApprovals()[0].Status != "pending" {
		t.Fatal("invalid submission consumed request")
	}
	if err := s.AnswerElicitation("8", "accept", `{"count":2,"confirm":false}`); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			Action  string         `json:"action"`
			Content map[string]any `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(writer.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result.Action != "accept" || response.Result.Content["count"] != float64(2) || response.Result.Content["confirm"] != false {
		t.Fatal(writer.String())
	}
}

func TestCommandRuleDecisionUsesOriginalObject(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()
	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.handleServerRequest(1, packet{ID: json.RawMessage(`9`), Method: "item/commandExecution/requestApproval", Params: json.RawMessage(`{"threadId":"t","turnId":"r","availableDecisions":[{"acceptWithExecpolicyAmendment":{"execpolicy_amendment":["git","status"]}},"cancel"]}`)})
	approval := s.GetApprovals()[0]
	if len(approval.DecisionOptions) != 2 {
		t.Fatal(approval)
	}
	if err := s.RespondApproval("9", approval.DecisionOptions[0].Value); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			Decision map[string]any `json:"decision"`
		} `json:"result"`
	}
	if err := json.Unmarshal(writer.Bytes(), &response); err != nil {
		t.Fatal("decision was serialized as a string", err)
	}
	if response.Result.Decision["acceptWithExecpolicyAmendment"] == nil {
		t.Fatal(writer.String())
	}
}

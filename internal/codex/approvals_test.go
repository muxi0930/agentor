package codex

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

type bufferWriter struct {
	bytes.Buffer // 协议响应的内存缓冲
}

// Close 关闭测试写入端而不清除缓冲内容
func (*bufferWriter) Close() error {
	return nil
}

// TestApprovalCanBeAnsweredOnlyOnce 验证并发回应仅能处理一次审批
func TestApprovalCanBeAnsweredOnlyOnce(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()

	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.approvals["7"] = Approval{
		ID:                 "7",
		Generation:         1,
		Status:             "pending",
		Method:             "item/commandExecution/requestApproval",
		AvailableDecisions: []string{"accept", "decline"},
	}
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

// TestCompletingOneTurnExpiresOnlyItsApproval 验证轮次完成只使对应审批失效
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

// TestReasoningHistoryDoesNotDecodeAsUserInput 验证思考历史使用独立内容字段解码
func TestReasoningHistoryDoesNotDecodeAsUserInput(t *testing.T) {
	var item Item
	if err := json.Unmarshal([]byte(`{"id":"r1","type":"reasoning","content":["detail"],"summary":["summary"]}`), &item); err != nil {
		t.Fatal(err)
	}
	if len(item.ReasoningContent) != 1 || item.ReasoningContent[0] != "detail" {
		t.Fatal(item)
	}
}

// TestPermissionResponseCannotBroadenRequest 验证权限响应保持原请求范围
func TestPermissionResponseCannotBroadenRequest(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()

	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.handleServerRequest(1, packet{
		ID:     json.RawMessage(`"permission"`),
		Method: "item/permissions/requestApproval",
		Params: json.RawMessage(`{"threadId":"t","turnId":"r","permissions":{"network":{"enabled":true}}}`),
	})
	if err := s.RespondPermissions(`"permission"`, true, "session"); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Result struct {
			Permissions struct {
				Network struct {
					Enabled bool `json:"enabled"` // 是否允许网络访问
				} `json:"network"` // 网络权限范围
			} `json:"permissions"` // 原始权限范围
			Scope string `json:"scope"` // 授权生效范围
		} `json:"result"` // 原始响应结果
	}
	if err := json.Unmarshal(writer.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Result.Permissions.Network.Enabled || result.Result.Scope != "session" {
		t.Fatal(writer.String())
	}
}

// TestMcpElicitationPreservesAnswerTypesAndRejectsInvalidAnswer 验证 MCP 问答保留答案类型并拒绝无效答案
func TestMcpElicitationPreservesAnswerTypesAndRejectsInvalidAnswer(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()

	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.handleServerRequest(1, packet{
		ID:     json.RawMessage(`8`),
		Method: "mcpServer/elicitation/request",
		Params: json.RawMessage(`{"threadId":"t","serverName":"test","mode":"form","message":"Answer","requestedSchema":{"type":"object","required":["count","confirm"],"properties":{"count":{"type":"integer","minimum":1},"confirm":{"type":"boolean"}}}}`),
	})
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
			Action  string         `json:"action"`  // MCP 问答处理动作
			Content map[string]any `json:"content"` // MCP 问答提交内容
		} `json:"result"` // 原始响应结果
	}
	if err := json.Unmarshal(writer.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result.Action != "accept" || response.Result.Content["count"] != float64(2) || response.Result.Content["confirm"] != false {
		t.Fatal(writer.String())
	}
}

// TestCommandRuleDecisionUsesOriginalObject 验证命令规则审批保留原始决策对象
func TestCommandRuleDecisionUsesOriginalObject(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()

	s.status = Status{Phase: "ready", Generation: 1}
	writer := &bufferWriter{}
	s.client = &client{stdin: writer}
	s.handleServerRequest(1, packet{
		ID:     json.RawMessage(`9`),
		Method: "item/commandExecution/requestApproval",
		Params: json.RawMessage(`{"threadId":"t","turnId":"r","availableDecisions":[{"acceptWithExecpolicyAmendment":{"execpolicy_amendment":["git","status"]}},"cancel"]}`),
	})
	approval := s.GetApprovals()[0]
	if len(approval.DecisionOptions) != 2 {
		t.Fatal(approval)
	}
	if err := s.RespondApproval("9", approval.DecisionOptions[0].Value); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			Decision map[string]any `json:"decision"` // 审批决策对象
		} `json:"result"` // 原始响应结果
	}
	if err := json.Unmarshal(writer.Bytes(), &response); err != nil {
		t.Fatal("decision was serialized as a string", err)
	}
	if response.Result.Decision["acceptWithExecpolicyAmendment"] == nil {
		t.Fatal(writer.String())
	}
}

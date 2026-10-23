package codex

import (
	"encoding/json"
	"sort"
	"time"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// GetApprovals 返回审批和问答记录
func (s *Service) GetApprovals() []codexmodel.Approval {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]codexmodel.Approval, 0, len(s.approvals))
	for _, approval := range s.approvals {
		result = append(result, approval)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// RespondApproval 校验上游选项后回复命令或文件审批
func (s *Service) RespondApproval(id, decision string) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	s.mu.Unlock()
	if !found || (approval.Method != constant.CodexMethodItemCommandExecutionRequestApproval && approval.Method != constant.CodexMethodItemFileChangeRequestApproval) {
		return constant.ErrCodexApprovalTypeInvalid
	}

	valid := false
	for _, available := range approval.AvailableDecisions {
		if decision == available {
			valid = true
		}
	}

	if !valid {
		return constant.ErrCodexApprovalOptionUnsupported
	}

	var value any = decision
	if len(decision) > 0 && decision[0] == '{' {
		if err := json.Unmarshal([]byte(decision), &value); err != nil {
			return err
		}
	}
	return s.resolveApproval(id, map[string]any{"decision": value})
}

// RespondPermissions 仅按原请求范围回复权限审批
func (s *Service) RespondPermissions(id string, allow bool, scope string) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	s.mu.Unlock()
	if !found || approval.Method != constant.CodexMethodItemPermissionsRequestApproval {
		return constant.ErrCodexPermissionApprovalRequired
	}
	if scope != "turn" && scope != "session" {
		return constant.ErrCodexPermissionScopeInvalid
	}

	permissions := json.RawMessage(`{}`)
	if allow {
		permissions = json.RawMessage(approval.PermissionsJSON)
	}
	return s.resolveApproval(id, map[string]any{"permissions": permissions, "scope": scope})
}

// AnswerQuestions 将结构化答案回复原问答请求
func (s *Service) AnswerQuestions(id string, answers map[string]codexmodel.QuestionAnswer) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	s.mu.Unlock()
	if !found || approval.Method != constant.CodexMethodItemToolRequestUserInput {
		return constant.ErrCodexQuestionApprovalRequired
	}

	for _, question := range approval.Questions {
		if len(answers[question.ID].Answers) == 0 {
			return constant.ErrCodexQuestionsIncomplete
		}
	}

	return s.resolveApproval(id, map[string]any{"answers": answers})
}

// handleServerRequest 登记真实上游审批或调用宿主工具
func (s *Service) handleServerRequest(generation uint64, message packet) {
	s.mu.Lock()
	if s.status.Generation != generation {
		s.mu.Unlock()
		return
	}

	c := s.client
	s.mu.Unlock()
	if c == nil {
		return
	}
	if message.Method == constant.CodexMethodItemToolCall {
		s.handleToolRequest(generation, message, c)
		return
	}
	if message.Method == "currentTime/read" {
		c.write(map[string]any{"id": message.ID, "result": map[string]int64{"currentTimeAt": time.Now().Unix()}})
		return
	}
	if message.Method != constant.CodexMethodItemCommandExecutionRequestApproval &&
		message.Method != constant.CodexMethodItemFileChangeRequestApproval &&
		message.Method != constant.CodexMethodItemToolRequestUserInput &&
		message.Method != constant.CodexMethodItemPermissionsRequestApproval &&
		message.Method != constant.CodexMethodMcpServerElicitationRequest {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32601, Message: constant.CodexUnsupportedServerRequestPrefix + message.Method}})
		return
	}

	var params struct {
		codexmodel.Approval                      // 上游审批基本信息
		AvailableDecisions     []json.RawMessage `json:"availableDecisions"`     // 上游允许的决策
		Permissions            json.RawMessage   `json:"permissions"`            // 原始权限范围
		RequestedSchema        json.RawMessage   `json:"requestedSchema"`        // MCP 请求的表单结构
		AdditionalPermissions  json.RawMessage   `json:"additionalPermissions"`  // 额外申请的权限
		NetworkApprovalContext json.RawMessage   `json:"networkApprovalContext"` // 网络审批上下文
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
		return
	}

	approval := params.Approval
	approval.ID, approval.Method, approval.Status, approval.Generation = string(message.ID), message.Method, constant.CodexStatusPending, generation
	for _, raw := range params.AvailableDecisions {
		var decision string
		if json.Unmarshal(raw, &decision) != nil {
			decision = string(raw)
		}
		approval.AvailableDecisions = append(approval.AvailableDecisions, decision)
	}

	if params.AvailableDecisions == nil {
		approval.AvailableDecisions = []string{constant.CodexApprovalAccept, constant.CodexApprovalAcceptForSession, constant.CodexApprovalDecline, constant.CodexApprovalCancel}
	}
	for _, value := range approval.AvailableDecisions {
		option := codexmodel.DecisionOption{Value: value, Label: map[string]string{"accept": constant.CodexAllowOnceLabel, "acceptForSession": constant.CodexAllowSessionLabel, "decline": constant.CodexDeclineLabel, "cancel": constant.CodexCancelLabel}[value]}
		if option.Label == "" {
			var object map[string]json.RawMessage
			json.Unmarshal([]byte(value), &object)
			if object["acceptWithExecpolicyAmendment"] != nil {
				option.Label = constant.CodexSaveCommandRuleLabel
			}
			if object["applyNetworkPolicyAmendment"] != nil {
				option.Label = constant.CodexSaveNetworkRuleLabel
			}
			option.Details = value
		}
		if option.Label != "" {
			approval.DecisionOptions = append(approval.DecisionOptions, option)
		}
	}

	if len(params.Permissions) > 0 {
		approval.PermissionsJSON = string(params.Permissions)
	}
	if len(params.AdditionalPermissions) > 0 && string(params.AdditionalPermissions) != "null" {
		approval.Details = constant.CodexAdditionalPermissionsHeading + string(params.AdditionalPermissions)
	}
	if len(params.NetworkApprovalContext) > 0 && string(params.NetworkApprovalContext) != "null" {
		approval.Details += constant.CodexNetworkRequestHeading + string(params.NetworkApprovalContext)
	}
	if message.Method == constant.CodexMethodMcpServerElicitationRequest {
		approval.SchemaJSON = string(params.RequestedSchema)
		approval.Fields = elicitationFields(params.RequestedSchema)
	}
	s.mu.Lock()
	if s.status.Generation == generation {
		approval.Changes = s.pendingChanges[approval.ThreadID][approval.ItemID]
		s.approvals[approval.ID] = approval
	}
	s.mu.Unlock()
	s.emitApprovals(generation)
}

// emitApprovals 发布审批变化并通知业务观察者
func (s *Service) emitApprovals(generation uint64) {
	event := codexmodel.Event{Generation: generation, Method: "host/approvalsUpdated", Params: "{}"}
	s.mu.Lock()
	handler := s.eventHandler
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit(constant.CodexEventMessage, event)
	}
	if handler != nil {
		handler(event)
	}
}

// resolveApproval 只回应当前仍有效的原始服务器请求
func (s *Service) resolveApproval(id string, result any) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	if !found || approval.Status != constant.CodexStatusPending || approval.Generation != s.status.Generation || s.status.Phase != constant.CodexPhaseReady {
		s.mu.Unlock()
		return constant.ErrCodexApprovalExpired
	}

	c := s.client
	approval.Status = constant.CodexApprovalResolved
	s.approvals[id] = approval
	s.mu.Unlock()
	err := c.write(map[string]any{"id": json.RawMessage(id), "result": result})
	if err != nil {
		s.mu.Lock()
		approval.Status = constant.CodexApprovalExpired
		if s.approvals[id].Generation == approval.Generation {
			s.approvals[id] = approval
		}
		s.mu.Unlock()
	}
	s.emitApprovals(approval.Generation)
	return err
}

// expireApprovalsLocked 使已结束轮次或连接的审批失效
func (s *Service) expireApprovalsLocked(threadID, turnID string) {
	for id, approval := range s.approvals {
		if approval.Status == constant.CodexStatusPending && (threadID == "" || (approval.ThreadID == threadID && approval.TurnID == turnID)) {
			approval.Status = constant.CodexApprovalExpired
			s.approvals[id] = approval
		}
	}
}

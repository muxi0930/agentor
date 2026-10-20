package codex

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type Question struct {
	ID       string           `json:"id"`
	Header   string           `json:"header"`
	Question string           `json:"question"`
	IsOther  bool             `json:"isOther"`
	IsSecret bool             `json:"isSecret"`
	Options  []QuestionOption `json:"options"`
}

type Approval struct {
	ID                 string             `json:"id"`
	Method             string             `json:"method"`
	ThreadID           string             `json:"threadId"`
	TurnID             string             `json:"turnId"`
	ItemID             string             `json:"itemId"`
	Command            string             `json:"command"`
	Reason             string             `json:"reason"`
	Cwd                string             `json:"cwd"`
	GrantRoot          string             `json:"grantRoot"`
	AvailableDecisions []string           `json:"availableDecisions"`
	Questions          []Question         `json:"questions"`
	Status             string             `json:"status"`
	Generation         uint64             `json:"generation"`
	DecisionOptions    []DecisionOption   `json:"decisionOptions"`
	PermissionsJSON    string             `json:"permissionsJSON"`
	Details            string             `json:"details"`
	ServerName         string             `json:"serverName"`
	Mode               string             `json:"mode"`
	Message            string             `json:"message"`
	URL                string             `json:"url"`
	SchemaJSON         string             `json:"schemaJSON"`
	Fields             []ElicitationField `json:"fields"`
	Changes            []FileChange       `json:"changes"`
}

type DecisionOption struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Details string `json:"details"`
}

type QuestionAnswer struct {
	Answers []string `json:"answers"`
}

// GetApprovals 返回审批和问答记录
func (s *Service) GetApprovals() []Approval {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Approval, 0, len(s.approvals))
	for _, approval := range s.approvals {
		result = append(result, approval)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// RespondApproval 校验上游选项后回复命令或文件审批
func (s *Service) RespondApproval(id, decision string) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	s.mu.Unlock()
	if !found || (approval.Method != "item/commandExecution/requestApproval" && approval.Method != "item/fileChange/requestApproval") {
		return errors.New("该请求不是命令或文件审批")
	}
	valid := false
	for _, available := range approval.AvailableDecisions {
		if decision == available {
			valid = true
		}
	}
	if !valid {
		return errors.New("该审批不支持此选项")
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
	if !found || approval.Method != "item/permissions/requestApproval" {
		return errors.New("该请求不是权限审批")
	}
	if scope != "turn" && scope != "session" {
		return errors.New("请选择本轮或本会话")
	}
	permissions := json.RawMessage(`{}`)
	if allow {
		permissions = json.RawMessage(approval.PermissionsJSON)
	}
	return s.resolveApproval(id, map[string]any{"permissions": permissions, "scope": scope})
}

// AnswerQuestions 将结构化答案回复原问答请求
func (s *Service) AnswerQuestions(id string, answers map[string]QuestionAnswer) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	s.mu.Unlock()
	if !found || approval.Method != "item/tool/requestUserInput" {
		return errors.New("该请求不是用户问答")
	}
	for _, question := range approval.Questions {
		if len(answers[question.ID].Answers) == 0 {
			return errors.New("请回答所有问题")
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
	if message.Method == "item/tool/call" {
		s.handleToolRequest(generation, message, c)
		return
	}
	if message.Method == "currentTime/read" {
		c.write(map[string]any{"id": message.ID, "result": map[string]int64{"currentTimeAt": time.Now().Unix()}})
		return
	}
	if message.Method != "item/commandExecution/requestApproval" && message.Method != "item/fileChange/requestApproval" && message.Method != "item/tool/requestUserInput" && message.Method != "item/permissions/requestApproval" && message.Method != "mcpServer/elicitation/request" {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32601, Message: "agentor 尚不支持该服务器请求: " + message.Method}})
		return
	}
	var params struct {
		Approval
		AvailableDecisions     []json.RawMessage `json:"availableDecisions"`
		Permissions            json.RawMessage   `json:"permissions"`
		RequestedSchema        json.RawMessage   `json:"requestedSchema"`
		AdditionalPermissions  json.RawMessage   `json:"additionalPermissions"`
		NetworkApprovalContext json.RawMessage   `json:"networkApprovalContext"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
		return
	}
	approval := params.Approval
	approval.ID, approval.Method, approval.Status, approval.Generation = string(message.ID), message.Method, "pending", generation
	for _, raw := range params.AvailableDecisions {
		var decision string
		if json.Unmarshal(raw, &decision) != nil {
			decision = string(raw)
		}
		approval.AvailableDecisions = append(approval.AvailableDecisions, decision)
	}
	if params.AvailableDecisions == nil {
		approval.AvailableDecisions = []string{"accept", "acceptForSession", "decline", "cancel"}
	}
	for _, value := range approval.AvailableDecisions {
		option := DecisionOption{Value: value, Label: map[string]string{"accept": "允许本次", "acceptForSession": "本会话允许", "decline": "拒绝", "cancel": "拒绝并停止"}[value]}
		if option.Label == "" {
			var object map[string]json.RawMessage
			json.Unmarshal([]byte(value), &object)
			if object["acceptWithExecpolicyAmendment"] != nil {
				option.Label = "允许并保存命令规则"
			}
			if object["applyNetworkPolicyAmendment"] != nil {
				option.Label = "保存网络规则"
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
		approval.Details = "额外权限：\n" + string(params.AdditionalPermissions)
	}
	if len(params.NetworkApprovalContext) > 0 && string(params.NetworkApprovalContext) != "null" {
		approval.Details += "\n网络请求：\n" + string(params.NetworkApprovalContext)
	}
	if message.Method == "mcpServer/elicitation/request" {
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
	event := Event{Generation: generation, Method: "host/approvalsUpdated", Params: "{}"}
	s.mu.Lock()
	handler := s.eventHandler
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit("codex:event", event)
	}
	if handler != nil {
		handler(event)
	}
}

// resolveApproval 只回应当前仍有效的原始服务器请求
func (s *Service) resolveApproval(id string, result any) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	if !found || approval.Status != "pending" || approval.Generation != s.status.Generation || s.status.Phase != "ready" {
		s.mu.Unlock()
		return errors.New("该请求已处理或失效")
	}
	c := s.client
	approval.Status = "resolved"
	s.approvals[id] = approval
	s.mu.Unlock()
	err := c.write(map[string]any{"id": json.RawMessage(id), "result": result})
	if err != nil {
		s.mu.Lock()
		approval.Status = "expired"
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
		if approval.Status == "pending" && (threadID == "" || (approval.ThreadID == threadID && approval.TurnID == turnID)) {
			approval.Status = "expired"
			s.approvals[id] = approval
		}
	}
}

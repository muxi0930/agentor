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
	ID                 string     `json:"id"`
	Method             string     `json:"method"`
	ThreadID           string     `json:"threadId"`
	TurnID             string     `json:"turnId"`
	ItemID             string     `json:"itemId"`
	Command            string     `json:"command"`
	Reason             string     `json:"reason"`
	Cwd                string     `json:"cwd"`
	GrantRoot          string     `json:"grantRoot"`
	AvailableDecisions []string   `json:"availableDecisions"`
	Questions          []Question `json:"questions"`
	Status             string     `json:"status"`
	Generation         uint64     `json:"generation"`
}

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
	if message.Method == "currentTime/read" {
		c.write(map[string]any{"id": message.ID, "result": map[string]int64{"currentTimeAt": time.Now().Unix()}})
		return
	}
	if message.Method != "item/commandExecution/requestApproval" && message.Method != "item/fileChange/requestApproval" && message.Method != "item/tool/requestUserInput" {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32601, Message: "agentor 尚不支持该服务器请求: " + message.Method}})
		return
	}
	var params struct {
		Approval
		AvailableDecisions []json.RawMessage `json:"availableDecisions"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
		return
	}
	approval := params.Approval
	approval.ID, approval.Method, approval.Status, approval.Generation = string(message.ID), message.Method, "pending", generation
	for _, raw := range params.AvailableDecisions {
		var decision string
		if json.Unmarshal(raw, &decision) == nil {
			approval.AvailableDecisions = append(approval.AvailableDecisions, decision)
		}
	}
	if len(params.AvailableDecisions) == 0 {
		approval.AvailableDecisions = []string{"accept", "acceptForSession", "decline", "cancel"}
	}
	s.mu.Lock()
	if s.status.Generation == generation {
		s.approvals[approval.ID] = approval
	}
	s.mu.Unlock()
	s.emitApprovals(generation)
}

func (s *Service) emitApprovals(generation uint64) {
	if app := application.Get(); app != nil {
		app.Event.Emit("codex:event", Event{Generation: generation, Method: "host/approvalsUpdated", Params: "{}"})
	}
}

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
	return s.resolveApproval(id, map[string]string{"decision": decision})
}

type QuestionAnswer struct {
	Answers []string `json:"answers"`
}

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

func (s *Service) expireApprovalsLocked(threadID, turnID string) {
	for id, approval := range s.approvals {
		if approval.Status == "pending" && (threadID == "" || (approval.ThreadID == threadID && approval.TurnID == turnID)) {
			approval.Status = "expired"
			s.approvals[id] = approval
		}
	}
}

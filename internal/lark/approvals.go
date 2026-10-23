package lark

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agentor/internal/appdata"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	larkmodel "agentor/internal/lark/model"
	tasksmodel "agentor/internal/tasks/model"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// StartPairing 为指定联系人生成一次性配对码
func (s *Service) StartPairing(senderID string) (larkmodel.Pairing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !hasIdentity(s.config.AllowedUsers, senderID) {
		return larkmodel.Pairing{}, constant.ErrLarkPairingRecipientUnauthorized
	}
	s.pairing = larkmodel.Pairing{Code: rand.Text()[:constant.LarkPairingCodeLength], SenderID: senderID, ExpiresAt: time.Now().UTC().Add(constant.LarkPairingDuration).Format(time.RFC3339Nano)}
	return s.pairing, nil
}

// ClearBinding 解除桌面任务的远程接收者绑定
func (s *Service) ClearBinding() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.config
	next.Binding = tasksmodel.Source{}
	if err := appdata.WriteJSON(s.configPath(), next); err != nil {
		return err
	}
	s.config = next
	s.changed()
	return nil
}

// OnCodexEvent 处理上游事件并同步所属任务状态
//
//wails:ignore
func (s *Service) OnCodexEvent(event codexmodel.Event) {
	if event.Method == "host/approvalsUpdated" || event.Method == constant.CodexMethodTurnCompleted || event.Method == "host/status" || event.Method == "serverRequest/resolved" {
		go s.syncApprovals()
	}
}

// configPath 返回当前应用的飞书私有配置路径
func (s *Service) configPath() string {
	return filepath.Join(s.root, constant.LarkConfigFilename)
}

// pair 核验配对码并绑定原单聊
func (s *Service) pair(source tasksmodel.Source, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, err := time.Parse(time.RFC3339Nano, s.pairing.ExpiresAt)
	if err != nil || time.Now().After(expires) || code != s.pairing.Code || source.SenderID != s.pairing.SenderID || source.ChatType != constant.LarkChatTypePrivate || source.AppID != s.config.AppID {
		return constant.ErrLarkPairingInvalid
	}
	next := s.config
	next.Binding = source
	next.Binding.MessageID, next.Binding.ReplyToMessageID = "", ""
	if err = appdata.WriteJSON(s.configPath(), next); err != nil {
		return err
	}
	s.config = next
	s.pairing = larkmodel.Pairing{}
	s.changed()
	return nil
}

// textControl 直接处理配对与审批响应而不进入任务队列
func (s *Service) textControl(message larkmodel.Message) (bool, error) {
	parts := strings.SplitN(strings.TrimSpace(message.Text), " ", 3)
	if len(parts) == 0 {
		return false, nil
	}
	command := parts[0]
	if command == "/pair" {
		if len(parts) != 2 {
			return true, constant.ErrLarkPairCommandInvalid
		}
		return true, s.pair(message.Source, strings.TrimSpace(parts[1]))
	}
	if command != "/approve" && command != "/answer" && command != "/elicitation" {
		return false, nil
	}
	if len(parts) != 3 {
		return true, constant.ErrLarkApprovalCommandInvalid
	}
	s.mu.Lock()
	var route larkmodel.ApprovalRoute
	for _, item := range s.state.Approvals {
		if item.ApprovalID == parts[1] && item.Status == constant.LarkStatusPending && item.Source.AppID == message.Source.AppID && item.Source.ChatID == message.Source.ChatID && item.Source.SenderID == message.Source.SenderID {
			route = item
			break
		}
	}
	s.mu.Unlock()
	if route.Key == "" {
		return true, constant.ErrLarkSenderApprovalNotFound
	}
	return true, s.respondRoute(route.Key, message.Source, parts[2], command)
}

// approvalSource 查找原任务来源或桌面配对接收者
func (s *Service) approvalSource(threadID string) tasksmodel.Source {
	state := s.tasks.GetState()
	for _, execution := range state.Executions {
		if execution.ThreadID == threadID && execution.Status == constant.TasksStatusRunning {
			if execution.Source.Type == "lark" {
				return execution.Source
			}
			break
		}
	}
	for _, request := range state.Requests {
		if request.ThreadID == threadID && request.Status == constant.TasksStatusRunning {
			if request.Source.Type == "lark" {
				return request.Source
			}
			break
		}
	}
	if s.pipelines != nil {
		if request, err := s.pipelines.ActiveSession(threadID); err == nil && request.Input.Source.Type == "lark" {
			return request.Input.Source
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Binding
}

// syncApprovals 登记真实审批并更新远程卡片状态
func (s *Service) syncApprovals() {
	if s.ctx.Err() != nil {
		return
	}
	approvals := s.runtime.GetApprovals()
	for _, approval := range approvals {
		key := fmt.Sprintf("%s:%d:%s", s.approvalSession, approval.Generation, approval.ID)
		s.mu.Lock()
		index := -1
		for i, route := range s.state.Approvals {
			if route.Key == key {
				index = i
				break
			}
		}
		if index >= 0 {
			route := s.state.Approvals[index]
			if route.Status != approval.Status {
				s.state.Approvals[index].Status = approval.Status
				s.persistLocked()
				s.mu.Unlock()
				s.changed()
				if route.MessageID != "" {
					go s.updateApprovalCard(route, approval.Status)
				}
			} else {
				s.mu.Unlock()
			}
			continue
		}
		s.mu.Unlock()
		if approval.Status != constant.LarkStatusPending {
			continue
		}
		source := s.approvalSource(approval.ThreadID)
		if source.Type != "lark" || source.ChatID == "" {
			continue
		}
		notificationID := rand.Text()
		route := larkmodel.ApprovalRoute{Key: key, ApprovalID: approval.ID, Generation: approval.Generation, Source: source, NotificationID: notificationID, Status: constant.LarkStatusPending}
		s.mu.Lock()
		exists := false
		for _, existing := range s.state.Approvals {
			if existing.Key == key {
				exists = true
				break
			}
		}
		if exists || s.ctx.Err() != nil {
			s.mu.Unlock()
			continue
		}
		s.state.Approvals = append(s.state.Approvals, route)
		err := s.persistLocked()
		s.mu.Unlock()
		if err != nil {
			continue
		}
		body := approvalBody(approval)
		err = s.tasks.RecordNotification(tasksmodel.Notification{ID: notificationID, Title: constant.LarkApprovalTitle, Body: body, Source: source, Parts: []tasksmodel.NotificationPart{{ID: rand.Text(), Kind: "approval", Channel: "lark", Target: source.ChatID, Status: constant.LarkStatusPending, PayloadJSON: jsonText(approvalCard(approval, key)), Content: body}}})
		if err != nil {
			s.mu.Lock()
			for i := range s.state.Approvals {
				if s.state.Approvals[i].Key == key {
					s.state.Approvals[i].Error = err.Error()
				}
			}
			s.persistLocked()
			s.mu.Unlock()
		}
		s.changed()
	}
}

// approvalBody 生成审批内容和文本回答说明
func approvalBody(approval codexmodel.Approval) string {
	parts := []string{constant.LarkApprovalIDLabel + approval.ID, constant.LarkThreadLabel + approval.ThreadID, constant.LarkTurnLabel + approval.TurnID, approval.Reason}
	if approval.Command != "" {
		parts = append(parts, "```\n"+approval.Command+"\n```")
	}
	if approval.Cwd != "" {
		parts = append(parts, constant.LarkDirectoryLabel+approval.Cwd)
	}
	if approval.GrantRoot != "" {
		parts = append(parts, constant.LarkWriteScopeLabel+approval.GrantRoot)
	}
	parts = append(parts, approval.Details, approval.PermissionsJSON)
	for _, change := range approval.Changes {
		diff := []rune(change.Diff)
		if len(diff) > 2000 {
			diff = append(diff[:2000], []rune(constant.LarkDiffTruncatedMessage)...)
		}
		parts = append(parts, constant.LarkFileLabel+change.Path+constant.LarkOperationLabel+change.Kind.Type+"\n```diff\n"+string(diff)+"\n```")
		if change.Kind.MovePath != "" {
			parts = append(parts, constant.LarkMoveTargetLabel+change.Kind.MovePath)
		}
	}
	for _, question := range approval.Questions {
		parts = append(parts, question.Header+"\n"+question.Question+constant.LarkQuestionIDLabel+question.ID)
		for _, option := range question.Options {
			parts = append(parts, "- "+option.Label+"："+option.Description)
		}
	}
	if approval.Method == constant.CodexMethodItemToolRequestUserInput {
		parts = append(parts, constant.LarkAnswerCommandPrefix+approval.ID+constant.LarkAnswerCommandSuffix)
	}
	if approval.Method == constant.CodexMethodMcpServerElicitationRequest {
		instruction := constant.LarkElicitationCommandPrefix + approval.ID + constant.LarkElicitationCommandSuffix
		if isEmptyElicitationForm(approval) {
			instruction = constant.LarkEmptyApprovalInstructions
		}
		parts = append(parts, approval.Message, approval.URL, approval.SchemaJSON, instruction)
	}
	body := []rune(strings.Join(parts, "\n\n"))
	if len(body) > 6000 {
		return string(body[:6000]) + constant.LarkRequestTruncatedMessage
	}
	return string(body)
}

// card 构造飞书第二版卡片结构
func card(title string, elements []any) map[string]any {
	return map[string]any{"schema": "2.0", "config": map[string]bool{"update_multi": true}, "header": map[string]any{"title": map[string]string{"tag": "plain_text", "content": title}}, "body": map[string]any{"elements": elements}}
}

// approvalCard 按上游实际支持的选项构造审批卡片
func approvalCard(approval codexmodel.Approval, key string) map[string]any {
	elements := []any{map[string]string{"tag": "markdown", "content": approvalBody(approval)}}
	options := approval.DecisionOptions
	if approval.Method == constant.CodexMethodItemPermissionsRequestApproval {
		options = []codexmodel.DecisionOption{{Value: constant.CodexApprovalAccept, Label: constant.LarkAllowTurnLabel}, {Value: constant.CodexApprovalDecline, Label: constant.LarkDeclineLabel}}
	}
	if approval.Method == constant.CodexMethodMcpServerElicitationRequest {
		options = []codexmodel.DecisionOption{{Value: constant.CodexApprovalDecline, Label: constant.LarkDeclineLabel}, {Value: constant.CodexApprovalCancel, Label: constant.LarkCancelLabel}}
		if isEmptyElicitationForm(approval) {
			options = append([]codexmodel.DecisionOption{{Value: constant.CodexApprovalAccept, Label: constant.LarkAcceptLabel}}, options...)
		}
	}
	if approval.Method == constant.CodexMethodItemToolRequestUserInput {
		options = nil
	}
	columns := make([]any, 0, len(options))
	for _, option := range options {
		button := map[string]any{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": option.Label}, "type": "default", "behaviors": []any{map[string]any{"type": "callback", "value": map[string]string{"routeKey": key, "decision": option.Value}}}}
		columns = append(columns, map[string]any{"tag": "column", "width": "auto", "elements": []any{button}})
	}
	if len(columns) > 0 {
		// 使用第二版分栏容器保持按钮同排，关闭窄屏流式换行
		elements = append(elements, map[string]any{"tag": "column_set", "flex_mode": "none", "horizontal_spacing": "8px", "columns": columns})
	}
	return card(constant.LarkApprovalTitle, elements)
}

// isEmptyElicitationForm 仅识别无需输入且没有额外约束的明确空对象表单
func isEmptyElicitationForm(approval codexmodel.Approval) bool {
	if approval.Method != constant.CodexMethodMcpServerElicitationRequest || (approval.Mode != "form" && approval.Mode != "openai/form") {
		return false
	}
	var schema map[string]json.RawMessage
	if json.Unmarshal([]byte(approval.SchemaJSON), &schema) != nil {
		return false
	}
	// 未识别的结构可能包含组合约束，不能仅凭解析后的 Fields 为空提供同意
	for key := range schema {
		switch key {
		case "type", "properties", "required", "title", "description":
		default:
			return false
		}
	}
	var kind string
	var properties map[string]json.RawMessage
	if json.Unmarshal(schema["type"], &kind) != nil || kind != "object" || json.Unmarshal(schema["properties"], &properties) != nil || properties == nil || len(properties) != 0 {
		return false
	}
	if raw, exists := schema["required"]; exists {
		var required []string
		if json.Unmarshal(raw, &required) != nil || required == nil || len(required) != 0 {
			return false
		}
	}
	return true
}

// respondRoute 核验接收者和当前请求后回复原审批
func (s *Service) respondRoute(key string, source tasksmodel.Source, answer, command string) error {
	s.mu.Lock()
	var route larkmodel.ApprovalRoute
	for _, item := range s.state.Approvals {
		if item.Key == key {
			route = item
			break
		}
	}
	appID := s.config.AppID
	allowed := hasIdentity(s.config.AllowedUsers, route.Source.SenderID)
	if route.Source.ChatType == constant.LarkChatTypeGroup {
		allowed = hasIdentity(s.config.AllowedChats, route.Source.ChatID)
	}
	if route.Source.MessageID == "" {
		binding := s.config.Binding
		allowed = allowed && binding.AppID == route.Source.AppID && binding.ChatID == route.Source.ChatID && binding.SenderID == route.Source.SenderID
	}
	s.mu.Unlock()
	if !allowed || route.Key == "" || route.Status != constant.LarkStatusPending || route.Source.AppID != appID || route.Source.AppID != source.AppID || route.Source.ChatID != source.ChatID || route.Source.SenderID != source.SenderID {
		return constant.ErrLarkApprovalScopeInvalid
	}
	var approval codexmodel.Approval
	for _, item := range s.runtime.GetApprovals() {
		if item.ID == route.ApprovalID && item.Generation == route.Generation {
			approval = item
			break
		}
	}
	if approval.ID == "" || approval.Status != constant.LarkStatusPending {
		return constant.ErrLarkApprovalExpired
	}
	if command == "/answer" {
		var input map[string][]string
		if err := json.Unmarshal([]byte(answer), &input); err != nil {
			return err
		}
		answers := map[string]codexmodel.QuestionAnswer{}
		for id, values := range input {
			answers[id] = codexmodel.QuestionAnswer{Answers: values}
		}
		return s.runtime.AnswerQuestions(approval.ID, answers)
	}
	if command == "/elicitation" {
		return s.runtime.AnswerElicitation(approval.ID, constant.CodexApprovalAccept, answer)
	}
	if approval.Method == constant.CodexMethodItemPermissionsRequestApproval {
		if answer != constant.CodexApprovalAccept && answer != constant.CodexApprovalDecline {
			return constant.ErrLarkPermissionDecisionInvalid
		}
		return s.runtime.RespondPermissions(approval.ID, answer == constant.CodexApprovalAccept, "turn")
	}
	if approval.Method == constant.CodexMethodMcpServerElicitationRequest {
		content := ""
		if answer == constant.CodexApprovalAccept {
			if !isEmptyElicitationForm(approval) {
				return constant.ErrLarkQuestionContentRequired
			}
			content = "{}"
		}
		return s.runtime.AnswerElicitation(approval.ID, answer, content)
	}
	return s.runtime.RespondApproval(approval.ID, answer)
}

// cardAction 将按钮回调直接回复到对应的上游审批
func (s *Service) cardAction(_ context.Context, generation uint64, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	response := &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: constant.LarkStatusError, Content: constant.LarkApprovalActionInvalidMessage}}
	if event == nil || event.Event == nil || event.Event.Operator == nil || event.Event.Context == nil || event.Event.Action == nil {
		return response, nil
	}
	action := event.Event
	s.mu.Lock()
	appID, current := s.config.AppID, s.status.Generation == generation
	s.mu.Unlock()
	if !current {
		return response, nil
	}
	key, _ := action.Action.Value["routeKey"].(string)
	decision, _ := action.Action.Value["decision"].(string)
	err := s.respondRoute(key, tasksmodel.Source{Type: "lark", AppID: appID, ChatID: action.Context.OpenChatID, SenderID: action.Operator.OpenID}, decision, "/approve")
	if err != nil {
		response.Toast.Content = err.Error()
		return response, nil
	}
	// 卡片由状态同步统一更新，避免回调覆盖原审批内容或较新的结果
	response.Toast = &callback.Toast{Type: "success", Content: constant.LarkApprovalRepliedMessage}
	return response, nil
}

// updateApprovalCard 将已处理状态更新到原审批卡片
func (s *Service) updateApprovalCard(route larkmodel.ApprovalRoute, status string) {
	client, parent, err := s.deliveryClient(route.Source)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, constant.LarkHTTPTimeout)
	defer cancel()
	notification, err := s.tasks.GetNotification(route.NotificationID)
	if err == nil {
		content := jsonText(card(constant.LarkApprovalStatusTitle, []any{map[string]string{"tag": "markdown", "content": notification.Body + constant.LarkStatusLabel + status}}))
		var response *larkim.PatchMessageResp
		response, err = client.Im.V1.Message.Patch(ctx, larkim.NewPatchMessageReqBuilder().MessageId(route.MessageID).Body(larkim.NewPatchMessageReqBodyBuilder().Content(content).Build()).Build())
		if err == nil && !response.Success() {
			err = fmt.Errorf("更新审批卡片失败（%d）：%s", response.Code, response.Msg)
		}
	}
	if err != nil {
		s.mu.Lock()
		for i := range s.state.Approvals {
			if s.state.Approvals[i].Key == route.Key {
				s.state.Approvals[i].Error = err.Error()
			}
		}
		s.persistLocked()
		s.mu.Unlock()
		s.changed()
	}
}

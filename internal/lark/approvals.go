package lark

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/codex"
	"agentor/internal/tasks"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// StartPairing 为指定联系人生成一次性配对码
func (s *Service) StartPairing(senderID string) (Pairing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !hasIdentity(s.config.AllowedUsers, senderID) {
		return Pairing{}, errors.New("请先将接收者加入允许的联系人")
	}
	s.pairing = Pairing{Code: rand.Text()[:8], SenderID: senderID, ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano)}
	return s.pairing, nil
}

// ClearBinding 解除桌面任务的远程接收者绑定
func (s *Service) ClearBinding() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.config
	next.Binding = tasks.Source{}
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
func (s *Service) OnCodexEvent(event codex.Event) {
	if event.Method == "host/approvalsUpdated" || event.Method == "turn/completed" || event.Method == "host/status" || event.Method == "serverRequest/resolved" {
		go s.syncApprovals()
	}
}

// configPath 返回当前应用的飞书私有配置路径
func (s *Service) configPath() string {
	return filepath.Join(s.root, "config.json")
}

// pair 核验配对码并绑定原单聊
func (s *Service) pair(source tasks.Source, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	expires, err := time.Parse(time.RFC3339Nano, s.pairing.ExpiresAt)
	if err != nil || time.Now().After(expires) || code != s.pairing.Code || source.SenderID != s.pairing.SenderID || source.ChatType != "p2p" || source.AppID != s.config.AppID {
		return errors.New("配对码无效、已过期或接收者不匹配")
	}
	next := s.config
	next.Binding = source
	next.Binding.MessageID, next.Binding.ReplyToMessageID = "", ""
	if err = appdata.WriteJSON(s.configPath(), next); err != nil {
		return err
	}
	s.config = next
	s.pairing = Pairing{}
	s.changed()
	return nil
}

// textControl 直接处理配对与审批响应而不进入任务队列
func (s *Service) textControl(message Message) (bool, error) {
	parts := strings.SplitN(strings.TrimSpace(message.Text), " ", 3)
	if len(parts) == 0 {
		return false, nil
	}
	command := parts[0]
	if command == "/pair" {
		if len(parts) != 2 {
			return true, errors.New("格式：/pair 配对码")
		}
		return true, s.pair(message.Source, strings.TrimSpace(parts[1]))
	}
	if command != "/approve" && command != "/answer" && command != "/elicitation" {
		return false, nil
	}
	if len(parts) != 3 {
		return true, errors.New("格式：/approve 审批ID accept|decline；/answer 审批ID {问题ID:[答案]}；/elicitation 审批ID JSON")
	}
	s.mu.Lock()
	var route ApprovalRoute
	for _, item := range s.state.Approvals {
		if item.ApprovalID == parts[1] && item.Status == "pending" && item.Source.AppID == message.Source.AppID && item.Source.ChatID == message.Source.ChatID && item.Source.SenderID == message.Source.SenderID {
			route = item
			break
		}
	}
	s.mu.Unlock()
	if route.Key == "" {
		return true, errors.New("没有属于当前发送者和会话的待处理审批")
	}
	return true, s.respondRoute(route.Key, message.Source, parts[2], command)
}

// approvalSource 查找原任务来源或桌面配对接收者
func (s *Service) approvalSource(threadID string) tasks.Source {
	state := s.tasks.GetState()
	for _, execution := range state.Executions {
		if execution.ThreadID == threadID && execution.Status == "running" {
			if execution.Source.Type == "lark" {
				return execution.Source
			}
			break
		}
	}
	for _, request := range state.Requests {
		if request.ThreadID == threadID && request.Status == "running" {
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
		if approval.Status != "pending" {
			continue
		}
		source := s.approvalSource(approval.ThreadID)
		if source.Type != "lark" || source.ChatID == "" {
			continue
		}
		notificationID := rand.Text()
		route := ApprovalRoute{Key: key, ApprovalID: approval.ID, Generation: approval.Generation, Source: source, NotificationID: notificationID, Status: "pending"}
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
		err = s.tasks.RecordNotification(tasks.Notification{ID: notificationID, Title: "Codex 审批与问答", Body: body, Source: source, Parts: []tasks.NotificationPart{{ID: rand.Text(), Kind: "approval", Channel: "lark", Target: source.ChatID, Status: "pending", PayloadJSON: jsonText(approvalCard(approval, key)), Content: body}}})
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
func approvalBody(approval codex.Approval) string {
	parts := []string{"审批 ID：" + approval.ID, "会话：" + approval.ThreadID, "轮次：" + approval.TurnID, approval.Reason}
	if approval.Command != "" {
		parts = append(parts, "```\n"+approval.Command+"\n```")
	}
	if approval.Cwd != "" {
		parts = append(parts, "目录："+approval.Cwd)
	}
	if approval.GrantRoot != "" {
		parts = append(parts, "写入范围："+approval.GrantRoot)
	}
	parts = append(parts, approval.Details, approval.PermissionsJSON)
	for _, change := range approval.Changes {
		diff := []rune(change.Diff)
		if len(diff) > 2000 {
			diff = append(diff[:2000], []rune("\n…完整修改请在桌面审批中心查看")...)
		}
		parts = append(parts, "文件："+change.Path+"\n操作："+change.Kind.Type+"\n```diff\n"+string(diff)+"\n```")
		if change.Kind.MovePath != "" {
			parts = append(parts, "移动至："+change.Kind.MovePath)
		}
	}
	for _, question := range approval.Questions {
		parts = append(parts, question.Header+"\n"+question.Question+"\n问题 ID："+question.ID)
		for _, option := range question.Options {
			parts = append(parts, "- "+option.Label+"："+option.Description)
		}
	}
	if approval.Method == "item/tool/requestUserInput" {
		parts = append(parts, "请在原会话发送 `/answer "+approval.ID+" {\"问题ID\":[\"答案\"]}`，或在桌面问答中心提交。")
	}
	if approval.Method == "mcpServer/elicitation/request" {
		parts = append(parts, approval.Message, approval.URL, approval.SchemaJSON, "发送 `/elicitation "+approval.ID+" JSON` 提交回答，或在桌面继续。")
	}
	body := []rune(strings.Join(parts, "\n\n"))
	if len(body) > 6000 {
		return string(body[:6000]) + "\n\n…完整请求请在桌面审批中心查看"
	}
	return string(body)
}

// card 构造飞书第二版卡片结构
func card(title string, elements []any) map[string]any {
	return map[string]any{"schema": "2.0", "config": map[string]bool{"update_multi": true}, "header": map[string]any{"title": map[string]string{"tag": "plain_text", "content": title}}, "body": map[string]any{"elements": elements}}
}

// approvalCard 按上游实际支持的选项构造审批卡片
func approvalCard(approval codex.Approval, key string) map[string]any {
	elements := []any{map[string]string{"tag": "markdown", "content": approvalBody(approval)}}
	options := approval.DecisionOptions
	if approval.Method == "item/permissions/requestApproval" {
		options = []codex.DecisionOption{{Value: "accept", Label: "允许本轮"}, {Value: "decline", Label: "拒绝"}}
	}
	if approval.Method == "mcpServer/elicitation/request" {
		options = []codex.DecisionOption{{Value: "decline", Label: "拒绝"}, {Value: "cancel", Label: "取消"}}
	}
	if approval.Method == "item/tool/requestUserInput" {
		options = nil
	}
	for _, option := range options {
		elements = append(elements, map[string]any{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": option.Label}, "type": "default", "behaviors": []any{map[string]any{"type": "callback", "value": map[string]string{"routeKey": key, "decision": option.Value}}}})
	}
	return card("Codex 审批与问答", elements)
}

// respondRoute 核验接收者和当前请求后回复原审批
func (s *Service) respondRoute(key string, source tasks.Source, answer, command string) error {
	s.mu.Lock()
	var route ApprovalRoute
	for _, item := range s.state.Approvals {
		if item.Key == key {
			route = item
			break
		}
	}
	appID := s.config.AppID
	allowed := hasIdentity(s.config.AllowedUsers, route.Source.SenderID)
	if route.Source.ChatType == "group" {
		allowed = hasIdentity(s.config.AllowedChats, route.Source.ChatID)
	}
	if route.Source.MessageID == "" {
		binding := s.config.Binding
		allowed = allowed && binding.AppID == route.Source.AppID && binding.ChatID == route.Source.ChatID && binding.SenderID == route.Source.SenderID
	}
	s.mu.Unlock()
	if !allowed || route.Key == "" || route.Status != "pending" || route.Source.AppID != appID || route.Source.AppID != source.AppID || route.Source.ChatID != source.ChatID || route.Source.SenderID != source.SenderID {
		return errors.New("审批已失效，或不属于当前接收者及会话")
	}
	var approval codex.Approval
	for _, item := range s.runtime.GetApprovals() {
		if item.ID == route.ApprovalID && item.Generation == route.Generation {
			approval = item
			break
		}
	}
	if approval.ID == "" || approval.Status != "pending" {
		return errors.New("该请求已处理或失效")
	}
	if command == "/answer" {
		var input map[string][]string
		if err := json.Unmarshal([]byte(answer), &input); err != nil {
			return err
		}
		answers := map[string]codex.QuestionAnswer{}
		for id, values := range input {
			answers[id] = codex.QuestionAnswer{Answers: values}
		}
		return s.runtime.AnswerQuestions(approval.ID, answers)
	}
	if command == "/elicitation" {
		return s.runtime.AnswerElicitation(approval.ID, "accept", answer)
	}
	if approval.Method == "item/permissions/requestApproval" {
		if answer != "accept" && answer != "decline" {
			return errors.New("权限审批只支持 accept 或 decline")
		}
		return s.runtime.RespondPermissions(approval.ID, answer == "accept", "turn")
	}
	if approval.Method == "mcpServer/elicitation/request" {
		return s.runtime.AnswerElicitation(approval.ID, answer, "")
	}
	return s.runtime.RespondApproval(approval.ID, answer)
}

// cardAction 将按钮回调直接回复到对应的上游审批
func (s *Service) cardAction(_ context.Context, generation uint64, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	response := &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "error", Content: "无效的审批操作"}}
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
	err := s.respondRoute(key, tasks.Source{Type: "lark", AppID: appID, ChatID: action.Context.OpenChatID, SenderID: action.Operator.OpenID}, decision, "/approve")
	if err != nil {
		response.Toast.Content = err.Error()
		return response, nil
	}
	response.Toast = &callback.Toast{Type: "success", Content: "已回复审批，最终执行结果仍以任务记录为准"}
	response.Card = &callback.Card{Type: "raw", Data: card("审批已处理", []any{map[string]string{"tag": "markdown", "content": "审批已回复，等待 Codex 执行结果。"}})}
	return response, nil
}

// updateApprovalCard 将已处理状态更新到原审批卡片
func (s *Service) updateApprovalCard(route ApprovalRoute, status string) {
	client, parent, err := s.deliveryClient(route.Source)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	content := jsonText(card("审批状态", []any{map[string]string{"tag": "markdown", "content": "审批 ID：" + route.ApprovalID + "\n\n状态：" + status}}))
	response, err := client.Im.V1.Message.Patch(ctx, larkim.NewPatchMessageReqBuilder().MessageId(route.MessageID).Body(larkim.NewPatchMessageReqBodyBuilder().Content(content).Build()).Build())
	if err == nil && !response.Success() {
		err = fmt.Errorf("更新审批卡片失败（%d）：%s", response.Code, response.Msg)
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

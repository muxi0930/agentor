package lark

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"agentor/internal/tasks"
	larksdk "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// OnNotification 将通知拆分为可独立跟踪的飞书发送项
//
//wails:ignore
func (s *Service) OnNotification(notification tasks.Notification) {
	if notification.Source.Type != "lark" || s.ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	if s.preparing[notification.ID] {
		s.mu.Unlock()
		return
	}
	s.preparing[notification.ID] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.preparing, notification.ID); s.mu.Unlock() }()
	current, err := s.tasks.GetNotification(notification.ID)
	if err != nil {
		return
	}
	hasParts := false
	for _, part := range current.Parts {
		if part.Channel == "lark" {
			hasParts = true
			break
		}
	}
	if !hasParts {
		// 原控制器统一汇报结果，辅助步骤和流水线记录仅保留桌面通知
		if notification.RequestID != "" && (notification.ExecutionID != "" || notification.PipelineRunID != "" || notification.PipelineSessionID != "") {
			return
		}
		parts := []tasks.NotificationPart{}
		body := []rune(current.Body)
		if len(body) == 0 {
			body = []rune(current.Title)
		}
		for len(body) > 0 {
			end := len(body)
			if end > 4000 {
				end = 4000
			}
			parts = append(parts, tasks.NotificationPart{ID: rand.Text(), Kind: "text", Channel: "lark", Target: current.Source.ChatID, Status: "pending", Content: string(body[:end])})
			body = body[end:]
		}
		if err = s.tasks.AddNotificationParts(current.ID, parts); err != nil {
			return
		}
	}
	s.kickDelivery(current.ID)
}

// RetryNotificationPart 仅重试明确失败的发送项
func (s *Service) RetryNotificationPart(id, partID string) error {
	notification, err := s.tasks.GetNotification(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	active := s.sending[id]
	s.mu.Unlock()
	if active {
		return errors.New("通知正在发送，请等待结果")
	}
	for _, part := range notification.Parts {
		if part.ID == partID && part.Channel == "lark" {
			if part.Status != "failed" {
				return errors.New("只可重试明确失败的发送项")
			}
			part.Status, part.Error = "pending", ""
			if err = s.tasks.UpdateNotificationPart(id, part); err != nil {
				return err
			}
			s.kickDelivery(id)
			return nil
		}
	}
	return errors.New("发送项不存在")
}

// IgnoreNotificationPart 保留用户说明并忽略失败或未知发送项
func (s *Service) IgnoreNotificationPart(id, partID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("请填写忽略原因")
	}
	notification, err := s.tasks.GetNotification(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	active := s.sending[id]
	s.mu.Unlock()
	if active {
		return errors.New("通知正在发送，请等待结果")
	}
	for _, part := range notification.Parts {
		if part.ID == partID && part.Channel == "lark" {
			if part.Status == "sent" {
				return errors.New("此项已发送")
			}
			part.Status, part.Error = "ignored", reason
			return s.tasks.UpdateNotificationPart(id, part)
		}
	}
	return errors.New("发送项不存在")
}

// ConfirmNotDelivered 在用户确认未送达后允许重试未知发送项
func (s *Service) ConfirmNotDelivered(id, partID string) error {
	notification, err := s.tasks.GetNotification(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	active := s.sending[id]
	s.mu.Unlock()
	if active {
		return errors.New("通知仍在发送，请等待结果")
	}
	for _, part := range notification.Parts {
		if part.ID == partID && part.Status == "unknown" && part.Channel == "lark" {
			part.Status, part.Error = "failed", "用户确认未送达"
			if err = s.tasks.UpdateNotificationPart(id, part); err != nil {
				return err
			}
			return s.RetryNotificationPart(id, partID)
		}
	}
	return errors.New("此项不是结果不明的发送项")
}

// ShareArtifact 将产物发送给原任务来源或配对接收者
func (s *Service) ShareArtifact(artifactID string) error {
	source, err := s.GetArtifactShareTarget(artifactID)
	if err != nil {
		return err
	}
	item, err := s.files.GetAttachment(artifactID)
	if err != nil {
		return err
	}
	return s.share(item.ID, item.ProjectID, source)
}

// GetArtifactShareTarget 查询产物将要发送的接收者
func (s *Service) GetArtifactShareTarget(artifactID string) (tasks.Source, error) {
	item, err := s.files.GetAttachment(artifactID)
	if err != nil {
		return tasks.Source{}, err
	}
	var source tasks.Source
	if item.Kind != "artifact" || item.Status != "ready" {
		return tasks.Source{}, errors.New("请选择已登记的可用产物")
	}
	for _, execution := range s.tasks.GetState().Executions {
		if execution.ID == item.ExecutionID {
			source = execution.Source
			break
		}
	}
	if item.PipelineRunID != "" && s.pipelines != nil {
		for _, run := range s.pipelines.GetState().Runs {
			if run.ID == item.PipelineRunID {
				source = run.Source
				break
			}
		}
	}
	if source.Type != "lark" {
		s.mu.Lock()
		source = s.config.Binding
		s.mu.Unlock()
	}
	if source.ChatID == "" {
		return tasks.Source{}, errors.New("请先配对飞书接收者")
	}
	return source, nil
}

// ShareArtifactFor 核验任务范围后登记产物分享
//
//wails:ignore
func (s *Service) ShareArtifactFor(artifactID, projectID, executionID string, source tasks.Source) error {
	item, err := s.files.GetAttachment(artifactID)
	if err != nil {
		return err
	}
	if item.Kind != "artifact" || projectID != "" && item.ProjectID != projectID || executionID != "" && item.ExecutionID != executionID {
		return errors.New("产物不属于当前任务范围")
	}
	if source.Type != "lark" {
		s.mu.Lock()
		source = s.config.Binding
		s.mu.Unlock()
	}
	return s.share(artifactID, item.ProjectID, source)
}

// drain 恢复尚未尝试发送的通知项
func (s *Service) drain() {
	for _, notification := range s.tasks.GetState().Notifications {
		s.OnNotification(notification)
	}
}

// kickDelivery 为一条通知启动串行发送
func (s *Service) kickDelivery(id string) {
	s.mu.Lock()
	if s.sending[id] || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.sending[id] = true
	s.mu.Unlock()
	go func() {
		defer func() { s.mu.Lock(); delete(s.sending, id); s.mu.Unlock(); s.changed() }()
		notification, err := s.tasks.GetNotification(id)
		if err != nil {
			return
		}
		for _, part := range notification.Parts {
			if part.Channel != "lark" || part.Status != "pending" {
				continue
			}
			s.deliver(notification, part)
		}
		s.finishReaction(notification.Source)
	}()
}

// deliveryClient 核验应用和接收范围并取得当前连接
func (s *Service) deliveryClient(source tasks.Source) (*larksdk.Client, context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return nil, nil, s.ctx.Err()
	}
	if !s.config.Enabled || s.client == nil || s.status.Phase != "ready" {
		return nil, nil, errors.New("飞书尚未连接，未发送")
	}
	if source.AppID != s.config.AppID {
		return nil, nil, errors.New("通知属于另一个飞书应用，未发送")
	}
	allowed := hasIdentity(s.config.AllowedUsers, source.SenderID)
	if source.ChatType == "group" {
		allowed = hasIdentity(s.config.AllowedChats, source.ChatID)
	}
	if !allowed {
		return nil, nil, errors.New("原接收者的授权已取消，未发送")
	}
	if source.ChatID == "" && source.ChatType != "p2p" {
		return nil, nil, errors.New("通知缺少原会话，未发送")
	}
	ctx := s.connectionCtx
	if ctx == nil {
		ctx = s.ctx
	}
	return s.client, ctx, nil
}

// deliver 先记录发送意图再上传或回复原消息
func (s *Service) deliver(notification tasks.Notification, part tasks.NotificationPart) {
	client, parent, err := s.deliveryClient(notification.Source)
	if err != nil {
		part.Status, part.Error = "failed", err.Error()
		s.tasks.UpdateNotificationPart(notification.ID, part)
		return
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	part.Status, part.AttemptedAt, part.Error = "unknown", now(), "发送中；未收到明确响应前请勿重发"
	if err = s.tasks.UpdateNotificationPart(notification.ID, part); err != nil {
		return
	}
	status, messageID := "unknown", ""
	type_, content := "text", jsonText(map[string]any{"text": part.Content})
	if part.Kind == "approval" {
		type_, content = "interactive", part.PayloadJSON
	}
	if part.Kind == "file" {
		item, readErr := s.files.GetAttachment(part.ArtifactID)
		if readErr != nil || item.Status != "ready" {
			status = "failed"
			err = errors.New("产物不可用")
		} else {
			image := strings.HasPrefix(item.MediaType, "image/")
			if part.UploadKey == "" {
				file, openErr := os.Open(item.LocalPath)
				if openErr != nil {
					status, err = "failed", openErr
				} else {
					if image {
						response, uploadErr := client.Im.V1.Image.Create(ctx, larkim.NewCreateImageReqBuilder().Body(larkim.NewCreateImageReqBodyBuilder().ImageType("message").Image(file).Build()).Build())
						err = uploadErr
						if err == nil {
							if !response.Success() || response.Data == nil {
								status = "failed"
								err = fmt.Errorf("上传图片失败（%d）：%s", response.Code, response.Msg)
							} else {
								part.UploadKey = value(response.Data.ImageKey)
							}
						}
					} else {
						response, uploadErr := client.Im.V1.File.Create(ctx, larkim.NewCreateFileReqBuilder().Body(larkim.NewCreateFileReqBodyBuilder().FileType("stream").FileName(item.Name).File(file).Build()).Build())
						err = uploadErr
						if err == nil {
							if !response.Success() || response.Data == nil {
								status = "failed"
								err = fmt.Errorf("上传文件失败（%d）：%s", response.Code, response.Msg)
							} else {
								part.UploadKey = value(response.Data.FileKey)
							}
						}
					}
					file.Close()
					if err == nil {
						if part.UploadKey == "" {
							err = errors.New("上传成功响应缺少资源 ID，结果不明")
						} else if saveErr := s.tasks.UpdateNotificationPart(notification.ID, part); saveErr != nil {
							return
						}
					}
				}
			}
			if err == nil {
				type_, content = "file", jsonText(map[string]any{"file_key": part.UploadKey})
				if image {
					type_, content = "image", jsonText(map[string]any{"image_key": part.UploadKey})
				}
			}
		}
	}
	if err == nil {
		messageID, status, err = sendMessage(ctx, client, notification.Source, type_, content, part.ID)
	}
	part.Status, part.MessageID = status, messageID
	if err != nil {
		part.Error = err.Error()
	} else {
		part.Error = ""
	}
	s.tasks.UpdateNotificationPart(notification.ID, part)
	if part.Kind == "approval" && status == "sent" {
		var resolved ApprovalRoute
		s.mu.Lock()
		for i := range s.state.Approvals {
			if s.state.Approvals[i].NotificationID == notification.ID {
				s.state.Approvals[i].MessageID = messageID
				if s.state.Approvals[i].Status != "pending" {
					resolved = s.state.Approvals[i]
				}
				break
			}
		}
		s.persistLocked()
		s.mu.Unlock()
		if resolved.Key != "" {
			go s.updateApprovalCard(resolved, resolved.Status)
		}
	}
}

// jsonText 将消息内容编码为 JSON 字符串
func jsonText(value any) string { raw, _ := json.Marshal(value); return string(raw) }

// sendMessage 回复原消息或发送到明确接收者
func sendMessage(ctx context.Context, client *larksdk.Client, source tasks.Source, type_, content, id string) (string, string, error) {
	if source.MessageID != "" {
		response, err := client.Im.V1.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().MessageId(source.MessageID).Body(larkim.NewReplyMessageReqBodyBuilder().MsgType(type_).Content(content).Uuid(id).Build()).Build())
		if err != nil {
			return "", "unknown", err
		}
		if !response.Success() {
			return "", "failed", fmt.Errorf("回复失败（%d）：%s", response.Code, response.Msg)
		}
		if response.Data == nil || value(response.Data.MessageId) == "" {
			return "", "unknown", errors.New("回复响应缺少消息 ID，结果不明")
		}
		return value(response.Data.MessageId), "sent", nil
	}
	receiveType, receiveID := "chat_id", source.ChatID
	if receiveID == "" && source.ChatType == "p2p" {
		receiveType, receiveID = "open_id", source.SenderID
	}
	response, err := client.Im.V1.Message.Create(ctx, larkim.NewCreateMessageReqBuilder().ReceiveIdType(receiveType).Body(larkim.NewCreateMessageReqBodyBuilder().ReceiveId(receiveID).MsgType(type_).Content(content).Uuid(id).Build()).Build())
	if err != nil {
		return "", "unknown", err
	}
	if !response.Success() {
		return "", "failed", fmt.Errorf("发送失败（%d）：%s", response.Code, response.Msg)
	}
	if response.Data == nil || value(response.Data.MessageId) == "" {
		return "", "unknown", errors.New("发送响应缺少消息 ID，结果不明")
	}
	return value(response.Data.MessageId), "sent", nil
}

// share 登记产物发送项并保留项目归属
func (s *Service) share(artifactID, projectID string, source tasks.Source) error {
	if source.Type != "lark" || source.ChatID == "" {
		return errors.New("请先在飞书设置中配对接收者")
	}
	if _, _, err := s.deliveryClient(source); err != nil {
		return err
	}
	return s.tasks.RecordNotification(tasks.Notification{Title: "分享产物", Body: "产物已进入发送队列", ProjectID: projectID, Source: source, Parts: []tasks.NotificationPart{{ID: rand.Text(), Kind: "file", Channel: "lark", Target: source.ChatID, Status: "pending", ArtifactID: artifactID}}})
}

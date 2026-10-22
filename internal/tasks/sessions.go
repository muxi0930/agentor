package tasks

import (
	"context"
	"crypto/rand"
	"errors"
)

// RunReadOnlyTurn 复用正式执行的事件归属和中断逻辑运行只读轮次
//
//wails:ignore
func (s *Service) RunReadOnlyTurn(ctx context.Context, threadID, text, model, effort, cwd, id string, started func(string)) (string, string, error) {
	waiter := &turnWaiter{items: map[string]string{}, done: make(chan turnOutcome, 1)}
	s.mu.Lock()
	if s.turns[threadID] != nil {
		s.mu.Unlock()
		return "error", "", errors.New("此专用会话已有活动请求")
	}
	s.turns[threadID] = waiter
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.turns[threadID] == waiter {
			delete(s.turns, threadID)
		}
		s.mu.Unlock()
	}()
	if err := ctx.Err(); err != nil {
		return "interrupted", "", err
	}
	turn, err := s.runtime.StartManagedTurn(threadID, text, model, effort, "read-only", cwd, id, nil, "")
	if err != nil {
		return "error", "", err
	}
	s.mu.Lock()
	waiter.turnID = turn.ID
	s.mu.Unlock()
	started(turn.ID)
	return s.awaitTurn(ctx, waiter, threadID)
}

// RecordNotification 持久化业务结果后通知发送入口
//
//wails:ignore
func (s *Service) RecordNotification(notification Notification) error {
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	if notification.ID == "" {
		notification.ID = rand.Text()
	}
	if notification.CreatedAt == "" {
		notification.CreatedAt = now()
	}
	if len(notification.Parts) == 0 {
		notification.Parts = []NotificationPart{{ID: rand.Text(), Kind: "text", Channel: "desktop", Status: "sent", Content: notification.Body}}
	}
	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.state.Notifications = append(s.state.Notifications, notification)
	err := s.persistLocked()
	s.mu.Unlock()
	s.changed()
	if err == nil {
		s.notify(notification)
	}
	return err
}

// ReportExternalResult 将外部业务结果交还原始控制中心请求
//
//wails:ignore
func (s *Service) ReportExternalResult(requestID, kind, resultID, projectID, text string) error {
	if requestID == "" {
		return nil
	}
	s.mu.Lock()
	var original Request
	for _, request := range s.state.Requests {
		if request.ID == requestID {
			original = request
			break
		}
	}
	s.mu.Unlock()
	if original.ID == "" {
		return errors.New("原始控制中心请求不存在")
	}
	_, err := s.submitRequest(ControllerInput{Text: text, ProjectID: projectID, Model: original.Model, Effort: original.Effort, FastMode: original.FastMode, Source: original.Source}, kind, resultID)
	return err
}

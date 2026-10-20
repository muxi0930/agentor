package lark

import (
	"crypto/rand"
	"errors"

	"agentor/internal/tasks"
)

// SendConnectionTest 向已授权联系人登记一条连接测试消息
func (s *Service) SendConnectionTest(openID string) error {
	s.mu.Lock()
	if !hasIdentity(s.config.AllowedUsers, openID) {
		s.mu.Unlock()
		return errors.New("请先授权此测试联系人")
	}
	source := tasks.Source{Type: "lark", AppID: s.config.AppID, SenderID: openID, ChatType: "p2p"}
	s.mu.Unlock()
	if _, _, err := s.deliveryClient(source); err != nil {
		return err
	}
	body := "agentor 连接验证：官方 Go SDK 已连接。请在此单聊回复一条测试任务；图片、文件和需要确认的操作会进入 agentor 的任务与审批流程。"
	return s.tasks.RecordNotification(tasks.Notification{Title: "飞书连接验证", Body: body, Source: source, Parts: []tasks.NotificationPart{{ID: rand.Text(), Kind: "text", Channel: "lark", Target: openID, Status: "pending", Content: body}}})
}

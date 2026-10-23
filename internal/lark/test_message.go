package lark

import (
	"crypto/rand"

	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// SendConnectionTest 向已授权联系人登记一条连接测试消息
func (s *Service) SendConnectionTest(openID string) error {
	s.mu.Lock()
	if !hasIdentity(s.config.AllowedUsers, openID) {
		s.mu.Unlock()
		return constant.ErrLarkTestRecipientUnauthorized
	}
	source := tasksmodel.Source{Type: "lark", AppID: s.config.AppID, SenderID: openID, ChatType: constant.LarkChatTypePrivate}
	s.mu.Unlock()
	if _, _, err := s.deliveryClient(source); err != nil {
		return err
	}
	body := constant.LarkConnectionTestMessage
	return s.tasks.RecordNotification(tasksmodel.Notification{Title: constant.LarkConnectionTestTitle, Body: body, Source: source, Parts: []tasksmodel.NotificationPart{{ID: rand.Text(), Kind: "text", Channel: "lark", Target: openID, Status: constant.LarkStatusPending, Content: body}}})
}

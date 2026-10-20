package tasks

import "errors"

// SetNotificationHandler 设置通知持久化后的发送入口
//
//wails:ignore
func (s *Service) SetNotificationHandler(handler func(Notification)) {
	s.mu.Lock()
	s.notificationHandler = handler
	s.mu.Unlock()
}

// GetNotification 读取独立的通知快照
//
//wails:ignore
func (s *Service) GetNotification(id string) (Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.state.Notifications {
		if item.ID == id {
			return copyState(State{Notifications: []Notification{item}}).Notifications[0], nil
		}
	}
	return Notification{}, errors.New("通知不存在")
}

// AddNotificationParts 为既有通知添加发送记录
//
//wails:ignore
func (s *Service) AddNotificationParts(id string, parts []NotificationPart) error {
	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	for i := range s.state.Notifications {
		if s.state.Notifications[i].ID == id {
			s.state.Notifications[i].Parts = append(s.state.Notifications[i].Parts, parts...)
			err := s.persistLocked()
			s.mu.Unlock()
			s.changed()
			return err
		}
	}
	s.mu.Unlock()
	return errors.New("通知不存在")
}

// UpdateNotificationPart 保存单个发送项的最终状态
//
//wails:ignore
func (s *Service) UpdateNotificationPart(id string, part NotificationPart) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	for i := range s.state.Notifications {
		notification := &s.state.Notifications[i]
		if notification.ID == id {
			for j := range notification.Parts {
				if notification.Parts[j].ID == part.ID {
					notification.Parts[j] = part
					err := s.persistLocked()
					s.changed()
					return err
				}
			}
		}
	}
	return errors.New("通知发送项不存在")
}

// SubmitRemoteRequest 使用预留请求标识提交远程任务
//
//wails:ignore
func (s *Service) SubmitRemoteRequest(input ControllerInput, requestID string) (Request, error) {
	if requestID == "" {
		return Request{}, errors.New("缺少远程请求 ID")
	}
	return s.submitRequest(input, "instruction", "", requestID)
}

// notify 异步通知已登记的业务结果
func (s *Service) notify(notification Notification) {
	s.mu.Lock()
	handler := s.notificationHandler
	s.mu.Unlock()
	if notification.ID != "" && handler != nil && s.ctx.Err() == nil {
		go handler(notification)
	}
}

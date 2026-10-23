package tasks

import (
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// SetNotificationHandler 设置通知持久化后的发送入口
//
//wails:ignore
func (s *Service) SetNotificationHandler(handler func(tasksmodel.Notification)) {
	s.mu.Lock()
	s.notificationHandler = handler
	s.mu.Unlock()
}

// GetNotification 读取独立的通知快照
//
//wails:ignore
func (s *Service) GetNotification(id string) (tasksmodel.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.state.Notifications {
		if item.ID == id {
			return copyState(tasksmodel.State{Notifications: []tasksmodel.Notification{item}}).Notifications[0], nil
		}
	}
	return tasksmodel.Notification{}, constant.ErrTasksNotificationNotFound
}

// AddNotificationParts 为既有通知添加发送记录
//
//wails:ignore
func (s *Service) AddNotificationParts(id string, parts []tasksmodel.NotificationPart) error {
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
	return constant.ErrTasksNotificationNotFound
}

// UpdateNotificationPart 保存单个发送项的最终状态
//
//wails:ignore
func (s *Service) UpdateNotificationPart(id string, part tasksmodel.NotificationPart) error {
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
	return constant.ErrTasksNotificationPartNotFound
}

// SubmitRemoteRequest 使用预留请求标识提交远程任务
//
//wails:ignore
func (s *Service) SubmitRemoteRequest(input tasksmodel.ControllerInput, requestID string) (tasksmodel.Request, error) {
	if requestID == "" {
		return tasksmodel.Request{}, constant.ErrTasksRemoteRequestIDRequired
	}
	return s.submitRequest(input, constant.TasksRequestKindInstruction, "", requestID)
}

// notify 异步通知已登记的业务结果
func (s *Service) notify(notification tasksmodel.Notification) {
	s.mu.Lock()
	handler := s.notificationHandler
	s.mu.Unlock()
	if notification.ID != "" && handler != nil && s.ctx.Err() == nil {
		go handler(notification)
	}
}

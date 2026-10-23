package codex

import (
	"slices"

	"agentor/internal/constant"
)

// EnsureOpen 在正式执行前恢复线程，已打开的新线程无需读取尚未落盘的历史
//
//wails:ignore
func (s *Service) EnsureOpen(threadID string) error {
	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()

	return s.ensureOpen(threadID)
}

// ensureOpen 在订阅锁内恢复线程并记录普通会话的订阅顺序
func (s *Service) ensureOpen(threadID string) error {
	s.mu.Lock()
	opened := s.opened[threadID]
	s.mu.Unlock()
	if opened {
		return nil
	}

	if err := s.makeSubscriptionRoom(threadID); err != nil {
		return err
	}

	params, err := s.resumeParameters(threadID)
	if err != nil {
		return err
	}
	if err := s.call(constant.CodexMethodThreadResume, params, nil); err != nil {
		return err
	}

	s.mu.Lock()
	s.opened[threadID] = true
	if s.data.GetSettings().ManagedThreads[threadID] == "" {
		s.subscriptions = append(s.subscriptions, threadID)
	}
	s.mu.Unlock()
	return nil
}

// makeSubscriptionRoom 在订阅锁内为普通会话淘汰最早订阅的空闲会话
func (s *Service) makeSubscriptionRoom(threadID string) error {
	if s.data.GetSettings().ManagedThreads[threadID] != "" {
		return nil
	}

	s.mu.Lock()
	if len(s.subscriptions) < constant.CodexThreadSubscriptionLimit {
		s.mu.Unlock()
		return nil
	}

	oldest := ""
	for _, id := range s.subscriptions {
		if s.active[id] == "" {
			oldest = id
			break
		}
	}
	s.mu.Unlock()
	if oldest == "" {
		return constant.ErrCodexSubscriptionLimit
	}

	return s.unsubscribeThread(oldest)
}

// unsubscribeThread 在订阅锁内取消本连接订阅并清理打开状态
func (s *Service) unsubscribeThread(threadID string) error {
	s.mu.Lock()
	generation := s.status.Generation
	s.mu.Unlock()
	if err := s.call(constant.CodexMethodThreadUnsubscribe, map[string]string{"threadId": threadID}, nil); err != nil {
		return err
	}

	s.mu.Lock()
	if s.status.Generation == generation {
		s.forgetThreadLocked(threadID)
	}
	s.mu.Unlock()
	return nil
}

// forgetThreadLocked 在状态锁内移除会话的本地订阅记录
func (s *Service) forgetThreadLocked(threadID string) {
	delete(s.opened, threadID)
	s.subscriptions = slices.DeleteFunc(s.subscriptions, func(id string) bool { return id == threadID })
}

// resumeParameters 将独立账号当前的提供方与模型应用到共享历史线程
func (s *Service) resumeParameters(threadID string) (map[string]any, error) {
	params := map[string]any{"threadId": threadID, "excludeTurns": true}
	settings := s.data.GetSettings()
	if settings.ReuseCLI {
		return params, nil
	}

	stored, err := s.profile(settings.ActiveAccountID)
	if err != nil {
		return nil, err
	}

	params["modelProvider"] = "openai"
	if stored.Profile.Type == constant.CodexAccountTypeAPI {
		params["modelProvider"] = "agentor_api"
	}
	if settings.Model != "" {
		params["model"] = settings.Model
	} else {
		models, err := s.GetModels()
		if err != nil {
			return nil, err
		}

		for _, model := range models {
			if model.IsDefault {
				params["model"] = model.Model
				break
			}
		}
	}
	return params, nil
}

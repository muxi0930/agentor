package codex

// EnsureOpen 在正式执行前恢复线程，已打开的新线程无需读取尚未落盘的历史
//
//wails:ignore
func (s *Service) EnsureOpen(threadID string) error {
	s.mu.Lock()
	opened := s.opened[threadID]
	s.mu.Unlock()
	if opened {
		return nil
	}

	params, err := s.resumeParameters(threadID)
	if err != nil {
		return err
	}
	if err := s.call("thread/resume", params, nil); err != nil {
		return err
	}

	s.mu.Lock()
	s.opened[threadID] = true
	s.mu.Unlock()
	return nil
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
	if stored.Profile.Type == "api" {
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

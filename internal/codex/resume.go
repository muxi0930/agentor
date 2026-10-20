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

	if err := s.call("thread/resume", map[string]any{"threadId": threadID, "excludeTurns": true}, nil); err != nil {
		return err
	}

	s.mu.Lock()
	s.opened[threadID] = true
	s.mu.Unlock()
	return nil
}

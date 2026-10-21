package codex

import "testing"

// TestOldConnectionCannotOverwriteStatus 验证旧连接不能覆盖当前连接状态
func TestOldConnectionCannotOverwriteStatus(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()

	s.status.Generation = 2
	current := s.setStatus(2, "ready", "connected")
	stale := s.setStatus(1, "error", "old process exited")
	if stale.Phase != "ready" || stale.Revision != current.Revision {
		t.Fatal("old connection overwrote the new status")
	}
}

// TestClosedProcessCannotBecomeReady 验证已关闭进程不能进入就绪状态
func TestClosedProcessCannotBecomeReady(t *testing.T) {
	s := NewService(nil, nil)
	defer s.cancel()

	done := make(chan struct{})
	close(done)
	s.client = &client{done: done}
	s.status.Generation = 1
	if status := s.setStatus(1, "ready", "connected"); status.Phase != "error" {
		t.Fatal("closed process was marked ready")
	}
}

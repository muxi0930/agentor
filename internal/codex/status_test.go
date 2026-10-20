package codex

import "testing"

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

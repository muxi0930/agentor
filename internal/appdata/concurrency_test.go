package appdata

import "testing"

// TestMaxConcurrentAgents 验证并发上限默认值、边界和持久化
func TestMaxConcurrentAgents(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.GetSettings().MaxConcurrentAgents != 5 {
		t.Fatal(s.GetSettings().MaxConcurrentAgents)
	}
	for _, limit := range []int{1, 50} {
		if _, err := s.SetMaxConcurrentAgents(limit); err != nil {
			t.Fatal(err)
		}
		reopened, err := NewService("test")
		if err != nil {
			t.Fatal(err)
		}
		if reopened.GetSettings().MaxConcurrentAgents != limit {
			t.Fatal(reopened.GetSettings().MaxConcurrentAgents)
		}
	}
	for _, limit := range []int{-1, 0, 51} {
		if _, err := s.SetMaxConcurrentAgents(limit); err == nil {
			t.Fatalf("接受了无效上限 %d", limit)
		}
	}
	if s.GetSettings().MaxConcurrentAgents != 50 {
		t.Fatal("无效输入改变了配置")
	}
}

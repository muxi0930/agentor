package appdata

import (
	"path/filepath"
	"testing"
)

// TestAgentTimeoutSettings 验证旧配置默认两小时及不限时配置持久化
func TestAgentTimeoutSettings(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	// 旧配置未包含时限字段时仍应使用两小时
	if err := WriteJSON(filepath.Join(s.root, "settings.json"), map[string]any{"maxConcurrentAgents": 5}); err != nil {
		t.Fatal(err)
	}
	s, err = NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.GetSettings().AgentTimeoutSeconds != 7200 {
		t.Fatal(s.GetSettings().AgentTimeoutSeconds)
	}
	for _, seconds := range []int{60, 0} {
		if _, err := s.SetAgentTimeoutSeconds(seconds); err != nil {
			t.Fatal(err)
		}
		reopened, err := NewService("test")
		if err != nil {
			t.Fatal(err)
		}
		if reopened.GetSettings().AgentTimeoutSeconds != seconds {
			t.Fatal(reopened.GetSettings().AgentTimeoutSeconds)
		}
	}
	if _, err := s.SetAgentTimeoutSeconds(-1); err == nil {
		t.Fatal("接受了负数时限")
	}
}

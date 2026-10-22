package appdata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestChatWidthPersists 验证聊天宽度默认标准并在重载后恢复选择
func TestChatWidthPersists(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.GetSettings().ChatWidth != "standard" {
		t.Fatal("聊天宽度未默认使用标准布局")
	}
	if _, err = s.SetChatWidth("wide"); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if reopened.GetSettings().ChatWidth != "wide" {
		t.Fatal("重载后未恢复更宽布局")
	}
	if _, err = reopened.SetChatWidth("standard"); err != nil {
		t.Fatal(err)
	}
	if reopened.GetSettings().ChatWidth != "standard" {
		t.Fatal("未切回标准布局")
	}
}

// TestObsoleteWorkspaceLayout 验证旧布局配置不影响其他偏好且不再写回
func TestObsoleteWorkspaceLayout(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOR_DATA_DIR", root)
	t.Setenv("CODEX_HOME", t.TempDir())
	path := filepath.Join(root, "settings.json")
	if err := os.WriteFile(path, []byte(`{"workspaceLayout":"workbench","theme":"dark","mode":"workspace"}`), 0600); err != nil {
		t.Fatal(err)
	}

	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if settings := s.GetSettings(); settings.Theme != "dark" || settings.Mode != "workspace" {
		t.Fatal("读取旧布局配置时改变了其他偏好")
	}
	if _, err := s.SetTheme("light"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if _, exists := saved["workspaceLayout"]; exists {
		t.Fatal("已移除的布局配置仍被写回")
	}
	reopened, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if settings := reopened.GetSettings(); settings.Theme != "light" || settings.Mode != "workspace" {
		t.Fatal("重新打开后未保留其他偏好")
	}
}

package appdata

import "testing"

// TestAccountPreferencesStayWithIdentity 验证账号切换恢复各自的模型与默认权限
func TestAccountPreferencesStayWithIdentity(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	s, err := NewService()
	if err != nil {
		t.Fatal(err)
	}
	s.SetModelSelection("cli-model", "medium")
	s.SelectAccount("independent", false)
	s.SetModelSelection("own-model", "high")
	s.SetPermission("default", "", "workspace-write")
	s.SelectAccount("", true)
	if settings := s.GetSettings(); settings.Model != "cli-model" || settings.Effort != "medium" || settings.PermissionMode != "read-only" {
		t.Fatal("CLI 偏好被独立账号覆盖")
	}
	s.SelectAccount("independent", false)
	if settings := s.GetSettings(); settings.Model != "own-model" || settings.Effort != "high" || settings.PermissionMode != "workspace-write" {
		t.Fatal("独立账号偏好没有恢复")
	}
}

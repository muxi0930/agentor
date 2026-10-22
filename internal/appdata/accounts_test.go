package appdata

import "testing"

// TestAccountPreferencesStayWithIdentity 验证账号切换恢复各自的模型与默认权限
func TestAccountPreferencesStayWithIdentity(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	s.SetModelSelection("cli-model", "medium")
	if _, err := s.SetPermission("default", "", "read-only"); err != nil {
		t.Fatal(err)
	}
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

// TestFastModePersistsPerIdentity 验证快速模式默认关闭并随身份持久化恢复
func TestFastModePersistsPerIdentity(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.GetSettings().FastMode {
		t.Fatal("快速模式不应默认启用")
	}
	if _, err = s.SetFastMode(true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SelectAccount("independent", false); err != nil {
		t.Fatal(err)
	}
	if s.GetSettings().FastMode {
		t.Fatal("新身份继承了其他身份的快速模式")
	}
	if _, err = s.SelectAccount("", true); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService("test")
	if err != nil || !reopened.GetSettings().FastMode {
		t.Fatalf("快速模式未在身份切换和重载后恢复：%v", err)
	}
	if _, err = reopened.SetFastMode(false); err != nil {
		t.Fatal(err)
	}
	if reopened.GetSettings().AccountPreferences["cli"].FastMode {
		t.Fatal("关闭快速模式未更新身份偏好")
	}
}

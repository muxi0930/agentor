package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInitialPermission 验证未配置权限时使用项目内读写并保留已保存的显式权限
func TestInitialPermission(t *testing.T) {
	for _, test := range []struct {
		name string // 配置场景
		data string // 已有配置
		want string // 预期权限
	}{
		{"首次启动", "", "workspace-write"},
		{"未配置权限", `{}`, "workspace-write"},
		{"显式只读", `{"permissionMode":"read-only"}`, "read-only"},
		{"显式项目内读写", `{"permissionMode":"workspace-write"}`, "workspace-write"},
		{"显式完全访问", `{"permissionMode":"danger-full-access"}`, "danger-full-access"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("AGENTOR_DATA_DIR", root)
			t.Setenv("CODEX_HOME", t.TempDir())
			if test.data != "" {
				if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(test.data), 0600); err != nil {
					t.Fatal(err)
				}
			}

			s, err := NewService("test")
			if err != nil {
				t.Fatal(err)
			}
			if got := s.GetSettings().PermissionMode; got != test.want {
				t.Fatalf("默认权限 = %q，预期 %q", got, test.want)
			}
			if got := s.EffectivePermission("", ""); got != test.want {
				t.Fatalf("实际执行权限 = %q，预期 %q", got, test.want)
			}
		})
	}
}

// TestNewAccountPermission 验证新账号及预先选择模型的账号默认使用项目内读写
func TestNewAccountPermission(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAccountModel("with-model", "model"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"new-account", "with-model"} {
		settings, err := s.SelectAccount(id, false)
		if err != nil {
			t.Fatal(err)
		}
		if settings.PermissionMode != "workspace-write" {
			t.Fatalf("账号 %s 的默认权限 = %q", id, settings.PermissionMode)
		}
	}
}

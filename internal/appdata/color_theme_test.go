package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

// TestColorThemePersists 验证旧配置保留默认配色及各配色持久化且不改变明暗模式
func TestColorThemePersists(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOR_DATA_DIR", root)
	t.Setenv("CODEX_HOME", t.TempDir())
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"theme":"dark"}`), 0600); err != nil {
		t.Fatal(err)
	}

	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.GetSettings().ColorTheme != "default" {
		t.Fatal("旧配置未沿用默认配色")
	}
	for _, theme := range []string{"blue", "green", "purple", "orange", "rose", "default"} {
		if _, err := s.SetColorTheme(theme); err != nil {
			t.Fatal(err)
		}
		reopened, err := NewService("test")
		if err != nil {
			t.Fatal(err)
		}
		if settings := reopened.GetSettings(); settings.ColorTheme != theme || settings.Theme != "dark" {
			t.Fatalf("配色 %s 未恢复或改变了明暗模式：%+v", theme, settings)
		}
	}
	if _, err := s.SetColorTheme("unknown"); err == nil {
		t.Fatal("不应保存未知配色")
	}
	if _, err := s.SetTheme("light"); err != nil {
		t.Fatal(err)
	}
	if settings := s.GetSettings(); settings.ColorTheme != "default" || settings.Theme != "light" {
		t.Fatal("切换明暗模式改变了配色")
	}
}

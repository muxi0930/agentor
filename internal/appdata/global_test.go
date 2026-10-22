package appdata

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGlobalConfigStartup 验证首次生成、配置读取、环境覆盖和切换后的数据隔离
func TestGlobalConfigStartup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "system-config"))
	t.Setenv("AppData", filepath.Join(home, "system-config"))
	t.Setenv("AGENTOR_DATA_DIR", "")
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if s.root != filepath.Join(base, "agentor") {
		t.Fatalf("默认目录错误：%s", s.root)
	}
	path := filepath.Join(home, ".config", "agentor", "conf.yaml")
	if s.GetInfo().GlobalConfigPath != path {
		t.Fatal("配置文件位置错误")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "selected-data")
	if err := s.SetDataDirectory(target); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if reopened.root != target || reopened.GetSettings().Theme != "light" {
		t.Fatal("新目录未生效或意外迁移了旧设置")
	}
	if s.root != filepath.Join(base, "agentor") || s.GetSettings().Theme != "dark" {
		t.Fatal("保存配置改变了当前运行实例")
	}
	if _, err := os.Stat(filepath.Join(s.root, "settings.json")); err != nil {
		t.Fatal("旧数据不应被删除", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	override := t.TempDir()
	t.Setenv("AGENTOR_DATA_DIR", override)
	overridden, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if overridden.root != override || !overridden.GetInfo().DataDirectoryFromEnv {
		t.Fatal("环境变量未优先于配置文件")
	}
	if err := overridden.SetDataDirectory(target); err == nil {
		t.Fatal("环境覆盖时不应允许界面切换")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(raw) {
		t.Fatal("环境覆盖不应改写配置文件")
	}
}

// TestGlobalConfigInvalidYAML 验证配置损坏时报告错误而不覆盖文件，环境变量仍可覆盖
func TestGlobalConfigInvalidYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AGENTOR_DATA_DIR", "")
	path := filepath.Join(home, ".config", "agentor", "conf.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("AGENTOR_DATA_DIR: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveDataDirectory(); err == nil {
		t.Fatal("应报告 YAML 格式错误")
	}
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	if _, _, err := resolveDataDirectory(); err != nil {
		t.Fatal("环境覆盖不应读取损坏的 YAML", err)
	}
}

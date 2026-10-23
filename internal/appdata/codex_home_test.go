package appdata

import (
	"os"
	"path/filepath"
	"testing"

	appdatamodel "agentor/internal/appdata/model"
)

// TestExternalThreadPreferencePersists 验证默认关闭、持久化及独立于账号目录的会话读取设置
func TestExternalThreadPreferencePersists(t *testing.T) {
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.GetSettings().ReadExternalThreads {
		t.Fatal("初始设置意外开启外部会话")
	}
	before := s.GetSettings()
	settings, err := s.SetReadExternalThreads(true)
	if err != nil || !settings.ReadExternalThreads || settings.CodexHomeMode != before.CodexHomeMode || settings.ReuseCLI != before.ReuseCLI {
		t.Fatal("会话读取设置改写了账号或目录", err)
	}
	reopened, err := NewService("test")
	if err != nil || !reopened.GetSettings().ReadExternalThreads {
		t.Fatal("会话读取设置未持久化", err)
	}
}

// TestCodexHomeAndAccountSelectionsStayIndependent 验证账号和目录互不改写且重启后保持选择
func TestCodexHomeAndAccountSelectionsStayIndependent(t *testing.T) {
	root, systemHome, customHome := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("AGENTOR_DATA_DIR", root)
	t.Setenv("CODEX_HOME", systemHome)
	s, err := NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.GetInfo().CodexHome != systemHome || s.GetSettings().CodexHomeMode != "system" {
		t.Fatal("初始目录没有沿用系统默认值")
	}
	if _, err := s.SelectAccount("account-a", false); err != nil {
		t.Fatal(err)
	}
	if s.GetInfo().CodexHome != systemHome {
		t.Fatal("切换独立账号改变了数据目录")
	}
	if err := s.SetCodexHome("custom", customHome); err != nil {
		t.Fatal(err)
	}
	if settings := s.GetSettings(); settings.ActiveAccountID != "account-a" || settings.ReuseCLI {
		t.Fatal("切换目录改变了账号")
	}
	if _, err := s.SelectAccount("", true); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService("test")
	if err != nil || reopened.GetInfo().CodexHome != customHome || !reopened.GetSettings().ReuseCLI {
		t.Fatal("重启后未保持独立的目录与账号选择", err)
	}
	if err := reopened.SetCodexHome("system", ""); err != nil || reopened.GetInfo().CodexHome != systemHome {
		t.Fatal("无法返回系统默认目录", err)
	}
}

// TestLegacyAccountKeepsOriginalHome 验证已有独立账号目录不会迁移到系统目录
func TestLegacyAccountKeepsOriginalHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTOR_DATA_DIR", root)
	t.Setenv("CODEX_HOME", t.TempDir())
	if err := WriteJSON(filepath.Join(root, "settings.json"), appdatamodel.Settings{ActiveAccountID: "old-account"}); err != nil {
		t.Fatal(err)
	}
	s, err := NewService("test")
	if err != nil || s.GetInfo().CodexHome != filepath.Join(root, "codex") || s.GetSettings().CodexHomeMode != "custom" {
		t.Fatal("已有独立账号的数据目录发生变化", err)
	}
}

// TestCodexHomeValidationAndFallback 验证目录输入检查和未设置环境变量时的默认目录
func TestCodexHomeValidationAndFallback(t *testing.T) {
	s := &Service{}
	t.Setenv("CODEX_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := s.ResolveCodexHome("system", "")
	if err != nil || resolved != filepath.Join(home, ".codex") {
		t.Fatal("系统默认目录解析错误", err)
	}
	for _, input := range [][2]string{{"unknown", ""}, {"custom", ""}, {"custom", "relative/path"}} {
		if _, err := s.ResolveCodexHome(input[0], input[1]); err == nil {
			t.Fatalf("接受了无效目录选择：%v", input)
		}
	}
}

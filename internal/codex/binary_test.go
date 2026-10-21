package codex

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestResolveBinaryFromLoginShell 验证桌面 PATH 缺少 Codex 时读取登录 Shell 并传递搜索路径
func TestResolveBinaryFromLoginShell(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("仅 macOS 补充登录 Shell 环境")
	}
	directory := t.TempDir()
	native := filepath.Join(directory, "codex")
	if err := os.WriteFile(native, []byte{0xcf, 0xfa, 0xed, 0xfe}, 0700); err != nil {
		t.Fatal(err)
	}
	shell := filepath.Join(t.TempDir(), "shell")
	// 模拟启动脚本输出，确保提示信息不会混入 PATH
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nprintf 'shell startup output\\n'\nprintf '\\000%s\\000' \"$AGENTOR_TEST_SHELL_PATH\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	initialPath := t.TempDir()
	t.Setenv("PATH", initialPath)
	t.Setenv("SHELL", shell)
	t.Setenv("AGENTOR_TEST_SHELL_PATH", directory)
	path, searchPath, err := resolveBinary("")
	expected, expectedErr := filepath.EvalSymlinks(native)
	if err != nil || expectedErr != nil || path != expected || searchPath != directory {
		t.Fatalf("未从登录 Shell 找到 Codex：path=%q PATH=%q err=%v", path, searchPath, err)
	}
	if os.Getenv("PATH") != initialPath {
		t.Fatal("不应修改宿主进程的 PATH")
	}
	env := processEnvironment("/agentor-home", false, searchPath)
	if !slices.Contains(env, "PATH="+directory) || slices.Contains(env, "PATH="+initialPath) {
		t.Fatal("子进程未继承发现 Codex 时使用的 PATH")
	}
}

// TestResolveBinaryPrefersConfiguredPath 验证手动路径优先且不会调用登录 Shell
func TestResolveBinaryPrefersConfiguredPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codex.exe")
	if err := os.WriteFile(path, []byte("MZ00"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SHELL", filepath.Join(t.TempDir(), "missing-shell"))
	resolved, searchPath, err := resolveBinary(path)
	expected, expectedErr := filepath.EvalSymlinks(path)
	if err != nil || expectedErr != nil || resolved != expected || searchPath != os.Getenv("PATH") {
		t.Fatalf("手动路径未保持优先：path=%q err=%v", resolved, err)
	}
}

// TestNPMWindowsEntrypoints 验证 Windows 命令入口及嵌套、提升安装的原生平台包
func TestNPMWindowsEntrypoints(t *testing.T) {
	for _, extension := range []string{".cmd", ".bat", ".CMD", ".js"} {
		for _, nested := range []bool{false, true} {
			t.Run(extension+map[bool]string{true: "嵌套", false: "同级"}[nested], func(t *testing.T) {
				prefix := t.TempDir()
				packageRoot := filepath.Join(prefix, "node_modules", "@openai", "codex")
				entry := filepath.Join(prefix, "codex"+extension)
				if extension == ".js" {
					entry = filepath.Join(packageRoot, "bin", "codex.js")
				}
				modules := filepath.Join(prefix, "node_modules")
				if nested {
					modules = filepath.Join(packageRoot, "node_modules")
				}
				native := filepath.Join(modules, "@openai", "codex-windows-x64", "vendor", "x86_64-pc-windows-msvc", "bin", "codex.exe")
				if err := os.MkdirAll(filepath.Dir(native), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(native, []byte("MZ00"), 0700); err != nil {
					t.Fatal(err)
				}
				resolved, err := npmBinary(entry, "windows", "amd64")
				if err != nil || resolved != native {
					t.Fatalf("未定位 Windows 原生程序：path=%q err=%v", resolved, err)
				}
			})
		}
	}
}

// TestCLIAuthenticationDoesNotInheritHostTaskIdentity 验证沿用 CLI 认证时隔离宿主任务身份
func TestCLIAuthenticationDoesNotInheritHostTaskIdentity(t *testing.T) {
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "parent-app")
	t.Setenv("CODEX_THREAD_ID", "parent-thread")
	t.Setenv("CODEX_APP_TOOLS_PIPE_PATH", "/parent-tools")
	t.Setenv("OPENAI_API_KEY", "test-key")
	env := processEnvironment("/agentor-home", true, "/test-path")
	for _, entry := range env {
		if strings.HasPrefix(entry, "CODEX_INTERNAL_ORIGINATOR_OVERRIDE=") || strings.HasPrefix(entry, "CODEX_THREAD_ID=") || strings.HasPrefix(entry, "CODEX_APP_TOOLS_PIPE_PATH=") {
			t.Fatal("host identity leaked into child")
		}
	}

	if !slices.Contains(env, "CODEX_HOME=/agentor-home") || !slices.Contains(env, "OPENAI_API_KEY=test-key") {
		t.Fatal("CLI authentication configuration was lost")
	}
	if slices.Contains(processEnvironment("/agentor-home", false, "/test-path"), "OPENAI_API_KEY=test-key") {
		t.Fatal("independent configuration inherited CLI credentials")
	}
}

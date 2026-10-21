package codex

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// CheckPath 检测指定路径的原生程序能否运行，不改变当前连接
func (s *Service) CheckPath(path string) error {
	resolved, searchPath, err := resolveBinary(path)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, resolved, "--version")
	hideProcessWindow(command)
	command.Env = processEnvironment(s.data.GetInfo().CodexHome, s.data.GetSettings().ReuseCLI, searchPath)
	_, err = command.Output()
	return err
}

// resolveBinary 定位原生 Codex，并返回子进程应继承的搜索路径
func resolveBinary(path string) (string, string, error) {
	searchPath := os.Getenv("PATH")
	if path == "" {
		var err error
		path, err = exec.LookPath("codex")
		if err != nil && runtime.GOOS == "darwin" {
			searchPath, err = loginShellPath()
			if err != nil {
				return "", "", fmt.Errorf("无法读取登录 Shell 的 PATH，请在设置中选择原生程序: %w", err)
			}
			for _, directory := range filepath.SplitList(searchPath) {
				path, err = exec.LookPath(filepath.Join(directory, "codex"))
				if err == nil {
					break
				}
			}
		}
		if err != nil || path == "" {
			return "", "", errors.New("未在 PATH 中找到 Codex，请在设置中选择原生程序")
		}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", err
	}
	extension := strings.ToLower(filepath.Ext(resolved))
	if extension == ".js" || extension == ".cmd" || extension == ".bat" {
		resolved, err = npmBinary(resolved, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return "", "", err
		}
	}
	file, err := os.Open(resolved)
	if err != nil {
		return "", "", fmt.Errorf("未找到原生 Codex，请手动选择可执行文件: %w", err)
	}

	defer file.Close()

	var header [4]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return "", "", err
	}

	magic := binary.BigEndian.Uint32(header[:])
	native := string(header[:]) == "\x7fELF" || string(header[:2]) == "MZ" || magic == 0xfeedfacf || magic == 0xcffaedfe || magic == 0xcafebabe || magic == 0xbebafeca
	if !native {
		return "", "", errors.New("请选择 Codex 原生可执行文件，不支持 Node、Shell 或其他脚本入口")
	}

	resolved, err = filepath.Abs(resolved)
	return resolved, searchPath, err
}

// loginShellPath 读取用户登录交互式 Shell 的 PATH，忽略启动脚本的普通输出
func loginShellPath() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, shell, "-ilc", `printf '\000%s\000' "$PATH"`)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(output), "\x00")
	if len(parts) < 3 || parts[len(parts)-2] == "" {
		return "", errors.New("登录 Shell 未返回 PATH")
	}
	return parts[len(parts)-2], nil
}

// npmBinary 将 npm 的 JavaScript 或 Windows 命令入口解析到对应平台的原生程序
func npmBinary(path, platform, architecture string) (string, error) {
	packageRoot := filepath.Dir(filepath.Dir(path))
	if extension := strings.ToLower(filepath.Ext(path)); extension == ".cmd" || extension == ".bat" {
		packageRoot = filepath.Join(filepath.Dir(path), "node_modules", "@openai", "codex")
	}
	triplets := map[string]string{
		"darwin/arm64": "aarch64-apple-darwin", "darwin/amd64": "x86_64-apple-darwin",
		"linux/arm64": "aarch64-unknown-linux-musl", "linux/amd64": "x86_64-unknown-linux-musl",
		"windows/arm64": "aarch64-pc-windows-msvc", "windows/amd64": "x86_64-pc-windows-msvc",
	}
	arch := architecture
	if arch == "amd64" {
		arch = "x64"
	}
	name := "codex"
	if platform == "windows" {
		name += ".exe"
	}
	relative := filepath.Join("@openai", "codex-"+platform+"-"+arch, "vendor", triplets[platform+"/"+architecture], "bin", name)
	// npm 的平台包可能嵌套安装，也可能提升到同级 node_modules
	for _, modules := range []string{filepath.Join(packageRoot, "node_modules"), filepath.Dir(filepath.Dir(packageRoot))} {
		native := filepath.Join(modules, relative)
		if _, err := os.Stat(native); err == nil {
			return native, nil
		}
	}
	return "", errors.New("未找到 npm 包内的 Codex 原生程序，请在设置中选择原生程序")
}

// processEnvironment 按身份隔离凭据并排除宿主线程与工具管道
func processEnvironment(home string, reuseCLI bool, searchPath string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PATH") || key == "AGENTOR_ACCOUNT_API_KEY" {
			continue
		}
		// 沿用 CLI 认证时，仍隔离宿主线程身份、沙盒标识与工具管道
		if strings.HasPrefix(key, "CODEX_") && key != "CODEX_API_KEY" && key != "CODEX_AUTH_JSON" {
			continue
		}
		if !reuseCLI && (key == "OPENAI_API_KEY" || key == "OPENAI_BASE_URL" || key == "CODEX_API_KEY" || key == "CODEX_AUTH_JSON") {
			continue
		}
		env = append(env, entry)
	}

	return append(env, "PATH="+searchPath, "CODEX_HOME="+home)
}

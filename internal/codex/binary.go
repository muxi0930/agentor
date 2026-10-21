package codex

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveBinary 定位并检查原生 Codex 可执行文件
func resolveBinary(path string) (string, error) {
	if path == "" {
		var err error
		path, err = exec.LookPath("codex")
		if err != nil {
			return "", errors.New("未在 PATH 中找到 Codex，请在设置中选择原生程序")
		}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(resolved), ".js") {
		// 官方 npm 包包含原生程序，定位后直接运行原生文件
		triplets := map[string]string{
			"darwin/arm64":  "aarch64-apple-darwin",
			"darwin/amd64":  "x86_64-apple-darwin",
			"linux/arm64":   "aarch64-unknown-linux-musl",
			"linux/amd64":   "x86_64-unknown-linux-musl",
			"windows/arm64": "aarch64-pc-windows-msvc",
			"windows/amd64": "x86_64-pc-windows-msvc",
		}
		target := triplets[runtime.GOOS+"/"+runtime.GOARCH]
		arch := runtime.GOARCH
		if arch == "amd64" {
			arch = "x64"
		}
		name := "codex"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		resolved = filepath.Join(filepath.Dir(filepath.Dir(resolved)), "node_modules", "@openai", "codex-"+runtime.GOOS+"-"+arch, "vendor", target, "bin", name)
	}
	file, err := os.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("未找到原生 Codex，请手动选择可执行文件: %w", err)
	}

	defer file.Close()

	var header [4]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return "", err
	}

	magic := binary.BigEndian.Uint32(header[:])
	native := string(header[:]) == "\x7fELF" || string(header[:2]) == "MZ" || magic == 0xfeedfacf || magic == 0xcffaedfe || magic == 0xcafebabe || magic == 0xbebafeca
	if !native {
		return "", errors.New("请选择 Codex 原生可执行文件，不支持 Node、Shell 或其他脚本入口")
	}

	return filepath.Abs(resolved)
}

// processEnvironment 按身份隔离凭据并排除宿主线程与工具管道
func processEnvironment(home string, reuseCLI bool) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key == "AGENTOR_ACCOUNT_API_KEY" {
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

	return append(env, "CODEX_HOME="+home)
}

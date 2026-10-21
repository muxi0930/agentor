package appdata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ResolveCodexHome 解析独立于账号选择的 Codex 数据目录
//
//wails:ignore
func (s *Service) ResolveCodexHome(mode, directory string) (string, error) {
	switch mode {
	case "system":
		directory = os.Getenv("CODEX_HOME")
		if directory == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			directory = filepath.Join(home, ".codex")
		}
	case "custom":
		directory = strings.TrimSpace(directory)
		if directory == "" || !filepath.IsAbs(directory) {
			return "", errors.New("请选择自定义 CODEX_HOME 的绝对路径")
		}
	default:
		return "", errors.New("未知的 CODEX_HOME 模式")
	}
	return filepath.Abs(directory)
}

// SetCodexHome 在原运行时关闭后保存目录选择，保留账号与项目设置
//
//wails:ignore
func (s *Service) SetCodexHome(mode, directory string) error {
	home, err := s.ResolveCodexHome(mode, directory)
	if err != nil {
		return err
	}
	_, err = s.update(func(settings *Settings) {
		settings.CodexHomeMode = mode
		if mode == "custom" {
			settings.CodexHomePath = home
		}
	})
	return err
}

// SetReadExternalThreads 保存会话读取范围，不改变目录与账号选择
func (s *Service) SetReadExternalThreads(enabled bool) (Settings, error) {
	return s.update(func(settings *Settings) { settings.ReadExternalThreads = enabled })
}

// RememberExternalThread 保存外部会话目录并保留其来源，避免打开后被当成本应用创建
//
//wails:ignore
func (s *Service) RememberExternalThread(threadID, cwd string) error {
	_, err := s.update(func(settings *Settings) {
		threads := make(map[string]string, len(settings.ChatThreads)+1)
		for id, directory := range settings.ChatThreads {
			threads[id] = directory
		}
		threads[threadID] = cwd
		settings.ChatThreads = threads
		external := make(map[string]bool, len(settings.ExternalThreads)+1)
		for id, value := range settings.ExternalThreads {
			external[id] = value
		}
		external[threadID] = true
		settings.ExternalThreads = external
	})
	return err
}

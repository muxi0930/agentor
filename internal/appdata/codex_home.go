package appdata

import (
	"os"
	"path/filepath"
	"strings"

	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/constant"
)

// ResolveCodexHome 解析独立于账号选择的 Codex 数据目录
//
//wails:ignore
func (s *Service) ResolveCodexHome(mode, directory string) (string, error) {
	switch mode {
	case constant.AppDataCodexHomeSystem:
		directory = os.Getenv(constant.AppDataCodexHomeEnvironment)
		if directory == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			directory = filepath.Join(home, ".codex")
		}
	case constant.AppDataCodexHomeCustom:
		directory = strings.TrimSpace(directory)
		if directory == "" || !filepath.IsAbs(directory) {
			return "", constant.ErrAppDataCodexHomePathInvalid
		}
	default:
		return "", constant.ErrAppDataCodexHomeModeUnknown
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
	_, err = s.update(func(settings *appdatamodel.Settings) {
		settings.CodexHomeMode = mode
		if mode == constant.AppDataCodexHomeCustom {
			settings.CodexHomePath = home
		}
	})
	return err
}

// SetReadExternalThreads 保存会话读取范围，不改变目录与账号选择
func (s *Service) SetReadExternalThreads(enabled bool) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) { settings.ReadExternalThreads = enabled })
}

// RememberExternalThread 保存外部会话目录并保留其来源，避免打开后被当成本应用创建
//
//wails:ignore
func (s *Service) RememberExternalThread(threadID, cwd string) error {
	_, err := s.update(func(settings *appdatamodel.Settings) {
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

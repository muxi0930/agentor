package appdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type globalConfig struct {
	DataDirectory string `yaml:"AGENTOR_DATA_DIR"` // 应用数据根目录
}

// SetDataDirectory 保存下次启动使用的数据目录，不改变当前服务的目录
//
//wails:ignore
func (s *Service) SetDataDirectory(directory string) error {
	if os.Getenv("AGENTOR_DATA_DIR") != "" {
		return errors.New("当前数据目录由 AGENTOR_DATA_DIR 环境变量指定，请先移除该变量再切换目录")
	}
	directory = strings.TrimSpace(directory)
	if directory == "" || !filepath.IsAbs(directory) {
		return errors.New("请选择应用数据根目录的绝对路径")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	return writeGlobalConfig(s.configPath, globalConfig{DataDirectory: filepath.Clean(directory)})
}

// resolveDataDirectory 按环境变量、固定配置文件和系统默认目录的顺序确定数据目录
func resolveDataDirectory() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	path := filepath.Join(home, ".config", "agentor", "conf.yaml")
	if directory := os.Getenv("AGENTOR_DATA_DIR"); directory != "" {
		return directory, path, nil
	}

	var config globalConfig
	raw, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return "", "", err
	}
	if !missing {
		if err := yaml.Unmarshal(raw, &config); err != nil {
			return "", "", fmt.Errorf("读取 %s: %w", path, err)
		}
	}
	if config.DataDirectory == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", "", err
		}
		config.DataDirectory = filepath.Join(base, "agentor")
	}
	if missing {
		if err := writeGlobalConfig(path, config); err != nil {
			return "", "", err
		}
	}
	return config.DataDirectory, path, nil
}

// writeGlobalConfig 使用 YAML 保存独立于数据目录的启动配置
func writeGlobalConfig(path string, config globalConfig) error {
	raw, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0600)
}

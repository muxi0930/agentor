package appdata

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/constant"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// API 按职责列出服务的全部公开方法
type API interface {
	GetInfo() appdatamodel.Info                                                                 // 返回当前应用路径和运行平台
	GetSettings() appdatamodel.Settings                                                         // 读取应用设置快照
	SetAgentTimeoutSeconds(seconds int) (appdatamodel.Settings, error)                          // 保存功能子代理最大执行时间
	SetMaxConcurrentAgents(limit int) (appdatamodel.Settings, error)                            // 保存工作队列功能任务的并发上限
	SetTheme(theme string) (appdatamodel.Settings, error)                                       // 保存浅色或深色主题
	SetColorTheme(theme string) (appdatamodel.Settings, error)                                  // 保存界面配色方案
	SetChatWidth(width string) (appdatamodel.Settings, error)                                   // 保存聊天消息区域宽度
	SetMode(mode string) (appdatamodel.Settings, error)                                         // 保存工作模式
	SetProjectDirectory(path string) (appdatamodel.Settings, error)                             // 检查并保存当前项目目录
	SetCodexPath(path string) (appdatamodel.Settings, error)                                    // 保存原生 Codex 程序路径
	SetModelSelection(model, effort string) (appdatamodel.Settings, error)                      // 保存当前身份的模型和推理强度
	SetFastMode(enabled bool) (appdatamodel.Settings, error)                                    // 保存当前身份的快速模式偏好
	SetLastThread(threadID string) (appdatamodel.Settings, error)                               // 保存最近打开的会话
	RememberThread(threadID, cwd string) (appdatamodel.Settings, error)                         // 登记由本应用打开的普通会话
	RememberManagedThread(threadID, cwd, role string) error                                     // 登记业务角色专用线程
	SetProjectAlias(cwd, name string) (appdatamodel.Settings, error)                            // 保存或清除项目分组显示名称
	HideProject(cwd string) (appdatamodel.Settings, error)                                      // 隐藏项目分组而不删除目录
	ChooseDirectory() (string, error)                                                           // 通过原生对话框选择项目目录
	ChooseFile() (string, error)                                                                // 通过原生对话框选择普通文件
	OpenDirectory(path string) error                                                            // 在系统文件管理器打开目录
	OpenFile(path string) error                                                                 // 通过系统应用打开文件
	OpenURL(url string) error                                                                   // 通过系统浏览器打开链接
	SetAccountModel(id, model string) (appdatamodel.Settings, error)                            // 保存账号模型，在更换模型时清除旧推理强度
	SelectAccount(id string, reuseCLI bool) (appdatamodel.Settings, error)                      // 切换独立账号或 CLI 接入并恢复各自偏好
	ToggleThreadPin(threadID string) (appdatamodel.Settings, error)                             // 切换会话的置顶状态
	SetChatOrder(cwd string, ids []string) (appdatamodel.Settings, error)                       // 保存项目或项目内会话的显示顺序
	CreateWorktree(cwd, branch, directory string) (appdatamodel.Settings, error)                // 基于当前提交创建分支工作树并选中其目录
	ResolveCodexHome(mode, directory string) (string, error)                                    // 解析独立于账号选择的 Codex 数据目录
	SetCodexHome(mode, directory string) error                                                  // 在原运行时关闭后保存目录选择，保留账号与项目设置
	SetReadExternalThreads(enabled bool) (appdatamodel.Settings, error)                         // 保存会话读取范围，不改变目录与账号选择
	RememberExternalThread(threadID, cwd string) error                                          // 保存外部会话目录并保留其来源，避免打开后被当成本应用创建
	SetControllerSettings(input appdatamodel.ControllerSettings) (appdatamodel.Settings, error) // 保存控制中心提示词与执行者选择范围
	SetDataDirectory(directory string) error                                                    // 保存下次启动使用的数据目录，不改变当前服务的目录
	SetPermission(scope, id, mode string) (appdatamodel.Settings, error)                        // 保存账号默认权限或项目会话覆盖值
	EffectivePermission(threadID, cwd string) string                                            // 按会话项目账号的顺序取得最终权限
}

var _ API = (*Service)(nil)

type Service struct {
	configPath string                // 全局启动配置文件
	mu         sync.Mutex            // 设置读写锁
	root       string                // 应用数据目录
	version    string                // 由主程序传入的应用版本
	settings   appdatamodel.Settings // 当前设置
}

// NewService 初始化服务并读取本地记录
func NewService(version string) (*Service, error) {
	root, configPath, err := resolveDataDirectory()
	if err != nil {
		return nil, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	s := &Service{root: root, configPath: configPath, version: version, settings: appdatamodel.Settings{AgentTimeoutSeconds: constant.AppDataDefaultAgentTimeoutSeconds, Controller: appdatamodel.ControllerSettings{Models: []string{""}, Efforts: []string{""}, Prompt: constant.AppDataDefaultControllerPrompt}, Theme: constant.AppDataThemeLight, ColorTheme: constant.AppDataColorThemeDefault, ChatWidth: constant.AppDataChatWidthStandard, Mode: constant.AppDataModeChat, ReuseCLI: true, PermissionMode: constant.AppDataPermissionWorkspaceWrite}}
	if err := ReadJSON(filepath.Join(root, constant.AppDataSettingsFilename), &s.settings); err != nil {
		return nil, err
	}
	if s.settings.MaxConcurrentAgents < 1 || s.settings.MaxConcurrentAgents > constant.AppDataMaxConcurrentAgents {
		s.settings.MaxConcurrentAgents = constant.AppDataDefaultConcurrentAgents
	}
	// 已有独立账号继续使用原目录，之后目录选择不再随账号变化
	if s.settings.CodexHomeMode == "" {
		s.settings.CodexHomeMode = constant.AppDataCodexHomeSystem
		if !s.settings.ReuseCLI {
			s.settings.CodexHomeMode = constant.AppDataCodexHomeCustom
			s.settings.CodexHomePath = filepath.Join(root, "codex")
		}
	}
	home, err := s.ResolveCodexHome(s.settings.CodexHomeMode, s.settings.CodexHomePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return nil, err
	}
	return s, nil
}

// GetInfo 返回当前应用路径和运行平台
func (s *Service) GetInfo() appdatamodel.Info {
	settings := s.GetSettings()
	home, _ := s.ResolveCodexHome(settings.CodexHomeMode, settings.CodexHomePath)
	systemHome, _ := s.ResolveCodexHome(constant.AppDataCodexHomeSystem, "")
	return appdatamodel.Info{
		GlobalConfigPath:     s.configPath,
		DataDirectoryFromEnv: os.Getenv(constant.AppDataDirectoryEnvironment) != "",
		Name:                 constant.AppDataApplicationName,
		Version:              s.version,
		GoVersion:            runtime.Version(),
		OS:                   runtime.GOOS,
		Arch:                 runtime.GOARCH,
		DataDirectory:        s.root,
		CodexHome:            home,
		SystemCodexHome:      systemHome,
	}
}

// GetSettings 读取应用设置快照
func (s *Service) GetSettings() appdatamodel.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.settings
}

// SetAgentTimeoutSeconds 保存功能子代理最大执行时间
func (s *Service) SetAgentTimeoutSeconds(seconds int) (appdatamodel.Settings, error) {
	if seconds < 0 || int64(seconds) > int64((1<<63-1)/1000000000) {
		return appdatamodel.Settings{}, constant.ErrAppDataAgentTimeoutInvalid
	}

	return s.update(func(settings *appdatamodel.Settings) { settings.AgentTimeoutSeconds = seconds })
}

// SetMaxConcurrentAgents 保存工作队列功能任务的并发上限
func (s *Service) SetMaxConcurrentAgents(limit int) (appdatamodel.Settings, error) {
	if limit < 1 || limit > constant.AppDataMaxConcurrentAgents {
		return appdatamodel.Settings{}, constant.ErrAppDataConcurrencyLimitInvalid
	}

	return s.update(func(settings *appdatamodel.Settings) { settings.MaxConcurrentAgents = limit })
}

// SetTheme 保存浅色或深色主题
func (s *Service) SetTheme(theme string) (appdatamodel.Settings, error) {
	if theme != constant.AppDataThemeLight && theme != constant.AppDataThemeDark {
		return appdatamodel.Settings{}, constant.ErrAppDataThemeInvalid
	}

	return s.update(func(settings *appdatamodel.Settings) { settings.Theme = theme })
}

// SetColorTheme 保存界面配色方案
func (s *Service) SetColorTheme(theme string) (appdatamodel.Settings, error) {
	switch theme {
	case constant.AppDataColorThemeDefault, constant.AppDataColorThemeBlue, constant.AppDataColorThemeGreen, constant.AppDataColorThemePurple, constant.AppDataColorThemeOrange, constant.AppDataColorThemeRose:
		return s.update(func(settings *appdatamodel.Settings) { settings.ColorTheme = theme })
	default:
		return appdatamodel.Settings{}, constant.ErrAppDataColorThemeInvalid
	}
}

// SetChatWidth 保存聊天消息区域宽度
func (s *Service) SetChatWidth(width string) (appdatamodel.Settings, error) {
	if width != constant.AppDataChatWidthStandard && width != constant.AppDataChatWidthWide {
		return appdatamodel.Settings{}, constant.ErrAppDataChatWidthInvalid
	}

	return s.update(func(settings *appdatamodel.Settings) { settings.ChatWidth = width })
}

// SetMode 保存工作模式
func (s *Service) SetMode(mode string) (appdatamodel.Settings, error) {
	if mode != constant.AppDataModeChat && mode != constant.AppDataModeWorkspace {
		return appdatamodel.Settings{}, constant.ErrAppDataModeUnknown
	}

	return s.update(func(settings *appdatamodel.Settings) { settings.Mode = mode })
}

// SetProjectDirectory 检查并保存当前项目目录
func (s *Service) SetProjectDirectory(path string) (appdatamodel.Settings, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return appdatamodel.Settings{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return appdatamodel.Settings{}, err
	}
	if !info.IsDir() {
		return appdatamodel.Settings{}, constant.ErrAppDataProjectDirectoryRequired
	}

	return s.update(func(settings *appdatamodel.Settings) {
		settings.ProjectDirectory = path

		hidden := make([]string, 0, len(settings.HiddenProjects))
		for _, existing := range settings.HiddenProjects {
			if existing != path {
				hidden = append(hidden, existing)
			}
		}
		settings.HiddenProjects = hidden
	})
}

// SetCodexPath 保存原生 Codex 程序路径
func (s *Service) SetCodexPath(path string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) { settings.CodexPath = path })
}

// SetModelSelection 保存当前身份的模型和推理强度
func (s *Service) SetModelSelection(model, effort string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		settings.Model, settings.Effort = model, effort
		rememberPreferences(settings)
	})
}

// SetFastMode 保存当前身份的快速模式偏好
func (s *Service) SetFastMode(enabled bool) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		settings.FastMode = enabled
		rememberPreferences(settings)
	})
}

// SetLastThread 保存最近打开的会话
func (s *Service) SetLastThread(threadID string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) { settings.LastThreadID = threadID })
}

// RememberThread 登记由本应用打开的普通会话
func (s *Service) RememberThread(threadID, cwd string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		threads := make(map[string]string, len(settings.ChatThreads)+1)
		for id, path := range settings.ChatThreads {
			threads[id] = path
		}
		threads[threadID] = cwd
		settings.ChatThreads = threads
	})
}

// RememberManagedThread 登记业务角色专用线程
//
//wails:ignore
func (s *Service) RememberManagedThread(threadID, cwd, role string) error {
	_, err := s.update(func(settings *appdatamodel.Settings) {
		threads := map[string]string{}
		for id, path := range settings.ChatThreads {
			threads[id] = path
		}
		threads[threadID] = cwd
		settings.ChatThreads = threads

		managed := map[string]string{}
		for id, value := range settings.ManagedThreads {
			managed[id] = value
		}
		managed[threadID] = role
		settings.ManagedThreads = managed
	})
	return err
}

// SetProjectAlias 保存或清除项目分组显示名称
func (s *Service) SetProjectAlias(cwd, name string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		aliases := make(map[string]string, len(settings.ProjectAliases)+1)
		for path, alias := range settings.ProjectAliases {
			aliases[path] = alias
		}
		if name == "" {
			delete(aliases, cwd)
		} else {
			aliases[cwd] = name
		}
		settings.ProjectAliases = aliases
	})
}

// HideProject 隐藏项目分组而不删除目录
func (s *Service) HideProject(cwd string) (appdatamodel.Settings, error) {
	return s.update(func(settings *appdatamodel.Settings) {
		for _, path := range settings.HiddenProjects {
			if path == cwd {
				return
			}
		}
		settings.HiddenProjects = append(append([]string{}, settings.HiddenProjects...), cwd)
	})
}

// ChooseDirectory 通过原生对话框选择项目目录
func (s *Service) ChooseDirectory() (string, error) {
	app := application.Get()
	return app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle(constant.AppDataChooseProjectDirectoryTitle).CanChooseFiles(false).CanChooseDirectories(true).PromptForSingleSelection()
}

// ChooseFile 通过原生对话框选择普通文件
func (s *Service) ChooseFile() (string, error) {
	app := application.Get()
	return app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle(constant.AppDataChooseFileTitle).CanChooseFiles(true).CanChooseDirectories(false).PromptForSingleSelection()
}

// OpenDirectory 在系统文件管理器打开目录
func (s *Service) OpenDirectory(path string) error {
	return application.Get().Env.OpenFileManager(path, false)
}

// OpenFile 通过系统应用打开文件
func (s *Service) OpenFile(path string) error {
	return application.Get().Browser.OpenFile(filepath.Clean(path))
}

// OpenURL 通过系统浏览器打开链接
func (s *Service) OpenURL(url string) error {
	return application.Get().Browser.OpenURL(url)
}

// update 写入新设置后替换当前状态
func (s *Service) update(change func(*appdatamodel.Settings)) (appdatamodel.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.settings
	change(&next)
	if err := WriteJSON(filepath.Join(s.root, constant.AppDataSettingsFilename), next); err != nil {
		return s.settings, err
	}
	s.settings = next
	return next, nil
}

package appdata

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Settings struct {
	AgentTimeoutSeconds int                           `json:"agentTimeoutSeconds"` // 功能子代理最大执行秒数，零表示不限时
	MaxConcurrentAgents int                           `json:"maxConcurrentAgents"` // 工作队列功能任务的最大并发数
	Controller          ControllerSettings            `json:"controller"`          // 控制中心配置
	ActiveAccountID     string                        `json:"activeAccountId"`     // 当前独立账号，空值表示尚未选择
	AccountPreferences  map[string]AccountPreferences `json:"accountPreferences"`  // 每个连接身份的模型与权限偏好
	Theme               string                        `json:"theme"`               // 界面主题
	Mode                string                        `json:"mode"`                // 当前工作模式
	ProjectDirectory    string                        `json:"projectDirectory"`    // 当前项目目录
	CodexPath           string                        `json:"codexPath"`           // 原生 Codex 程序路径
	CodexHomeMode       string                        `json:"codexHomeMode"`       // Codex 数据目录模式：system 或 custom
	CodexHomePath       string                        `json:"codexHomePath"`       // 自定义 Codex 数据目录
	ReadExternalThreads bool                          `json:"readExternalThreads"` // 是否读取非本应用创建的会话
	ExternalThreads     map[string]bool               `json:"externalThreads"`     // 在本应用打开过的外部会话
	ReuseCLI            bool                          `json:"reuseCLI"`            // 是否沿用 CLI 配置
	Model               string                        `json:"model"`               // 当前模型
	Effort              string                        `json:"effort"`              // 当前推理强度
	LastThreadID        string                        `json:"lastThreadId"`        // 最近打开的会话
	ChatThreads         map[string]string             `json:"chatThreads"`         // 本应用登记的会话及其目录
	ProjectAliases      map[string]string             `json:"projectAliases"`      // 项目目录的显示别名
	HiddenProjects      []string                      `json:"hiddenProjects"`      // 已隐藏的项目目录
	PermissionMode      string                        `json:"permissionMode"`      // 当前账号的默认权限
	ProjectPermissions  map[string]string             `json:"projectPermissions"`  // 项目权限覆盖值
	ThreadPermissions   map[string]string             `json:"threadPermissions"`   // 会话权限覆盖值
	PinnedThreads       map[string]bool               `json:"pinnedThreads"`       // 已置顶的会话
	ChatOrder           map[string][]string           `json:"chatOrder"`           // 各项目内的会话顺序
	ProjectOrder        []string                      `json:"projectOrder"`        // 项目显示顺序
	ManagedThreads      map[string]string             `json:"managedThreads"`      // 业务专用会话及其角色
}

type Info struct {
	Name                string `json:"name"`                // 应用名称
	Version             string `json:"version"`             // 应用版本
	GoVersion           string `json:"goVersion"`           // Go 运行时版本
	OS                  string `json:"os"`                  // 操作系统
	Arch                string `json:"arch"`                // 处理器架构
	DataDirectory       string `json:"dataDirectory"`       // 应用数据目录
	ControllerDirectory string `json:"controllerDirectory"` // 控制中心工作目录
	CodexHome           string `json:"codexHome"`           // 本应用的 Codex 数据目录
	SystemCodexHome     string `json:"systemCodexHome"`     // 系统默认 Codex 数据目录
}

type Service struct {
	mu       sync.Mutex // 设置读写锁
	root     string     // 应用数据目录
	version  string     // 由主程序传入的应用版本
	settings Settings   // 当前设置
}

// NewService 初始化服务并读取本地记录
func NewService(version string) (*Service, error) {
	root := os.Getenv("AGENTOR_DATA_DIR")
	if root == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		root = filepath.Join(base, "agentor")
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "control_session"), 0700); err != nil {
		return nil, err
	}
	s := &Service{root: root, version: version, settings: Settings{AgentTimeoutSeconds: 7200, Controller: ControllerSettings{Models: []string{""}, Efforts: []string{""}, Prompt: DefaultControllerPrompt}, Theme: "light", Mode: "chat", ReuseCLI: true, PermissionMode: "workspace-write"}}
	if err := ReadJSON(filepath.Join(root, "settings.json"), &s.settings); err != nil {
		return nil, err
	}
	if s.settings.MaxConcurrentAgents < 1 || s.settings.MaxConcurrentAgents > 50 {
		s.settings.MaxConcurrentAgents = 5
	}
	// 已有独立账号继续使用原目录，之后目录选择不再随账号变化
	if s.settings.CodexHomeMode == "" {
		s.settings.CodexHomeMode = "system"
		if !s.settings.ReuseCLI {
			s.settings.CodexHomeMode = "custom"
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
func (s *Service) GetInfo() Info {
	settings := s.GetSettings()
	home, _ := s.ResolveCodexHome(settings.CodexHomeMode, settings.CodexHomePath)
	systemHome, _ := s.ResolveCodexHome("system", "")
	return Info{
		Name:                "agentor",
		Version:             s.version,
		GoVersion:           runtime.Version(),
		OS:                  runtime.GOOS,
		Arch:                runtime.GOARCH,
		DataDirectory:       s.root,
		ControllerDirectory: filepath.Join(s.root, "control_session"),
		CodexHome:           home,
		SystemCodexHome:     systemHome,
	}
}

// GetSettings 读取应用设置快照
func (s *Service) GetSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.settings
}

// SetAgentTimeoutSeconds 保存功能子代理最大执行时间
func (s *Service) SetAgentTimeoutSeconds(seconds int) (Settings, error) {
	if seconds < 0 || int64(seconds) > int64((1<<63-1)/1000000000) {
		return Settings{}, errors.New("最大执行时间必须为非负整数且不能超过计时范围")
	}

	return s.update(func(settings *Settings) { settings.AgentTimeoutSeconds = seconds })
}

// SetMaxConcurrentAgents 保存工作队列功能任务的并发上限
func (s *Service) SetMaxConcurrentAgents(limit int) (Settings, error) {
	if limit < 1 || limit > 50 {
		return Settings{}, errors.New("最大并发 agent 数必须为 1–50 的整数")
	}

	return s.update(func(settings *Settings) { settings.MaxConcurrentAgents = limit })
}

// SetTheme 保存浅色或深色主题
func (s *Service) SetTheme(theme string) (Settings, error) {
	if theme != "light" && theme != "dark" {
		return Settings{}, errors.New("请选择浅色或深色主题")
	}

	return s.update(func(settings *Settings) { settings.Theme = theme })
}

// SetMode 保存工作模式
func (s *Service) SetMode(mode string) (Settings, error) {
	if mode != "chat" && mode != "workspace" {
		return Settings{}, errors.New("未知工作模式")
	}

	return s.update(func(settings *Settings) { settings.Mode = mode })
}

// SetProjectDirectory 检查并保存当前项目目录
func (s *Service) SetProjectDirectory(path string) (Settings, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Settings{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Settings{}, err
	}
	if !info.IsDir() {
		return Settings{}, errors.New("请选择项目目录")
	}

	return s.update(func(settings *Settings) {
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
func (s *Service) SetCodexPath(path string) (Settings, error) {
	return s.update(func(settings *Settings) { settings.CodexPath = path })
}

// SetModelSelection 保存当前身份的模型和推理强度
func (s *Service) SetModelSelection(model, effort string) (Settings, error) {
	return s.update(func(settings *Settings) {
		settings.Model, settings.Effort = model, effort
		rememberPreferences(settings)
	})
}

// SetLastThread 保存最近打开的会话
func (s *Service) SetLastThread(threadID string) (Settings, error) {
	return s.update(func(settings *Settings) { settings.LastThreadID = threadID })
}

// RememberThread 登记由本应用打开的普通会话
func (s *Service) RememberThread(threadID, cwd string) (Settings, error) {
	return s.update(func(settings *Settings) {
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
	_, err := s.update(func(settings *Settings) {
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
func (s *Service) SetProjectAlias(cwd, name string) (Settings, error) {
	return s.update(func(settings *Settings) {
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
func (s *Service) HideProject(cwd string) (Settings, error) {
	return s.update(func(settings *Settings) {
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
	return app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle("选择项目目录").CanChooseFiles(false).CanChooseDirectories(true).PromptForSingleSelection()
}

// ChooseFile 通过原生对话框选择普通文件
func (s *Service) ChooseFile() (string, error) {
	app := application.Get()
	return app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle("选择文件").CanChooseFiles(true).CanChooseDirectories(false).PromptForSingleSelection()
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
func (s *Service) update(change func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.settings
	change(&next)
	if err := WriteJSON(filepath.Join(s.root, "settings.json"), next); err != nil {
		return s.settings, err
	}
	s.settings = next
	return next, nil
}

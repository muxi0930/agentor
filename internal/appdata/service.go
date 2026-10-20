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
	ActiveAccountID    string                        `json:"activeAccountId"`    // 当前独立账号，空值表示尚未选择
	AccountPreferences map[string]AccountPreferences `json:"accountPreferences"` // 每个连接身份的模型与权限偏好
	Theme              string                        `json:"theme"`
	Mode               string                        `json:"mode"`
	ProjectDirectory   string                        `json:"projectDirectory"`
	CodexPath          string                        `json:"codexPath"`
	ReuseCLI           bool                          `json:"reuseCLI"`
	Model              string                        `json:"model"`
	Effort             string                        `json:"effort"`
	LastThreadID       string                        `json:"lastThreadId"`
	ChatThreads        map[string]string             `json:"chatThreads"`
	ProjectAliases     map[string]string             `json:"projectAliases"`
	HiddenProjects     []string                      `json:"hiddenProjects"`
	PermissionMode     string                        `json:"permissionMode"`
	ProjectPermissions map[string]string             `json:"projectPermissions"`
	ThreadPermissions  map[string]string             `json:"threadPermissions"`
	PinnedThreads      map[string]bool               `json:"pinnedThreads"`
	ChatOrder          map[string][]string           `json:"chatOrder"`
	ProjectOrder       []string                      `json:"projectOrder"`
	ManagedThreads     map[string]string             `json:"managedThreads"`
}

type Info struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	GoVersion     string `json:"goVersion"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	DataDirectory string `json:"dataDirectory"`
	CodexHome     string `json:"codexHome"`
}

type Service struct {
	mu       sync.Mutex
	root     string
	settings Settings
}

// NewService 初始化服务并读取本地记录
func NewService() (*Service, error) {
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
	if err := os.MkdirAll(filepath.Join(root, "codex"), 0700); err != nil {
		return nil, err
	}
	s := &Service{root: root, settings: Settings{Theme: "light", Mode: "chat", ReuseCLI: true, PermissionMode: "read-only"}}
	if err := ReadJSON(filepath.Join(root, "settings.json"), &s.settings); err != nil {
		return nil, err
	}
	return s, nil
}

// GetInfo 返回当前应用路径和运行平台
func (s *Service) GetInfo() Info {
	return Info{Name: "agentor", Version: "0.1.0", GoVersion: runtime.Version(),
		OS: runtime.GOOS, Arch: runtime.GOARCH, DataDirectory: s.root, CodexHome: filepath.Join(s.root, "codex")}
}

// GetSettings 读取应用设置快照
func (s *Service) GetSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
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

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
	Theme              string              `json:"theme"`
	Mode               string              `json:"mode"`
	ProjectDirectory   string              `json:"projectDirectory"`
	CodexPath          string              `json:"codexPath"`
	ReuseCLI           bool                `json:"reuseCLI"`
	Model              string              `json:"model"`
	Effort             string              `json:"effort"`
	LastThreadID       string              `json:"lastThreadId"`
	ChatThreads        map[string]string   `json:"chatThreads"`
	ProjectAliases     map[string]string   `json:"projectAliases"`
	HiddenProjects     []string            `json:"hiddenProjects"`
	PermissionMode     string              `json:"permissionMode"`
	ProjectPermissions map[string]string   `json:"projectPermissions"`
	ThreadPermissions  map[string]string   `json:"threadPermissions"`
	PinnedThreads      map[string]bool     `json:"pinnedThreads"`
	ChatOrder          map[string][]string `json:"chatOrder"`
	ProjectOrder       []string            `json:"projectOrder"`
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

func (s *Service) GetInfo() Info {
	return Info{Name: "agentor", Version: "0.1.0", GoVersion: runtime.Version(),
		OS: runtime.GOOS, Arch: runtime.GOARCH, DataDirectory: s.root, CodexHome: filepath.Join(s.root, "codex")}
}

func (s *Service) GetSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

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

func (s *Service) SetTheme(theme string) (Settings, error) {
	if theme != "light" && theme != "dark" {
		return Settings{}, errors.New("请选择浅色或深色主题")
	}
	return s.update(func(settings *Settings) { settings.Theme = theme })
}

func (s *Service) SetMode(mode string) (Settings, error) {
	if mode != "chat" && mode != "workspace" {
		return Settings{}, errors.New("未知工作模式")
	}
	return s.update(func(settings *Settings) { settings.Mode = mode })
}

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

func (s *Service) SetCodexPath(path string) (Settings, error) {
	return s.update(func(settings *Settings) { settings.CodexPath = path })
}

func (s *Service) SetModelSelection(model, effort string) (Settings, error) {
	return s.update(func(settings *Settings) { settings.Model, settings.Effort = model, effort })
}

func (s *Service) SetLastThread(threadID string) (Settings, error) {
	return s.update(func(settings *Settings) { settings.LastThreadID = threadID })
}

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

func (s *Service) ChooseDirectory() (string, error) {
	app := application.Get()
	return app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle("选择项目目录").CanChooseFiles(false).CanChooseDirectories(true).PromptForSingleSelection()
}

func (s *Service) ChooseFile() (string, error) {
	app := application.Get()
	return app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle("选择文件").CanChooseFiles(true).CanChooseDirectories(false).PromptForSingleSelection()
}

func (s *Service) OpenDirectory(path string) error {
	return application.Get().Env.OpenFileManager(path, false)
}

func (s *Service) OpenFile(path string) error {
	return application.Get().Browser.OpenFile(filepath.Clean(path))
}

func (s *Service) OpenURL(url string) error {
	return application.Get().Browser.OpenURL(url)
}

package codex

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Status struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	Path       string `json:"path"`
	Version    string `json:"version"`
	Home       string `json:"home"`
	ReuseCLI   bool   `json:"reuseCLI"`
	Generation uint64 `json:"generation"`
	Revision   uint64 `json:"revision"`
}

type Event struct {
	Generation uint64 `json:"generation"`
	Method     string `json:"method"`
	Params     string `json:"params"`
}

type Service struct {
	data           *appdata.Service
	connectMu      sync.Mutex
	mu             sync.Mutex
	client         *client
	status         Status
	ctx            context.Context
	cancel         context.CancelFunc
	active         map[string]string
	opened         map[string]bool
	approvals      map[string]Approval
	files          *artifact.Service
	usage          map[string]*TokenUsage
	pendingChanges map[string]map[string][]FileChange
}

func init() {
	application.RegisterEvent[Status]("codex:status")
	application.RegisterEvent[Event]("codex:event")
}

func NewService(data *appdata.Service, files *artifact.Service) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{data: data, files: files, ctx: ctx, cancel: cancel, active: make(map[string]string), opened: make(map[string]bool), approvals: make(map[string]Approval), usage: make(map[string]*TokenUsage), pendingChanges: make(map[string]map[string][]FileChange), status: Status{Phase: "stopped", Message: "尚未连接 Codex"}}
}

func (s *Service) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Service) ServiceStartup(context.Context, application.ServiceOptions) error {
	go func() { s.Connect() }()
	return nil
}

func (s *Service) setStatus(generation uint64, phase, message string) Status {
	s.mu.Lock()
	if s.status.Generation != generation {
		status := s.status
		s.mu.Unlock()
		return status
	}
	if phase == "ready" && s.client != nil {
		select {
		case <-s.client.done:
			phase, message = "error", "Codex 在初始化完成前已断开"
		default:
		}
	}
	s.status.Phase, s.status.Message = phase, message
	s.status.Revision++
	status := s.status
	s.mu.Unlock()
	if app := application.Get(); app != nil {
		app.Event.Emit("codex:status", status)
	}
	return status
}

func (s *Service) Connect() (Status, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return s.GetStatus(), err
	}
	s.mu.Lock()
	if len(s.active) > 0 {
		status := s.status
		s.mu.Unlock()
		return status, errors.New("请先停止活动任务，再重新连接")
	}
	old := s.client
	s.client = nil
	s.opened = make(map[string]bool)
	s.pendingChanges = make(map[string]map[string][]FileChange)
	s.expireApprovalsLocked("", "")
	s.status.Generation++
	generation := s.status.Generation
	s.mu.Unlock()
	if old != nil {
		old.close()
	}
	s.setStatus(generation, "starting", "正在启动 Codex app-server")
	settings := s.data.GetSettings()
	home := s.data.GetInfo().CodexHome
	if settings.ReuseCLI {
		home = os.Getenv("CODEX_HOME")
		if home == "" {
			userHome, err := os.UserHomeDir()
			if err != nil {
				return s.connectionError(err)
			}
			home = filepath.Join(userHome, ".codex")
		}
	}
	path, err := resolveBinary(settings.CodexPath)
	if err != nil {
		return s.connectionError(err)
	}
	env := processEnvironment(home, settings.ReuseCLI)
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	versionCmd := exec.CommandContext(ctx, path, "--version")
	versionCmd.Env = env
	version, err := versionCmd.Output()
	if err != nil {
		return s.connectionError(err)
	}
	s.mu.Lock()
	s.status.Path, s.status.Home, s.status.Version, s.status.ReuseCLI = path, home, strings.TrimSpace(string(version)), settings.ReuseCLI
	s.mu.Unlock()
	c, err := startClient(path, home, env, func(message packet) {
		s.mu.Lock()
		current := s.status.Generation == generation
		s.mu.Unlock()
		if !current {
			return
		}
		if len(message.ID) > 0 {
			s.handleServerRequest(generation, message)
			return
		}
		s.trackNotification(generation, message)
		if !strings.HasPrefix(message.Method, "thread/") && !strings.HasPrefix(message.Method, "turn/") && !strings.HasPrefix(message.Method, "item/") && !strings.HasPrefix(message.Method, "account/") && message.Method != "serverRequest/resolved" && message.Method != "error" {
			return
		}
		if app := application.Get(); app != nil {
			app.Event.Emit("codex:event", Event{Generation: generation, Method: message.Method, Params: string(message.Params)})
		}
	}, func(err error) {
		s.mu.Lock()
		current := s.status.Generation == generation
		s.mu.Unlock()
		if current {
			s.mu.Lock()
			if s.status.Generation == generation {
				s.active = make(map[string]string)
				s.opened = make(map[string]bool)
				s.expireApprovalsLocked("", "")
			}
			s.mu.Unlock()
			s.setStatus(generation, "error", "Codex app-server 已断开: "+err.Error())
		}
	})
	if err != nil {
		return s.connectionError(err)
	}
	s.mu.Lock()
	s.client = c
	s.mu.Unlock()
	var initialized struct {
		UserAgent string `json:"userAgent"`
	}
	err = c.request(ctx, "initialize", map[string]any{
		"clientInfo":   map[string]string{"name": "agentor", "title": "agentor", "version": "0.1.0"},
		"capabilities": map[string]bool{"experimentalApi": true},
	}, &initialized)
	if err == nil {
		err = c.notify("initialized", map[string]any{})
	}
	if err != nil {
		c.close()
		return s.connectionError(err)
	}
	status := s.setStatus(generation, "ready", "Codex app-server 已连接")
	if status.Phase != "ready" {
		return status, errors.New(status.Message)
	}
	return status, nil
}

func (s *Service) connectionError(err error) (Status, error) {
	return s.setStatus(s.GetStatus().Generation, "error", err.Error()), err
}

func (s *Service) call(method string, params, result any) error {
	s.mu.Lock()
	c, phase := s.client, s.status.Phase
	s.mu.Unlock()
	if c == nil || phase != "ready" {
		return errors.New("Codex 尚未就绪，请先连接")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 60*time.Second)
	defer cancel()
	return c.request(ctx, method, params, result)
}

type Account struct {
	Type     string `json:"type"`
	Email    string `json:"email"`
	PlanType string `json:"planType"`
}

type AccountInfo struct {
	Account            *Account `json:"account"`
	RequiresOpenAIAuth bool     `json:"requiresOpenaiAuth"`
}

func (s *Service) GetAccount() (AccountInfo, error) {
	var result AccountInfo
	err := s.call("account/read", map[string]bool{"refreshToken": false}, &result)
	return result, err
}

func (s *Service) ServiceShutdown() error {
	s.cancel()
	s.connectMu.Lock()
	defer s.connectMu.Unlock()
	s.mu.Lock()
	s.status.Generation++
	c := s.client
	s.client = nil
	s.mu.Unlock()
	if c != nil {
		c.close()
	}
	return nil
}

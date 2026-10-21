package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Status struct {
	Phase      string `json:"phase"`      // 当前连接阶段
	Message    string `json:"message"`    // 可读提示信息
	Path       string `json:"path"`       // 本地资源路径
	Version    string `json:"version"`    // 原生程序版本
	Home       string `json:"home"`       // 原生运行时数据目录
	ReuseCLI   bool   `json:"reuseCLI"`   // 是否沿用 CLI 配置
	Generation uint64 `json:"generation"` // 所属连接代次
	Revision   uint64 `json:"revision"`   // 状态修订号
}

type Event struct {
	Generation uint64 `json:"generation"` // 所属连接代次
	Method     string `json:"method"`     // 协议请求方法
	Params     string `json:"params"`     // 原始请求参数
}

type Service struct {
	data             *appdata.Service                   // 应用设置与数据服务
	connectMu        sync.Mutex                         // 串行保护连接切换
	mu               sync.Mutex                         // 保护连接状态
	closing          bool                               // 应用退出时阻止提交新轮次
	turnRequests     sync.WaitGroup                     // 等待退出前已提交的轮次启动请求
	shutdownRequests sync.WaitGroup                     // 等待退出时发出的中断请求
	client           *client                            // 当前原生协议连接
	status           Status                             // 当前连接状态
	ctx              context.Context                    // 服务生命周期上下文
	cancel           context.CancelFunc                 // 取消服务生命周期
	active           map[string]string                  // 按会话索引的活动轮次
	activeText       map[string]string                  // 活动轮次的输入摘要
	opened           map[string]bool                    // 当前连接已恢复的会话
	approvals        map[string]Approval                // 按请求索引的审批记录
	files            *artifact.Service                  // 附件与产物服务
	usage            map[string]*TokenUsage             // 按会话索引的令牌用量
	pendingChanges   map[string]map[string][]FileChange // 按会话与过程节点索引的文件变更
	toolHandler      func(ToolCall) (any, error)        // 宿主工具执行入口
	eventHandler     func(Event)                        // 上游事件业务观察者
	profiles         profileStore                       // 仅含本应用创建的独立账号
	profilesErr      error                              // 账号文件读取错误
	profileID        string                             // 当前原生进程实际使用的独立身份
	login            LoginState                         // 当前原生交互式登录状态
	balances         map[string]BalanceResult           // 按账号隔离的短时余额缓存
	accountGuard     func(bool) error                   // 参数指定是否禁止切换有排队任务的身份
}

type Account struct {
	Type     string `json:"type"`     // 当前账号类型
	Email    string `json:"email"`    // 当前账号邮箱
	PlanType string `json:"planType"` // 当前账号套餐类型
}

type AccountInfo struct {
	Account            *Account `json:"account"`            // 当前认证账号
	RequiresOpenAIAuth bool     `json:"requiresOpenaiAuth"` // 是否需要 OpenAI 登录
}

// NewService 初始化服务并读取本地记录
func NewService(data *appdata.Service, files *artifact.Service) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		data:           data,
		files:          files,
		ctx:            ctx,
		cancel:         cancel,
		balances:       map[string]BalanceResult{},
		active:         make(map[string]string),
		activeText:     make(map[string]string),
		opened:         make(map[string]bool),
		approvals:      make(map[string]Approval),
		usage:          make(map[string]*TokenUsage),
		pendingChanges: make(map[string]map[string][]FileChange),
		status:         Status{Phase: "stopped", Message: "尚未连接 Codex"},
	}
	if data != nil {
		s.profilesErr = appdata.ReadJSON(s.profilesPath(), &s.profiles)
	}
	return s
}

// GetStatus 读取当前原生连接状态
func (s *Service) GetStatus() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.status
}

// ServiceStartup 在应用启动时建立原生连接
func (s *Service) ServiceStartup(context.Context, application.ServiceOptions) error {
	go func() {
		s.Connect()
	}()
	return nil
}

// Connect 启动当前所选身份的原生运行时
func (s *Service) Connect() (Status, error) {
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	if err := s.accountIdle(false); err != nil {
		return s.GetStatus(), err
	}

	return s.connect()
}

// SetEventHandler 设置上游事件的业务观察者
//
//wails:ignore
func (s *Service) SetEventHandler(handler func(Event)) {
	s.mu.Lock()
	s.eventHandler = handler
	s.mu.Unlock()
}

// SetToolHandler 设置宿主工具请求的处理入口
//
//wails:ignore
func (s *Service) SetToolHandler(handler func(ToolCall) (any, error)) {
	s.mu.Lock()
	s.toolHandler = handler
	s.mu.Unlock()
}

// GetAccount 读取上游返回的当前身份
func (s *Service) GetAccount() (AccountInfo, error) {
	var result AccountInfo
	err := s.call("account/read", map[string]bool{"refreshToken": false}, &result)
	return result, err
}

// ServiceShutdown 结束进程并保存本应用登录状态
func (s *Service) ServiceShutdown() error {
	s.PrepareShutdown()
	s.turnRequests.Wait()
	s.shutdownRequests.Wait()
	s.cancel()
	s.connectMu.Lock()
	defer s.connectMu.Unlock()

	s.mu.Lock()
	s.status.Generation++
	c := s.client
	s.client = nil
	generation := s.status.Generation
	s.mu.Unlock()
	s.setStatus(generation, "stopped", "应用正在退出")
	if c != nil {
		c.close()
	}
	return s.captureAuth()
}

// init 注册业务变更事件
func init() {
	application.RegisterEvent[Status]("codex:status")
	application.RegisterEvent[Event]("codex:event")
}

// setStatus 仅更新当前连接代次的状态
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
	s.observe(Event{Generation: generation, Method: "host/status", Params: mustJSON(status)})
	if app := application.Get(); app != nil {
		app.Event.Emit("codex:status", status)
	}
	return status
}

// connect 在连接操作串行锁内启动当前选择的原生身份
func (s *Service) connect() (Status, error) {
	if err := s.ctx.Err(); err != nil {
		return s.GetStatus(), err
	}

	s.mu.Lock()
	if s.closing {
		status := s.status
		s.mu.Unlock()
		return status, errors.New("应用正在退出")
	}
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
	if err := s.captureAuth(); err != nil {
		return s.connectionError(err)
	}

	s.mu.Lock()
	s.profileID = ""
	if s.login.Status == "pending" {
		s.login.Status, s.login.Error = "cancelled", "连接已重建，请重新发起登录"
	}
	s.mu.Unlock()
	s.setStatus(generation, "starting", "正在启动 Codex app-server")
	settings := s.data.GetSettings()
	var selected storedProfile
	home := s.data.GetInfo().CodexHome
	if !settings.ReuseCLI {
		var err error
		selected, err = s.profile(settings.ActiveAccountID)
		if err != nil {
			return s.connectionError(err)
		}
		if err = s.prepareProfile(selected); err != nil {
			return s.connectionError(err)
		}
	}
	path, err := resolveBinary(settings.CodexPath)
	if err != nil {
		return s.connectionError(err)
	}

	env := processEnvironment(home, settings.ReuseCLI)
	if selected.Profile.Type == "api" {
		env = append(env, "AGENTOR_ACCOUNT_API_KEY="+selected.APIKey)
	}
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
		if message.Method == "account/login/completed" {
			go s.trackLogin(generation, message.Params)
		}
		s.observe(Event{Generation: generation, Method: message.Method, Params: string(message.Params)})
		if !strings.HasPrefix(message.Method, "thread/") &&
			!strings.HasPrefix(message.Method, "turn/") &&
			!strings.HasPrefix(message.Method, "item/") &&
			!strings.HasPrefix(message.Method, "account/") &&
			message.Method != "serverRequest/resolved" && message.Method != "error" {
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
				s.activeText = make(map[string]string)
				s.opened = make(map[string]bool)
				s.expireApprovalsLocked("", "")
				if s.login.Status == "pending" {
					s.login.Status, s.login.URL, s.login.Error = "error", "", "原生连接已断开，请重新发起登录"
				}
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
		UserAgent string `json:"userAgent"` // 原生初始化返回的客户端标识
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

	s.mu.Lock()
	s.profileID = selected.Profile.ID
	s.mu.Unlock()
	status := s.setStatus(generation, "ready", "Codex app-server 已连接")
	if status.Phase != "ready" {
		return status, errors.New(status.Message)
	}

	return status, nil
}

// mustJSON 将协议内容编码为 JSON 字符串
func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

// observe 通知业务模块处理上游事件
func (s *Service) observe(event Event) {
	s.mu.Lock()
	handler := s.eventHandler
	s.mu.Unlock()
	if handler != nil {
		handler(event)
	}
}

// connectionError 记录原生连接失败原因
func (s *Service) connectionError(err error) (Status, error) {
	return s.setStatus(s.GetStatus().Generation, "error", err.Error()), err
}

// call 向当前就绪进程发送协议请求
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

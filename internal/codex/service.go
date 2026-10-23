package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"sync"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// API 按职责列出服务的全部公开方法
type API interface {
	GetStatus() codexmodel.Status                                                                                                                                 // 读取当前原生连接状态
	ServiceStartup(context.Context, application.ServiceOptions) error                                                                                             // 在应用启动时建立原生连接
	Connect() (codexmodel.Status, error)                                                                                                                          // 启动当前所选身份的原生运行时
	SetEventHandler(handler func(codexmodel.Event))                                                                                                               // 设置上游事件的业务观察者
	SetToolHandler(handler func(codexmodel.ToolCall) (any, error))                                                                                                // 设置宿主工具请求的处理入口
	GetAccount() (codexmodel.AccountInfo, error)                                                                                                                  // 读取上游返回的当前身份
	ServiceShutdown() error                                                                                                                                       // 结束进程并保存本应用登录状态
	GetAccounts() (codexmodel.AccountsView, error)                                                                                                                // 读取独立账号列表和登录进度，不返回任何凭据
	SaveProfile(input codexmodel.ProfileInput) (codexmodel.AccountsView, error)                                                                                   // 创建或编辑独立账号，空密钥保留原值
	DeleteProfile(id string) (codexmodel.AccountsView, error)                                                                                                     // 删除未使用账号的私有记录，保留共享会话库
	UseAccount(id string) (codexmodel.Status, error)                                                                                                              // 在所有业务空闲后切换身份，空标识表示沿用 CLI
	SetAccountGuard(guard func(bool) error)                                                                                                                       // 将连接切换限制到控制中心和流水线均空闲时
	GetApprovals() []codexmodel.Approval                                                                                                                          // 返回审批和问答记录
	RespondApproval(id, decision string) error                                                                                                                    // 校验上游选项后回复命令或文件审批
	RespondPermissions(id string, allow bool, scope string) error                                                                                                 // 仅按原请求范围回复权限审批
	AnswerQuestions(id string, answers map[string]codexmodel.QuestionAnswer) error                                                                                // 将结构化答案回复原问答请求
	GetBalance(id string, force bool) (codexmodel.BalanceResult, error)                                                                                           // 查询指定 API 账号的余额，短时缓存与聊天连接互不影响
	CheckAccountConnection(id string) (string, error)                                                                                                             // 对独立 API 发起一次模型接口请求或读取当前 ChatGPT 真实额度
	CheckPath(path string) error                                                                                                                                  // 检测指定路径的原生程序能否运行，不改变当前连接
	GetTokenUsage(threadID string) *codexmodel.TokenUsage                                                                                                         // 读取会话最近一次上报的令牌用量快照
	GetModels() ([]codexmodel.Model, error)                                                                                                                       // 读取原生模型目录并补充当前 API 配置的自定义模型
	ListThreads(cursor string) (codexmodel.ThreadPage, error)                                                                                                     // 读取本应用可见的会话列表
	CreateThread(cwd, model string) (codexmodel.Thread, error)                                                                                                    // 在当前项目创建普通对话
	CreateToolThread(cwd, model, permission, role, instructions string, tools []codexmodel.DynamicToolSpec) (codexmodel.Thread, error)                            // 创建带角色指令和宿主工具的业务线程
	OpenThread(threadID string) (result codexmodel.Conversation, err error)                                                                                       // 检查占用后加载历史，只读会话在读取后取消本连接订阅
	ListTurns(threadID, cursor string) (codexmodel.TurnPage, error)                                                                                               // 分页读取当前线程的轮次
	StartTurn(threadID, text, model, effort, clientMessageID string, attachmentIDs []string) (codexmodel.Turn, error)                                             // 按会话实际权限发起一轮普通对话
	StartManagedTurn(threadID, text, model, effort, permission, cwd, clientMessageID string, attachmentIDs []string, serviceTier string) (codexmodel.Turn, error) // 按正式任务冻结的权限发起一轮对话
	SteerTurn(threadID, text, clientMessageID string, attachmentIDs []string) error                                                                               // 向原活动轮次追加输入
	StopTurn(threadID string) error                                                                                                                               // 中断当前线程的活动轮次
	IsThreadActive(threadID string) bool                                                                                                                          // 检查指定线程是否有活动轮次
	RenameThread(threadID, name string) error                                                                                                                     // 更新上游会话标题
	ArchiveThread(threadID string) error                                                                                                                          // 归档上游会话
	ArchiveProject(cwd string) (int, error)                                                                                                                       // 归档当前目录下本应用的普通对话
	AnswerElicitation(id, action, contentJSON string) error                                                                                                       // 校验并回复当前 MCP 问答请求
	StartLogin() (codexmodel.LoginState, error)                                                                                                                   // 通过当前原生协议启动独立 ChatGPT 账号登录
	ContinueLogin() error                                                                                                                                         // 用系统浏览器继续当前仍有效的登录
	CancelLogin() (codexmodel.LoginState, error)                                                                                                                  // 取消当前原生登录流程并保留可重试状态
	GetRateLimits() (codexmodel.RateLimits, error)                                                                                                                // 读取上游真实额度，不用模型列表推断账号权益
	EnsureOpen(threadID string) error                                                                                                                             // 在正式执行前恢复线程，已打开的新线程无需读取尚未落盘的历史
	GetActiveTurns() []codexmodel.ActiveTurn                                                                                                                      // 返回活动轮次的本地快照
	PrepareShutdown()                                                                                                                                             // 阻止新轮次并同时中断所有已启动的轮次
}

var _ API = (*Service)(nil)

type Service struct {
	data             *appdata.Service                              // 应用设置与数据服务
	connectMu        sync.Mutex                                    // 串行保护连接切换
	subscriptionMu   sync.Mutex                                    // 串行保护会话恢复与取消订阅
	mu               sync.Mutex                                    // 保护连接状态
	closing          bool                                          // 应用退出时阻止提交新轮次
	turnRequests     sync.WaitGroup                                // 等待退出前已提交的轮次启动请求
	shutdownRequests sync.WaitGroup                                // 等待退出时发出的中断请求
	client           *client                                       // 当前原生协议连接
	status           codexmodel.Status                             // 当前连接状态
	ctx              context.Context                               // 服务生命周期上下文
	cancel           context.CancelFunc                            // 取消服务生命周期
	active           map[string]string                             // 按会话索引的活动轮次
	activeText       map[string]string                             // 活动轮次的输入摘要
	opened           map[string]bool                               // 当前连接已恢复的会话
	subscriptions    []string                                      // 按订阅时间排列的普通会话
	threadNames      map[string]*managedThreadName                 // 本次运行新建业务会话的标题前缀
	approvals        map[string]codexmodel.Approval                // 按请求索引的审批记录
	files            *artifact.Service                             // 附件与产物服务
	usage            map[string]*codexmodel.TokenUsage             // 按会话索引的令牌用量
	pendingChanges   map[string]map[string][]codexmodel.FileChange // 按会话与过程节点索引的文件变更
	toolHandler      func(codexmodel.ToolCall) (any, error)        // 宿主工具执行入口
	eventHandler     func(codexmodel.Event)                        // 上游事件业务观察者
	profiles         profileStore                                  // 仅含本应用创建的独立账号
	profilesErr      error                                         // 账号文件读取错误
	profileID        string                                        // 当前原生进程实际使用的独立身份
	login            codexmodel.LoginState                         // 当前原生交互式登录状态
	balances         map[string]codexmodel.BalanceResult           // 按账号隔离的短时余额缓存
	accountGuard     func(bool) error                              // 参数指定是否禁止切换有排队任务的身份
}

// NewService 初始化服务并读取本地记录
func NewService(data *appdata.Service, files *artifact.Service) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		data:           data,
		files:          files,
		ctx:            ctx,
		cancel:         cancel,
		balances:       map[string]codexmodel.BalanceResult{},
		active:         make(map[string]string),
		activeText:     make(map[string]string),
		opened:         make(map[string]bool),
		threadNames:    make(map[string]*managedThreadName),
		approvals:      make(map[string]codexmodel.Approval),
		usage:          make(map[string]*codexmodel.TokenUsage),
		pendingChanges: make(map[string]map[string][]codexmodel.FileChange),
		status:         codexmodel.Status{Phase: constant.CodexPhaseStopped, Message: constant.CodexDisconnectedMessage},
	}
	if data != nil {
		s.profilesErr = appdata.ReadJSON(s.profilesPath(), &s.profiles)
	}
	return s
}

// GetStatus 读取当前原生连接状态
func (s *Service) GetStatus() codexmodel.Status {
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
func (s *Service) Connect() (codexmodel.Status, error) {
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
func (s *Service) SetEventHandler(handler func(codexmodel.Event)) {
	s.mu.Lock()
	s.eventHandler = handler
	s.mu.Unlock()
}

// SetToolHandler 设置宿主工具请求的处理入口
//
//wails:ignore
func (s *Service) SetToolHandler(handler func(codexmodel.ToolCall) (any, error)) {
	s.mu.Lock()
	s.toolHandler = handler
	s.mu.Unlock()
}

// GetAccount 读取上游返回的当前身份
func (s *Service) GetAccount() (codexmodel.AccountInfo, error) {
	var result codexmodel.AccountInfo
	err := s.call(constant.CodexMethodAccountRead, map[string]bool{"refreshToken": false}, &result)
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
	s.setStatus(generation, constant.CodexPhaseStopped, constant.CodexClosingMessage)
	if c != nil {
		c.close()
	}
	return s.captureAuth()
}

// init 注册业务变更事件
func init() {
	application.RegisterEvent[codexmodel.Status](constant.CodexEventStatus)
	application.RegisterEvent[codexmodel.Event](constant.CodexEventMessage)
}

// setStatus 仅更新当前连接代次的状态
func (s *Service) setStatus(generation uint64, phase, message string) codexmodel.Status {
	s.mu.Lock()
	if s.status.Generation != generation {
		status := s.status
		s.mu.Unlock()
		return status
	}
	if phase == constant.CodexPhaseReady && s.client != nil {
		select {
		case <-s.client.done:
			phase, message = constant.CodexStatusError, constant.CodexInitializationDisconnectedMessage
		default:
		}
	}
	s.status.Phase, s.status.Message = phase, message
	s.status.Revision++
	status := s.status
	s.mu.Unlock()
	s.observe(codexmodel.Event{Generation: generation, Method: "host/status", Params: mustJSON(status)})
	if app := application.Get(); app != nil {
		app.Event.Emit(constant.CodexEventStatus, status)
	}
	return status
}

// connect 在连接操作串行锁内启动当前选择的原生身份
func (s *Service) connect() (codexmodel.Status, error) {
	s.subscriptionMu.Lock()
	defer s.subscriptionMu.Unlock()

	if err := s.ctx.Err(); err != nil {
		return s.GetStatus(), err
	}

	s.mu.Lock()
	if s.closing {
		status := s.status
		s.mu.Unlock()
		return status, constant.ErrCodexClosing
	}
	if len(s.active) > 0 {
		status := s.status
		s.mu.Unlock()
		return status, constant.ErrCodexActiveTasks
	}

	old := s.client
	s.client = nil
	s.opened = make(map[string]bool)
	s.subscriptions = nil
	s.pendingChanges = make(map[string]map[string][]codexmodel.FileChange)
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
	if s.login.Status == constant.CodexStatusPending {
		s.login.Status, s.login.Error = constant.CodexStatusCancelled, constant.CodexLoginRebuiltMessage
	}
	s.mu.Unlock()
	s.setStatus(generation, constant.CodexPhaseStarting, constant.CodexConnectingMessage)
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
	path, searchPath, err := resolveBinary(settings.CodexPath)
	if err != nil {
		return s.connectionError(err)
	}

	env := processEnvironment(home, settings.ReuseCLI, searchPath)
	if selected.Profile.Type == constant.CodexAccountTypeAPI {
		env = append(env, "AGENTOR_ACCOUNT_API_KEY="+selected.APIKey)
	}
	ctx, cancel := context.WithTimeout(s.ctx, constant.CodexConnectTimeout)
	defer cancel()

	versionCmd := exec.CommandContext(ctx, path, "--version")
	hideProcessWindow(versionCmd)
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
		if message.Method == constant.CodexMethodAccountLoginCompleted {
			go s.trackLogin(generation, message.Params)
		}
		s.observe(codexmodel.Event{Generation: generation, Method: message.Method, Params: string(message.Params)})
		if !strings.HasPrefix(message.Method, "thread/") &&
			!strings.HasPrefix(message.Method, "turn/") &&
			!strings.HasPrefix(message.Method, "item/") &&
			!strings.HasPrefix(message.Method, "account/") &&
			message.Method != "serverRequest/resolved" && message.Method != constant.CodexStatusError {
			return
		}
		if app := application.Get(); app != nil {
			app.Event.Emit(constant.CodexEventMessage, codexmodel.Event{Generation: generation, Method: message.Method, Params: string(message.Params)})
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
				s.subscriptions = nil
				s.expireApprovalsLocked("", "")
				if s.login.Status == constant.CodexStatusPending {
					s.login.Status, s.login.URL, s.login.Error = constant.CodexStatusError, "", constant.CodexLoginDisconnectedMessage
				}
			}
			s.mu.Unlock()
			s.setStatus(generation, constant.CodexStatusError, constant.CodexServerDisconnectedPrefix+err.Error())
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
		"clientInfo":   map[string]string{"name": constant.AppDataApplicationName, "title": constant.AppDataApplicationName, "version": "0.1.0"},
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
	status := s.setStatus(generation, constant.CodexPhaseReady, constant.CodexConnectedMessage)
	if status.Phase != constant.CodexPhaseReady {
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
func (s *Service) observe(event codexmodel.Event) {
	s.mu.Lock()
	handler := s.eventHandler
	s.mu.Unlock()
	if handler != nil {
		handler(event)
	}
}

// connectionError 记录原生连接失败原因
func (s *Service) connectionError(err error) (codexmodel.Status, error) {
	return s.setStatus(s.GetStatus().Generation, constant.CodexStatusError, err.Error()), err
}

// call 向当前就绪进程发送协议请求
func (s *Service) call(method string, params, result any) error {
	s.mu.Lock()
	c, phase := s.client, s.status.Phase
	s.mu.Unlock()
	if c == nil || phase != constant.CodexPhaseReady {
		return constant.ErrCodexConnectionRequired
	}

	ctx, cancel := context.WithTimeout(s.ctx, constant.CodexRequestTimeout)
	defer cancel()

	return c.request(ctx, method, params, result)
}

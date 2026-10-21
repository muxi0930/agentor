package lark

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
	larksdk "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Service struct {
	mu               sync.Mutex         // 保护连接与业务状态
	root             string             // 当前应用的飞书数据目录
	config           credentials        // 含密钥的私有配置
	state            records            // 接收与审批记录
	status           Status             // 公开连接状态
	ctx              context.Context    // 应用生命周期
	cancel           context.CancelFunc // 结束应用生命周期
	connectionCancel context.CancelFunc // 取消当前连接
	connectionCtx    context.Context    // 当前连接生命周期
	ws               *larkws.Client     // 官方长连接客户端
	client           *larksdk.Client    // 官方接口客户端
	tasks            *tasks.Service     // 统一任务服务
	work             *workspace.Service // 工作区定义服务
	files            *artifact.Service  // 托管附件服务
	runtime          *codex.Service     // Codex 原生运行时
	pairing          Pairing            // 仅内存保存的一次性配对
	sending          map[string]bool    // 正在发送的通知
	preparing        map[string]bool    // 正在准备分项的通知
	approvalSession  string             // 当前进程的审批路由标识
	pipelines        *pipeline.Service  // 流水线服务
}

type Changed struct{}

type quietLogger struct{}

// NewService 初始化服务并读取本地记录
func NewService(root string, executions *tasks.Service, work *workspace.Service, files *artifact.Service, runtime *codex.Service, pipelines *pipeline.Service) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		pipelines:       pipelines,
		approvalSession: rand.Text(),
		root:            filepath.Join(root, "lark"),
		tasks:           executions,
		work:            work,
		files:           files,
		runtime:         runtime,
		ctx:             ctx,
		cancel:          cancel,
		sending:         map[string]bool{},
		preparing:       map[string]bool{},
		config:          credentials{Settings: Settings{Region: "feishu"}},
		status:          Status{Phase: "unconfigured", Message: "请配置 App ID 和 App Secret"},
	}
	if err := appdata.ReadJSON(filepath.Join(s.root, "config.json"), &s.config); err != nil {
		cancel()
		return nil, err
	}
	if err := appdata.ReadJSON(filepath.Join(s.root, "records.json"), &s.state); err != nil {
		cancel()
		return nil, err
	}

	changed := false
	for i := range s.state.Incoming {
		item := &s.state.Incoming[i]
		if item.Status == "receiving" {
			item.Status, item.Error = "interrupted", "应用退出时接收尚未完成，未自动重复提交"
			changed = true
		}
	}
	for i := range s.state.Approvals {
		if s.state.Approvals[i].Status == "pending" {
			s.state.Approvals[i].Status = "expired"
			changed = true
		}
	}
	if changed {
		if err := s.persistLocked(); err != nil {
			cancel()
			return nil, err
		}
	}
	if s.config.AppID != "" && s.config.AppSecret != "" {
		s.status = Status{Phase: "unconfigured", Message: "等待建立连接"}
	}
	return s, nil
}

// GetState 返回不包含密钥的状态快照
func (s *Service) GetState() View {
	s.mu.Lock()
	defer s.mu.Unlock()

	return copyValue(View{
		Settings:         s.config.Settings,
		SecretConfigured: s.config.AppSecret != "",
		Status:           s.status,
		Binding:          s.config.Binding,
		Candidates:       s.state.Candidates,
		Incoming:         s.state.Incoming,
		Approvals:        s.state.Approvals,
	})
}

// DeleteCandidate 删除指定类型的已发现身份记录
func (s *Service) DeleteCandidate(kind, id string) error {
	s.mu.Lock()
	next := copyValue(s.state)
	next.Candidates = slices.DeleteFunc(next.Candidates, func(candidate Candidate) bool {
		return candidate.Kind == kind && candidate.ID == id
	})
	err := appdata.WriteJSON(filepath.Join(s.root, "records.json"), next)
	if err == nil {
		s.state = next
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}

	s.changed()
	return nil
}

// SaveSettings 保存配置成功后建立连接
func (s *Service) SaveSettings(input SettingsInput) (View, error) {
	if input.Region != "feishu" && input.Region != "lark" {
		return View{}, errors.New("区域请选择飞书或 Lark")
	}
	if input.DefaultProjectID != "" {
		found := false
		for _, project := range s.work.GetState().Projects {
			if project.ID == input.DefaultProjectID {
				found = true
			}
		}
		if !found {
			return View{}, errors.New("默认项目不存在")
		}
	}
	s.mu.Lock()
	next := s.config
	if next.AppID != input.AppID {
		next.AppSecret = ""
		next.Binding = tasks.Source{}
	}
	next.Settings = copyValue(input.Settings)
	if input.ClearSecret {
		next.AppSecret = ""
	} else if input.AppSecret != "" {
		next.AppSecret = input.AppSecret
	}
	err := appdata.WriteJSON(filepath.Join(s.root, "config.json"), next)
	if err == nil {
		s.config = next
	}
	s.mu.Unlock()
	if err != nil {
		return View{}, err
	}

	s.connect()
	s.changed()
	return s.GetState(), nil
}

// Disconnect 关闭当前长连接并取消等待
func (s *Service) Disconnect() {
	s.mu.Lock()
	s.status.Generation++
	cancel, ws := s.connectionCancel, s.ws
	s.connectionCancel, s.ws, s.client = nil, nil, nil
	s.connectionCtx = nil
	s.status.Phase, s.status.Message, s.status.BotOpenID = "disconnected", "连接已关闭", ""
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ws != nil {
		ws.Close()
	}
	s.changed()
}

// ServiceStartup 在应用启动时连接已配置的服务
func (s *Service) ServiceStartup(context.Context, application.ServiceOptions) error {
	s.mu.Lock()
	configured := s.config.AppID != "" && s.config.AppSecret != ""
	s.mu.Unlock()
	if configured {
		s.connect()
	}
	return nil
}

// ServiceShutdown 取消活动请求并关闭服务连接
func (s *Service) ServiceShutdown() error {
	s.cancel()
	s.Disconnect()
	return nil
}

// Debug 忽略 SDK 调试日志以免泄露请求凭据
func (quietLogger) Debug(context.Context, ...interface{}) {}

// Info 忽略 SDK 普通日志以免泄露请求凭据
func (quietLogger) Info(context.Context, ...interface{}) {}

// Warn 忽略 SDK 警告日志以免泄露请求凭据
func (quietLogger) Warn(context.Context, ...interface{}) {}

// Error 忽略 SDK 错误日志以免泄露请求凭据
func (quietLogger) Error(context.Context, ...interface{}) {}

// connect 启动官方 SDK 长连接
func (s *Service) connect() Status {
	s.Disconnect()
	s.mu.Lock()
	cfg := copyValue(s.config)
	s.status.Generation++
	generation := s.status.Generation
	if cfg.AppID == "" || cfg.AppSecret == "" {
		s.status.Phase, s.status.Message = "unconfigured", "请配置 App ID 和 App Secret"
		value := s.status
		s.mu.Unlock()
		s.changed()
		return value
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.connectionCancel = cancel
	s.connectionCtx = ctx
	s.status.Phase, s.status.Message = "connecting", "正在读取机器人身份并建立长连接"
	value := s.status
	s.mu.Unlock()
	s.changed()

	go func() {
		client := larksdk.NewClient(cfg.AppID, cfg.AppSecret,
			larksdk.WithOpenBaseUrl(domain(cfg.Region)),
			larksdk.WithLogger(quietLogger{}),
			larksdk.WithHttpClient(&http.Client{Timeout: 20 * time.Second}),
		)
		botID, err := botIdentity(ctx, client)
		if err != nil {
			s.setStatus(generation, "error", err.Error(), "")
			return
		}

		handler := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(func(_ context.Context, event *larkim.P2MessageReceiveV1) error {
			message, err := normalize(event, cfg.AppID, botID)
			if err != nil {
				return nil
			}
			return s.receive(ctx, generation, client, message)
		}).OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			return s.cardAction(ctx, generation, event)
		})
		ws := larkws.NewClient(cfg.AppID, cfg.AppSecret,
			larkws.WithDomain(domain(cfg.Region)),
			larkws.WithLogger(quietLogger{}),
			larkws.WithHttpClient(&http.Client{Timeout: 20 * time.Second}),
			larkws.WithEventHandler(handler),
			larkws.WithOnReady(func() {
				s.setStatus(generation, "ready", "长连接已建立", botID)
				go s.drain()
			}),
			larkws.WithOnReconnecting(func() {
				s.setStatus(generation, "connecting", "SDK 正在重连", botID)
			}),
			larkws.WithOnReconnected(func() {
				s.setStatus(generation, "ready", "长连接已恢复", botID)
				go s.drain()
			}),
			larkws.WithOnError(func(err error) {
				s.setStatus(generation, "error", err.Error(), botID)
			}),
			larkws.WithOnDisconnected(func() {
				s.setStatus(generation, "disconnected", "长连接已断开", botID)
			}),
		)
		handler.Config.Logger = quietLogger{}

		s.mu.Lock()
		if s.status.Generation != generation || ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		s.client, s.ws = client, ws
		s.status.BotOpenID = botID
		s.mu.Unlock()

		if err := ws.Start(ctx); err != nil && ctx.Err() == nil {
			s.setStatus(generation, "error", err.Error(), botID)
		}
	}()
	return value
}

// init 注册业务变更事件
func init() {
	application.RegisterEvent[Changed]("lark:changed")
}

// now 返回当前 UTC 时间
func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// copyValue 复制可变状态以隔离调用方
func copyValue[T any](value T) T {
	raw, _ := json.Marshal(value)
	var result T
	json.Unmarshal(raw, &result)
	return result
}

// persistLocked 在持有状态锁时保存记录
func (s *Service) persistLocked() error {
	return appdata.WriteJSON(filepath.Join(s.root, "records.json"), s.state)
}

// changed 发布状态变更事件
func (s *Service) changed() {
	if s.ctx.Err() != nil {
		return
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("lark:changed", Changed{})
	}
}

// setStatus 仅更新当前连接代次的状态
func (s *Service) setStatus(generation uint64, phase, message, botID string) {
	s.mu.Lock()
	if s.status.Generation != generation || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	if s.config.AppSecret != "" {
		message = strings.ReplaceAll(message, s.config.AppSecret, "[redacted]")
	}
	s.status.Phase, s.status.Message = phase, message
	if botID != "" {
		s.status.BotOpenID = botID
	}
	s.mu.Unlock()
	s.changed()
}

// domain 选择飞书区域对应的服务地址
func domain(region string) string {
	if region == "lark" {
		return "https://open.larksuite.com"
	}
	return "https://open.feishu.cn"
}

// botIdentity 通过官方接口读取当前机器人身份
func botIdentity(ctx context.Context, client *larksdk.Client) (string, error) {
	response, err := client.Do(ctx, &larkcore.ApiReq{
		HttpMethod:                http.MethodGet,
		ApiPath:                   "/open-apis/bot/v3/info",
		SupportedAccessTokenTypes: []larkcore.AccessTokenType{larkcore.AccessTokenTypeTenant},
	})
	if err != nil {
		return "", err
	}

	var body struct {
		Code int    `json:"code"` // 接口业务状态码
		Msg  string `json:"msg"`  // 接口状态说明
		Bot  struct {
			OpenID string `json:"open_id"` // 当前机器人的开放标识
		} `json:"bot"` // 机器人身份
	}
	if err = json.Unmarshal(response.RawBody, &body); err != nil {
		return "", err
	}
	if body.Code != 0 || body.Bot.OpenID == "" {
		return "", fmt.Errorf("读取机器人身份失败（%d）：%s", body.Code, body.Msg)
	}
	return body.Bot.OpenID, nil
}

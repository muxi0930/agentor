package tasks

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	"agentor/internal/workspace"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Service struct {
	activeExecutions    atomic.Int32                                             // 当前占用并发名额的功能任务数
	mu                  sync.Mutex                                               // 任务状态与队列读写锁
	workerGroup         sync.WaitGroup                                           // 退出时等待步骤与控制中心队列结束
	data                *appdata.Service                                         // 应用设置服务
	runtime             *codex.Service                                           // Codex 原生运行时
	work                *workspace.Service                                       // 工作区定义服务
	files               *artifact.Service                                        // 托管附件服务
	path                string                                                   // 任务状态文件路径
	state               State                                                    // 任务与通知记录
	ctx                 context.Context                                          // 任务服务生命周期
	cancel              context.CancelFunc                                       // 取消任务服务生命周期
	workers             map[string]bool                                          // 各功能的队列运行状态
	cancellations       map[string]context.CancelFunc                            // 活动执行与请求的取消入口
	turns               map[string]*turnWaiter                                   // 按会话登记的轮次等待者
	completed           map[string][]chan Execution                              // 等待指定执行完成的通道
	toolSpecs           func(string) []codex.DynamicToolSpec                     // 各业务角色可用的宿主工具
	runExecution        func(context.Context, Execution) (string, string, error) // 步骤执行入口
	controllerWorker    bool                                                     // 控制中心队列是否运行
	runRequest          func(context.Context, Request) (string, string, error)   // 控制中心请求执行入口
	deleting            map[string]bool                                          // 正在移除的步骤
	notificationHandler func(Notification)                                       // 通知持久化后的发送入口
}

type Changed struct{}

// NewService 初始化服务并读取本地记录
func NewService(data *appdata.Service, runtime *codex.Service, work *workspace.Service, files *artifact.Service) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		data:          data,
		runtime:       runtime,
		work:          work,
		files:         files,
		path:          filepath.Join(data.GetInfo().DataDirectory, "tasks.json"),
		ctx:           ctx,
		cancel:        cancel,
		workers:       map[string]bool{},
		cancellations: map[string]context.CancelFunc{},
		turns:         map[string]*turnWaiter{},
		completed:     map[string][]chan Execution{},
		state:         State{Executions: []Execution{}, Notifications: []Notification{}},
	}
	if err := appdata.ReadJSON(s.path, &s.state); err != nil {
		cancel()
		return nil, err
	}

	changed := false
	if s.state.LastSteps == nil {
		s.state.LastSteps = map[string]string{}
	}
	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.Status == "running" {
			request.Status, request.Error, request.CompletedAt = "interrupted", "应用退出时控制中心尚未结束；未自动重放", now()
			changed = true
		}
		if request.Status == "queued" {
			if len(request.Executor.Models) == 0 {
				request.Executor = appdata.ControllerSettings{Models: []string{request.Model}, Efforts: []string{request.Effort}, Prompt: appdata.DefaultControllerPrompt}
			}
			s.state.Paused = true
			changed = true
		}
	}
	for i := range s.state.Executions {
		execution := &s.state.Executions[i]
		if execution.Status == "running" {
			execution.Status, execution.Error, execution.CompletedAt = "interrupted", "应用退出时任务尚未结束；未自动重放", now()
			changed = true
		}
		if execution.Status == "queued" {
			s.state.Paused = true
			changed = true
		}
	}
	if changed {
		if err := appdata.WriteJSON(s.path, s.state); err != nil {
			cancel()
			return nil, err
		}
	}
	s.runExecution = s.execute
	s.runRequest = s.executeController
	s.deleting = map[string]bool{}
	return s, nil
}

// GetState 返回不包含密钥的状态快照
func (s *Service) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return copyState(s.state)
}

// ConfigureTools 配置各业务角色可用的宿主工具
//
//wails:ignore
func (s *Service) ConfigureTools(specs func(string) []codex.DynamicToolSpec) {
	s.toolSpecs = specs
}

// SelectModel 核验模型与推理强度并返回有效选择
//
//wails:ignore
func (s *Service) SelectModel(model, effort string) (string, string, error) {
	settings := s.data.GetSettings()
	if model == "" {
		model = settings.Model
		if effort == "" {
			effort = settings.Effort
		}
	}
	if model == "" || effort == "" {
		models, err := s.runtime.GetModels()
		if err != nil {
			return "", "", err
		}
		for _, option := range models {
			if (model == "" && option.IsDefault) || option.Model == model {
				model = option.Model
				if effort == "" {
					effort = option.DefaultReasoningEffort
				}
				break
			}
		}
	}
	if model == "" {
		return "", "", errors.New("请选择可用模型")
	}
	return model, effort, nil
}

// Dispatch 冻结步骤参数并加入正式执行队列
func (s *Service) Dispatch(input DispatchInput) (Execution, error) {
	if s.runtime.GetStatus().Phase != "ready" {
		return Execution{}, errors.New("Codex 尚未就绪")
	}
	context, err := s.work.GetStepContext(input.StepID)
	if err != nil {
		return Execution{}, err
	}
	if strings.TrimSpace(input.Instruction) == "" && strings.TrimSpace(context.Step.Prompt) == "" {
		return Execution{}, errors.New("请填写执行指令或步骤提示词")
	}
	if input.TimeoutSeconds < 0 {
		return Execution{}, errors.New("超时秒数不能为负数")
	}

	model, effort, err := s.SelectModel(input.Model, input.Effort)
	if err != nil {
		return Execution{}, err
	}
	permission := context.Step.PermissionMode
	if permission == "" {
		permission = s.data.EffectivePermission(context.Step.ThreadID, context.Directory)
	}
	if input.Source.Type == "" {
		input.Source.Type = "desktop"
	}

	execution := Execution{
		AttachmentRequestID: input.AttachmentRequestID,
		PipelineRunID:       input.PipelineRunID,
		ID:                  rand.Text(),
		StepID:              input.StepID,
		ProjectID:           context.ProjectID,
		FeatureID:           context.FeatureID,
		Instruction:         input.Instruction,
		CompletionCriteria:  input.CompletionCriteria,
		Model:               model,
		Effort:              effort,
		Permission:          permission,
		Source:              input.Source,
		RequestID:           input.RequestID,
		Context:             context,
		Status:              "queued",
		CreatedAt:           now(),
		TimeoutSeconds:      input.TimeoutSeconds,
	}

	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return Execution{}, err
	}
	if s.deleting[input.StepID] {
		s.mu.Unlock()
		return Execution{}, errors.New("此步骤正在移除，请等待操作结束")
	}
	if _, err := s.work.GetStepContext(input.StepID); err != nil {
		s.mu.Unlock()
		return Execution{}, err
	}
	s.state.Executions = append(s.state.Executions, execution)
	previousTarget := s.state.LastSteps[sourceKey(execution.Source)]
	s.state.LastSteps[sourceKey(execution.Source)] = execution.StepID
	err = s.persistLocked()
	if err != nil {
		s.state.Executions = s.state.Executions[:len(s.state.Executions)-1]
		if previousTarget == "" {
			delete(s.state.LastSteps, sourceKey(execution.Source))
		} else {
			s.state.LastSteps[sourceKey(execution.Source)] = previousTarget
		}
	}
	s.mu.Unlock()
	if err != nil {
		return Execution{}, err
	}
	s.changed()
	s.kick(execution.FeatureID)
	return execution, nil
}

// StopExecution 取消排队或正在运行的执行
func (s *Service) StopExecution(id string) error {
	s.mu.Lock()
	for i := range s.state.Executions {
		execution := &s.state.Executions[i]
		if execution.ID != id {
			continue
		}
		if execution.Status == "queued" {
			execution.Status, execution.CompletedAt = "interrupted", now()
			execution.Error = "用户取消排队"
			for _, channel := range s.completed[id] {
				channel <- *execution
				close(channel)
			}
			delete(s.completed, id)
			err := s.persistLocked()
			s.mu.Unlock()
			s.changed()
			return err
		}
		if execution.Status != "running" {
			s.mu.Unlock()
			return nil
		}
		execution.StopRequested = true
		cancel := s.cancellations[id]
		err := s.persistLocked()
		s.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		s.changed()
		return err
	}
	s.mu.Unlock()
	return errors.New("执行记录不存在")
}

// SteerExecution 向当前执行追加指令
func (s *Service) SteerExecution(id, text string) error {
	s.mu.Lock()
	threadID := ""
	for _, execution := range s.state.Executions {
		if execution.ID == id && execution.Status == "running" {
			threadID = execution.ThreadID
			break
		}
	}
	s.mu.Unlock()
	if threadID == "" {
		return errors.New("任务尚未建立活动会话")
	}
	return s.runtime.SteerTurn(threadID, text, rand.Text(), nil)
}

// ResumeQueue 由用户恢复重启后保留的任务队列
func (s *Service) ResumeQueue() error {
	if s.runtime.GetStatus().Phase != "ready" {
		return errors.New("请先连接 Codex")
	}

	s.mu.Lock()
	s.state.Paused = false
	err := s.persistLocked()
	features := map[string]bool{}
	for _, execution := range s.state.Executions {
		if execution.Status == "queued" {
			features[execution.FeatureID] = true
		}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	for id := range features {
		s.kick(id)
	}
	s.kickController()
	s.changed()
	return nil
}

// WaitExecution 等待指定执行达到终态
//
//wails:ignore
func (s *Service) WaitExecution(ctx context.Context, id string) (Execution, error) {
	s.mu.Lock()
	for _, execution := range s.state.Executions {
		if execution.ID == id {
			if execution.Status != "queued" && execution.Status != "running" {
				s.mu.Unlock()
				return execution, nil
			}
			channel := make(chan Execution, 1)
			s.completed[id] = append(s.completed[id], channel)
			s.mu.Unlock()
			select {
			case result := <-channel:
				return result, nil
			case <-ctx.Done():
				return Execution{}, ctx.Err()
			}
		}
	}
	s.mu.Unlock()
	return Execution{}, errors.New("执行记录不存在")
}

// MarkNotificationRead 保存通知已读状态
func (s *Service) MarkNotificationRead(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Notifications {
		if s.state.Notifications[i].ID == id {
			s.state.Notifications[i].Read = true
			return s.persistLocked()
		}
	}
	return errors.New("通知不存在")
}

// RegisterArtifact 将当前执行生成的文件登记为产物
//
//wails:ignore
func (s *Service) RegisterArtifact(executionID, path string) (artifact.Attachment, error) {
	s.mu.Lock()
	var found *Execution
	for _, execution := range s.state.Executions {
		if execution.ID == executionID && execution.Status == "running" {
			copy := execution
			found = &copy
			break
		}
	}
	s.mu.Unlock()
	if found == nil {
		return artifact.Attachment{}, errors.New("此任务没有活动执行")
	}
	item, err := s.files.RegisterProduced(artifact.ProducedOwner{ThreadID: found.ThreadID, ExecutionID: found.ID, StepID: found.StepID, ProjectID: found.ProjectID, RequestID: found.RequestID}, path, found.Context.Directory)
	if err == nil {
		s.changed()
	}
	return item, err
}

// PrepareShutdown 停止接收任务并取消运行与排队请求
//
//wails:ignore
func (s *Service) PrepareShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancel()
	for _, cancel := range s.cancellations {
		cancel()
	}

	for i := range s.state.Executions {
		execution := &s.state.Executions[i]
		if execution.Status == "running" || execution.Status == "queued" {
			status := "interrupted"
			if execution.Status == "queued" {
				status = "cancelled"
			}
			execution.Status, execution.Error, execution.CompletedAt = status, "应用退出，任务已取消", now()
			for _, channel := range s.completed[execution.ID] {
				channel <- *execution
				close(channel)
			}
			delete(s.completed, execution.ID)
		}
	}

	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.Status == "running" || request.Status == "queued" {
			status := "interrupted"
			if request.Status == "queued" {
				status = "cancelled"
			}
			request.Status, request.Error, request.CompletedAt = status, "应用退出，请求已取消", now()
		}
	}
	return s.persistLocked()
}

// ServiceShutdown 取消所有任务并等待队列工作线程结束
func (s *Service) ServiceShutdown() error {
	err := s.PrepareShutdown()
	s.workerGroup.Wait()
	return err
}

// init 注册业务变更事件
func init() {
	application.RegisterEvent[Changed]("tasks:changed")
}

// now 返回当前 UTC 时间
func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// copyState 复制任务状态以隔离调用方
func copyState(state State) State {
	raw, _ := json.Marshal(state)
	var result State
	json.Unmarshal(raw, &result)
	return result
}

// persistLocked 在持有状态锁时保存记录
func (s *Service) persistLocked() error {
	if err := appdata.WriteJSON(s.path, s.state); err != nil {
		return err
	}
	return s.syncCompletionLocked()
}

// syncCompletionLocked 以派发顺序中的最后一条记录同步步骤完成状态
func (s *Service) syncCompletionLocked() error {
	if s.work == nil {
		return nil
	}
	latest := make(map[string]workspace.StepCompletion)
	for _, execution := range s.state.Executions {
		completion := workspace.StepCompletion{Completed: execution.Status == "completed", DispatchedAt: execution.CreatedAt}
		if completion.Completed {
			completion.CompletedAt = execution.CompletedAt
		}
		latest[execution.StepID] = completion
	}
	return s.work.SyncCompletion(latest)
}

// changed 发布状态变更事件
func (s *Service) changed() {
	if s.ctx.Err() != nil {
		return
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("tasks:changed", Changed{})
	}
}

// kick 启动尚未运行的功能队列
func (s *Service) kick(featureID string) {
	s.mu.Lock()
	if s.state.Paused || s.workers[featureID] || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.workers[featureID] = true
	s.workerGroup.Add(1)
	s.mu.Unlock()
	go s.runFeatureQueue(featureID)
}

// runFeatureQueue 按顺序执行同一功能下各步骤的排队任务
func (s *Service) runFeatureQueue(featureID string) {
	defer s.workerGroup.Done()

	for {
		s.mu.Lock()
		if s.state.Paused || s.ctx.Err() != nil || s.runtime != nil && s.runtime.GetStatus().Phase != "ready" {
			delete(s.workers, featureID)
			s.mu.Unlock()
			return
		}

		index := -1
		for i, execution := range s.state.Executions {
			if execution.FeatureID == featureID && execution.Status == "queued" {
				index = i
				break
			}
		}
		if index < 0 {
			delete(s.workers, featureID)
			s.mu.Unlock()
			return
		}

		active := s.activeExecutions.Load()
		settings := s.data.GetSettings()
		if active >= int32(settings.MaxConcurrentAgents) || !s.activeExecutions.CompareAndSwap(active, active+1) {
			s.mu.Unlock()
			select {
			case <-s.ctx.Done():
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}

		s.state.Executions[index].Status, s.state.Executions[index].StartedAt = "running", now()
		if limit := settings.AgentTimeoutSeconds; limit > 0 && (s.state.Executions[index].TimeoutSeconds == 0 || limit < s.state.Executions[index].TimeoutSeconds) {
			s.state.Executions[index].TimeoutSeconds = limit
		}
		execution := s.state.Executions[index]
		ctx, cancel := context.WithCancel(s.ctx)
		if execution.TimeoutSeconds > 0 {
			cancel()
			ctx, cancel = context.WithTimeout(s.ctx, time.Duration(execution.TimeoutSeconds)*time.Second)
		}
		s.cancellations[execution.ID] = cancel
		err := s.persistLocked()
		s.mu.Unlock()
		s.changed()

		status, result := "error", ""
		if err == nil {
			status, result, err = s.runExecution(ctx, execution)
		}
		cancel()
		s.finish(execution.ID, status, result, err)
		s.activeExecutions.Add(-1)
	}
}

// finish 保存执行终态并通知等待者和原请求
func (s *Service) finish(id, status, result string, err error) {
	var notification Notification
	s.mu.Lock()
	delete(s.cancellations, id)
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	for i := range s.state.Executions {
		execution := &s.state.Executions[i]
		if execution.ID == id && execution.Status == "running" {
			execution.Status, execution.Result, execution.CompletedAt = status, result, now()
			if err != nil {
				execution.Error = err.Error()
			}
			body := result
			if body == "" {
				body = execution.Error
			}
			if body == "" {
				body = "任务已结束"
			}

			notification = Notification{
				ID:            rand.Text(),
				Title:         execution.Context.Step.Title + " · " + status,
				Body:          body,
				Source:        execution.Source,
				ProjectID:     execution.ProjectID,
				PipelineRunID: execution.PipelineRunID,
				ExecutionID:   id,
				RequestID:     execution.RequestID,
				CreatedAt:     now(),
				Parts:         []NotificationPart{{ID: rand.Text(), Kind: "text", Channel: "desktop", Status: "sent", Content: body}},
			}
			s.state.Notifications = append(s.state.Notifications, notification)
			for _, channel := range s.completed[id] {
				channel <- *execution
				close(channel)
			}
			delete(s.completed, id)
			if saveErr := s.persistLocked(); saveErr != nil {
				execution.Error += "；保存记录失败：" + saveErr.Error()
				s.state.Paused = true
			}
			break
		}
	}
	s.mu.Unlock()
	s.changed()
	s.notify(notification)
	s.reportExecution(id)
}

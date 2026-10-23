package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// API 按职责列出服务的全部公开方法
type API interface {
	GetState() pipelinemodel.State                                                                    // 读取流水线状态副本
	SaveTask(projectID, text string) (pipelinemodel.Task, error)                                      // 校验并保存当前项目的流水线任务
	DeleteTask(projectID, id string) error                                                            // 移除没有调度或活动运行的任务
	RunTask(input pipelinemodel.RunInput) (pipelinemodel.Run, error)                                  // 启动指定流水线任务
	ControlRun(projectID, id, operation string) (pipelinemodel.Run, error)                            // 执行受控运行命令
	StopRun(id string) error                                                                          // 取消指定流水线运行
	ConfigureTools(specs func(string) []codexmodel.DynamicToolSpec)                                   // 设置专用会话使用的宿主工具目录
	ServiceStartup(context.Context, application.ServiceOptions) error                                 // 启动调度检查与定时轮询
	PrepareShutdown() error                                                                           // 停止调度并取消运行与排队请求
	ServiceShutdown() error                                                                           // 取消所有任务并等待后台工作结束
	ResolveScheduleTime(local, timezone string) (string, error)                                       // 将指定时区的本地时间转换为 UTC
	SaveSchedule(input pipelinemodel.Schedule) (pipelinemodel.Schedule, error)                        // 校验并保存任务调度
	SetScheduleEnabled(projectID, id string, enabled bool) error                                      // 切换调度状态并恢复下次触发时间
	DeleteSchedule(projectID, id string) error                                                        // 移除当前项目的指定调度
	TriggerSchedule(projectID, id string) (pipelinemodel.Run, error)                                  // 手动触发调度并记录调度来源
	TriggerScheduleFor(projectID, id string, input pipelinemodel.RunInput) (pipelinemodel.Run, error) // 按指定来源触发调度对应的任务
	SubmitSession(input pipelinemodel.SessionInput) (pipelinemodel.SessionRequest, error)             // 向专用会话提交用户请求
	StopSession(id string) error                                                                      // 停止指定会话请求
	ResumeSessions() error                                                                            // 由用户恢复重启后保留的会话队列
	ActiveSession(threadID string) (pipelinemodel.SessionRequest, error)                              // 查找线程当前处理的专用会话请求
	GetActions() []pipelinemodel.ActionSpec                                                           // 读取流水线支持的固定动作目录
	ValidateTask(projectID, text string) (pipelinemodel.Task, error)                                  // 解析并校验当前项目的任务配置
}

var _ API = (*Service)(nil)

type Service struct {
	mu            sync.Mutex                                // 保护流水线状态与工作记录的互斥锁
	workerGroup   sync.WaitGroup                            // 退出时等待运行、会话与调度结束
	data          *appdata.Service                          // 应用配置服务
	work          *workspace.Service                        // 工作区项目与知识服务
	tasks         *tasks.Service                            // 步骤执行与通知服务
	files         *artifact.Service                         // 流水线产物服务
	runtime       *codex.Service                            // 原生 Codex 会话服务
	root          string                                    // 流水线状态保存目录
	state         pipelinemodel.State                       // 当前流水线状态
	ctx           context.Context                           // 服务生命周期上下文
	cancel        context.CancelFunc                        // 取消服务生命周期
	cancellations map[string]context.CancelFunc             // 按运行或请求标识保存的取消入口
	workers       map[string]bool                           // 按专用会话记录的队列运行状态
	toolSpecs     func(string) []codexmodel.DynamicToolSpec // 按会话角色提供的宿主工具目录
}

// NewService 初始化流水线服务并恢复已保存状态
func NewService(data *appdata.Service, work *workspace.Service, executions *tasks.Service, files *artifact.Service, runtime *codex.Service) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		data:          data,
		work:          work,
		tasks:         executions,
		files:         files,
		runtime:       runtime,
		root:          filepath.Join(data.GetInfo().DataDirectory, constant.PipelineDirectoryName),
		ctx:           ctx,
		cancel:        cancel,
		cancellations: map[string]context.CancelFunc{},
		workers:       map[string]bool{},
	}

	if err := appdata.ReadJSON(filepath.Join(s.root, constant.PipelineStateFilename), &s.state); err != nil {
		cancel()
		return nil, err
	}

	if s.state.Threads == nil {
		s.state.Threads = map[string]string{}
	}

	changed := false
	for i := range s.state.Runs {
		run := &s.state.Runs[i]
		if run.Status == constant.TasksStatusQueued || run.Status == constant.TasksStatusRunning {
			run.Status, run.Error, run.CompletedAt = constant.TasksStatusInterrupted, constant.PipelineRunInterruptedMessage, now()
			for j := range run.Actions {
				if run.Actions[j].Status == constant.TasksStatusRunning {
					run.Actions[j].Status, run.Actions[j].Error = constant.TasksStatusInterrupted, run.Error
				}
			}

			changed = true
		}
	}

	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.Status == constant.TasksStatusRunning {
			request.Status, request.Error, request.CompletedAt = constant.TasksStatusInterrupted, constant.PipelineSessionInterruptedMessage, now()
			changed = true
		}

		if request.Status == constant.TasksStatusQueued {
			s.state.Paused = true
			changed = true
		}
	}

	if changed {
		if err := s.persistLocked(); err != nil {
			cancel()
			return nil, err
		}
	}

	return s, nil
}

// GetState 读取流水线状态副本
func (s *Service) GetState() pipelinemodel.State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return clone(s.state)
}

// SaveTask 校验并保存当前项目的流水线任务
func (s *Service) SaveTask(projectID, text string) (pipelinemodel.Task, error) {
	task, err := s.ValidateTask(projectID, text)
	if err != nil {
		return task, err
	}

	if task.ID == "" {
		task.ID = rand.Text()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	next := clone(s.state)
	found := false
	for i, existing := range next.Tasks {
		if existing.ID == task.ID {
			if existing.ProjectID != projectID {
				return pipelinemodel.Task{}, constant.ErrPipelineTaskOutsideProject
			}

			next.Tasks[i] = task
			found = true
			break
		}
	}

	if !found {
		next.Tasks = append(next.Tasks, task)
	}

	if err = appdata.WriteJSON(filepath.Join(s.root, constant.PipelineStateFilename), next); err != nil {
		return pipelinemodel.Task{}, err
	}

	s.state = next
	s.changed()
	return clone(task), nil
}

// DeleteTask 移除没有调度或活动运行的任务
func (s *Service) DeleteTask(projectID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, schedule := range s.state.Schedules {
		if schedule.TaskID == id {
			return constant.ErrPipelineTaskHasSchedules
		}
	}

	for _, run := range s.state.Runs {
		if run.TaskID == id && activeRun(run.Status) {
			return constant.ErrPipelineRunActive
		}
	}

	next := clone(s.state)
	for i, task := range next.Tasks {
		if task.ID == id && task.ProjectID == projectID {
			next.Tasks = append(next.Tasks[:i], next.Tasks[i+1:]...)
			if err := appdata.WriteJSON(filepath.Join(s.root, constant.PipelineStateFilename), next); err != nil {
				return err
			}

			s.state = next
			s.changed()
			return nil
		}
	}

	return constant.ErrPipelineTaskNotFound
}

// RunTask 启动指定流水线任务
func (s *Service) RunTask(input pipelinemodel.RunInput) (pipelinemodel.Run, error) {
	return s.startRun(input, "")
}

// StopRun 取消指定流水线运行
func (s *Service) StopRun(id string) error {
	for _, run := range s.GetState().Runs {
		if run.ID == id {
			_, err := s.ControlRun(run.ProjectID, id, constant.PipelineControlStop)
			return err
		}
	}
	return constant.ErrPipelineRunNotFound
}

// ConfigureTools 设置专用会话使用的宿主工具目录
//
//wails:ignore
func (s *Service) ConfigureTools(specs func(string) []codexmodel.DynamicToolSpec) {
	s.toolSpecs = specs
}

// ServiceStartup 启动调度检查与定时轮询
func (s *Service) ServiceStartup(context.Context, application.ServiceOptions) error {
	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}

	s.workerGroup.Add(1)
	s.mu.Unlock()

	s.tick(time.Now().UTC(), true)
	go func() {
		defer s.workerGroup.Done()

		ticker := time.NewTicker(constant.PipelineSchedulePollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.tick(time.Now().UTC(), false)
			case <-s.ctx.Done():
				return
			}
		}
	}()
	return nil
}

// PrepareShutdown 停止调度并取消运行与排队请求
//
//wails:ignore
func (s *Service) PrepareShutdown() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cancel()
	for _, cancel := range s.cancellations {
		cancel()
	}

	for i := range s.state.Runs {
		run := &s.state.Runs[i]
		if run.Status == constant.TasksStatusQueued || run.Status == constant.TasksStatusRunning {
			status := constant.TasksStatusInterrupted
			if run.Status == constant.TasksStatusQueued {
				status = constant.TasksStatusCancelled
			}

			run.Status, run.Error, run.CompletedAt = status, constant.PipelineRunShutdownMessage, now()
			for j := range run.Actions {
				if run.Actions[j].Status == constant.TasksStatusRunning {
					run.Actions[j].Status, run.Actions[j].Error, run.Actions[j].CompletedAt = constant.TasksStatusInterrupted, run.Error, run.CompletedAt
				}
			}
		}
	}

	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.Status == constant.TasksStatusRunning || request.Status == constant.TasksStatusQueued {
			status := constant.TasksStatusInterrupted
			if request.Status == constant.TasksStatusQueued {
				status = constant.TasksStatusCancelled
			}

			request.Status, request.Error, request.CompletedAt = status, constant.PipelineSessionShutdownMessage, now()
		}
	}

	return s.persistLocked()
}

// ServiceShutdown 取消所有任务并等待后台工作结束
func (s *Service) ServiceShutdown() error {
	err := s.PrepareShutdown()
	s.workerGroup.Wait()
	return err
}

// init 注册流水线状态变化事件
func init() {
	application.RegisterEvent[pipelinemodel.Changed](constant.PipelineChangedEvent)
}

// now 返回当前 UTC 时间
func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// clone 复制可序列化状态，隔离调用方的修改
func clone[T any](value T) T {
	raw, _ := json.Marshal(value)
	var copy T
	json.Unmarshal(raw, &copy)
	return copy
}

// persistLocked 在持锁期间保存流水线状态
func (s *Service) persistLocked() error {
	return appdata.WriteJSON(filepath.Join(s.root, constant.PipelineStateFilename), s.state)
}

// changed 向界面通知流水线状态变化
func (s *Service) changed() {
	if s.ctx.Err() != nil {
		return
	}

	if app := application.Get(); app != nil {
		app.Event.Emit(constant.PipelineChangedEvent, pipelinemodel.Changed{})
	}
}

// project 读取当前工作区中的指定项目
func (s *Service) project(id string) (workspacemodel.Project, error) {
	for _, project := range s.work.GetState().Projects {
		if project.ID == id {
			return project, nil
		}
	}

	return workspacemodel.Project{}, constant.ErrPipelineProjectNotFound
}

// startRun 冻结任务参数并登记流水线运行
func (s *Service) startRun(input pipelinemodel.RunInput, scheduleID string) (pipelinemodel.Run, error) {
	s.mu.Lock()
	var task pipelinemodel.Task
	for _, item := range s.state.Tasks {
		if item.ID == input.TaskID {
			task = clone(item)
			break
		}
	}

	s.mu.Unlock()
	if task.ID == "" {
		return pipelinemodel.Run{}, constant.ErrPipelinePipelineTaskNotFound
	}

	if err := s.validateTask(task); err != nil {
		return pipelinemodel.Run{}, err
	}

	if err := validateInputs(task, input.Inputs); err != nil {
		return pipelinemodel.Run{}, err
	}

	model, effort, err := s.tasks.SelectModel(input.Model, input.Effort)
	if err != nil {
		return pipelinemodel.Run{}, err
	}

	if input.Source.Type == "" {
		input.Source.Type = constant.TasksSourceDesktop
	}

	run := pipelinemodel.Run{
		SessionRequestID: input.SessionRequestID,
		ID:               rand.Text(),
		ProjectID:        task.ProjectID,
		TaskID:           task.ID,
		Task:             task,
		Inputs:           clone(input.Inputs),
		Model:            model,
		Effort:           effort,
		Source:           input.Source,
		RequestID:        input.RequestID,
		ScheduleID:       scheduleID,
		Status:           constant.TasksStatusQueued,
		CreatedAt:        now(),
		Actions:          []pipelinemodel.ActionResult{},
		ArtifactIDs:      []string{},
	}

	s.mu.Lock()
	if err = s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return pipelinemodel.Run{}, err
	}

	for _, existing := range s.state.Runs {
		if existing.TaskID == task.ID && activeRun(existing.Status) {
			s.mu.Unlock()
			return pipelinemodel.Run{}, constant.ErrPipelineTaskAlreadyRunning
		}
	}

	// 模型选择后重新持锁确认任务仍然存在
	found := false
	for _, current := range s.state.Tasks {
		if current.ID == task.ID && current.ProjectID == task.ProjectID {
			found = true
			break
		}
	}

	if !found {
		s.mu.Unlock()
		return pipelinemodel.Run{}, constant.ErrPipelineTaskRemoved
	}

	s.state.Runs = append(s.state.Runs, run)
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancellations[run.ID] = cancel
	err = s.persistLocked()
	if err != nil {
		s.state.Runs = s.state.Runs[:len(s.state.Runs)-1]
		delete(s.cancellations, run.ID)
		cancel()
	}

	if err == nil {
		s.workerGroup.Add(1)
	}

	s.mu.Unlock()
	if err != nil {
		return pipelinemodel.Run{}, err
	}

	s.changed()
	go s.execute(ctx, run)
	return clone(run), nil
}

// updateRun 更新指定运行并保存状态
func (s *Service) updateRun(id string, change func(*pipelinemodel.Run)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ctx.Err(); err != nil {
		return err
	}

	for i := range s.state.Runs {
		if s.state.Runs[i].ID == id {
			next := clone(s.state)
			change(&next.Runs[i])
			if err := appdata.WriteJSON(filepath.Join(s.root, constant.PipelineStateFilename), next); err != nil {
				return err
			}
			s.state = next
			return nil
		}
	}

	return constant.ErrPipelineRunNotFound
}

// markdown 将文本或结构化结果转换为 Markdown
func markdown(value any) string {
	if text, ok := value.(string); ok {
		return text
	}

	raw, _ := json.MarshalIndent(value, "", "  ")
	return "```json\n" + string(raw) + "\n```\n"
}

// stringParam 读取并去除字符串参数两端空白
func stringParam(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return strings.TrimSpace(value)
}

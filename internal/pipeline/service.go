package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Service struct {
	mu            sync.Mutex                           // 保护流水线状态与工作记录的互斥锁
	workerGroup   sync.WaitGroup                       // 退出时等待运行、会话与调度结束
	data          *appdata.Service                     // 应用配置服务
	work          *workspace.Service                   // 工作区项目与知识服务
	tasks         *tasks.Service                       // 步骤执行与通知服务
	files         *artifact.Service                    // 流水线产物服务
	runtime       *codex.Service                       // 原生 Codex 会话服务
	root          string                               // 流水线状态保存目录
	state         State                                // 当前流水线状态
	ctx           context.Context                      // 服务生命周期上下文
	cancel        context.CancelFunc                   // 取消服务生命周期
	cancellations map[string]context.CancelFunc        // 按运行或请求标识保存的取消入口
	workers       map[string]bool                      // 按专用会话记录的队列运行状态
	toolSpecs     func(string) []codex.DynamicToolSpec // 按会话角色提供的宿主工具目录
}

type Changed struct{}

// NewService 初始化流水线服务并恢复已保存状态
func NewService(data *appdata.Service, work *workspace.Service, executions *tasks.Service, files *artifact.Service, runtime *codex.Service) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		data:          data,
		work:          work,
		tasks:         executions,
		files:         files,
		runtime:       runtime,
		root:          filepath.Join(data.GetInfo().DataDirectory, "pipelines"),
		ctx:           ctx,
		cancel:        cancel,
		cancellations: map[string]context.CancelFunc{},
		workers:       map[string]bool{},
	}

	if err := appdata.ReadJSON(filepath.Join(s.root, "state.json"), &s.state); err != nil {
		cancel()
		return nil, err
	}

	if s.state.Threads == nil {
		s.state.Threads = map[string]string{}
	}

	changed := false
	for i := range s.state.Runs {
		run := &s.state.Runs[i]
		if run.Status == "queued" || run.Status == "running" {
			run.Status, run.Error, run.CompletedAt = "interrupted", "应用退出时流水线尚未结束，未自动重放", now()
			for j := range run.Actions {
				if run.Actions[j].Status == "running" {
					run.Actions[j].Status, run.Actions[j].Error = "interrupted", run.Error
				}
			}

			changed = true
		}
	}

	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.Status == "running" {
			request.Status, request.Error, request.CompletedAt = "interrupted", "应用退出时会话请求尚未结束，未自动重放", now()
			changed = true
		}

		if request.Status == "queued" {
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
func (s *Service) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return clone(s.state)
}

// SaveTask 校验并保存当前项目的流水线任务
func (s *Service) SaveTask(projectID, text string) (Task, error) {
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
				return Task{}, errors.New("任务不属于此项目")
			}

			next.Tasks[i] = task
			found = true
			break
		}
	}

	if !found {
		next.Tasks = append(next.Tasks, task)
	}

	if err = appdata.WriteJSON(filepath.Join(s.root, "state.json"), next); err != nil {
		return Task{}, err
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
			return errors.New("请先移除此任务的调度")
		}
	}

	for _, run := range s.state.Runs {
		if run.TaskID == id && (run.Status == "running" || run.Status == "queued") {
			return errors.New("请先停止当前流水线")
		}
	}

	next := clone(s.state)
	for i, task := range next.Tasks {
		if task.ID == id && task.ProjectID == projectID {
			next.Tasks = append(next.Tasks[:i], next.Tasks[i+1:]...)
			if err := appdata.WriteJSON(filepath.Join(s.root, "state.json"), next); err != nil {
				return err
			}

			s.state = next
			s.changed()
			return nil
		}
	}

	return errors.New("任务不存在")
}

// RunTask 启动指定流水线任务
func (s *Service) RunTask(input RunInput) (Run, error) {
	return s.startRun(input, "")
}

// StopRun 取消指定流水线运行
func (s *Service) StopRun(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, run := range s.state.Runs {
		if run.ID == id {
			if cancel := s.cancellations[id]; cancel != nil {
				cancel()
			}

			return nil
		}
	}

	return errors.New("运行记录不存在")
}

// ConfigureTools 设置专用会话使用的宿主工具目录
//
//wails:ignore
func (s *Service) ConfigureTools(specs func(string) []codex.DynamicToolSpec) {
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

		ticker := time.NewTicker(time.Second)
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
		if run.Status == "queued" || run.Status == "running" {
			status := "interrupted"
			if run.Status == "queued" {
				status = "cancelled"
			}

			run.Status, run.Error, run.CompletedAt = status, "应用退出，流水线已取消", now()
			for j := range run.Actions {
				if run.Actions[j].Status == "running" {
					run.Actions[j].Status, run.Actions[j].Error, run.Actions[j].CompletedAt = "interrupted", run.Error, run.CompletedAt
				}
			}
		}
	}

	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.Status == "running" || request.Status == "queued" {
			status := "interrupted"
			if request.Status == "queued" {
				status = "cancelled"
			}

			request.Status, request.Error, request.CompletedAt = status, "应用退出，会话请求已取消", now()
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
	application.RegisterEvent[Changed]("pipeline:changed")
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
	return appdata.WriteJSON(filepath.Join(s.root, "state.json"), s.state)
}

// changed 向界面通知流水线状态变化
func (s *Service) changed() {
	if s.ctx.Err() != nil {
		return
	}

	if app := application.Get(); app != nil {
		app.Event.Emit("pipeline:changed", Changed{})
	}
}

// project 读取当前工作区中的指定项目
func (s *Service) project(id string) (workspace.Project, error) {
	for _, project := range s.work.GetState().Projects {
		if project.ID == id {
			return project, nil
		}
	}

	return workspace.Project{}, errors.New("工作区项目不存在")
}

// startRun 冻结任务参数并登记流水线运行
func (s *Service) startRun(input RunInput, scheduleID string) (Run, error) {
	s.mu.Lock()
	var task Task
	for _, item := range s.state.Tasks {
		if item.ID == input.TaskID {
			task = clone(item)
			break
		}
	}

	s.mu.Unlock()
	if task.ID == "" {
		return Run{}, errors.New("流水线任务不存在")
	}

	if err := s.validateTask(task); err != nil {
		return Run{}, err
	}

	if err := validateInputs(task, input.Inputs); err != nil {
		return Run{}, err
	}

	model, effort, err := s.tasks.SelectModel(input.Model, input.Effort)
	if err != nil {
		return Run{}, err
	}

	if input.Source.Type == "" {
		input.Source.Type = "desktop"
	}

	run := Run{
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
		Status:           "queued",
		CreatedAt:        now(),
		Actions:          []ActionResult{},
		ArtifactIDs:      []string{},
	}

	s.mu.Lock()
	if err = s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return Run{}, err
	}

	for _, existing := range s.state.Runs {
		if existing.TaskID == task.ID && (existing.Status == "queued" || existing.Status == "running") {
			s.mu.Unlock()
			return Run{}, errors.New("此任务已有活动运行，本次跳过")
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
		return Run{}, errors.New("流水线任务已移除")
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
		return Run{}, err
	}

	s.changed()
	go s.execute(ctx, run)
	return clone(run), nil
}

// updateRun 更新指定运行并保存状态
func (s *Service) updateRun(id string, change func(*Run)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ctx.Err(); err != nil {
		return err
	}

	for i := range s.state.Runs {
		if s.state.Runs[i].ID == id {
			change(&s.state.Runs[i])
			return s.persistLocked()
		}
	}

	return errors.New("运行记录不存在")
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

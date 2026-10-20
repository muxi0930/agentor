package tasks

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
	"agentor/internal/workspace"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Service struct {
	mu               sync.Mutex
	data             *appdata.Service
	runtime          *codex.Service
	work             *workspace.Service
	files            *artifact.Service
	path             string
	state            State
	ctx              context.Context
	cancel           context.CancelFunc
	workers          map[string]bool
	cancellations    map[string]context.CancelFunc
	turns            map[string]*turnWaiter
	completed        map[string][]chan Execution
	toolSpecs        func(string) []codex.DynamicToolSpec
	runExecution     func(context.Context, Execution) (string, string, error)
	controllerWorker bool
	runRequest       func(context.Context, Request) (string, string, error)
	deleting         map[string]bool
}

type Changed struct{}

func init() { application.RegisterEvent[Changed]("tasks:changed") }

func NewService(data *appdata.Service, runtime *codex.Service, work *workspace.Service, files *artifact.Service) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{data: data, runtime: runtime, work: work, files: files, path: filepath.Join(data.GetInfo().DataDirectory, "tasks.json"), ctx: ctx, cancel: cancel, workers: map[string]bool{}, cancellations: map[string]context.CancelFunc{}, turns: map[string]*turnWaiter{}, completed: map[string][]chan Execution{}, state: State{Executions: []Execution{}, Notifications: []Notification{}}}
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
			request.Status, request.Error, request.CompletedAt = "interrupted", "应用退出时控制器尚未结束；未自动重放", now()
			changed = true
		}
		if request.Status == "queued" {
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
		if err := s.persistLocked(); err != nil {
			cancel()
			return nil, err
		}
	}
	s.runExecution = s.execute
	s.runRequest = s.executeController
	s.deleting = map[string]bool{}
	return s, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func copyState(state State) State {
	raw, _ := json.Marshal(state)
	var result State
	json.Unmarshal(raw, &result)
	return result
}
func (s *Service) persistLocked() error { return appdata.WriteJSON(s.path, s.state) }
func (s *Service) GetState() State      { s.mu.Lock(); defer s.mu.Unlock(); return copyState(s.state) }
func (s *Service) changed() {
	if s.ctx.Err() != nil {
		return
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("tasks:changed", Changed{})
	}
}

//wails:ignore
func (s *Service) ConfigureTools(specs func(string) []codex.DynamicToolSpec) { s.toolSpecs = specs }

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
	execution := Execution{PipelineRunID: input.PipelineRunID, ID: rand.Text(), StepID: input.StepID, ProjectID: context.ProjectID, FeatureID: context.FeatureID, Instruction: input.Instruction, CompletionCriteria: input.CompletionCriteria, Model: model, Effort: effort, Permission: permission, Source: input.Source, RequestID: input.RequestID, Context: context, Status: "queued", CreatedAt: now(), TimeoutSeconds: input.TimeoutSeconds}
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
	s.kick(input.StepID)
	return execution, nil
}

func (s *Service) kick(stepID string) {
	s.mu.Lock()
	if s.state.Paused || s.workers[stepID] || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.workers[stepID] = true
	s.mu.Unlock()
	go s.runStepQueue(stepID)
}

func (s *Service) runStepQueue(stepID string) {
	for {
		s.mu.Lock()
		if s.state.Paused || s.ctx.Err() != nil || s.runtime != nil && s.runtime.GetStatus().Phase != "ready" {
			delete(s.workers, stepID)
			s.mu.Unlock()
			return
		}
		index := -1
		for i, execution := range s.state.Executions {
			if execution.StepID == stepID && execution.Status == "queued" {
				index = i
				break
			}
		}
		if index < 0 {
			delete(s.workers, stepID)
			s.mu.Unlock()
			return
		}
		s.state.Executions[index].Status, s.state.Executions[index].StartedAt = "running", now()
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
	}
}

func (s *Service) finish(id, status, result string, err error) {
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
			s.state.Notifications = append(s.state.Notifications, Notification{ID: rand.Text(), Title: execution.Context.Step.Title + " · " + status, Body: body, Source: execution.Source, ProjectID: execution.ProjectID, PipelineRunID: execution.PipelineRunID, ExecutionID: id, RequestID: execution.RequestID, CreatedAt: now(), Parts: []NotificationPart{{ID: rand.Text(), Kind: "text", Channel: "desktop", Status: "sent", Content: body}}})
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
	s.reportExecution(id)
}

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

func (s *Service) ResumeQueue() error {
	if s.runtime.GetStatus().Phase != "ready" {
		return errors.New("请先连接 Codex")
	}
	s.mu.Lock()
	s.state.Paused = false
	err := s.persistLocked()
	steps := map[string]bool{}
	for _, execution := range s.state.Executions {
		if execution.Status == "queued" {
			steps[execution.StepID] = true
		}
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	for id := range steps {
		s.kick(id)
	}
	s.kickController()
	s.changed()
	return nil
}

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

func (s *Service) ServiceShutdown() error {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cancel := range s.cancellations {
		cancel()
	}
	for i := range s.state.Executions {
		execution := &s.state.Executions[i]
		if execution.Status == "running" {
			execution.Status, execution.Error, execution.CompletedAt = "interrupted", "应用退出，未自动重放", now()
			for _, channel := range s.completed[execution.ID] {
				channel <- *execution
				close(channel)
			}
			delete(s.completed, execution.ID)
		}
	}
	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.Status == "running" {
			request.Status, request.Error, request.CompletedAt = "interrupted", "应用退出，未自动重放", now()
		}
	}
	return s.persistLocked()
}

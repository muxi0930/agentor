package tasks

import (
	"context"
	"crypto/rand"
	"time"

	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// RunManagedTurn 复用正式执行的事件归属和中断逻辑运行指定权限的轮次
//
//wails:ignore
func (s *Service) RunManagedTurn(ctx context.Context, threadID, text, model, effort, permission, cwd, id string, started func(string)) (string, string, error) {
	// 流水线 Agent 与普通步骤共用全局并发上限
	for {
		if err := ctx.Err(); err != nil {
			return constant.TasksStatusInterrupted, "", err
		}
		active := s.activeExecutions.Load()
		if active < int32(s.data.GetSettings().MaxConcurrentAgents) && s.activeExecutions.CompareAndSwap(active, active+1) {
			break
		}
		select {
		case <-ctx.Done():
			return constant.TasksStatusInterrupted, "", ctx.Err()
		case <-time.After(constant.TasksQueuePollInterval):
		}
	}
	defer s.activeExecutions.Add(-1)

	waiter := &turnWaiter{items: map[string]string{}, done: make(chan turnOutcome, 1)}
	s.mu.Lock()
	if s.turns[threadID] != nil {
		s.mu.Unlock()
		return constant.TasksStatusError, "", constant.ErrTasksSessionBusy
	}
	s.turns[threadID] = waiter
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.turns[threadID] == waiter {
			delete(s.turns, threadID)
		}
		s.mu.Unlock()
	}()
	if err := ctx.Err(); err != nil {
		return constant.TasksStatusInterrupted, "", err
	}
	turn, err := s.runtime.StartManagedTurn(threadID, text, model, effort, permission, cwd, id, nil, "")
	if err != nil {
		return constant.TasksStatusError, "", err
	}
	s.mu.Lock()
	waiter.turnID = turn.ID
	s.mu.Unlock()
	started(turn.ID)
	return s.awaitTurn(ctx, waiter, threadID)
}

// RecordNotification 持久化业务结果后通知发送入口
//
//wails:ignore
func (s *Service) RecordNotification(notification tasksmodel.Notification) error {
	if s.ctx.Err() != nil {
		return s.ctx.Err()
	}
	if notification.ID == "" {
		notification.ID = rand.Text()
	}
	if notification.CreatedAt == "" {
		notification.CreatedAt = now()
	}
	if len(notification.Parts) == 0 {
		notification.Parts = []tasksmodel.NotificationPart{{ID: rand.Text(), Kind: "text", Channel: constant.TasksSourceDesktop, Status: constant.TasksNotificationStatusSent, Content: notification.Body}}
	}
	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.state.Notifications = append(s.state.Notifications, notification)
	err := s.persistLocked()
	s.mu.Unlock()
	s.changed()
	if err == nil {
		s.notify(notification)
	}
	return err
}

// ReportExternalResult 将外部业务结果交还原始控制中心请求
//
//wails:ignore
func (s *Service) ReportExternalResult(requestID, kind, resultID, projectID, text string) error {
	if requestID == "" {
		return nil
	}
	s.mu.Lock()
	var original tasksmodel.Request
	for _, request := range s.state.Requests {
		if request.ID == requestID {
			original = request
			break
		}
	}
	s.mu.Unlock()
	if original.ID == "" {
		return constant.ErrTasksOriginalRequestNotFound
	}
	_, err := s.submitRequest(tasksmodel.ControllerInput{Text: text, ProjectID: projectID, Model: original.Model, Effort: original.Effort, FastMode: original.FastMode, Source: original.Source}, kind, resultID)
	return err
}

// RecordPipelineExecution 将流水线管理的尝试纳入项目步骤记录
//
//wails:ignore
func (s *Service) RecordPipelineExecution(execution tasksmodel.Execution, cancel context.CancelFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if execution.PipelineRunID == "" || execution.ID == "" || execution.StepID == "" {
		return constant.ErrTasksActiveThreadNotFound
	}
	next := copyState(s.state)
	found := false
	for i, previous := range next.Executions {
		if previous.ID == execution.ID {
			if previous.PipelineRunID != execution.PipelineRunID {
				return constant.ErrTasksActiveThreadNotFound
			}
			execution.StopRequested = previous.StopRequested
			next.Executions[i] = execution
			found = true
			break
		}
		if previous.FeatureID == execution.FeatureID && (previous.Status == constant.TasksStatusRunning || previous.Status == constant.TasksStatusQueued) {
			return constant.ErrTasksSessionBusy
		}
	}
	if !found {
		if s.deleting[execution.StepID] {
			return constant.ErrTasksSessionBusy
		}
		next.Executions = append(next.Executions, execution)
	}
	previous := s.state
	s.state = next
	if err := s.persistLocked(); err != nil {
		s.state = previous
		return err
	}
	if cancel != nil {
		s.cancellations[execution.ID] = cancel
	} else {
		delete(s.cancellations, execution.ID)
	}
	if execution.Status != constant.TasksStatusQueued && execution.Status != constant.TasksStatusRunning {
		for _, channel := range s.completed[execution.ID] {
			channel <- execution
			close(channel)
		}
		delete(s.completed, execution.ID)
	}
	s.changed()
	return nil
}

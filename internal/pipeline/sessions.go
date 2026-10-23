package pipeline

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
)

// SubmitSession 向专用会话提交用户请求
func (s *Service) SubmitSession(input pipelinemodel.SessionInput) (pipelinemodel.SessionRequest, error) {
	return s.submitSession(input, constant.TasksRequestKindInstruction, "")
}

// StopSession 停止指定会话请求
func (s *Service) StopSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.ID == id {
			if request.Status == constant.TasksStatusQueued {
				request.Status, request.Error, request.CompletedAt = constant.TasksStatusInterrupted, constant.PipelineQueueCancelledMessage, now()
				err := s.persistLocked()
				s.changed()
				return err
			}

			if cancel := s.cancellations[id]; cancel != nil {
				cancel()
			}

			return nil
		}
	}

	return constant.ErrPipelineSessionNotFound
}

// ResumeSessions 由用户恢复重启后保留的会话队列
func (s *Service) ResumeSessions() error {
	if s.runtime.GetStatus().Phase != constant.CodexPhaseReady {
		return constant.ErrPipelineConnectionRequired
	}

	s.mu.Lock()
	s.state.Paused = false
	err := s.persistLocked()
	pending := append([]pipelinemodel.SessionRequest{}, s.state.Requests...)
	s.mu.Unlock()
	if err != nil {
		return err
	}

	for _, request := range pending {
		if request.Status == constant.TasksStatusQueued {
			s.kickSession(request.Input.ProjectID, request.Input.Role)
		}
	}

	s.changed()
	return nil
}

// ActiveSession 查找线程当前处理的专用会话请求
//
//wails:ignore
func (s *Service) ActiveSession(threadID string) (pipelinemodel.SessionRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, request := range s.state.Requests {
		if request.ThreadID == threadID && request.Status == constant.TasksStatusRunning {
			return clone(request), nil
		}
	}

	return pipelinemodel.SessionRequest{}, constant.ErrPipelineActiveSessionNotFound
}

// sessionKey 区分同项目的管理与配置会话
func sessionKey(projectID, role string) string {
	return projectID + ":" + role
}

// submitSession 冻结参数并登记会话请求
func (s *Service) submitSession(input pipelinemodel.SessionInput, kind, runID string) (pipelinemodel.SessionRequest, error) {
	if input.Role != constant.ToolRouterRolePipelineManager && input.Role != constant.ToolRouterRolePipelineConfig {
		return pipelinemodel.SessionRequest{}, constant.ErrPipelineSessionRoleInvalid
	}

	if strings.TrimSpace(input.Text) == "" {
		return pipelinemodel.SessionRequest{}, constant.ErrPipelineSessionTextRequired
	}

	if kind == constant.TasksRequestKindInstruction && s.runtime.GetStatus().Phase != constant.CodexPhaseReady {
		return pipelinemodel.SessionRequest{}, constant.ErrPipelineConnectionRequired
	}

	if _, err := s.project(input.ProjectID); err != nil {
		return pipelinemodel.SessionRequest{}, err
	}

	model, effort, err := s.tasks.SelectModel(input.Model, input.Effort)
	if err != nil {
		return pipelinemodel.SessionRequest{}, err
	}

	input.Model, input.Effort = model, effort
	if input.Source.Type == "" {
		input.Source.Type = constant.TasksSourceDesktop
	}

	request := pipelinemodel.SessionRequest{
		ID:              rand.Text(),
		Input:           input,
		Status:          constant.TasksStatusQueued,
		CreatedAt:       now(),
		Kind:            kind,
		CompletionRunID: runID,
	}

	s.mu.Lock()
	if err = s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return pipelinemodel.SessionRequest{}, err
	}

	s.state.Requests = append(s.state.Requests, request)
	if s.runtime.GetStatus().Phase != constant.CodexPhaseReady {
		s.state.Paused = true
	}

	err = s.persistLocked()
	if err != nil {
		s.state.Requests = s.state.Requests[:len(s.state.Requests)-1]
	}

	s.mu.Unlock()
	if err != nil {
		return pipelinemodel.SessionRequest{}, err
	}

	s.changed()
	s.kickSession(input.ProjectID, input.Role)
	return request, nil
}

// kickSession 启动尚未运行的专用会话队列
func (s *Service) kickSession(projectID, role string) {
	key := sessionKey(projectID, role)
	s.mu.Lock()
	if s.workers[key] || s.state.Paused || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}

	s.workers[key] = true
	s.workerGroup.Add(1)
	s.mu.Unlock()
	go s.sessionQueue(projectID, role)
}

// sessionQueue 按顺序处理同一专用会话的请求
func (s *Service) sessionQueue(projectID, role string) {
	defer s.workerGroup.Done()

	key := sessionKey(projectID, role)
	for {
		s.mu.Lock()
		if s.ctx.Err() != nil || s.state.Paused || s.runtime.GetStatus().Phase != constant.CodexPhaseReady {
			delete(s.workers, key)
			s.mu.Unlock()
			return
		}

		index := -1
		for i, request := range s.state.Requests {
			if request.Input.ProjectID == projectID && request.Input.Role == role && request.Status == constant.TasksStatusQueued {
				index = i
				break
			}
		}

		if index < 0 {
			delete(s.workers, key)
			s.mu.Unlock()
			return
		}

		s.state.Requests[index].Status = constant.TasksStatusRunning
		request := clone(s.state.Requests[index])
		ctx, cancel := context.WithCancel(s.ctx)
		s.cancellations[request.ID] = cancel
		err := s.persistLocked()
		s.mu.Unlock()
		s.changed()
		status, result := constant.TasksStatusError, ""
		if err == nil {
			status, result, err = s.runSession(ctx, request)
		}

		cancel()
		s.mu.Lock()
		delete(s.cancellations, request.ID)
		if s.ctx.Err() != nil {
			delete(s.workers, key)
			s.mu.Unlock()
			return
		}

		for i := range s.state.Requests {
			current := &s.state.Requests[i]
			if current.ID == request.ID {
				current.Status, current.Result, current.CompletedAt = status, result, now()
				if err != nil {
					current.Error = err.Error()
				}

				break
			}
		}

		if saveErr := s.persistLocked(); saveErr != nil {
			s.state.Paused = true
		}

		s.mu.Unlock()
		s.changed()
		body := result
		if err != nil && body == "" {
			body = err.Error()
		}

		s.tasks.RecordNotification(tasksmodel.Notification{
			PipelineSessionID: request.ID,
			Title:             constant.PipelineSessionTitlePrefix + status,
			Body:              body,
			ProjectID:         projectID,
			Source:            request.Input.Source,
			RequestID:         request.Input.RequestID,
		})
		s.tasks.ReportExternalResult(request.Input.RequestID, constant.PipelineRequestKindSessionResult, request.ID, projectID, body)
	}
}

// runSession 恢复专用线程并等待实际运行结果
func (s *Service) runSession(ctx context.Context, request pipelinemodel.SessionRequest) (string, string, error) {
	input := request.Input
	project, err := s.project(input.ProjectID)
	if err != nil {
		return constant.TasksStatusError, "", err
	}

	key := sessionKey(input.ProjectID, input.Role)
	s.mu.Lock()
	threadID := s.state.Threads[key]
	s.mu.Unlock()
	if threadID == "" {
		var specs []codexmodel.DynamicToolSpec
		if s.toolSpecs != nil {
			specs = s.toolSpecs(input.Role)
		}

		instructions := constant.PipelineManagerInstructions
		if input.Role == constant.ToolRouterRolePipelineConfig {
			instructions = constant.PipelineConfigInstructions
		}

		thread, err := s.runtime.CreateToolThread(project.Directory, input.Model, constant.AppDataPermissionReadOnly, input.Role, instructions, specs)
		if err != nil {
			return constant.TasksStatusError, "", err
		}

		threadID = thread.ID
	} else {
		if err = s.runtime.EnsureOpen(threadID); err != nil {
			return constant.TasksStatusError, "", err
		}
	}

	if err = ctx.Err(); err != nil {
		return constant.TasksStatusInterrupted, "", err
	}

	s.mu.Lock()
	if err = s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return constant.TasksStatusInterrupted, "", err
	}

	s.state.Threads[key] = threadID
	for i := range s.state.Requests {
		if s.state.Requests[i].ID == request.ID {
			s.state.Requests[i].ThreadID = threadID
			break
		}
	}

	err = s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return constant.TasksStatusError, "", err
	}

	s.changed()
	return s.tasks.RunReadOnlyTurn(ctx, threadID, fmt.Sprintf(constant.PipelineSessionPromptTemplate, project.ID, project.Name, request.Kind, input.Text), input.Model, input.Effort, project.Directory, request.ID, func(turnID string) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.ctx.Err() != nil {
			return
		}

		for i := range s.state.Requests {
			if s.state.Requests[i].ID == request.ID {
				s.state.Requests[i].TurnID = turnID
				break
			}
		}

		s.persistLocked()
		s.changed()
	})
}

// reportRun 将流水线结果交还原始请求来源
func (s *Service) reportRun(run pipelinemodel.Run, text string) {
	if run.SessionRequestID == "" || s.ctx.Err() != nil {
		return
	}

	s.mu.Lock()
	var original pipelinemodel.SessionRequest
	for _, request := range s.state.Requests {
		if request.ID == run.SessionRequestID {
			original = clone(request)
			break
		}
	}

	s.mu.Unlock()
	if original.ID == "" {
		return
	}

	input := original.Input
	input.Text = text
	input.RequestID = ""
	s.submitSession(input, constant.PipelineRequestKindRunResult, run.ID)
}

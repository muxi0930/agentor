package pipeline

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"agentor/internal/codex"
	"agentor/internal/tasks"
)

func sessionKey(projectID, role string) string { return projectID + ":" + role }
func (s *Service) SubmitSession(input SessionInput) (SessionRequest, error) {
	return s.submitSession(input, "instruction", "")
}
func (s *Service) submitSession(input SessionInput, kind, runID string) (SessionRequest, error) {
	if input.Role != "pipeline_manager" && input.Role != "pipeline_config" {
		return SessionRequest{}, errors.New("请选择管理会话或配置会话")
	}
	if strings.TrimSpace(input.Text) == "" {
		return SessionRequest{}, errors.New("请输入会话请求")
	}
	if kind == "instruction" && s.runtime.GetStatus().Phase != "ready" {
		return SessionRequest{}, errors.New("请先连接 Codex")
	}
	if _, err := s.project(input.ProjectID); err != nil {
		return SessionRequest{}, err
	}
	model, effort, err := s.tasks.SelectModel(input.Model, input.Effort)
	if err != nil {
		return SessionRequest{}, err
	}
	input.Model, input.Effort = model, effort
	if input.Source.Type == "" {
		input.Source.Type = "desktop"
	}
	request := SessionRequest{ID: rand.Text(), Input: input, Status: "queued", CreatedAt: now(), Kind: kind, CompletionRunID: runID}
	s.mu.Lock()
	if err = s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return SessionRequest{}, err
	}
	s.state.Requests = append(s.state.Requests, request)
	if s.runtime.GetStatus().Phase != "ready" {
		s.state.Paused = true
	}
	err = s.persistLocked()
	if err != nil {
		s.state.Requests = s.state.Requests[:len(s.state.Requests)-1]
	}
	s.mu.Unlock()
	if err != nil {
		return SessionRequest{}, err
	}
	s.changed()
	s.kickSession(input.ProjectID, input.Role)
	return request, nil
}
func (s *Service) kickSession(projectID, role string) {
	key := sessionKey(projectID, role)
	s.mu.Lock()
	if s.workers[key] || s.state.Paused || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.workers[key] = true
	s.mu.Unlock()
	go s.sessionQueue(projectID, role)
}
func (s *Service) sessionQueue(projectID, role string) {
	key := sessionKey(projectID, role)
	for {
		s.mu.Lock()
		if s.ctx.Err() != nil || s.state.Paused || s.runtime.GetStatus().Phase != "ready" {
			delete(s.workers, key)
			s.mu.Unlock()
			return
		}
		index := -1
		for i, request := range s.state.Requests {
			if request.Input.ProjectID == projectID && request.Input.Role == role && request.Status == "queued" {
				index = i
				break
			}
		}
		if index < 0 {
			delete(s.workers, key)
			s.mu.Unlock()
			return
		}
		s.state.Requests[index].Status = "running"
		request := clone(s.state.Requests[index])
		ctx, cancel := context.WithCancel(s.ctx)
		s.cancellations[request.ID] = cancel
		err := s.persistLocked()
		s.mu.Unlock()
		s.changed()
		status, result := "error", ""
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
		s.tasks.RecordNotification(tasks.Notification{PipelineSessionID: request.ID, Title: "流水线会话 · " + status, Body: body, ProjectID: projectID, Source: request.Input.Source, RequestID: request.Input.RequestID})
		s.tasks.ReportExternalResult(request.Input.RequestID, "pipeline_session_result", request.ID, projectID, body)
	}
}
func (s *Service) runSession(ctx context.Context, request SessionRequest) (string, string, error) {
	input := request.Input
	project, err := s.project(input.ProjectID)
	if err != nil {
		return "error", "", err
	}
	key := sessionKey(input.ProjectID, input.Role)
	s.mu.Lock()
	threadID := s.state.Threads[key]
	s.mu.Unlock()
	if threadID == "" {
		var specs []codex.DynamicToolSpec
		if s.toolSpecs != nil {
			specs = s.toolSpecs(input.Role)
		}
		instructions := "你是 agentor 流水线管理助手。只用宿主工具查询固定动作、任务与记录，运行已有任务；不写脚本、不修改项目文件。输入引用仅为整个参数值的 {input:字段名}，结果引用仅为 {result:已完成动作ID}。"
		if input.Role == "pipeline_config" {
			instructions = "你是 agentor 流水线配置助手。通过宿主工具保存/删除任务和调度、验证 JSON 配置；不执行任务，不写脚本或修改项目文件。先读取固定动作目录。任务由 inputs 与顺序 actions 组成，只支持整个参数值的 {input:字段名}、{result:已完成动作ID} 引用。所有对象归当前项目。"
		}
		thread, err := s.runtime.CreateToolThread(project.Directory, input.Model, "read-only", input.Role, instructions, specs)
		if err != nil {
			return "error", "", err
		}
		threadID = thread.ID
	} else {
		if _, err = s.runtime.OpenThread(threadID); err != nil {
			return "error", "", err
		}
	}
	if err = ctx.Err(); err != nil {
		return "interrupted", "", err
	}
	s.mu.Lock()
	if err = s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return "interrupted", "", err
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
		return "error", "", err
	}
	s.changed()
	return s.tasks.RunReadOnlyTurn(ctx, threadID, fmt.Sprintf("项目 ID：%s\n项目名称：%s\n请求类型：%s（非 instruction 时只汇报完成结果，不重新执行）\n\n用户请求或完成事件：\n%s", project.ID, project.Name, request.Kind, input.Text), input.Model, input.Effort, project.Directory, request.ID, func(turnID string) {
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
func (s *Service) StopSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.ID == id {
			if request.Status == "queued" {
				request.Status, request.Error, request.CompletedAt = "interrupted", "用户取消排队", now()
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
	return errors.New("会话请求不存在")
}
func (s *Service) ResumeSessions() error {
	if s.runtime.GetStatus().Phase != "ready" {
		return errors.New("请先连接 Codex")
	}
	s.mu.Lock()
	s.state.Paused = false
	err := s.persistLocked()
	pending := append([]SessionRequest{}, s.state.Requests...)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	for _, request := range pending {
		if request.Status == "queued" {
			s.kickSession(request.Input.ProjectID, request.Input.Role)
		}
	}
	s.changed()
	return nil
}

//wails:ignore
func (s *Service) ActiveSession(threadID string) (SessionRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, request := range s.state.Requests {
		if request.ThreadID == threadID && request.Status == "running" {
			return clone(request), nil
		}
	}
	return SessionRequest{}, errors.New("此线程没有活动流水线会话")
}

func (s *Service) reportRun(run Run, text string) {
	if run.SessionRequestID == "" || s.ctx.Err() != nil {
		return
	}
	s.mu.Lock()
	var original SessionRequest
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
	s.submitSession(input, "run_result", run.ID)
}

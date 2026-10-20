package tasks

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agentor/internal/codex"
)

func sourceKey(source Source) string {
	raw, _ := json.Marshal([]string{source.Type, source.AppID, source.ChatID, source.SenderID})
	return string(raw)
}
func isContinue(text string) bool {
	text = strings.Trim(strings.ToLower(strings.TrimSpace(text)), "。.!！?？ ")
	return text == "继续" || text == "continue"
}

func (s *Service) SubmitRequest(input ControllerInput) (Request, error) {
	return s.submitRequest(input, "instruction", "")
}

func (s *Service) submitRequest(input ControllerInput, kind, completionID string) (Request, error) {
	if strings.TrimSpace(input.Text) == "" {
		return Request{}, errors.New("请填写控制器请求")
	}
	if input.Source.Type == "" {
		input.Source.Type = "desktop"
	}
	s.mu.Lock()
	for _, request := range s.state.Requests {
		if input.Source.MessageID != "" && sourceKey(request.Source) == sourceKey(input.Source) && request.Source.MessageID == input.Source.MessageID && request.Kind == kind && request.CompletionExecutionID == completionID {
			s.mu.Unlock()
			return request, nil
		}
	}
	s.mu.Unlock()
	if kind == "instruction" && s.runtime.GetStatus().Phase != "ready" {
		return Request{}, errors.New("Codex 尚未就绪")
	}
	needsTarget := false
	lockedTarget := input.StepID != "" || isContinue(input.Text)
	if kind == "instruction" {
		var err error
		input, needsTarget, err = s.resolveTarget(input)
		if err != nil {
			return Request{}, err
		}
	}
	model, effort, err := s.modelSelection(input.Model, input.Effort)
	if err != nil {
		return Request{}, err
	}
	request := Request{ID: rand.Text(), Kind: kind, Text: input.Text, ProjectID: input.ProjectID, StepID: input.StepID, Model: model, Effort: effort, Source: input.Source, Status: "queued", CreatedAt: now(), CompletionExecutionID: completionID, NeedsTarget: needsTarget, LockedTarget: lockedTarget}
	s.mu.Lock()
	// Two deliveries can pass the initial read while model selection is in flight.
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return Request{}, err
	}
	for _, existing := range s.state.Requests {
		if input.Source.MessageID != "" && sourceKey(existing.Source) == sourceKey(input.Source) && existing.Source.MessageID == input.Source.MessageID && existing.Kind == kind && existing.CompletionExecutionID == completionID {
			s.mu.Unlock()
			return existing, nil
		}
	}
	s.state.Requests = append(s.state.Requests, request)
	if kind == "execution_result" && s.runtime.GetStatus().Phase != "ready" {
		s.state.Paused = true
	}
	err = s.persistLocked()
	if err != nil {
		s.state.Requests = s.state.Requests[:len(s.state.Requests)-1]
	}
	s.mu.Unlock()
	if err != nil {
		return Request{}, err
	}
	s.changed()
	s.kickController()
	return request, nil
}

func (s *Service) kickController() {
	s.mu.Lock()
	if s.controllerWorker || s.state.Paused || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.controllerWorker = true
	s.mu.Unlock()
	go s.runControllerQueue()
}

func (s *Service) runControllerQueue() {
	for {
		s.mu.Lock()
		if s.state.Paused || s.ctx.Err() != nil || s.runtime != nil && s.runtime.GetStatus().Phase != "ready" {
			s.controllerWorker = false
			s.mu.Unlock()
			return
		}
		index := -1
		for i, request := range s.state.Requests {
			if request.Status == "queued" {
				index = i
				break
			}
		}
		if index < 0 {
			s.controllerWorker = false
			s.mu.Unlock()
			return
		}
		s.state.Requests[index].Status = "running"
		request := s.state.Requests[index]
		ctx, cancel := context.WithCancel(s.ctx)
		s.cancellations[request.ID] = cancel
		err := s.persistLocked()
		s.mu.Unlock()
		s.changed()
		status, result := "error", ""
		if err == nil {
			status, result, err = s.runRequest(ctx, request)
		}
		cancel()
		s.mu.Lock()
		delete(s.cancellations, request.ID)
		if s.ctx.Err() != nil {
			s.controllerWorker = false
			s.mu.Unlock()
			return
		}
		for i := range s.state.Requests {
			current := &s.state.Requests[i]
			if current.ID == request.ID && current.Status == "running" {
				current.Status, current.Result, current.CompletedAt = status, result, now()
				if err != nil {
					current.Error = err.Error()
				}
				body := result
				if body == "" {
					body = current.Error
				}
				s.state.Notifications = append(s.state.Notifications, Notification{ID: rand.Text(), Title: "控制器 · " + status, Body: body, RequestID: request.ID, CreatedAt: now(), Parts: []NotificationPart{{ID: rand.Text(), Kind: "text", Channel: "desktop", Status: "sent", Content: body}}})
				break
			}
		}
		if err := s.persistLocked(); err != nil {
			s.state.Paused = true
		}
		s.mu.Unlock()
		s.changed()
	}
}

func (s *Service) executeController(ctx context.Context, request Request) (string, string, error) {
	cwd := s.data.GetInfo().DataDirectory
	if request.ProjectID != "" {
		for _, project := range s.work.GetState().Projects {
			if project.ID == request.ProjectID {
				cwd = project.Directory
				break
			}
		}
	}
	s.mu.Lock()
	threadID := s.state.ControllerThreadID
	s.mu.Unlock()
	if threadID == "" {
		var tools []codex.DynamicToolSpec
		if s.toolSpecs != nil {
			tools = s.toolSpecs("controller")
		}
		thread, err := s.runtime.CreateToolThread(cwd, request.Model, "read-only", "controller", controllerInstructions, tools)
		if err != nil {
			return "error", "", err
		}
		threadID = thread.ID
		s.mu.Lock()
		s.state.ControllerThreadID = threadID
		s.mu.Unlock()
	} else {
		if _, err := s.runtime.OpenThread(threadID); err != nil {
			return "error", "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "interrupted", "", err
	}
	waiter := &turnWaiter{items: map[string]string{}, done: make(chan turnOutcome, 1)}
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return "interrupted", "", err
	}
	s.turns[threadID] = waiter
	for i := range s.state.Requests {
		if s.state.Requests[i].ID == request.ID {
			s.state.Requests[i].ThreadID = threadID
			break
		}
	}
	err := s.persistLocked()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.turns[threadID] == waiter {
			delete(s.turns, threadID)
		}
		s.mu.Unlock()
	}()
	if err != nil {
		return "error", "", err
	}
	s.changed()
	prompt := fmt.Sprintf("请求类型：%s\n请求 ID：%s\n项目范围：%s（空表示用户允许查询整个工作区）\n明确步骤目标：%s\n需要用户明确目标：%t\n\n用户请求或宿主完成事件：\n%s", request.Kind, request.ID, request.ProjectID, request.StepID, request.NeedsTarget, request.Text)
	turn, err := s.runtime.StartManagedTurn(threadID, prompt, request.Model, request.Effort, "read-only", cwd, request.ID, nil)
	if err != nil {
		return "error", "", err
	}
	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		s.runtime.StopTurn(threadID)
		return "interrupted", "", err
	}
	waiter.turnID = turn.ID
	for i := range s.state.Requests {
		if s.state.Requests[i].ID == request.ID {
			s.state.Requests[i].TurnID = turn.ID
			break
		}
	}
	err = s.persistLocked()
	s.mu.Unlock()
	s.changed()
	if err != nil {
		s.runtime.StopTurn(threadID)
		return "error", "", err
	}
	return s.awaitTurn(ctx, waiter, threadID)
}

func (s *Service) StopRequest(id string) error {
	s.mu.Lock()
	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.ID == id {
			if request.Status == "queued" {
				request.Status, request.Error, request.CompletedAt = "interrupted", "用户取消排队", now()
				err := s.persistLocked()
				s.mu.Unlock()
				s.changed()
				return err
			}
			if request.Status != "running" {
				s.mu.Unlock()
				return errors.New("请求已结束")
			}
			cancel := s.cancellations[id]
			s.mu.Unlock()
			if cancel != nil {
				cancel()
			}
			return nil
		}
	}
	s.mu.Unlock()
	return errors.New("请求不存在")
}

//wails:ignore
func (s *Service) CurrentRequest(threadID string) (Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, request := range s.state.Requests {
		if request.ThreadID == threadID && request.Status == "running" {
			return request, nil
		}
	}
	return Request{}, errors.New("此线程没有活动控制器请求")
}

//wails:ignore
func (s *Service) DispatchFromController(threadID, stepID, instruction, criteria string) (Execution, error) {
	request, err := s.CurrentRequest(threadID)
	if err != nil {
		return Execution{}, err
	}
	if request.Kind == "execution_result" {
		return Execution{}, errors.New("完成汇报不能重新派发任务")
	}
	if request.NeedsTarget {
		return Execution{}, errors.New("当前来源没有明确目标，请先询问用户")
	}
	if stepID == "" {
		stepID = request.StepID
	}
	if request.LockedTarget && request.StepID != "" && stepID != request.StepID {
		return Execution{}, errors.New("本次请求已明确指定步骤，请勿派发其他目标")
	}
	context, err := s.work.GetStepContext(stepID)
	if err != nil {
		return Execution{}, err
	}
	if request.ProjectID != "" && request.ProjectID != context.ProjectID {
		return Execution{}, errors.New("步骤不属于本次请求的项目范围")
	}
	return s.Dispatch(DispatchInput{StepID: stepID, Instruction: instruction, CompletionCriteria: criteria, Model: request.Model, Effort: request.Effort, Source: request.Source, RequestID: request.ID})
}

func (s *Service) reportExecution(id string) {
	s.mu.Lock()
	var execution Execution
	var original Request
	for _, item := range s.state.Executions {
		if item.ID == id {
			execution = item
			break
		}
	}
	for _, item := range s.state.Requests {
		if item.ID == execution.RequestID {
			original = item
			break
		}
	}
	s.mu.Unlock()
	if original.ID == "" || s.ctx.Err() != nil {
		return
	}
	text := fmt.Sprintf("执行 %s 已结束。步骤：%s；状态：%s。\n结果：\n%s\n错误：%s\n只向用户汇报结果，不再次执行。", execution.ID, execution.Context.Step.Title, execution.Status, execution.Result, execution.Error)
	s.submitRequest(ControllerInput{Text: text, ProjectID: execution.ProjectID, StepID: execution.StepID, Model: original.Model, Effort: original.Effort, Source: original.Source}, "execution_result", execution.ID)
}

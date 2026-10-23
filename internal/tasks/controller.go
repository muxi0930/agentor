package tasks

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// SubmitRequest 将用户指令加入控制中心队列
func (s *Service) SubmitRequest(input tasksmodel.ControllerInput) (tasksmodel.Request, error) {
	return s.submitRequest(input, constant.TasksRequestKindInstruction, "")
}

// StopRequest 取消排队或正在处理的控制中心请求
func (s *Service) StopRequest(id string) error {
	s.mu.Lock()
	for i := range s.state.Requests {
		request := &s.state.Requests[i]
		if request.ID == id {
			if request.Status == constant.TasksStatusQueued {
				request.Status, request.Error, request.CompletedAt = constant.TasksStatusInterrupted, constant.TasksQueueCancelledMessage, now()
				err := s.persistLocked()
				s.mu.Unlock()
				s.changed()
				return err
			}
			if request.Status != constant.TasksStatusRunning {
				s.mu.Unlock()
				return constant.ErrTasksRequestFinished
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
	return constant.ErrTasksRequestNotFound
}

// CurrentRequest 查找线程当前处理的控制中心请求
//
//wails:ignore
func (s *Service) CurrentRequest(threadID string) (tasksmodel.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, request := range s.state.Requests {
		if request.ThreadID == threadID && request.Status == constant.TasksStatusRunning {
			return request, nil
		}
	}
	return tasksmodel.Request{}, constant.ErrTasksActiveRequestNotFound
}

// DispatchFromController 从当前控制中心范围派发正式步骤
//
//wails:ignore
func (s *Service) DispatchFromController(threadID, stepID, instruction, criteria, model, effort string) (tasksmodel.Execution, error) {
	request, err := s.CurrentRequest(threadID)
	if err != nil {
		return tasksmodel.Execution{}, err
	}
	if request.Kind != constant.TasksRequestKindInstruction {
		return tasksmodel.Execution{}, constant.ErrTasksCompletionDispatchForbidden
	}
	if request.NeedsTarget {
		return tasksmodel.Execution{}, constant.ErrTasksTargetRequired
	}
	if stepID == "" {
		stepID = request.StepID
	}
	if request.LockedTarget && request.StepID != "" && stepID != request.StepID {
		return tasksmodel.Execution{}, constant.ErrTasksLockedTargetMismatch
	}
	context, err := s.work.GetStepContext(stepID)
	if err != nil {
		return tasksmodel.Execution{}, err
	}
	if request.ProjectID != "" && request.ProjectID != context.ProjectID {
		return tasksmodel.Execution{}, constant.ErrTasksStepOutsideRequestProject
	}

	model, effort, err = s.SelectExecutor(request, model, effort)
	if err != nil {
		return tasksmodel.Execution{}, err
	}
	return s.Dispatch(tasksmodel.DispatchInput{StepID: stepID, Instruction: instruction, CompletionCriteria: criteria, Model: model, Effort: effort, Source: request.Source, RequestID: request.ID})
}

// sourceKey 区分来源应用会话与发送者
func sourceKey(source tasksmodel.Source) string {
	raw, _ := json.Marshal([]string{source.Type, source.AppID, source.ChatID, source.SenderID})
	return string(raw)
}

// isContinue 识别需要沿用原任务目标的继续请求
func isContinue(text string) bool {
	text = strings.Trim(strings.ToLower(strings.TrimSpace(text)), "。.!！?？ ")
	return text == constant.TasksContinueCommand || text == "continue"
}

// submitRequest 保留来源并登记去重后的控制中心请求
func (s *Service) submitRequest(input tasksmodel.ControllerInput, kind, completionID string, reservedID ...string) (tasksmodel.Request, error) {
	if strings.TrimSpace(input.Text) == "" {
		return tasksmodel.Request{}, constant.ErrTasksRequestTextRequired
	}
	if input.Source.Type == "" {
		input.Source.Type = constant.TasksSourceDesktop
	}
	s.mu.Lock()
	for _, request := range s.state.Requests {
		if input.Source.MessageID != "" && sourceKey(request.Source) == sourceKey(input.Source) && request.Source.MessageID == input.Source.MessageID && request.Kind == kind && request.CompletionID == completionID {
			s.mu.Unlock()
			return request, nil
		}
	}
	s.mu.Unlock()
	if kind == constant.TasksRequestKindInstruction && s.runtime.GetStatus().Phase != constant.CodexPhaseReady {
		return tasksmodel.Request{}, constant.ErrTasksRuntimeNotReady
	}
	needsTarget := false
	lockedTarget := input.StepID != "" || isContinue(input.Text)
	if kind == constant.TasksRequestKindInstruction {
		var err error
		input, needsTarget, err = s.resolveTarget(input)
		if err != nil {
			return tasksmodel.Request{}, err
		}
	}
	model, effort, err := s.SelectModel(input.Model, input.Effort)
	if err != nil {
		return tasksmodel.Request{}, err
	}
	request := tasksmodel.Request{
		Executor:     s.executorSettings(model, effort),
		ID:           rand.Text(),
		Kind:         kind,
		Text:         input.Text,
		ProjectID:    input.ProjectID,
		StepID:       input.StepID,
		Model:        model,
		Effort:       effort,
		FastMode:     input.FastMode,
		Source:       input.Source,
		Status:       constant.TasksStatusQueued,
		CreatedAt:    now(),
		CompletionID: completionID,
		NeedsTarget:  needsTarget,
		LockedTarget: lockedTarget,
	}
	if len(reservedID) > 0 {
		request.ID = reservedID[0]
	}

	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return tasksmodel.Request{}, err
	}
	// 模型选择期间可能收到重复投递，在入队前持锁再次检查
	for _, existing := range s.state.Requests {
		if input.Source.MessageID != "" && sourceKey(existing.Source) == sourceKey(input.Source) && existing.Source.MessageID == input.Source.MessageID && existing.Kind == kind && existing.CompletionID == completionID {
			s.mu.Unlock()
			return existing, nil
		}
	}
	s.state.Requests = append(s.state.Requests, request)
	if kind != constant.TasksRequestKindInstruction && s.runtime.GetStatus().Phase != constant.CodexPhaseReady {
		s.state.Paused = true
	}
	err = s.persistLocked()
	if err != nil {
		s.state.Requests = s.state.Requests[:len(s.state.Requests)-1]
	}
	s.mu.Unlock()
	if err != nil {
		return tasksmodel.Request{}, err
	}
	s.changed()
	s.kickController()
	return request, nil
}

// kickController 启动尚未运行的控制中心队列
func (s *Service) kickController() {
	s.mu.Lock()
	if s.controllerWorker || s.state.Paused || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.controllerWorker = true
	s.workerGroup.Add(1)
	s.mu.Unlock()
	go s.runControllerQueue()
}

// runControllerQueue 按顺序处理控制中心请求
func (s *Service) runControllerQueue() {
	defer s.workerGroup.Done()

	for {
		s.mu.Lock()
		if s.state.Paused || s.ctx.Err() != nil || s.runtime != nil && s.runtime.GetStatus().Phase != constant.CodexPhaseReady {
			s.controllerWorker = false
			s.mu.Unlock()
			return
		}

		index := -1
		for i, request := range s.state.Requests {
			if request.Status == constant.TasksStatusQueued {
				index = i
				break
			}
		}
		if index < 0 {
			s.controllerWorker = false
			s.mu.Unlock()
			return
		}

		s.state.Requests[index].Status = constant.TasksStatusRunning
		request := s.state.Requests[index]
		ctx, cancel := context.WithCancel(s.ctx)
		s.cancellations[request.ID] = cancel
		err := s.persistLocked()
		s.mu.Unlock()
		s.changed()

		status, result := constant.TasksStatusError, ""
		if err == nil {
			status, result, err = s.runRequest(ctx, request)
		}
		cancel()

		var notification tasksmodel.Notification
		s.mu.Lock()
		delete(s.cancellations, request.ID)
		if s.ctx.Err() != nil {
			s.controllerWorker = false
			s.mu.Unlock()
			return
		}
		for i := range s.state.Requests {
			current := &s.state.Requests[i]
			if current.ID == request.ID && current.Status == constant.TasksStatusRunning {
				current.Status, current.Result, current.CompletedAt = status, result, now()
				if err != nil {
					current.Error = err.Error()
				}
				body := result
				if body == "" {
					body = current.Error
				}

				notification = tasksmodel.Notification{
					ID:        rand.Text(),
					Title:     constant.TasksControllerTitlePrefix + status,
					Body:      body,
					Source:    request.Source,
					ProjectID: request.ProjectID,
					RequestID: request.ID,
					CreatedAt: now(),
					Parts:     []tasksmodel.NotificationPart{{ID: rand.Text(), Kind: "text", Channel: constant.TasksSourceDesktop, Status: constant.TasksNotificationStatusSent, Content: body}},
				}
				s.state.Notifications = append(s.state.Notifications, notification)
				break
			}
		}
		if err := s.persistLocked(); err != nil {
			s.state.Paused = true
		}
		s.mu.Unlock()
		s.changed()
		s.notify(notification)
	}
}

// executeController 恢复控制中心线程并处理当前请求
func (s *Service) executeController(ctx context.Context, request tasksmodel.Request) (string, string, error) {
	cwd := s.data.GetInfo().CodexHome
	s.mu.Lock()
	threadID := s.state.ControllerThreadID
	previousInstructions := s.state.ControllerInstructions
	s.mu.Unlock()
	instructions := controllerPrompt(request.Executor)
	if threadID == "" || previousInstructions != instructions {
		var tools []codexmodel.DynamicToolSpec
		if s.toolSpecs != nil {
			tools = s.toolSpecs(constant.ToolRouterRoleController)
		}
		thread, err := s.runtime.CreateToolThread(cwd, request.Model, constant.AppDataPermissionReadOnly, constant.ToolRouterRoleController, instructions, tools)
		if err != nil {
			return constant.TasksStatusError, "", err
		}
		threadID = thread.ID
		s.mu.Lock()
		s.state.ControllerThreadID = threadID
		s.state.ControllerInstructions = instructions
		s.mu.Unlock()
	} else {
		if err := s.runtime.EnsureOpen(threadID); err != nil {
			return constant.TasksStatusError, "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return constant.TasksStatusInterrupted, "", err
	}
	waiter := &turnWaiter{items: map[string]string{}, done: make(chan turnOutcome, 1)}
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return constant.TasksStatusInterrupted, "", err
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
		return constant.TasksStatusError, "", err
	}
	s.changed()
	prompt := instructions + "\n\n" + fmt.Sprintf(constant.TasksControllerRequestTemplate, request.Kind, request.ID, request.ProjectID, request.StepID, request.NeedsTarget, request.Text)
	attachments, err := s.files.CopyRequestInputs(request.ID, threadID, "", "")
	if err != nil {
		return constant.TasksStatusError, "", err
	}
	tier := "default"
	if request.FastMode {
		tier = "fast"
	}
	turn, err := s.runtime.StartManagedTurn(threadID, prompt, request.Model, request.Effort, constant.AppDataPermissionReadOnly, cwd, request.ID, attachments, tier)
	if err != nil {
		return constant.TasksStatusError, "", err
	}
	s.mu.Lock()
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		s.runtime.StopTurn(threadID)
		return constant.TasksStatusInterrupted, "", err
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
		return constant.TasksStatusError, "", err
	}
	return s.awaitTurn(ctx, waiter, threadID)
}

// reportExecution 将步骤结果交还原始控制中心请求
func (s *Service) reportExecution(id string) {
	s.mu.Lock()
	var execution tasksmodel.Execution
	var original tasksmodel.Request
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
	text := fmt.Sprintf(constant.TasksCompletionPromptTemplate, execution.ID, execution.Context.Step.Title, execution.Status, execution.Result, execution.Error)
	s.submitRequest(tasksmodel.ControllerInput{Text: text, ProjectID: execution.ProjectID, StepID: execution.StepID, Model: original.Model, Effort: original.Effort, FastMode: original.FastMode, Source: original.Source}, constant.TasksRequestKindExecutionResult, execution.ID)
}

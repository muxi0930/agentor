package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentor/internal/codex"
)

type turnOutcome struct {
	status string
	result string
	err    error
}
type turnWaiter struct {
	turnID string
	items  map[string]string
	order  []string
	done   chan turnOutcome
}

func executionPrompt(execution Execution) string {
	context := execution.Context
	parts := []string{"# 正式步骤执行", "## 项目：" + context.ProjectName, context.SharedContext, context.ProjectPrompt, context.Structure, "## 版本：" + context.VersionName, "## 功能：" + context.FeatureTitle, context.FeatureDescription, context.FeatureContext, context.FeaturePrompt, "## 步骤：" + context.Step.Title, context.Step.Description, context.Step.Prompt, "## 本次执行指令", execution.Instruction}
	if execution.CompletionCriteria != "" {
		parts = append(parts, "## 完成标准", execution.CompletionCriteria)
	}
	if len(context.Documents) > 0 {
		parts = append(parts, "## 知识文档索引（需要正文时调用 read_knowledge）")
		for _, document := range context.Documents {
			parts = append(parts, fmt.Sprintf("- %s (documentId: %s)", document.Title, document.ID))
		}
	}
	return strings.Join(parts, "\n\n")
}

func (s *Service) execute(ctx context.Context, execution Execution) (string, string, error) {
	current, err := s.work.GetStepContext(execution.StepID)
	if err != nil {
		return "error", "", err
	}
	threadID := current.Step.ThreadID
	if threadID == "" {
		var tools []codex.DynamicToolSpec
		if s.toolSpecs != nil {
			tools = s.toolSpecs("step")
		}
		thread, err := s.runtime.CreateToolThread(execution.Context.Directory, execution.Model, execution.Permission, "step", stepInstructions, tools)
		if err != nil {
			return "error", "", err
		}
		threadID = thread.ID
		if err = ctx.Err(); err != nil {
			return "interrupted", "", err
		}
		if err = s.work.BindStepThread(execution.StepID, threadID); err != nil {
			return "error", "", err
		}
	} else {
		if _, err = s.runtime.OpenThread(threadID); err != nil {
			return "error", "", err
		}
	}
	if err = ctx.Err(); err != nil {
		return "interrupted", "", err
	}
	waiter := &turnWaiter{items: map[string]string{}, done: make(chan turnOutcome, 1)}
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return "interrupted", "", err
	}
	s.turns[threadID] = waiter
	for i := range s.state.Executions {
		if s.state.Executions[i].ID == execution.ID {
			s.state.Executions[i].ThreadID = threadID
			break
		}
	}
	err = s.persistLocked()
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
	turn, err := s.runtime.StartManagedTurn(threadID, executionPrompt(execution), execution.Model, execution.Effort, execution.Permission, execution.Context.Directory, execution.ID, nil)
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
	for i := range s.state.Executions {
		if s.state.Executions[i].ID == execution.ID {
			s.state.Executions[i].TurnID = turn.ID
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

func (s *Service) awaitTurn(ctx context.Context, waiter *turnWaiter, threadID string) (string, string, error) {
	select {
	case outcome := <-waiter.done:
		return outcome.status, outcome.result, outcome.err
	case <-ctx.Done():
		interruptErr := s.runtime.StopTurn(threadID)
		select {
		case outcome := <-waiter.done:
			if outcome.status == "completed" {
				return outcome.status, outcome.result, outcome.err
			}
			return "interrupted", outcome.result, ctx.Err()
		case <-time.After(15 * time.Second):
			s.mu.Lock()
			s.state.Paused = true
			s.persistLocked()
			s.mu.Unlock()
			if interruptErr != nil {
				return "interrupted", "", fmt.Errorf("停止请求失败，队列已暂停：%w", interruptErr)
			}
			return "interrupted", "", errors.New("未收到停止完成事件，队列已暂停；请检查连接后手动恢复")
		}
	}
}

//wails:ignore
func (s *Service) OnCodexEvent(event codex.Event) {
	if event.Method == "host/status" {
		var status codex.Status
		if json.Unmarshal([]byte(event.Params), &status) != nil || (status.Phase != "error" && status.Phase != "stopped") {
			return
		}
		s.mu.Lock()
		for _, waiter := range s.turns {
			select {
			case waiter.done <- turnOutcome{status: "interrupted", err: errors.New(status.Message)}:
			default:
			}
		}
		s.mu.Unlock()
		return
	}
	if event.Method != "turn/started" && event.Method != "turn/completed" && event.Method != "item/completed" {
		return
	}
	var params struct {
		ThreadID string     `json:"threadId"`
		TurnID   string     `json:"turnId"`
		Turn     codex.Turn `json:"turn"`
		Item     codex.Item `json:"item"`
	}
	if json.Unmarshal([]byte(event.Params), &params) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	waiter := s.turns[params.ThreadID]
	if waiter == nil {
		return
	}
	if event.Method == "turn/started" {
		waiter.turnID = params.Turn.ID
		return
	}
	if params.TurnID != "" && waiter.turnID != "" && params.TurnID != waiter.turnID {
		return
	}
	add := func(item codex.Item) {
		if item.Type == "agentMessage" && item.Phase != "commentary" {
			if _, found := waiter.items[item.ID]; !found {
				waiter.order = append(waiter.order, item.ID)
			}
			waiter.items[item.ID] = item.Text
		}
	}
	if event.Method == "item/completed" {
		add(params.Item)
		return
	}
	if waiter.turnID != "" && params.Turn.ID != waiter.turnID {
		return
	}
	for _, item := range params.Turn.Items {
		add(item)
	}
	texts := []string{}
	for _, id := range waiter.order {
		texts = append(texts, waiter.items[id])
	}
	outcome := turnOutcome{status: params.Turn.Status, result: strings.Join(texts, "\n\n")}
	if outcome.status == "failed" {
		outcome.status = "error"
	}
	if params.Turn.Error != nil {
		outcome.err = errors.New(params.Turn.Error.Message)
	}
	select {
	case waiter.done <- outcome:
	default:
	}
}

//wails:ignore
func (s *Service) ScopeForThread(threadID string) (Scope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, execution := range s.state.Executions {
		if execution.ThreadID == threadID && execution.Status == "running" {
			return Scope{Role: "step", ProjectID: execution.ProjectID, FeatureID: execution.FeatureID, StepID: execution.StepID, ExecutionID: execution.ID, RequestID: execution.RequestID, Directory: execution.Context.Directory}, nil
		}
	}
	for _, request := range s.state.Requests {
		if request.ThreadID == threadID && request.Status == "running" {
			return Scope{Role: "controller", ProjectID: request.ProjectID, RequestID: request.ID, StepID: request.StepID}, nil
		}
	}
	return Scope{}, errors.New("此线程没有活动的工作对象")
}

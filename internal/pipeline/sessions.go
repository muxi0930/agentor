package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
)

// CompleteNode 仅接收当前运行、节点与尝试所属轮次的最终汇报
//
//wails:ignore
func (s *Service) CompleteNode(call codexmodel.ToolCall) (any, error) {
	var report pipelinemodel.NodeReport
	decoder := json.NewDecoder(strings.NewReader(string(call.Arguments)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return nil, err
	}
	if strings.TrimSpace(report.Summary) == "" || (report.Outcome != constant.PipelineBranchSuccess && report.Outcome != constant.PipelineBranchFailure) {
		return nil, constant.ErrPipelineReportInvalid
	}
	if report.Outcome == constant.PipelineBranchFailure && report.Branch != "" {
		return nil, constant.ErrPipelineFailureBranchForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return nil, s.ctx.Err()
	}
	next := clone(s.state)
	for i := range next.Runs {
		run := &next.Runs[i]
		if run.Status != constant.TasksStatusRunning || len(run.Actions) == 0 {
			continue
		}
		record := &run.Actions[len(run.Actions)-1]
		if record.ThreadID != call.ThreadID || record.TurnID == "" || record.TurnID != call.TurnID || record.Status != constant.TasksStatusRunning || record.ID != run.CurrentNode {
			continue
		}
		if ctx := s.nodeContexts[call.ThreadID]; ctx == nil || ctx.Err() != nil {
			return nil, constant.ErrPipelineAttemptInactive
		}
		valid := report.Outcome == constant.PipelineBranchFailure
		for _, node := range run.Task.Actions {
			if node.ID == record.ID && node.Kind == constant.PipelineKindAgent {
				for _, branch := range node.Branches {
					if branch.Name == report.Branch {
						valid = true
					}
				}
			}
		}
		if !valid {
			return nil, fmt.Errorf("分支 %s 未在本节点声明", report.Branch)
		}
		if record.Report != nil {
			if reflect.DeepEqual(*record.Report, report) {
				return map[string]bool{"accepted": true}, nil
			}
			return nil, constant.ErrPipelineReportAlreadySubmitted
		}
		record.Report = &report
		if err := s.saveState(next); err != nil {
			return nil, err
		}
		s.changed()
		return map[string]bool{"accepted": true}, nil
	}
	return nil, constant.ErrPipelineReportOutsideAttempt
}

// performAgent 创建隔离的尝试会话，并在汇报与正常轮次结束均发生后返回
func (s *Service) performAgent(ctx context.Context, run pipelinemodel.Run, node pipelinemodel.Action, params map[string]any) (report pipelinemodel.NodeReport, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(node.TimeoutSeconds)*time.Second)
	defer cancel()
	directory := run.Directory
	if run.Worktree != nil {
		directory = run.Worktree.Directory
	}
	if err = ctx.Err(); err != nil {
		return report, err
	}
	if s.toolSpecs == nil {
		return report, constant.ErrPipelineToolsUnavailable
	}
	thread, err := s.runtime.CreateToolThread(directory, run.Model, node.Permission, constant.ToolRouterRolePipelineNode, constant.PipelineAgentInstructions, s.toolSpecs(constant.ToolRouterRolePipelineNode))
	if err != nil {
		return report, err
	}
	s.mu.Lock()
	s.nodeContexts[thread.ID] = ctx
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.nodeContexts, thread.ID); s.mu.Unlock() }()
	execution := tasksmodel.Execution{}
	if context, ok := run.StepContexts[node.ID]; ok {
		record := run.Actions[len(run.Actions)-1]
		execution = tasksmodel.Execution{ID: run.ID + ":" + node.ID + fmt.Sprintf(":%d", record.Attempt), PipelineRunID: run.ID, StepID: context.Step.ID, ProjectID: context.ProjectID, FeatureID: context.FeatureID, Context: context, Instruction: node.Prompt, CompletionCriteria: node.Criteria, Model: run.Model, Effort: run.Effort, Permission: node.Permission, Source: run.Source, Status: constant.TasksStatusRunning, ThreadID: thread.ID, CreatedAt: now(), StartedAt: now()}
		s.mu.Lock()
		stop := s.cancellations[run.ID]
		s.mu.Unlock()
		if err = s.tasks.RecordPipelineExecution(execution, stop); err != nil {
			return report, err
		}
		defer func() {
			execution.Status, execution.CompletedAt = constant.TasksStatusCompleted, now()
			execution.Result = markdown(report.Output)
			execution.OutcomeSummary = report.Summary
			execution.Outcome = constant.TasksStatusCompleted
			if err != nil || report.Outcome != constant.PipelineBranchSuccess {
				execution.Status, execution.Outcome = constant.TasksStatusError, constant.TasksStatusError
				if err != nil {
					execution.Error = err.Error()
				}
			}
			if ctx.Err() != nil {
				execution.Status = constant.TasksStatusInterrupted
			}
			if saveErr := s.tasks.RecordPipelineExecution(execution, nil); saveErr != nil && err == nil {
				err = saveErr
			}
		}()
	}
	if err = s.updateRun(run.ID, func(value *pipelinemodel.Run) {
		record := &value.Actions[len(value.Actions)-1]
		record.ThreadID = thread.ID
		record.ExecutionID = execution.ID
	}); err != nil {
		return report, err
	}
	previous := ""
	for _, record := range run.Actions {
		if record.ID == node.ID && record.Error != "" {
			previous = record.Error
		}
	}
	prompt := fmt.Sprintf(constant.PipelineAgentPrompt, node.ID, node.Prompt, node.Criteria, markdown(params), markdown(node.Branches), markdown(run.StepContexts[node.ID]), previous)
	var startErr error
	status, _, err := s.tasks.RunManagedTurn(ctx, thread.ID, prompt, run.Model, run.Effort, node.Permission, directory, run.ID, func(turnID string) {
		execution.TurnID = turnID
		startErr = s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Actions[len(value.Actions)-1].TurnID = turnID })
		if startErr != nil {
			cancel()
		}
	})
	if startErr != nil {
		return report, startErr
	}
	if err != nil {
		return report, err
	}
	if err = ctx.Err(); err != nil {
		return report, err
	}
	if status != constant.TasksStatusCompleted {
		return report, fmt.Errorf("Agent 轮次未正常结束：%s", status)
	}
	for _, current := range s.GetState().Runs {
		if current.ID == run.ID {
			record := current.Actions[len(current.Actions)-1]
			if record.Report != nil {
				return *record.Report, nil
			}
		}
	}
	return report, constant.ErrPipelineReportMissing
}

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
)

// execute 每次只执行一个节点，持久化结果与下一位置后再继续
func (s *Service) execute(ctx context.Context, run pipelinemodel.Run) {
	defer s.workerGroup.Done()
	defer func() {
		s.mu.Lock()
		release := s.versionReleases[run.ID]
		delete(s.versionReleases, run.ID)
		if cancel := s.cancellations[run.ID]; cancel != nil {
			cancel()
			delete(s.cancellations, run.ID)
		}
		s.mu.Unlock()
		if release != nil {
			release()
		}
		s.changed()
	}()
	err := s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Status = constant.TasksStatusRunning })
	nodes := map[string]pipelinemodel.Action{}
	for _, node := range run.Task.Actions {
		nodes[node.ID] = node
	}
	results := map[string]any{}
	for _, record := range run.Actions {
		// 失败处理分支也可以引用失败节点的输出，恢复时保留最后一次已结束尝试
		if record.Status == constant.TasksStatusCompleted || record.Status == constant.TasksStatusError {
			results[record.ID] = record.Output
		}
	}
	for err == nil {
		if err = ctx.Err(); err != nil {
			break
		}
		var paused bool
		paused, err = s.pauseAtBoundary(run.ID)
		if err != nil {
			break
		}
		if paused {
			return
		}
		node, exists := nodes[run.CurrentNode]
		if !exists {
			err = fmt.Errorf("执行位置 %s 不存在", run.CurrentNode)
			break
		}
		if node.Kind == constant.PipelineKindEnd {
			if node.Type == constant.PipelineEndFailure {
				err = fmt.Errorf("到达失败终点 %s", node.ID)
			}
			break
		}
		params, resolveErr := resolveParams(node.Params, run.Inputs, results)
		attempt := 1
		for _, previous := range run.Actions {
			if previous.ID == node.ID {
				attempt++
			}
		}
		record := pipelinemodel.ActionResult{ID: node.ID, Kind: node.Kind, Type: node.Type, Attempt: attempt, Inputs: params, Status: constant.TasksStatusRunning, StartedAt: now()}
		err = s.updateRun(run.ID, func(value *pipelinemodel.Run) {
			value.Actions = append(value.Actions, clone(record))
			run = clone(*value)
		})
		if err != nil {
			break
		}
		s.changed()
		report := pipelinemodel.NodeReport{}
		actionErr := resolveErr
		if actionErr == nil {
			if node.Kind == constant.PipelineKindAgent {
				if node.Type == constant.PipelineActionResolveConflicts {
					if run.Worktree == nil {
						actionErr = constant.ErrPipelineWorktreeMissing
					} else {
						params["worktree"] = run.Worktree
					}
				}
				if actionErr == nil {
					report, actionErr = s.runAgent(ctx, run, node, params)
				}
				// 冲突解决后复用静态合并检查与状态登记，使后续清理能够继续
				if actionErr == nil && node.Type == constant.PipelineActionResolveConflicts && report.Outcome == constant.PipelineBranchSuccess {
					var merged any
					merged, actionErr = s.performWorktree(ctx, run, constant.PipelineActionMergeWorktree)
					if pending, ok := merged.(pipelinemodel.NodeReport); actionErr == nil && ok {
						actionErr = fmt.Errorf("冲突解决后仍无法合并：%s", pending.Branch)
					}
				}
			} else {
				report, actionErr = s.performStatic(ctx, run, node, params)
			}
		}
		if actionErr == nil && report.Outcome == constant.PipelineBranchFailure {
			actionErr = fmt.Errorf("%s", report.Summary)
		}
		if ctx.Err() != nil {
			actionErr = ctx.Err()
		}
		target := ""
		retry := false
		if actionErr != nil && ctx.Err() == nil && node.Kind == constant.PipelineKindAgent {
			retry = attempt <= node.MaxRetries
			if retry {
				target = node.ID
			} else {
				target = node.FailureTarget
			}
		} else if actionErr == nil {
			for _, branch := range node.Branches {
				if branch.Name == report.Branch {
					target = branch.Target
					break
				}
			}
			if target == "" {
				actionErr = fmt.Errorf("节点 %s 的结果 %s 没有连接处理分支", node.ID, report.Branch)
			}
		}
		record.Output, record.Branch, record.CompletedAt = report.Output, report.Branch, now()
		record.Status = constant.TasksStatusCompleted
		if actionErr != nil {
			record.Status, record.Error = constant.TasksStatusError, actionErr.Error()
		}
		if ctx.Err() != nil {
			record.Status = constant.TasksStatusInterrupted
		}
		err = s.updateRun(run.ID, func(value *pipelinemodel.Run) {
			previous := value.Actions[len(value.Actions)-1]
			record.ThreadID, record.TurnID, record.Report, record.ExecutionID = previous.ThreadID, previous.TurnID, previous.Report, previous.ExecutionID
			value.Actions[len(value.Actions)-1] = clone(record)
			if target != "" {
				value.CurrentNode = target
			}
			if node.Type == constant.PipelineActionWriteReport && actionErr == nil {
				if output, ok := report.Output.(map[string]any); ok {
					if id, ok := output["artifactId"].(string); ok {
						value.ArtifactIDs = append(value.ArtifactIDs, id)
					}
				}
			}
			run = clone(*value)
		})
		s.changed()
		if err != nil {
			break
		}
		if actionErr != nil && (target == "" || ctx.Err() != nil) {
			err = actionErr
			break
		}
		if !retry {
			results[node.ID] = report.Output
		}
	}
	status := constant.TasksStatusCompleted
	if err != nil {
		status = constant.TasksStatusError
	}
	if ctx.Err() != nil {
		status = constant.TasksStatusCancelled
		err = ctx.Err()
	}
	finishErr := s.updateRun(run.ID, func(value *pipelinemodel.Run) {
		value.PauseRequested = false
		value.Status, value.CompletedAt = status, now()
		if err != nil {
			value.Error = err.Error()
		}
		run = clone(*value)
	})
	if finishErr != nil || s.ctx.Err() != nil {
		return
	}
	s.changed()
	var summary strings.Builder
	fmt.Fprintf(&summary, constant.PipelineReportHeaderTemplate, run.Task.Name, status)
	for _, node := range run.Actions {
		fmt.Fprintf(&summary, "- %s (%s)：%s → %s\n", node.ID, node.Type, node.Status, node.Branch)
	}
	if err != nil {
		fmt.Fprintf(&summary, constant.PipelineReportErrorTemplate, err)
	}
	for _, id := range run.ArtifactIDs {
		fmt.Fprintf(&summary, constant.PipelineReportArtifactsTemplate, id)
	}
	s.tasks.RecordNotification(tasksmodel.Notification{Title: run.Task.Name + " · " + status, Body: summary.String(), PipelineRunID: run.ID, ProjectID: run.ProjectID, Source: run.Source, RequestID: run.RequestID})
	s.tasks.ReportExternalResult(run.RequestID, constant.TasksRequestKindPipelineResult, run.ID, run.ProjectID, summary.String())
}

// performStatic 将 Go 动作结果统一为业务分支，不重试任何异常
func (s *Service) performStatic(ctx context.Context, run pipelinemodel.Run, node pipelinemodel.Action, params map[string]any) (pipelinemodel.NodeReport, error) {
	for _, spec := range s.GetActions() {
		if spec.Name == node.Type {
			if err := validateParameters(spec, params, false); err != nil {
				return pipelinemodel.NodeReport{}, err
			}
		}
	}
	output, err := s.performValue(ctx, run, node.Type, params)
	if err != nil {
		return pipelinemodel.NodeReport{}, err
	}
	if report, ok := output.(pipelinemodel.NodeReport); ok {
		return report, nil
	}
	// 转成 JSON 值，使对象字段引用不依赖 Go 的具体结构体类型
	raw, err := json.Marshal(output)
	if err != nil {
		return pipelinemodel.NodeReport{}, err
	}
	var value any
	if err = json.Unmarshal(raw, &value); err != nil {
		return pipelinemodel.NodeReport{}, err
	}
	return pipelinemodel.NodeReport{Outcome: constant.PipelineBranchSuccess, Branch: constant.PipelineBranchSuccess, Output: value}, nil
}

// performValue 调用当前固定动作并返回其结果
func (s *Service) performValue(ctx context.Context, run pipelinemodel.Run, kind string, params map[string]any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	switch kind {
	case constant.PipelineActionCreateWorktree, constant.PipelineActionMergeWorktree, constant.PipelineActionRemoveWorktree:
		return s.performWorktree(ctx, run, kind)
	case constant.PipelineActionSnapshot:
		project, err := s.project(run.ProjectID)
		if err != nil {
			return nil, err
		}

		versionID, featureID := stringParam(params, "versionId"), stringParam(params, "featureId")
		versions := project.Versions[:0]
		matchedVersion, matchedFeature := versionID == "", featureID == ""
		for _, version := range project.Versions {
			if versionID != "" && version.ID != versionID {
				continue
			}

			matchedVersion = true
			features := version.Features[:0]
			for _, feature := range version.Features {
				if featureID != "" && feature.ID != featureID {
					continue
				}

				matchedFeature = true
				features = append(features, feature)
			}

			version.Features = features
			versions = append(versions, version)
		}

		project.Versions = versions
		if !matchedVersion || !matchedFeature {
			return nil, constant.ErrPipelineNodeOutsideProject
		}

		states := map[string]string{}
		for _, execution := range s.tasks.GetState().Executions {
			if execution.ProjectID == run.ProjectID {
				states[execution.StepID] = execution.Status
			}
		}

		return map[string]any{"project": project, "stepStatuses": states}, nil
	case constant.PipelineActionReadKnowledge:
		return s.work.ReadKnowledge(run.ProjectID, stringParam(params, "featureId"), stringParam(params, "documentId"))
	case constant.PipelineActionWriteReport:
		name := stringParam(params, "fileName")
		if err := reportName(name); err != nil {
			return nil, err
		}

		item, err := s.files.SavePipelineReport(run.ID, run.ProjectID, run.RequestID, name, markdown(params["content"]))
		if err != nil {
			return nil, err
		}

		return map[string]any{"artifactId": item.ID, "fileName": item.Name}, nil
	}

	return nil, constant.ErrPipelineActionUnknown
}

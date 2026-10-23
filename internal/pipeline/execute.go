package pipeline

import (
	"context"
	"fmt"
	"strings"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
)

// execute 执行当前任务并保存动作或轮次结果
func (s *Service) execute(ctx context.Context, run pipelinemodel.Run) {
	defer s.workerGroup.Done()

	err := s.updateRun(run.ID, func(record *pipelinemodel.Run) { record.Status = constant.TasksStatusRunning })
	s.changed()
	results := map[string]any{}
	for _, action := range run.Task.Actions {
		if err != nil {
			break
		}

		if err = ctx.Err(); err != nil {
			break
		}

		var params map[string]any
		params, err = resolveParams(action.Params, run.Inputs, results)
		if err != nil {
			break
		}

		record := pipelinemodel.ActionResult{ID: action.ID, Type: action.Type, Status: constant.TasksStatusRunning, Inputs: params, StartedAt: now()}
		err = s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Actions = append(value.Actions, record) })
		s.changed()
		if err != nil {
			break
		}

		var output any
		output, err = s.perform(ctx, run, action.Type, params)
		record.Output, record.CompletedAt = output, now()
		record.Status = constant.TasksStatusCompleted
		if err != nil {
			record.Status = constant.TasksStatusError
			record.Error = err.Error()
			if ctx.Err() != nil {
				record.Status = constant.TasksStatusInterrupted
			}
		}

		saveErr := s.updateRun(run.ID, func(value *pipelinemodel.Run) {
			value.Actions[len(value.Actions)-1] = record
			if action.Type == constant.PipelineActionWriteReport && err == nil {
				value.ArtifactIDs = append(value.ArtifactIDs, output.(map[string]any)["artifactId"].(string))
			}
		})
		s.changed()
		if err == nil {
			err = saveErr
		}

		results[action.ID] = output
	}

	status := constant.TasksStatusCompleted
	if err != nil {
		status = constant.TasksStatusError
		if ctx.Err() != nil {
			status = constant.TasksStatusInterrupted
		}
	}

	s.updateRun(run.ID, func(value *pipelinemodel.Run) {
		value.Status, value.CompletedAt = status, now()
		if err != nil {
			value.Error = err.Error()
		}

		run = clone(*value)
	})
	s.mu.Lock()
	if cancel := s.cancellations[run.ID]; cancel != nil {
		cancel()
		delete(s.cancellations, run.ID)
	}

	s.mu.Unlock()
	if s.ctx.Err() != nil {
		return
	}

	s.changed()
	var summary strings.Builder
	fmt.Fprintf(&summary, constant.PipelineReportHeaderTemplate, run.Task.Name, status)
	for _, action := range run.Actions {
		fmt.Fprintf(&summary, "- %s (%s)：%s\n", action.ID, action.Type, action.Status)
	}

	if err != nil {
		fmt.Fprintf(&summary, constant.PipelineReportErrorTemplate, err)
	}

	for _, id := range run.ArtifactIDs {
		fmt.Fprintf(&summary, constant.PipelineReportArtifactsTemplate, id)
	}

	s.tasks.RecordNotification(tasksmodel.Notification{
		Title:         run.Task.Name + " · " + status,
		Body:          summary.String(),
		PipelineRunID: run.ID,
		ProjectID:     run.ProjectID,
		Source:        run.Source,
		RequestID:     run.RequestID,
	})
	s.tasks.ReportExternalResult(run.RequestID, constant.TasksRequestKindPipelineResult, run.ID, run.ProjectID, summary.String())
	s.reportRun(run, summary.String())
}

// perform 调用当前固定动作并返回其结果
func (s *Service) perform(ctx context.Context, run pipelinemodel.Run, kind string, params map[string]any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	switch kind {
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
	case constant.PipelineActionRunStep:
		stepContext, err := s.work.GetStepContext(stringParam(params, "stepId"))
		if err != nil {
			return nil, err
		}

		if stepContext.ProjectID != run.ProjectID {
			return nil, constant.ErrPipelineStepOutsidePipelineProject
		}

		if err = ctx.Err(); err != nil {
			return nil, err
		}

		execution, err := s.tasks.Dispatch(tasksmodel.DispatchInput{
			AttachmentRequestID: run.RequestID,
			StepID:              stepContext.Step.ID,
			Instruction:         stringParam(params, "instruction"),
			CompletionCriteria:  stringParam(params, "completionCriteria"),
			Model:               run.Model,
			Effort:              run.Effort,
			Source:              run.Source,
			PipelineRunID:       run.ID,
		})
		if err != nil {
			return nil, err
		}

		result, err := s.tasks.WaitExecution(ctx, execution.ID)
		if err != nil {
			s.tasks.StopExecution(execution.ID)
			cleanup, cancel := context.WithTimeout(s.ctx, constant.PipelineActionCleanupTimeout)
			defer cancel()
			s.tasks.WaitExecution(cleanup, execution.ID)
			return nil, err
		}

		ids := []string{}
		for _, file := range s.files.GetExecutionArtifacts(result.ID) {
			ids = append(ids, file.ID)
		}

		output := map[string]any{"executionId": result.ID, "status": result.Status, "result": result.Result, "artifactIds": ids}
		if result.Status != constant.TasksStatusCompleted {
			return output, fmt.Errorf("步骤 %s：%s %s", result.Context.Step.Title, result.Status, result.Error)
		}

		return output, nil
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

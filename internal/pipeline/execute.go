package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentor/internal/tasks"
)

func (s *Service) execute(ctx context.Context, run Run) {
	err := s.updateRun(run.ID, func(record *Run) { record.Status = "running" })
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
		record := ActionResult{ID: action.ID, Type: action.Type, Status: "running", Inputs: params, StartedAt: now()}
		err = s.updateRun(run.ID, func(value *Run) { value.Actions = append(value.Actions, record) })
		s.changed()
		if err != nil {
			break
		}
		var output any
		output, err = s.perform(ctx, run, action.Type, params)
		record.Output, record.CompletedAt = output, now()
		record.Status = "completed"
		if err != nil {
			record.Status = "error"
			record.Error = err.Error()
			if ctx.Err() != nil {
				record.Status = "interrupted"
			}
		}
		saveErr := s.updateRun(run.ID, func(value *Run) {
			value.Actions[len(value.Actions)-1] = record
			if action.Type == "report.write" && err == nil {
				value.ArtifactIDs = append(value.ArtifactIDs, output.(map[string]any)["artifactId"].(string))
			}
		})
		s.changed()
		if err == nil {
			err = saveErr
		}
		results[action.ID] = output
	}
	status := "completed"
	if err != nil {
		status = "error"
		if ctx.Err() != nil {
			status = "interrupted"
		}
	}
	s.updateRun(run.ID, func(value *Run) {
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
	fmt.Fprintf(&summary, "流水线 **%s**：%s\n\n", run.Task.Name, status)
	for _, action := range run.Actions {
		fmt.Fprintf(&summary, "- %s (%s)：%s\n", action.ID, action.Type, action.Status)
	}
	if err != nil {
		fmt.Fprintf(&summary, "\n错误：%s\n", err)
	}
	for _, id := range run.ArtifactIDs {
		fmt.Fprintf(&summary, "\n报告产物：%s\n", id)
	}
	s.tasks.RecordNotification(tasks.Notification{Title: run.Task.Name + " · " + status, Body: summary.String(), PipelineRunID: run.ID, ProjectID: run.ProjectID, Source: run.Source, RequestID: run.RequestID})
	s.tasks.ReportExternalResult(run.RequestID, "pipeline_result", run.ID, run.ProjectID, summary.String())
	s.reportRun(run, summary.String())
}

func (s *Service) perform(ctx context.Context, run Run, kind string, params map[string]any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch kind {
	case "workspace.snapshot":
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
			return nil, errors.New("版本或功能不属于当前项目")
		}
		states := map[string]string{}
		for _, execution := range s.tasks.GetState().Executions {
			if execution.ProjectID == run.ProjectID {
				states[execution.StepID] = execution.Status
			}
		}
		return map[string]any{"project": project, "stepStatuses": states}, nil
	case "knowledge.read":
		return s.work.ReadKnowledge(run.ProjectID, stringParam(params, "featureId"), stringParam(params, "documentId"))
	case "step.run":
		stepContext, err := s.work.GetStepContext(stringParam(params, "stepId"))
		if err != nil {
			return nil, err
		}
		if stepContext.ProjectID != run.ProjectID {
			return nil, errors.New("步骤不属于当前流水线项目")
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		execution, err := s.tasks.Dispatch(tasks.DispatchInput{StepID: stepContext.Step.ID, Instruction: stringParam(params, "instruction"), CompletionCriteria: stringParam(params, "completionCriteria"), Model: run.Model, Effort: run.Effort, Source: run.Source, PipelineRunID: run.ID})
		if err != nil {
			return nil, err
		}
		result, err := s.tasks.WaitExecution(ctx, execution.ID)
		if err != nil {
			s.tasks.StopExecution(execution.ID)
			cleanup, cancel := context.WithTimeout(s.ctx, 20*time.Second)
			defer cancel()
			s.tasks.WaitExecution(cleanup, execution.ID)
			return nil, err
		}
		ids := []string{}
		for _, file := range s.files.GetExecutionArtifacts(result.ID) {
			ids = append(ids, file.ID)
		}
		output := map[string]any{"executionId": result.ID, "status": result.Status, "result": result.Result, "artifactIds": ids}
		if result.Status != "completed" {
			return output, fmt.Errorf("步骤 %s：%s %s", result.Context.Step.Title, result.Status, result.Error)
		}
		return output, nil
	case "report.write":
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
	return nil, errors.New("未知固定动作")
}

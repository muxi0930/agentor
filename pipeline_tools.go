package main

import (
	"encoding/json"
	"errors"

	"agentor/internal/codex"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
)

func (r *toolRouter) pipelineSpecs(role string) []codex.DynamicToolSpec {
	specs := []codex.DynamicToolSpec{
		tool("list_actions", "读取固定 Go 动作及参数/结果契约。", map[string]any{}),
		tool("list_pipeline_tasks", "查询当前项目的任务、调度和运行记录。", map[string]any{}),
		tool("validate_pipeline_task", "只读验证 JSON 任务，不执行动作。任务字段为 id（新建可空）、name、inputs、actions。", map[string]any{"json": stringField("完整 JSON 配置")}, "json"),
	}
	if role == "pipeline_manager" {
		return append(specs, tool("run_pipeline", "运行当前项目已有任务；模型和来源沿用此会话请求。", map[string]any{"taskId": stringField("任务 ID"), "inputs": map[string]any{"type": "object"}}, "taskId", "inputs"))
	}
	return append(specs,
		tool("save_pipeline_task", "验证并保存当前项目 JSON 任务；新建 ID 留空，修改保留 ID。", map[string]any{"json": stringField("完整 JSON 配置")}, "json"),
		tool("delete_pipeline_task", "删除当前项目的空闲任务；有关联调度时需先移除调度。", map[string]any{"taskId": stringField("任务 ID")}, "taskId"),
		tool("save_pipeline_schedule", "保存一次性或间隔调度。字段 id/taskId/name/kind/enabled/at/anchor/intervalSeconds/timezone/inputs/misfirePolicy。时间为带时区 RFC3339。", map[string]any{"json": stringField("完整调度 JSON")}, "json"),
		tool("delete_pipeline_schedule", "删除当前项目调度。", map[string]any{"scheduleId": stringField("调度 ID")}, "scheduleId"),
	)
}
func (r *toolRouter) pipelineState(projectID string) pipeline.State {
	state := r.pipeline.GetState()
	result := pipeline.State{Tasks: []pipeline.Task{}, Schedules: []pipeline.Schedule{}, Runs: []pipeline.Run{}}
	for _, task := range state.Tasks {
		if projectID == "" || task.ProjectID == projectID {
			result.Tasks = append(result.Tasks, task)
		}
	}
	for _, schedule := range state.Schedules {
		if projectID == "" || schedule.ProjectID == projectID {
			result.Schedules = append(result.Schedules, schedule)
		}
	}
	for _, run := range state.Runs {
		if projectID == "" || run.ProjectID == projectID {
			result.Runs = append(result.Runs, run)
		}
	}
	return result
}
func (r *toolRouter) pipelineCall(request pipeline.SessionRequest, call codex.ToolCall) (any, error) {
	allowed := false
	for _, spec := range r.pipelineSpecs(request.Input.Role) {
		if spec.Name == call.Tool {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errors.New("当前流水线角色不能调用此工具")
	}
	if request.Kind != "instruction" && call.Tool != "list_actions" && call.Tool != "list_pipeline_tasks" && call.Tool != "validate_pipeline_task" {
		return nil, errors.New("完成汇报只能查询，不能重新运行或修改配置")
	}
	var args struct {
		JSON       string         `json:"json"`
		TaskID     string         `json:"taskId"`
		ScheduleID string         `json:"scheduleId"`
		Inputs     map[string]any `json:"inputs"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}
	projectID := request.Input.ProjectID
	switch call.Tool {
	case "list_actions":
		return r.pipeline.GetActions(), nil
	case "list_pipeline_tasks":
		return r.pipelineState(projectID), nil
	case "validate_pipeline_task":
		return r.pipeline.ValidateTask(projectID, args.JSON)
	case "save_pipeline_task":
		return r.pipeline.SaveTask(projectID, args.JSON)
	case "delete_pipeline_task":
		err := r.pipeline.DeleteTask(projectID, args.TaskID)
		return map[string]bool{"deleted": err == nil}, err
	case "save_pipeline_schedule":
		var schedule pipeline.Schedule
		if err := json.Unmarshal([]byte(args.JSON), &schedule); err != nil {
			return nil, err
		}
		if schedule.ProjectID != "" && schedule.ProjectID != projectID {
			return nil, errors.New("调度不能改变项目范围")
		}
		schedule.ProjectID = projectID
		return r.pipeline.SaveSchedule(schedule)
	case "delete_pipeline_schedule":
		err := r.pipeline.DeleteSchedule(projectID, args.ScheduleID)
		return map[string]bool{"deleted": err == nil}, err
	case "run_pipeline":
		for _, task := range r.pipelineState(projectID).Tasks {
			if task.ID == args.TaskID {
				return r.pipeline.RunTask(pipeline.RunInput{SessionRequestID: request.ID, TaskID: task.ID, Inputs: args.Inputs, Model: request.Input.Model, Effort: request.Input.Effort, Source: request.Input.Source, RequestID: request.Input.RequestID})
			}
		}
		return nil, errors.New("任务不属于当前项目")
	}
	return nil, errors.New("未知流水线工具")
}
func (r *toolRouter) controllerPipelineCall(request tasks.Request, call codex.ToolCall) (any, error) {
	var args struct {
		TaskID     string         `json:"taskId"`
		ProjectID  string         `json:"projectId"`
		Role       string         `json:"role"`
		Text       string         `json:"text"`
		ScheduleID string         `json:"scheduleId"`
		Enabled    bool           `json:"enabled"`
		Inputs     map[string]any `json:"inputs"`
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}
	state := r.pipelineState(request.ProjectID)
	switch call.Tool {
	case "list_pipeline_tasks":
		return state, nil
	case "request_pipeline_session":
		if request.ProjectID != "" && request.ProjectID != args.ProjectID {
			return nil, errors.New("项目不属于本次请求范围")
		}
		return r.pipeline.SubmitSession(pipeline.SessionInput{ProjectID: args.ProjectID, Role: args.Role, Text: args.Text, Model: request.Model, Effort: request.Effort, Source: request.Source, RequestID: request.ID})
	case "run_pipeline":
		for _, task := range state.Tasks {
			if task.ID == args.TaskID {
				return r.pipeline.RunTask(pipeline.RunInput{TaskID: task.ID, Inputs: args.Inputs, Model: request.Model, Effort: request.Effort, Source: request.Source, RequestID: request.ID})
			}
		}
	case "set_schedule_enabled", "trigger_schedule":
		for _, schedule := range state.Schedules {
			if schedule.ID == args.ScheduleID {
				if call.Tool == "trigger_schedule" {
					return r.pipeline.TriggerScheduleFor(schedule.ProjectID, schedule.ID, pipeline.RunInput{Model: request.Model, Effort: request.Effort, Source: request.Source, RequestID: request.ID})
				}
				err := r.pipeline.SetScheduleEnabled(schedule.ProjectID, schedule.ID, args.Enabled)
				return map[string]bool{"updated": err == nil}, err
			}
		}
	}
	return nil, errors.New("任务或调度不属于本次请求范围")
}

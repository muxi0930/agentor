package toolrouter

import (
	"encoding/json"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
)

// pipelineState 读取指定项目范围内的流水线状态
func (r *Router) pipelineState(projectID string) pipelinemodel.State {
	state := r.pipeline.GetState()
	result := pipelinemodel.State{Tasks: []pipelinemodel.Task{}, Schedules: []pipelinemodel.Schedule{}, Runs: []pipelinemodel.Run{}}
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

// pipelineCall 在项目专用会话范围内处理流水线工具请求
func (r *Router) pipelineCall(request pipelinemodel.SessionRequest, call codexmodel.ToolCall) (any, error) {
	if _, err := authorizeTool(request.Input.Role, call.Tool, request.Kind != constant.TasksRequestKindInstruction, constant.ErrToolRouterPipelineRoleForbidden, constant.ErrToolRouterPipelineReadOnly); err != nil {
		return nil, err
	}

	var args struct {
		Enabled    bool           `json:"enabled"`    // 调度是否启用
		RunID      string         `json:"runId"`      // 运行标识
		Operation  string         `json:"operation"`  // 受控运行命令
		JSON       string         `json:"json"`       // 任务或调度定义
		TaskID     string         `json:"taskId"`     // 流水线任务标识
		ScheduleID string         `json:"scheduleId"` // 调度标识
		Inputs     map[string]any `json:"inputs"`     // 任务输入参数
		ArtifactID string         `json:"artifactId"` // 待分享产物标识
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}

	projectID := request.Input.ProjectID
	switch call.Tool {
	case constant.ToolRouterToolSetScheduleEnabled:
		err := r.pipeline.SetScheduleEnabled(projectID, args.ScheduleID, args.Enabled)
		return map[string]bool{"updated": err == nil}, err
	case constant.ToolRouterToolTriggerSchedule:
		return r.pipeline.TriggerScheduleFor(projectID, args.ScheduleID, pipelinemodel.RunInput{SessionRequestID: request.ID, Model: request.Input.Model, Effort: request.Input.Effort, Source: request.Input.Source, RequestID: request.Input.RequestID})
	case constant.ToolRouterToolControlPipelineRun:
		return r.pipeline.ControlRun(projectID, args.RunID, args.Operation)
	case constant.ToolRouterToolListActions:
		return r.pipeline.GetActions(), nil
	case constant.ToolRouterToolShareArtifact:
		err := r.lark.ShareArtifactFor(args.ArtifactID, projectID, "", request.Input.Source)
		return map[string]bool{"queued": err == nil}, err
	case constant.ToolRouterToolListPipelineTasks:
		return r.pipelineState(projectID), nil
	case constant.ToolRouterToolValidatePipelineTask:
		return r.pipeline.ValidateTask(projectID, args.JSON)
	case constant.ToolRouterToolSavePipelineTask:
		return r.pipeline.SaveTask(projectID, args.JSON)
	case constant.ToolRouterToolDeletePipelineTask:
		err := r.pipeline.DeleteTask(projectID, args.TaskID)
		return map[string]bool{"deleted": err == nil}, err
	case constant.ToolRouterToolSavePipelineSchedule:
		var schedule pipelinemodel.Schedule
		if err := json.Unmarshal([]byte(args.JSON), &schedule); err != nil {
			return nil, err
		}
		if schedule.ProjectID != "" && schedule.ProjectID != projectID {
			return nil, constant.ErrToolRouterScheduleProjectChanged
		}
		schedule.ProjectID = projectID
		return r.pipeline.SaveSchedule(schedule)
	case constant.ToolRouterToolDeletePipelineSchedule:
		err := r.pipeline.DeleteSchedule(projectID, args.ScheduleID)
		return map[string]bool{"deleted": err == nil}, err
	case constant.ToolRouterToolRunPipeline:
		for _, task := range r.pipelineState(projectID).Tasks {
			if task.ID == args.TaskID {
				return r.pipeline.RunTask(pipelinemodel.RunInput{SessionRequestID: request.ID, TaskID: task.ID, Inputs: args.Inputs, Model: request.Input.Model, Effort: request.Input.Effort, Source: request.Input.Source, RequestID: request.Input.RequestID})
			}
		}
		return nil, constant.ErrToolRouterPipelineTaskOutsideProject
	}
	return nil, constant.ErrToolRouterPipelineToolUnknown
}

// controllerPipelineCall 处理控制中心派发的项目流水线操作
func (r *Router) controllerPipelineCall(request tasksmodel.Request, call codexmodel.ToolCall) (any, error) {
	var args struct {
		Model      string         `json:"model"`      // 执行者模型
		Effort     string         `json:"effort"`     // 执行者推理强度
		TaskID     string         `json:"taskId"`     // 流水线任务标识
		ProjectID  string         `json:"projectId"`  // 目标项目标识
		Role       string         `json:"role"`       // 专用会话角色
		Text       string         `json:"text"`       // 会话请求正文
		ScheduleID string         `json:"scheduleId"` // 调度标识
		Enabled    bool           `json:"enabled"`    // 调度是否启用
		Inputs     map[string]any `json:"inputs"`     // 任务输入参数
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}

	model, effort := request.Model, request.Effort
	if call.Tool == constant.ToolRouterToolRequestPipelineSession || call.Tool == constant.ToolRouterToolRunPipeline || call.Tool == constant.ToolRouterToolTriggerSchedule {
		var err error
		model, effort, err = r.tasks.SelectExecutor(request, args.Model, args.Effort)
		if err != nil {
			return nil, err
		}
	}
	state := r.pipelineState(request.ProjectID)
	switch call.Tool {
	case constant.ToolRouterToolListPipelineTasks:
		return state, nil
	case constant.ToolRouterToolRequestPipelineSession:
		if request.ProjectID != "" && request.ProjectID != args.ProjectID {
			return nil, constant.ErrToolRouterProjectOutsideScope
		}
		return r.pipeline.SubmitSession(pipelinemodel.SessionInput{ProjectID: args.ProjectID, Role: args.Role, Text: args.Text, Model: model, Effort: effort, Source: request.Source, RequestID: request.ID})
	case constant.ToolRouterToolRunPipeline:
		for _, task := range state.Tasks {
			if task.ID == args.TaskID {
				return r.pipeline.RunTask(pipelinemodel.RunInput{TaskID: task.ID, Inputs: args.Inputs, Model: model, Effort: effort, Source: request.Source, RequestID: request.ID})
			}
		}
	case constant.ToolRouterToolSetScheduleEnabled, constant.ToolRouterToolTriggerSchedule:
		for _, schedule := range state.Schedules {
			if schedule.ID == args.ScheduleID {
				if call.Tool == constant.ToolRouterToolTriggerSchedule {
					return r.pipeline.TriggerScheduleFor(schedule.ProjectID, schedule.ID, pipelinemodel.RunInput{Model: model, Effort: effort, Source: request.Source, RequestID: request.ID})
				}
				err := r.pipeline.SetScheduleEnabled(schedule.ProjectID, schedule.ID, args.Enabled)
				return map[string]bool{"updated": err == nil}, err
			}
		}
	}
	return nil, constant.ErrToolRouterPipelineObjectOutsideScope
}

package toolrouter

import (
	"encoding/json"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
)

// pipelineState 读取控制中心权限范围内的实例与公共模板
func (r *Router) pipelineState(projectID string) pipelinemodel.State {
	state := r.pipeline.GetState()
	result := pipelinemodel.State{Templates: state.Templates, Tasks: []pipelinemodel.Task{}, Schedules: []pipelinemodel.Schedule{}, Runs: []pipelinemodel.Run{}}
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

// controllerPipelineCall 通过宿主接口直接配置和管理流水线，不再转交专用会话
func (r *Router) controllerPipelineCall(request tasksmodel.Request, call codexmodel.ToolCall) (any, error) {
	var args struct {
		VersionID  string            `json:"versionId"`  // 目标版本
		Model      string            `json:"model"`      // 执行模型
		Effort     string            `json:"effort"`     // 推理强度
		TaskID     string            `json:"taskId"`     // 实例标识
		ProjectID  string            `json:"projectId"`  // 项目范围
		ScheduleID string            `json:"scheduleId"` // 定时触发标识
		RunID      string            `json:"runId"`      // 运行标识
		Operation  string            `json:"operation"`  // 状态机控制命令
		Enabled    bool              `json:"enabled"`    // 是否启用
		JSON       string            `json:"json"`       // 结构化定义
		Inputs     map[string]any    `json:"inputs"`     // 本次输入
		TemplateID string            `json:"templateId"` // 来源模板
		Name       string            `json:"name"`       // 实例名称
		Directory  string            `json:"directory"`  // 独立执行目录
		Bindings   map[string]string `json:"bindings"`   // 步骤插槽绑定
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}
	if request.ProjectID != "" && args.ProjectID != "" && args.ProjectID != request.ProjectID {
		return nil, constant.ErrToolRouterProjectOutsideScope
	}
	projectID := args.ProjectID
	if projectID == "" {
		projectID = request.ProjectID
	}
	state := r.pipelineState(request.ProjectID)
	model, effort := request.Model, request.Effort
	if call.Tool == constant.ToolRouterToolRunPipeline || call.Tool == constant.ToolRouterToolTriggerSchedule || call.Tool == constant.ToolRouterToolMergeVersionWorktree {
		var err error
		model, effort, err = r.tasks.SelectExecutor(request, args.Model, args.Effort)
		if err != nil {
			return nil, err
		}
	}
	switch call.Tool {
	case constant.ToolRouterToolGetVersionWorktree:
		_, version, err := r.work.GetVersion(projectID, args.VersionID)
		return version.Worktree, err
	case constant.ToolRouterToolPrepareVersionWorktree, constant.ToolRouterToolMergeVersionWorktree:
		operation := constant.PipelineSystemPrepare
		if call.Tool == constant.ToolRouterToolMergeVersionWorktree {
			operation = constant.PipelineSystemMerge
		}
		return r.pipeline.RunVersion(pipelinemodel.VersionRunInput{ProjectID: projectID, VersionID: args.VersionID, Operation: operation, Model: model, Effort: effort, Source: request.Source, RequestID: request.ID})
	case constant.ToolRouterToolListActions:
		return r.pipeline.GetActions(), nil
	case constant.ToolRouterToolListPipelineTasks:
		return state, nil
	case constant.ToolRouterToolValidatePipelineTask:
		return r.pipeline.ValidateTask(projectID, args.JSON)
	case constant.ToolRouterToolSavePipelineTask:
		return r.pipeline.SaveTask(projectID, args.JSON)
	case constant.ToolRouterToolSavePipelineTemplate:
		return r.pipeline.SaveTemplate(args.JSON)
	case constant.ToolRouterToolInstantiatePipeline:
		return r.pipeline.Instantiate(pipelinemodel.InstantiateInput{TemplateID: args.TemplateID, ProjectID: projectID, Name: args.Name, Directory: args.Directory, Bindings: args.Bindings})
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
	case constant.ToolRouterToolControlPipelineRun:
		for _, run := range state.Runs {
			if run.ID == args.RunID {
				return r.pipeline.ControlRun(run.ProjectID, run.ID, args.Operation)
			}
		}
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

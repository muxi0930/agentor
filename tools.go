package main

import (
	"encoding/json"
	"errors"

	"agentor/internal/codex"
	"agentor/internal/lark"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
)

type toolRouter struct {
	tasks    *tasks.Service
	work     *workspace.Service
	pipeline *pipeline.Service
	lark     *lark.Service
}

func tool(name, description string, properties map[string]any, required ...string) codex.DynamicToolSpec {
	if required == nil {
		required = []string{}
	}
	return codex.DynamicToolSpec{Type: "function", Name: name, Description: description, InputSchema: map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}}
}
func stringField(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func (r *toolRouter) specs(role string) []codex.DynamicToolSpec {
	if role == "pipeline_manager" || role == "pipeline_config" {
		return r.pipelineSpecs(role)
	}
	if role == "controller" {
		return []codex.DynamicToolSpec{
			tool("list_workspaces", "查询本次请求范围内的项目、版本、功能和步骤摘要及稳定 ID。", map[string]any{}),
			tool("get_work_item", "按稳定 ID 读取项目、版本、功能或步骤的详细定义。", map[string]any{"kind": stringField("project / version / feature / step"), "id": stringField("对象 ID")}, "kind", "id"),
			tool("create_feature", "在本次请求范围内的版本创建功能，不改变桌面选择。", map[string]any{"versionId": stringField("版本 ID"), "title": stringField("功能标题"), "description": stringField("功能说明"), "context": stringField("功能上下文"), "prompt": stringField("功能提示词")}, "versionId", "title", "description", "context", "prompt"),
			tool("create_step", "在本次请求范围内的功能创建步骤，执行权限继承项目配置。", map[string]any{"featureId": stringField("功能 ID"), "title": stringField("步骤标题"), "description": stringField("步骤说明"), "prompt": stringField("步骤提示词")}, "featureId", "title", "description", "prompt"),
			tool("dispatch_step", "正式派发步骤；同一步骤会排队。模型与 effort 使用本次请求入队时固定的值。", map[string]any{"stepId": stringField("明确的步骤 ID"), "instruction": stringField("执行指令"), "completionCriteria": stringField("完成标准，可为空")}, "stepId", "instruction", "completionCriteria"),
			tool("list_executions", "查询本次项目范围内的执行状态与结果。", map[string]any{}),
			tool("stop_execution", "停止一项明确的执行，或取消该项排队。", map[string]any{"executionId": stringField("执行记录 ID")}, "executionId"),
			tool("rebuild_step_context", "在步骤空闲时重建会话，保留已有执行记录，不重放任务。", map[string]any{"stepId": stringField("步骤 ID")}, "stepId"),
			tool("list_pipeline_tasks", "查询本次项目范围内的流水线任务、调度与运行记录。", map[string]any{}),
			tool("run_pipeline", "运行已有流水线任务，同一任务不重入。", map[string]any{"taskId": stringField("任务 ID"), "inputs": map[string]any{"type": "object", "description": "任务契约要求的输入值"}}, "taskId", "inputs"),
			tool("request_pipeline_session", "给项目专用管理/配置会话提交请求。管理会话运行已有任务，配置会话维护 JSON 和调度。", map[string]any{"projectId": stringField("当前范围内的项目 ID"), "role": stringField("pipeline_manager / pipeline_config"), "text": stringField("会话请求")}, "projectId", "role", "text"),
			tool("set_schedule_enabled", "启用或停用当前项目的调度。", map[string]any{"scheduleId": stringField("调度 ID"), "enabled": map[string]any{"type": "boolean"}}, "scheduleId", "enabled"),
			tool("trigger_schedule", "手动触发已有调度，同任务不重入。", map[string]any{"scheduleId": stringField("调度 ID")}, "scheduleId"),
			tool("share_artifact", "将当前项目的已登记产物发送到原始飞书会话；桌面任务使用已配对接收者。", map[string]any{"artifactId": stringField("产物 ID")}, "artifactId"),
		}
	}
	if role != "step" {
		return nil
	}
	return []codex.DynamicToolSpec{
		tool("get_step_context", "读取宿主绑定的当前步骤、项目和功能上下文，以及可读取的知识文档索引。", map[string]any{}),
		tool("search_knowledge", "在当前项目共享和当前功能绑定的 Markdown 中搜索标题与正文。", map[string]any{"query": stringField("检索文字")}, "query"),
		tool("read_knowledge", "读取当前步骤范围内的完整 Markdown。", map[string]any{"documentId": stringField("文档索引中的稳定 ID")}, "documentId"),
		tool("prepare_knowledge_document", "创建托管 Markdown 文档并绑定至当前功能；随后用 sync_knowledge_document 写入正文。", map[string]any{"title": stringField("文档标题")}, "title"),
		tool("bind_knowledge_document", "将当前项目内的文档绑定至当前功能。", map[string]any{"documentId": stringField("当前项目内的文档 ID")}, "documentId"),
		tool("sync_knowledge_document", "更新当前步骤可访问的托管 Markdown 正文，不修改原始导入文件。", map[string]any{"documentId": stringField("托管文档 ID"), "content": stringField("完整 Markdown 正文")}, "documentId", "content"),
		tool("register_artifact", "登记当前项目内由本次执行产生的文件，供用户查看或分享。", map[string]any{"path": stringField("项目内文件的相对或绝对路径")}, "path"),
		tool("share_artifact", "显式分享本次执行登记的产物，发往原始飞书会话或桌面已配对接收者。", map[string]any{"artifactId": stringField("register_artifact 返回的 ID")}, "artifactId"),
	}
}

func (r *toolRouter) call(call codex.ToolCall) (any, error) {
	scope, err := r.tasks.ScopeForThread(call.ThreadID)
	if err != nil {
		if r.pipeline != nil {
			if request, sessionErr := r.pipeline.ActiveSession(call.ThreadID); sessionErr == nil {
				return r.pipelineCall(request, call)
			}
		}
		return nil, err
	}
	allowed := false
	for _, spec := range r.specs(scope.Role) {
		if spec.Name == call.Tool {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errors.New("当前会话角色不能调用此工具")
	}
	if scope.Role == "controller" {
		return r.controllerCall(scope, call)
	}
	var args struct {
		Query      string `json:"query"`
		DocumentID string `json:"documentId"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		Path       string `json:"path"`
		ArtifactID string `json:"artifactId"`
	}
	if err = json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}
	switch call.Tool {
	case "get_step_context":
		return r.work.GetStepContext(scope.StepID)
	case "search_knowledge":
		return r.work.SearchKnowledge(scope.ProjectID, scope.FeatureID, args.Query)
	case "read_knowledge":
		return r.work.ReadKnowledge(scope.ProjectID, scope.FeatureID, args.DocumentID)
	case "prepare_knowledge_document":
		return r.work.PrepareKnowledge(scope.ProjectID, scope.FeatureID, args.Title)
	case "bind_knowledge_document":
		_, err := r.work.BindKnowledge(scope.ProjectID, scope.FeatureID, args.DocumentID, true)
		return map[string]bool{"bound": err == nil}, err
	case "sync_knowledge_document":
		return r.work.SyncKnowledge(scope.ProjectID, scope.FeatureID, args.DocumentID, args.Content)
	case "register_artifact":
		return r.tasks.RegisterArtifact(scope.ExecutionID, args.Path)
	case "share_artifact":
		for _, execution := range r.tasks.GetState().Executions {
			if execution.ID == scope.ExecutionID {
				err := r.lark.ShareArtifactFor(args.ArtifactID, scope.ProjectID, scope.ExecutionID, execution.Source)
				return map[string]bool{"queued": err == nil}, err
			}
		}
		return nil, errors.New("当前执行不存在")
	}
	return nil, errors.New("未知工具")
}

func (r *toolRouter) controllerCall(scope tasks.Scope, call codex.ToolCall) (any, error) {
	request, err := r.tasks.CurrentRequest(call.ThreadID)
	if err != nil {
		return nil, err
	}
	readOnly := call.Tool == "list_workspaces" || call.Tool == "get_work_item" || call.Tool == "list_executions" || call.Tool == "list_pipeline_tasks"
	if !readOnly && (request.NeedsTarget || request.Kind != "instruction") {
		return nil, errors.New("此请求需要明确目标或仅用于完成汇报，不能修改或再次派发")
	}
	if call.Tool == "share_artifact" {
		var args struct {
			ArtifactID string `json:"artifactId"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		err := r.lark.ShareArtifactFor(args.ArtifactID, scope.ProjectID, "", request.Source)
		return map[string]bool{"queued": err == nil}, err
	}
	if r.pipeline != nil {
		switch call.Tool {
		case "list_pipeline_tasks", "run_pipeline", "request_pipeline_session", "set_schedule_enabled", "trigger_schedule":
			return r.controllerPipelineCall(request, call)
		}
	}
	var args struct {
		Kind               string `json:"kind"`
		ID                 string `json:"id"`
		VersionID          string `json:"versionId"`
		FeatureID          string `json:"featureId"`
		StepID             string `json:"stepId"`
		ExecutionID        string `json:"executionId"`
		Title              string `json:"title"`
		Description        string `json:"description"`
		Context            string `json:"context"`
		Prompt             string `json:"prompt"`
		Instruction        string `json:"instruction"`
		CompletionCriteria string `json:"completionCriteria"`
	}
	if err = json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}
	state := r.work.GetState()
	if call.Tool == "list_workspaces" {
		projects := []map[string]any{}
		for _, project := range state.Projects {
			if scope.ProjectID != "" && scope.ProjectID != project.ID {
				continue
			}
			versions := []map[string]any{}
			for _, version := range project.Versions {
				features := []map[string]any{}
				for _, feature := range version.Features {
					steps := []map[string]string{}
					for _, step := range feature.Steps {
						steps = append(steps, map[string]string{"id": step.ID, "title": step.Title})
					}
					features = append(features, map[string]any{"id": feature.ID, "title": feature.Title, "steps": steps})
				}
				versions = append(versions, map[string]any{"id": version.ID, "name": version.Name, "features": features})
			}
			projects = append(projects, map[string]any{"id": project.ID, "name": project.Name, "directory": project.Directory, "versions": versions})
		}
		return projects, nil
	}
	if call.Tool == "list_executions" {
		result := []map[string]any{}
		for _, execution := range r.tasks.GetState().Executions {
			if scope.ProjectID != "" && scope.ProjectID != execution.ProjectID {
				continue
			}
			result = append(result, map[string]any{"id": execution.ID, "stepId": execution.StepID, "title": execution.Context.Step.Title, "status": execution.Status, "result": execution.Result, "error": execution.Error, "requestId": execution.RequestID})
		}
		return result, nil
	}
	if call.Tool == "dispatch_step" {
		return r.tasks.DispatchFromController(call.ThreadID, args.StepID, args.Instruction, args.CompletionCriteria)
	}
	if call.Tool == "stop_execution" {
		for _, execution := range r.tasks.GetState().Executions {
			if execution.ID == args.ExecutionID {
				if scope.ProjectID != "" && scope.ProjectID != execution.ProjectID {
					return nil, errors.New("执行不属于本次项目范围")
				}
				err := r.tasks.StopExecution(execution.ID)
				return map[string]bool{"stopRequested": err == nil}, err
			}
		}
		return nil, errors.New("执行记录不存在")
	}
	if call.Tool == "rebuild_step_context" {
		context, err := r.work.GetStepContext(args.StepID)
		if err != nil {
			return nil, err
		}
		if scope.ProjectID != "" && scope.ProjectID != context.ProjectID {
			return nil, errors.New("步骤不属于本次项目范围")
		}
		err = r.tasks.RebuildStepContext(args.StepID)
		return map[string]bool{"rebuilt": err == nil}, err
	}
	for _, project := range state.Projects {
		if scope.ProjectID != "" && scope.ProjectID != project.ID {
			continue
		}
		if call.Tool == "get_work_item" && args.Kind == "project" && args.ID == project.ID {
			return project, nil
		}
		for _, version := range project.Versions {
			if call.Tool == "get_work_item" && args.Kind == "version" && args.ID == version.ID {
				return version, nil
			}
			if call.Tool == "create_feature" && args.VersionID == version.ID {
				return r.work.CreateFeature(project.ID, version.ID, workspace.Feature{Title: args.Title, Description: args.Description, Context: args.Context, Prompt: args.Prompt})
			}
			for _, feature := range version.Features {
				if call.Tool == "get_work_item" && args.Kind == "feature" && args.ID == feature.ID {
					return feature, nil
				}
				if call.Tool == "create_step" && args.FeatureID == feature.ID {
					return r.work.CreateStep(project.ID, feature.ID, workspace.Step{Title: args.Title, Description: args.Description, Prompt: args.Prompt})
				}
				for _, step := range feature.Steps {
					if call.Tool == "get_work_item" && args.Kind == "step" && args.ID == step.ID {
						return r.work.GetStepContext(step.ID)
					}
				}
			}
		}
	}
	return nil, errors.New("对象不存在或不属于本次请求范围")
}

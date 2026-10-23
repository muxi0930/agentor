package toolrouter

import (
	"encoding/json"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
	workspacemodel "agentor/internal/workspace/model"
)

// controllerCall 在控制中心绑定的项目范围内处理工具调用
func (r *Router) controllerCall(scope tasksmodel.Scope, call codexmodel.ToolCall) (any, error) {
	request, err := r.tasks.CurrentRequest(call.ThreadID)
	if err != nil {
		return nil, err
	}

	definition, err := authorizeTool(scope.Role, call.Tool, request.NeedsTarget || request.Kind != constant.TasksRequestKindInstruction, constant.ErrToolRouterRoleForbidden, constant.ErrToolRouterControllerReadOnly)
	if err != nil {
		return nil, err
	}

	if call.Tool == constant.ToolRouterToolShareArtifact {
		var args struct {
			ArtifactID string `json:"artifactId"` // 待分享产物标识
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		err := r.lark.ShareArtifactFor(args.ArtifactID, scope.ProjectID, "", request.Source)
		return map[string]bool{"queued": err == nil}, err
	}
	if r.pipeline != nil && definition.group == constant.ToolRouterGroupControllerPipeline {
		return r.controllerPipelineCall(request, call)
	}

	var args struct {
		Kind               string `json:"kind"`               // 工作项类型
		ID                 string `json:"id"`                 // 工作项标识
		ProjectID          string `json:"projectId"`          // 所属项目标识
		Name               string `json:"name"`               // 版本名称
		VersionID          string `json:"versionId"`          // 所属版本标识
		FeatureID          string `json:"featureId"`          // 所属功能标识
		StepID             string `json:"stepId"`             // 步骤标识
		ExecutionID        string `json:"executionId"`        // 执行记录标识
		Title              string `json:"title"`              // 工作项标题
		Description        string `json:"description"`        // 工作项说明
		Context            string `json:"context"`            // 工作项上下文
		Prompt             string `json:"prompt"`             // 步骤提示词
		Instruction        string `json:"instruction"`        // 本次执行指令
		CompletionCriteria string `json:"completionCriteria"` // 本次完成标准
		Model              string `json:"model"`              // 执行者模型
		Effort             string `json:"effort"`             // 执行者推理强度
	}
	if err = json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}
	state := r.work.GetState()
	switch call.Tool {
	case constant.ToolRouterToolListWorkspaces:
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
	case constant.ToolRouterToolListExecutions:
		result := []map[string]any{}
		for _, execution := range r.tasks.GetState().Executions {
			if scope.ProjectID != "" && scope.ProjectID != execution.ProjectID {
				continue
			}
			result = append(result, map[string]any{"id": execution.ID, "stepId": execution.StepID, "title": execution.Context.Step.Title, "status": execution.Status, "result": execution.Result, "error": execution.Error, "requestId": execution.RequestID})
		}
		return result, nil
	case constant.ToolRouterToolDispatchStep:
		return r.tasks.DispatchFromController(call.ThreadID, args.StepID, args.Instruction, args.CompletionCriteria, args.Model, args.Effort)
	case constant.ToolRouterToolStopExecution:
		for _, execution := range r.tasks.GetState().Executions {
			if execution.ID == args.ExecutionID {
				if scope.ProjectID != "" && scope.ProjectID != execution.ProjectID {
					return nil, constant.ErrToolRouterExecutionOutsideProject
				}
				err := r.tasks.StopExecution(execution.ID)
				return map[string]bool{"stopRequested": err == nil}, err
			}
		}
		return nil, constant.ErrToolRouterExecutionNotFound
	case constant.ToolRouterToolRebuildStepContext:
		context, err := r.work.GetStepContext(args.StepID)
		if err != nil {
			return nil, err
		}
		if scope.ProjectID != "" && scope.ProjectID != context.ProjectID {
			return nil, constant.ErrToolRouterStepOutsideProject
		}
		err = r.tasks.RebuildStepContext(args.StepID)
		return map[string]bool{"rebuilt": err == nil}, err
	}
	switch call.Tool {
	case constant.ToolRouterToolGetWorkItem:
		return r.getWorkItem(state, scope.ProjectID, args.Kind, args.ID)
	case constant.ToolRouterToolCreateVersion:
		project, _, _ := findWorkItem(state, scope.ProjectID, constant.WorkspaceKindProject, args.ProjectID)
		if project != nil {
			return r.work.CreateVersion(project.ID, workspacemodel.Version{Name: args.Name, Description: args.Description})
		}
	case constant.ToolRouterToolCreateFeature:
		project, version, _ := findWorkItem(state, scope.ProjectID, constant.WorkspaceKindVersion, args.VersionID)
		if version != nil {
			return r.work.CreateFeature(project.ID, version.ID, workspacemodel.Feature{Title: args.Title, Description: args.Description, Context: args.Context, Prompt: args.Prompt})
		}
	case constant.ToolRouterToolCreateStep:
		project, _, feature := findWorkItem(state, scope.ProjectID, constant.WorkspaceKindFeature, args.FeatureID)
		if feature != nil {
			return r.work.CreateStep(project.ID, feature.ID, workspacemodel.Step{Title: args.Title, Description: args.Description, Prompt: args.Prompt})
		}
	}
	return nil, constant.ErrToolRouterWorkItemOutsideScope
}

// getWorkItem 返回当前请求范围内的工作项定义
func (r *Router) getWorkItem(state workspacemodel.State, projectID, kind, id string) (any, error) {
	project, version, feature := findWorkItem(state, projectID, kind, id)
	if project == nil {
		return nil, constant.ErrToolRouterWorkItemOutsideScope
	}
	switch kind {
	case constant.WorkspaceKindProject:
		return *project, nil
	case constant.WorkspaceKindVersion:
		return *version, nil
	case constant.WorkspaceKindFeature:
		return *feature, nil
	case constant.WorkspaceKindStep:
		return r.work.GetStepContext(id)
	}
	return nil, constant.ErrToolRouterWorkItemOutsideScope
}

// findWorkItem 在请求的项目范围内定位节点及其所属层级
func findWorkItem(state workspacemodel.State, projectID, kind, id string) (*workspacemodel.Project, *workspacemodel.Version, *workspacemodel.Feature) {
	for i := range state.Projects {
		project := &state.Projects[i]
		if projectID != "" && projectID != project.ID {
			continue
		}
		if kind == constant.WorkspaceKindProject && id == project.ID {
			return project, nil, nil
		}
		for j := range project.Versions {
			version := &project.Versions[j]
			if kind == constant.WorkspaceKindVersion && id == version.ID {
				return project, version, nil
			}
			for k := range version.Features {
				feature := &version.Features[k]
				if kind == constant.WorkspaceKindFeature && id == feature.ID {
					return project, version, feature
				}
				if kind != constant.WorkspaceKindStep {
					continue
				}
				for _, step := range feature.Steps {
					if id == step.ID {
						return project, version, feature
					}
				}
			}
		}
	}
	return nil, nil, nil
}

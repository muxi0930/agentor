package toolrouter

import (
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

type toolDefinition struct {
	spec     codexmodel.DynamicToolSpec // 工具协议
	readOnly bool                       // 是否允许查询及完成汇报请求使用
	group    string                     // 实际业务处理分组
}

// roleTools 按既有顺序返回角色工具目录
func roleTools(role string) []toolDefinition {
	switch role {
	case constant.ToolRouterRoleController:
		return append(controllerPipelineTools(), []toolDefinition{
			defineTool(true, constant.ToolRouterRoleController, constant.ToolRouterToolListWorkspaces, constant.ToolRouterListWorkspacesDescription, map[string]any{}),
			defineTool(true, constant.ToolRouterRoleController, constant.ToolRouterToolGetWorkItem, constant.ToolRouterGetWorkItemDescription, map[string]any{"kind": stringField(constant.ToolRouterWorkItemKindDescription), "id": stringField(constant.ToolRouterWorkItemIDDescription)}, "kind", "id"),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolCreateVersion, constant.ToolRouterCreateVersionDescription, map[string]any{"projectId": stringField(constant.ToolRouterExplicitProjectIDDescription), "name": stringField(constant.ToolRouterVersionNameDescription), "description": stringField(constant.ToolRouterVersionSummaryDescription)}, "projectId", "name", "description"),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolCreateFeature, constant.ToolRouterCreateFeatureDescription, map[string]any{"versionId": stringField(constant.ToolRouterVersionIDDescription), "title": stringField(constant.ToolRouterFeatureTitleDescription), "description": stringField(constant.ToolRouterFeatureSummaryDescription), "context": stringField(constant.ToolRouterFeatureContextDescription), "prompt": stringField(constant.ToolRouterFeaturePromptDescription)}, "versionId", "title", "description", "context", "prompt"),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolCreateStep, constant.ToolRouterCreateStepDescription, map[string]any{"featureId": stringField(constant.ToolRouterFeatureIDDescription), "title": stringField(constant.ToolRouterStepTitleDescription), "description": stringField(constant.ToolRouterStepSummaryDescription), "prompt": stringField(constant.ToolRouterStepPromptDescription)}, "featureId", "title", "description", "prompt"),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolDispatchStep, constant.ToolRouterDispatchStepDescription, map[string]any{"stepId": stringField(constant.ToolRouterExplicitStepIDDescription), "instruction": stringField(constant.ToolRouterInstructionDescription), "completionCriteria": stringField(constant.ToolRouterCompletionCriteriaDescription), "model": stringField(constant.ToolRouterExecutorModelDescription), "effort": stringField(constant.ToolRouterExecutorEffortDescription)}, "stepId", "instruction", "completionCriteria", "model", "effort"),
			defineTool(true, constant.ToolRouterRoleController, constant.ToolRouterToolListExecutions, constant.ToolRouterListExecutionsDescription, map[string]any{}),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolStopExecution, constant.ToolRouterStopExecutionDescription, map[string]any{"executionId": stringField(constant.ToolRouterExecutionIDDescription)}, "executionId"),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolRebuildStepContext, constant.ToolRouterRebuildStepDescription, map[string]any{"stepId": stringField(constant.ToolRouterStepIDDescription)}, "stepId"),
			defineTool(true, constant.ToolRouterGroupControllerPipeline, constant.ToolRouterToolListPipelineTasks, constant.ToolRouterControllerListTasksDescription, map[string]any{}),
			defineTool(false, constant.ToolRouterGroupControllerPipeline, constant.ToolRouterToolRunPipeline, constant.ToolRouterControllerRunDescription, map[string]any{"taskId": stringField(constant.ToolRouterTaskIDDescription), "inputs": map[string]any{"type": "object", "description": constant.ToolRouterTaskInputsDescription}, "model": stringField(constant.ToolRouterExecutorModelDescription), "effort": stringField(constant.ToolRouterExecutorEffortDescription)}, "taskId", "inputs", "model", "effort"),
			defineTool(false, constant.ToolRouterGroupControllerPipeline, constant.ToolRouterToolSetScheduleEnabled, constant.ToolRouterSetScheduleEnabledDescription, map[string]any{"scheduleId": stringField(constant.ToolRouterScheduleIDDescription), "enabled": map[string]any{"type": "boolean"}}, "scheduleId", "enabled"),
			defineTool(false, constant.ToolRouterGroupControllerPipeline, constant.ToolRouterToolTriggerSchedule, constant.ToolRouterTriggerScheduleDescription, map[string]any{"scheduleId": stringField(constant.ToolRouterScheduleIDDescription), "model": stringField(constant.ToolRouterExecutorModelDescription), "effort": stringField(constant.ToolRouterExecutorEffortDescription)}, "scheduleId", "model", "effort"),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolShareArtifact, constant.ToolRouterControllerShareDescription, map[string]any{"artifactId": stringField(constant.ToolRouterArtifactIDDescription)}, "artifactId"),
		}...)
	case constant.ToolRouterRoleStep:
		return []toolDefinition{
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolReportStepOutcome, constant.ToolRouterReportStepOutcomeDescription, map[string]any{"success": map[string]any{"type": "boolean"}, "summary": map[string]any{"type": "string"}}, "success", "summary"),
			defineTool(true, constant.ToolRouterRoleStep, constant.ToolRouterToolGetStepContext, constant.ToolRouterGetStepContextDescription, map[string]any{}),
			defineTool(true, constant.ToolRouterRoleStep, constant.ToolRouterToolSearchKnowledge, constant.ToolRouterSearchKnowledgeDescription, map[string]any{"query": stringField(constant.ToolRouterQueryDescription)}, "query"),
			defineTool(true, constant.ToolRouterRoleStep, constant.ToolRouterToolReadKnowledge, constant.ToolRouterReadKnowledgeDescription, map[string]any{"documentId": stringField(constant.ToolRouterIndexedDocumentIDDescription)}, "documentId"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolPrepareKnowledgeDocument, constant.ToolRouterPrepareKnowledgeDescription, map[string]any{"title": stringField(constant.ToolRouterDocumentTitleDescription)}, "title"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolBindKnowledgeDocument, constant.ToolRouterBindKnowledgeDescription, map[string]any{"documentId": stringField(constant.ToolRouterProjectDocumentIDDescription)}, "documentId"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolSyncKnowledgeDocument, constant.ToolRouterSyncKnowledgeDescription, map[string]any{"documentId": stringField(constant.ToolRouterManagedDocumentIDDescription), "content": stringField(constant.ToolRouterDocumentContentDescription)}, "documentId", "content"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolRegisterArtifact, constant.ToolRouterRegisterArtifactDescription, map[string]any{"path": stringField(constant.ToolRouterArtifactPathDescription)}, "path"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolShareArtifact, constant.ToolRouterStepShareDescription, map[string]any{"artifactId": stringField(constant.ToolRouterProducedArtifactIDDescription)}, "artifactId"),
		}
	case constant.ToolRouterRolePipelineNode:
		return []toolDefinition{defineTool(false, constant.ToolRouterRolePipelineNode, constant.ToolRouterToolCompleteNode, constant.ToolRouterCompleteNodeDescription, map[string]any{
			"outcome": map[string]any{"type": "string", "enum": []string{constant.PipelineBranchSuccess, constant.PipelineBranchFailure}},
			"branch":  map[string]any{"type": "string"}, "summary": map[string]any{"type": "string"}, "output": map[string]any{},
		}, "outcome", "branch", "summary", "output")}

	}
	return nil
}

// authorizeTool 根据公开目录核验角色与请求的调用权限
func authorizeTool(role, name string, readOnly bool, roleError, readOnlyError error) (toolDefinition, error) {
	for _, definition := range roleTools(role) {
		if definition.spec.Name != name {
			continue
		}
		if readOnly && !definition.readOnly {
			return toolDefinition{}, readOnlyError
		}
		return definition, nil
	}
	return toolDefinition{}, roleError
}

// defineTool 将协议、只读属性和处理分组保存在同一目录项中
func defineTool(readOnly bool, group, name, description string, properties map[string]any, required ...string) toolDefinition {
	if required == nil {
		required = []string{}
	}
	return toolDefinition{readOnly: readOnly, group: group, spec: codexmodel.DynamicToolSpec{
		Type: "function", Name: name, Description: description,
		InputSchema: map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false},
	}}
}

// stringField 构造字符串类型的工具参数
func stringField(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

// controllerPipelineTools 声明控制中心可用的配置能力，全部经过 Go 校验
func controllerPipelineTools() []toolDefinition {
	group := constant.ToolRouterGroupControllerPipeline
	version := map[string]any{"projectId": stringField(constant.ToolRouterScopedProjectIDDescription), "versionId": stringField(constant.ToolRouterVersionIDDescription)}
	versionRun := map[string]any{"projectId": stringField(constant.ToolRouterScopedProjectIDDescription), "versionId": stringField(constant.ToolRouterVersionIDDescription), "model": stringField(constant.ToolRouterExecutorModelDescription), "effort": stringField(constant.ToolRouterExecutorEffortDescription)}
	config := map[string]any{"projectId": stringField(constant.ToolRouterScopedProjectIDDescription), "json": stringField(constant.ToolRouterTaskJSONDescription)}
	return []toolDefinition{
		defineTool(false, group, constant.ToolRouterToolPrepareVersionWorktree, constant.ToolRouterPrepareVersionWorktreeDescription, version, "versionId"),
		defineTool(false, group, constant.ToolRouterToolMergeVersionWorktree, constant.ToolRouterMergeVersionWorktreeDescription, versionRun, "versionId", "model", "effort"),
		defineTool(true, group, constant.ToolRouterToolGetVersionWorktree, constant.ToolRouterGetVersionWorktreeDescription, version, "versionId"),
		defineTool(true, group, constant.ToolRouterToolListActions, constant.ToolRouterListActionsDescription, map[string]any{}),
		defineTool(true, group, constant.ToolRouterToolValidatePipelineTask, constant.ToolRouterValidateTaskDescription, config, "json"),
		defineTool(false, group, constant.ToolRouterToolSavePipelineTask, constant.ToolRouterSaveTaskDescription, config, "json"),
		defineTool(false, group, constant.ToolRouterToolDeletePipelineTask, constant.ToolRouterDeleteTaskDescription, map[string]any{"projectId": stringField(constant.ToolRouterScopedProjectIDDescription), "taskId": stringField(constant.ToolRouterTaskIDDescription)}, "taskId"),
		defineTool(false, group, constant.ToolRouterToolSavePipelineTemplate, constant.ToolRouterSaveTemplateDescription, map[string]any{"json": stringField(constant.ToolRouterTaskJSONDescription)}, "json"),
		defineTool(false, group, constant.ToolRouterToolInstantiatePipeline, constant.ToolRouterInstantiateDescription, map[string]any{"templateId": map[string]any{"type": "string"}, "projectId": stringField(constant.ToolRouterScopedProjectIDDescription), "name": map[string]any{"type": "string"}, "directory": map[string]any{"type": "string"}, "bindings": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}}}, "templateId", "bindings"),
		defineTool(false, group, constant.ToolRouterToolSavePipelineSchedule, constant.ToolRouterSaveScheduleDescription, config, "json"),
		defineTool(false, group, constant.ToolRouterToolDeletePipelineSchedule, constant.ToolRouterDeleteScheduleDescription, map[string]any{"projectId": stringField(constant.ToolRouterScopedProjectIDDescription), "scheduleId": stringField(constant.ToolRouterScheduleIDDescription)}, "scheduleId"),
		defineTool(false, group, constant.ToolRouterToolControlPipelineRun, constant.ToolRouterControlPipelineDescription, map[string]any{"runId": stringField(constant.ToolRouterRunIDDescription), "operation": map[string]any{"type": "string", "enum": []string{constant.PipelineControlPause, constant.PipelineControlResume, constant.PipelineControlStop}}}, "runId", "operation"),
	}
}

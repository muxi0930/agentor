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
		return []toolDefinition{
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
			defineTool(false, constant.ToolRouterGroupControllerPipeline, constant.ToolRouterToolRequestPipelineSession, constant.ToolRouterRequestPipelineSessionDescription, map[string]any{"projectId": stringField(constant.ToolRouterScopedProjectIDDescription), "role": stringField(constant.ToolRouterSessionRoleDescription), "text": stringField(constant.ToolRouterSessionTextDescription), "model": stringField(constant.ToolRouterExecutorModelDescription), "effort": stringField(constant.ToolRouterExecutorEffortDescription)}, "projectId", "role", "text", "model", "effort"),
			defineTool(false, constant.ToolRouterGroupControllerPipeline, constant.ToolRouterToolSetScheduleEnabled, constant.ToolRouterSetScheduleEnabledDescription, map[string]any{"scheduleId": stringField(constant.ToolRouterScheduleIDDescription), "enabled": map[string]any{"type": "boolean"}}, "scheduleId", "enabled"),
			defineTool(false, constant.ToolRouterGroupControllerPipeline, constant.ToolRouterToolTriggerSchedule, constant.ToolRouterTriggerScheduleDescription, map[string]any{"scheduleId": stringField(constant.ToolRouterScheduleIDDescription), "model": stringField(constant.ToolRouterExecutorModelDescription), "effort": stringField(constant.ToolRouterExecutorEffortDescription)}, "scheduleId", "model", "effort"),
			defineTool(false, constant.ToolRouterRoleController, constant.ToolRouterToolShareArtifact, constant.ToolRouterControllerShareDescription, map[string]any{"artifactId": stringField(constant.ToolRouterArtifactIDDescription)}, "artifactId"),
		}
	case constant.ToolRouterRoleStep:
		return []toolDefinition{
			defineTool(true, constant.ToolRouterRoleStep, constant.ToolRouterToolGetStepContext, constant.ToolRouterGetStepContextDescription, map[string]any{}),
			defineTool(true, constant.ToolRouterRoleStep, constant.ToolRouterToolSearchKnowledge, constant.ToolRouterSearchKnowledgeDescription, map[string]any{"query": stringField(constant.ToolRouterQueryDescription)}, "query"),
			defineTool(true, constant.ToolRouterRoleStep, constant.ToolRouterToolReadKnowledge, constant.ToolRouterReadKnowledgeDescription, map[string]any{"documentId": stringField(constant.ToolRouterIndexedDocumentIDDescription)}, "documentId"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolPrepareKnowledgeDocument, constant.ToolRouterPrepareKnowledgeDescription, map[string]any{"title": stringField(constant.ToolRouterDocumentTitleDescription)}, "title"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolBindKnowledgeDocument, constant.ToolRouterBindKnowledgeDescription, map[string]any{"documentId": stringField(constant.ToolRouterProjectDocumentIDDescription)}, "documentId"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolSyncKnowledgeDocument, constant.ToolRouterSyncKnowledgeDescription, map[string]any{"documentId": stringField(constant.ToolRouterManagedDocumentIDDescription), "content": stringField(constant.ToolRouterDocumentContentDescription)}, "documentId", "content"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolRegisterArtifact, constant.ToolRouterRegisterArtifactDescription, map[string]any{"path": stringField(constant.ToolRouterArtifactPathDescription)}, "path"),
			defineTool(false, constant.ToolRouterRoleStep, constant.ToolRouterToolShareArtifact, constant.ToolRouterStepShareDescription, map[string]any{"artifactId": stringField(constant.ToolRouterProducedArtifactIDDescription)}, "artifactId"),
		}
	case constant.ToolRouterRolePipelineManager, constant.ToolRouterRolePipelineConfig:
		specs := []toolDefinition{
			defineTool(true, constant.ToolRouterGroupPipeline, constant.ToolRouterToolListActions, constant.ToolRouterListActionsDescription, map[string]any{}),
			defineTool(true, constant.ToolRouterGroupPipeline, constant.ToolRouterToolListPipelineTasks, constant.ToolRouterPipelineListTasksDescription, map[string]any{}),
			defineTool(true, constant.ToolRouterGroupPipeline, constant.ToolRouterToolValidatePipelineTask, constant.ToolRouterValidateTaskDescription, map[string]any{"json": stringField(constant.ToolRouterTaskJSONDescription)}, "json"),
		}
		if role == constant.ToolRouterRolePipelineManager {
			return append(specs, defineTool(false, constant.ToolRouterGroupPipeline, constant.ToolRouterToolRunPipeline, constant.ToolRouterPipelineRunDescription, map[string]any{"taskId": stringField(constant.ToolRouterTaskIDDescription), "inputs": map[string]any{"type": "object"}}, "taskId", "inputs"), defineTool(false, constant.ToolRouterGroupPipeline, constant.ToolRouterToolShareArtifact, constant.ToolRouterPipelineShareDescription, map[string]any{"artifactId": stringField(constant.ToolRouterRegisteredArtifactIDDescription)}, "artifactId"))
		}
		return append(specs,
			defineTool(false, constant.ToolRouterGroupPipeline, constant.ToolRouterToolSavePipelineTask, constant.ToolRouterSaveTaskDescription, map[string]any{"json": stringField(constant.ToolRouterTaskJSONDescription)}, "json"),
			defineTool(false, constant.ToolRouterGroupPipeline, constant.ToolRouterToolDeletePipelineTask, constant.ToolRouterDeleteTaskDescription, map[string]any{"taskId": stringField(constant.ToolRouterTaskIDDescription)}, "taskId"),
			defineTool(false, constant.ToolRouterGroupPipeline, constant.ToolRouterToolSavePipelineSchedule, constant.ToolRouterSaveScheduleDescription, map[string]any{"json": stringField(constant.ToolRouterScheduleJSONDescription)}, "json"),
			defineTool(false, constant.ToolRouterGroupPipeline, constant.ToolRouterToolDeletePipelineSchedule, constant.ToolRouterDeleteScheduleDescription, map[string]any{"scheduleId": stringField(constant.ToolRouterScheduleIDDescription)}, "scheduleId"),
		)
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

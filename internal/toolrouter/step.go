package toolrouter

import (
	"encoding/json"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// stepCall 处理宿主绑定的当前步骤工具
func (r *Router) stepCall(scope tasksmodel.Scope, call codexmodel.ToolCall) (any, error) {
	var args struct {
		Query      string `json:"query"`      // 知识检索内容
		DocumentID string `json:"documentId"` // 知识文档标识
		Title      string `json:"title"`      // 新建文档标题
		Content    string `json:"content"`    // 完整文档正文
		Path       string `json:"path"`       // 待登记文件路径
		ArtifactID string `json:"artifactId"` // 待分享产物标识
	}
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return nil, err
	}

	switch call.Tool {
	case constant.ToolRouterToolGetStepContext:
		return r.work.GetStepContext(scope.StepID)
	case constant.ToolRouterToolSearchKnowledge:
		return r.work.SearchKnowledge(scope.ProjectID, scope.FeatureID, args.Query)
	case constant.ToolRouterToolReadKnowledge:
		return r.work.ReadKnowledge(scope.ProjectID, scope.FeatureID, args.DocumentID)
	case constant.ToolRouterToolPrepareKnowledgeDocument:
		return r.work.PrepareKnowledge(scope.ProjectID, scope.FeatureID, args.Title)
	case constant.ToolRouterToolBindKnowledgeDocument:
		_, err := r.work.BindKnowledge(scope.ProjectID, scope.FeatureID, args.DocumentID, true)
		return map[string]bool{"bound": err == nil}, err
	case constant.ToolRouterToolSyncKnowledgeDocument:
		return r.work.SyncKnowledge(scope.ProjectID, scope.FeatureID, args.DocumentID, args.Content)
	case constant.ToolRouterToolRegisterArtifact:
		return r.tasks.RegisterArtifact(scope.ExecutionID, args.Path)
	case constant.ToolRouterToolShareArtifact:
		for _, execution := range r.tasks.GetState().Executions {
			if execution.ID == scope.ExecutionID {
				err := r.lark.ShareArtifactFor(args.ArtifactID, scope.ProjectID, scope.ExecutionID, execution.Source)
				return map[string]bool{"queued": err == nil}, err
			}
		}
		return nil, constant.ErrToolRouterActiveExecutionNotFound
	}
	return nil, constant.ErrToolRouterToolUnknown
}

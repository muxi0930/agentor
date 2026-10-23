package toolrouter

import (
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	"agentor/internal/lark"
	"agentor/internal/pipeline"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
)

// API 按职责列出服务的全部公开方法
type API interface {
	Specs(role string) []codexmodel.DynamicToolSpec // 返回当前业务角色可以调用的工具
	Call(call codexmodel.ToolCall) (any, error)     // 按会话角色转交实际宿主工具请求
}

var _ API = (*Router)(nil)

type Router struct {
	tasks    *tasks.Service     // 步骤与控制中心任务
	work     *workspace.Service // 工作区与知识文档
	pipeline *pipeline.Service  // 流水线任务与会话
	lark     *lark.Service      // 飞书消息与产物分享
}

// New 组合宿主工具路由依赖
func New(executions *tasks.Service, work *workspace.Service, pipelines *pipeline.Service, remote *lark.Service) *Router {
	return &Router{tasks: executions, work: work, pipeline: pipelines, lark: remote}
}

// Specs 返回当前业务角色可以调用的工具
func (r *Router) Specs(role string) []codexmodel.DynamicToolSpec {
	definitions := roleTools(role)
	if definitions == nil {
		return nil
	}
	specs := make([]codexmodel.DynamicToolSpec, 0, len(definitions))
	for _, definition := range definitions {
		specs = append(specs, definition.spec)
	}
	return specs
}

// Call 按会话角色转交实际宿主工具请求
func (r *Router) Call(call codexmodel.ToolCall) (any, error) {
	if call.Tool == constant.ToolRouterToolCompleteNode && r.pipeline != nil {
		return r.pipeline.CompleteNode(call)
	}
	if r.pipeline != nil {
		for _, run := range r.pipeline.GetState().Runs {
			for _, attempt := range run.Actions {
				if attempt.ThreadID == call.ThreadID && call.ThreadID != "" {
					return nil, constant.ErrToolRouterRoleForbidden
				}
			}
		}
	}

	scope, err := r.tasks.ScopeForThread(call.ThreadID)
	if err != nil {
		return nil, err
	}

	if _, err := authorizeTool(scope.Role, call.Tool, false, constant.ErrToolRouterRoleForbidden, nil); err != nil {
		return nil, err
	}

	if scope.Role == constant.ToolRouterRoleController {
		return r.controllerCall(scope, call)
	}
	return r.stepCall(scope, call)
}

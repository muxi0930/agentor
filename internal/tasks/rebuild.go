package tasks

import (
	"errors"

	"agentor/internal/codex"
)

const stepInstructions = "你正在执行 agentor 的正式步骤。知识和产物工具的范围由宿主绑定至当前步骤。使用 read_knowledge 获取绑定文档，使用 prepare_knowledge_document / sync_knowledge_document 维护文档，使用 register_artifact 登记本次产物。遵守用户的步骤指令；不要伪造工具结果。"
const controllerInstructions = "你是 agentor 的工作控制器。通过宿主工具查询项目、创建功能和步骤、正式派发与停止任务；不要用文件或 shell 修改工作定义。宿主会约束每个请求的项目与来源。无法唯一确定目标时先询问用户；没有最近目标的‘继续’请求不可猜测。派发步骤后可以先说明已安排，完成事件到达后再汇报。收到 execution_result 时只汇报结果，不自动重新派发。"

func (s *Service) RebuildStepContext(stepID string) error {
	s.mu.Lock()
	if s.workers[stepID] {
		s.mu.Unlock()
		return errors.New("步骤正在处理队列，请先停止后再重建")
	}
	for _, execution := range s.state.Executions {
		if execution.StepID == stepID && execution.Status == "running" {
			s.mu.Unlock()
			return errors.New("请先停止当前步骤，再重建上下文")
		}
	}
	s.workers[stepID] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.workers, stepID); s.mu.Unlock(); s.kick(stepID) }()
	context, err := s.work.GetStepContext(stepID)
	if err != nil {
		return err
	}
	model, _, err := s.modelSelection("", "")
	if err != nil {
		return err
	}
	permission := context.Step.PermissionMode
	if permission == "" {
		permission = s.data.EffectivePermission(context.Step.ThreadID, context.Directory)
	}
	var tools []codex.DynamicToolSpec
	if s.toolSpecs != nil {
		tools = s.toolSpecs("step")
	}
	thread, err := s.runtime.CreateToolThread(context.Directory, model, permission, "step", stepInstructions, tools)
	if err != nil {
		return err
	}
	if err = s.work.BindStepThread(stepID, thread.ID); err != nil {
		return err
	}
	s.changed()
	return nil
}

func (s *Service) RebuildControllerContext() error {
	s.mu.Lock()
	if s.controllerWorker {
		s.mu.Unlock()
		return errors.New("请等待或停止控制器请求，再重建上下文")
	}
	s.controllerWorker = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.controllerWorker = false; s.mu.Unlock(); s.kickController() }()
	model, _, err := s.modelSelection("", "")
	if err != nil {
		return err
	}
	var tools []codex.DynamicToolSpec
	if s.toolSpecs != nil {
		tools = s.toolSpecs("controller")
	}
	thread, err := s.runtime.CreateToolThread(s.data.GetInfo().DataDirectory, model, "read-only", "controller", controllerInstructions, tools)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.state.ControllerThreadID = thread.ID
	err = s.persistLocked()
	s.mu.Unlock()
	s.changed()
	return err
}

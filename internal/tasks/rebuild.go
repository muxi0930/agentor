package tasks

import (
	"errors"

	"agentor/internal/codex"
)

const stepInstructions = "你正在执行 agentor 的正式步骤。知识和产物工具的范围由宿主绑定至当前步骤。使用 read_knowledge 获取绑定文档，使用 prepare_knowledge_document / sync_knowledge_document 维护文档，使用 register_artifact 登记本次产物。遵守用户的步骤指令；不要伪造工具结果。"

// RebuildStepContext 重建步骤专用会话并恢复队列处理
func (s *Service) RebuildStepContext(stepID string) error {
	context, err := s.work.GetStepContext(stepID)
	if err != nil {
		return err
	}
	featureID := context.FeatureID
	s.mu.Lock()
	if s.workers[featureID] {
		s.mu.Unlock()
		return errors.New("功能正在处理队列，请先停止后再重建")
	}
	for _, execution := range s.state.Executions {
		if execution.StepID == stepID && execution.Status == "running" {
			s.mu.Unlock()
			return errors.New("请先停止当前步骤，再重建上下文")
		}
	}
	s.workers[featureID] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.workers, featureID)
		s.mu.Unlock()
		s.kick(featureID)
	}()

	model, _, err := s.SelectModel("", "")
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

// RebuildControllerContext 重建控制中心专用会话并恢复队列处理
func (s *Service) RebuildControllerContext() error {
	s.mu.Lock()
	if s.controllerWorker {
		s.mu.Unlock()
		return errors.New("请等待或停止控制中心请求，再重建上下文")
	}
	s.controllerWorker = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.controllerWorker = false
		s.mu.Unlock()
		s.kickController()
	}()

	model, effort, err := s.SelectModel("", "")
	if err != nil {
		return err
	}
	instructions := controllerPrompt(s.executorSettings(model, effort))
	var tools []codex.DynamicToolSpec
	if s.toolSpecs != nil {
		tools = s.toolSpecs("controller")
	}
	thread, err := s.runtime.CreateToolThread(s.data.GetInfo().CodexHome, model, "read-only", "controller", instructions, tools)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.state.ControllerThreadID = thread.ID
	s.state.ControllerInstructions = instructions
	err = s.persistLocked()
	s.mu.Unlock()
	s.changed()
	return err
}

package tasks

import (
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

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
		return constant.ErrTasksFeatureBusy
	}
	for _, execution := range s.state.Executions {
		if execution.StepID == stepID && execution.Status == constant.TasksStatusRunning {
			s.mu.Unlock()
			return constant.ErrTasksStepBusy
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
	var tools []codexmodel.DynamicToolSpec
	if s.toolSpecs != nil {
		tools = s.toolSpecs(constant.ToolRouterRoleStep)
	}
	thread, err := s.runtime.CreateToolThread(context.Directory, model, permission, constant.WorkspaceKindStep, constant.TasksStepInstructions, tools)
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
		return constant.ErrTasksControllerBusy
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
	var tools []codexmodel.DynamicToolSpec
	if s.toolSpecs != nil {
		tools = s.toolSpecs(constant.ToolRouterRoleController)
	}
	thread, err := s.runtime.CreateToolThread(s.data.GetInfo().CodexHome, model, constant.AppDataPermissionReadOnly, constant.ToolRouterRoleController, instructions, tools)
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

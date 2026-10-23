package tasks

import (
	"context"

	"agentor/internal/constant"
)

// PrepareDelete 停止关联执行后允许删除步骤并返回保护解除入口
//
//wails:ignore
func (s *Service) PrepareDelete(stepIDs []string) (func(), error) {
	s.mu.Lock()
	ids := map[string]bool{}
	for _, id := range stepIDs {
		s.deleting[id] = true
		ids[id] = true
	}
	executions := []string{}
	for _, execution := range s.state.Executions {
		if ids[execution.StepID] && (execution.Status == constant.TasksStatusQueued || execution.Status == constant.TasksStatusRunning) {
			executions = append(executions, execution.ID)
		}
	}
	s.mu.Unlock()

	release := func() {
		s.mu.Lock()
		for _, id := range stepIDs {
			delete(s.deleting, id)
		}
		s.mu.Unlock()
	}
	for _, id := range executions {
		if err := s.StopExecution(id); err != nil {
			release()
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(s.ctx, constant.TasksDeleteTimeout)
	defer cancel()
	for _, id := range executions {
		result, err := s.WaitExecution(ctx, id)
		if err != nil {
			release()
			return nil, err
		}
		if result.ThreadID != "" && s.runtime.IsThreadActive(result.ThreadID) {
			release()
			return nil, constant.ErrTasksUpstreamStillRunning
		}
	}
	return release, nil
}

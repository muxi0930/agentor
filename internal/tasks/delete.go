package tasks

import (
	"context"
	"errors"
	"time"
)

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
		if ids[execution.StepID] && (execution.Status == "queued" || execution.Status == "running") {
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
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	for _, id := range executions {
		result, err := s.WaitExecution(ctx, id)
		if err != nil {
			release()
			return nil, err
		}
		if result.ThreadID != "" && s.runtime.IsThreadActive(result.ThreadID) {
			release()
			return nil, errors.New("上游执行仍未停止，请检查连接后再移除工作定义")
		}
	}
	return release, nil
}

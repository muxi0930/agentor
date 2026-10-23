package tasks

import (
	"strings"

	"agentor/internal/constant"
)

// ReportOutcome 保存当前执行的结构化业务结果，不直接改变执行状态
func (s *Service) ReportOutcome(id string, success bool, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(summary) == "" {
		return constant.ErrTasksOutcomeRequired
	}
	for i := range s.state.Executions {
		execution := &s.state.Executions[i]
		if execution.ID == id && execution.Status == constant.TasksStatusRunning {
			previous := *execution
			execution.Outcome, execution.OutcomeSummary = constant.TasksStatusError, summary
			if success {
				execution.Outcome = constant.TasksStatusCompleted
			}
			if err := s.persistLocked(); err != nil {
				*execution = previous
				return err
			}
			return nil
		}
	}
	return constant.ErrTasksOutcomeInactive
}

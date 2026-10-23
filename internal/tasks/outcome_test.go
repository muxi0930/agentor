package tasks

import (
	"testing"

	tasksmodel "agentor/internal/tasks/model"
)

// TestReportOutcomeKeepsTurnState 验证业务汇报不能直接宣告执行完成且必须绑定活动执行
func TestReportOutcomeKeepsTurnState(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	s.state.Executions = []tasksmodel.Execution{{ID: "active", Status: "running"}, {ID: "done", Status: "completed"}}
	if err = s.ReportOutcome("active", true, "tests passed"); err != nil {
		t.Fatal(err)
	}
	got := s.GetState().Executions[0]
	if got.Status != "running" || got.Outcome != "completed" || got.OutcomeSummary != "tests passed" {
		t.Fatal(got)
	}
	if err = s.ReportOutcome("active", false, "verification failed"); err != nil {
		t.Fatal(err)
	}
	if s.GetState().Executions[0].Outcome != "error" {
		t.Fatal("failure report lost")
	}
	for _, id := range []string{"missing", "done"} {
		if err = s.ReportOutcome(id, true, "done"); err == nil {
			t.Fatal("inactive report accepted", id)
		}
	}
	if err = s.ReportOutcome("active", true, " "); err == nil {
		t.Fatal("empty evidence accepted")
	}
}

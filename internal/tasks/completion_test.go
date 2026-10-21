package tasks

import (
	"path/filepath"
	"testing"

	"agentor/internal/appdata"
	"agentor/internal/workspace"
)

// TestCompletionWithoutStartupBackfill 验证启动不回填历史状态且后续重复派发仍正常同步
func TestCompletionWithoutStartupBackfill(t *testing.T) {
	data := testData(t)
	root := data.GetInfo().DataDirectory
	initial := workspace.State{Projects: []workspace.Project{{ID: "p", Versions: []workspace.Version{{ID: "v", Features: []workspace.Feature{
		{ID: "f", Steps: []workspace.Step{{ID: "a"}, {ID: "b"}}},
	}}}}}}
	if err := appdata.WriteJSON(filepath.Join(root, "workspace.json"), initial); err != nil {
		t.Fatal(err)
	}
	work, err := workspace.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	completedAt := "2026-10-21T01:00:00Z"
	history := State{Executions: []Execution{
		{ID: "a1", StepID: "a", Status: "completed", CompletedAt: completedAt},
		{ID: "b1", StepID: "b", Status: "completed", CompletedAt: completedAt},
		{ID: "b2", StepID: "b", Status: "error"},
		{ID: "b3", StepID: "b", Status: "queued"},
	}}
	if err := appdata.WriteJSON(filepath.Join(root, "tasks.json"), history); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(data, nil, work, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	feature := work.GetState().Projects[0].Versions[0].Features[0]
	if feature.Steps[0].Completed || feature.Steps[0].CompletedAt != "" || feature.Steps[1].Completed || feature.Completed {
		t.Fatal("启动时不应从历史记录回填完成状态", feature)
	}
	s.state.Executions = append(s.state.Executions, Execution{ID: "a2", StepID: "a", Status: "running"})
	s.finish("a2", "completed", "完成", nil)
	s.state.Executions[3].Status = "running"
	s.state.Executions = append(s.state.Executions,
		Execution{ID: "b4", StepID: "b", Status: "queued"},
	)
	s.finish("b3", "completed", "完成", nil)
	feature = work.GetState().Projects[0].Versions[0].Features[0]
	if feature.Completed || feature.Steps[1].Completed {
		t.Fatal("存在后续排队执行时不应提前完成", feature)
	}
	s.state.Executions[len(s.state.Executions)-1].Status = "running"
	s.finish("b4", "completed", "完成", nil)
	feature = work.GetState().Projects[0].Versions[0].Features[0]
	if !feature.Completed || feature.CompletedAt == "" || !feature.Steps[1].Completed {
		t.Fatal("最后一次执行完成后未同步功能完成状态", feature)
	}
}

package workspace

import (
	"path/filepath"
	"testing"

	"agentor/internal/appdata"
)

// TestCompletionAggregation 验证完成状态持久化及新增、删除步骤后的功能汇总
func TestCompletionAggregation(t *testing.T) {
	root := t.TempDir()
	initial := State{Projects: []Project{{ID: "p", Versions: []Version{{ID: "v", Features: []Feature{
		{ID: "f", Steps: []Step{{ID: "a"}, {ID: "b"}}},
		{ID: "empty"},
	}}}}}}
	if err := appdata.WriteJSON(filepath.Join(root, "workspace.json"), initial); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	early, late := "2026-10-21T01:00:00Z", "2026-10-21T02:00:00Z"
	if err := s.SyncCompletion(map[string]StepCompletion{
		"a": {Completed: true, CompletedAt: late},
		"b": {Completed: true, CompletedAt: early},
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	features := reopened.GetState().Projects[0].Versions[0].Features
	if !features[0].Completed || features[0].CompletedAt != late || features[1].Completed {
		t.Fatal("功能完成状态或最晚完成时间不正确", features)
	}
	state, err := s.SaveStep("p", "f", Step{Title: "新增步骤"})
	if err != nil {
		t.Fatal(err)
	}
	feature := state.Projects[0].Versions[0].Features[0]
	if feature.Completed || feature.CompletedAt != "" {
		t.Fatal("新增步骤后应清除功能完成状态", feature)
	}
	state, err = s.DeleteNode("step", state.SelectedID)
	if err != nil || !state.Projects[0].Versions[0].Features[0].Completed {
		t.Fatal("删除未完成步骤后应重新汇总", err)
	}
	if err := s.SyncCompletion(map[string]StepCompletion{"a": {}}); err != nil {
		t.Fatal(err)
	}
	feature = s.GetState().Projects[0].Versions[0].Features[0]
	if feature.Completed || feature.CompletedAt != "" || feature.Steps[0].Completed || feature.Steps[0].CompletedAt != "" {
		t.Fatal("重新派发后应清除步骤和功能的完成信息", feature)
	}
}

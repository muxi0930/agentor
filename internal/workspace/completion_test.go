package workspace

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"agentor/internal/appdata"
)

// TestCompleteStep 验证手动完成持久化、功能汇总及重复操作保留完成时间
func TestCompleteStep(t *testing.T) {
	root := t.TempDir()
	initial := State{SelectedID: "a", Projects: []Project{{ID: "p", Versions: []Version{{ID: "v", Features: []Feature{
		{ID: "f", Steps: []Step{{ID: "a", Title: "步骤", Prompt: "执行指令", ThreadID: "thread", PermissionMode: "workspace-write"}, {ID: "b", Completed: true, CompletedAt: "2026-10-21T01:00:00Z"}}},
	}}}}}}
	if err := appdata.WriteJSON(filepath.Join(root, "workspace.json"), initial); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.CompleteStep("a")
	if err != nil {
		t.Fatal(err)
	}
	feature := state.Projects[0].Versions[0].Features[0]
	step := feature.Steps[0]
	if !step.Completed || step.CompletedAt == "" || !feature.Completed || feature.CompletedAt != step.CompletedAt {
		t.Fatal("手动完成未同步步骤与功能状态", feature)
	}
	want := initial.Projects[0].Versions[0].Features[0].Steps[0]
	want.Completed, want.CompletedAt = true, step.CompletedAt
	if !reflect.DeepEqual(step, want) || state.SelectedID != initial.SelectedID {
		t.Fatal("手动完成不应改变步骤定义、会话或选择", state)
	}
	reopened, err := NewService(root)
	if err != nil || !reflect.DeepEqual(reopened.GetState(), state) {
		t.Fatal("重新读取后未保留完成状态", err)
	}
	again, err := s.CompleteStep("a")
	if err != nil || !reflect.DeepEqual(again, state) {
		t.Fatal("重复完成不应改变完成时间", err)
	}
	if _, err := s.CompleteStep("missing"); err == nil || !reflect.DeepEqual(s.GetState(), state) {
		t.Fatal("不存在的步骤应返回错误且不改变状态")
	}
	completedAt, err := time.Parse(time.RFC3339Nano, step.CompletedAt)
	if err != nil {
		t.Fatal(err)
	}
	for _, dispatchedAt := range []string{completedAt.Add(-time.Hour).Format(time.RFC3339Nano), completedAt.Add(time.Hour).Format(time.RFC3339Nano)} {
		if err := s.SyncCompletion(map[string]StepCompletion{"a": {DispatchedAt: dispatchedAt}}); err != nil {
			t.Fatal(err)
		}
		current := s.GetState().Projects[0].Versions[0].Features[0]
		if current.Completed != (dispatchedAt < step.CompletedAt) {
			t.Fatal("旧执行应保留手动完成，新派发应重置完成状态", current)
		}
	}
}

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

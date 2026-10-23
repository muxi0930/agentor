package tasks

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"agentor/internal/appdata"
	codexmodel "agentor/internal/codex/model"
	tasksmodel "agentor/internal/tasks/model"
	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"
)

// TestDeletingStepStopsItsRunningAndQueuedExecutionsFirst 验证删除步骤前先停止关联运行与排队执行
func TestDeletingStepStopsItsRunningAndQueuedExecutionsFirst(t *testing.T) {
	data := testData(t)
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspacemodel.Project{Name: "delete check", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspacemodel.Version{Name: "v"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspacemodel.Feature{Title: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveStep(projectID, state.SelectedID, workspacemodel.Step{Title: "step"})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID
	stepContext, err := work.GetStepContext(stepID)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewService(data, nil, work, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	work.SetDeleteGuard(s.PrepareDelete)
	started := make(chan struct{})
	stoppedBeforeRemoval := make(chan bool, 1)
	s.runExecution = func(ctx context.Context, execution tasksmodel.Execution) (string, string, error) {
		close(started)
		<-ctx.Done()
		_, err := work.GetStepContext(stepID)
		stoppedBeforeRemoval <- err == nil
		return "interrupted", "", ctx.Err()
	}
	s.state.Executions = []tasksmodel.Execution{{ID: "running", StepID: stepID, FeatureID: stepContext.FeatureID, Status: "queued"}, {ID: "queued", StepID: stepID, FeatureID: stepContext.FeatureID, Status: "queued"}}
	s.kick(stepContext.FeatureID)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("execution did not start")
	}
	if _, err = work.DeleteNode("step", stepID); err != nil {
		t.Fatal(err)
	}
	if !<-stoppedBeforeRemoval {
		t.Fatal("work definition was removed before cancellation finished")
	}
	for _, execution := range s.GetState().Executions {
		if execution.Status != "interrupted" {
			t.Fatal(execution)
		}
	}
	if _, err = work.GetStepContext(stepID); err == nil {
		t.Fatal("step was not removed")
	}
}

// TestFeatureQueueIsSerialAndDifferentFeaturesRunTogether 验证同功能不同步骤及重复步骤串行而不同功能可并行执行
func TestFeatureQueueIsSerialAndDifferentFeaturesRunTogether(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.ServiceShutdown() })
	started := make(chan string, 4)
	release := map[string]chan struct{}{"a1": make(chan struct{}), "a2": make(chan struct{}), "a3": make(chan struct{}), "b1": make(chan struct{})}
	s.runExecution = func(ctx context.Context, execution tasksmodel.Execution) (string, string, error) {
		started <- execution.ID
		select {
		case <-release[execution.ID]:
			return "completed", execution.ID, nil
		case <-ctx.Done():
			return "interrupted", "", ctx.Err()
		}
	}
	s.state.Executions = []tasksmodel.Execution{{ID: "a1", ProjectID: "project", FeatureID: "a", StepID: "step-a1", Status: "queued"}, {ID: "a2", ProjectID: "project", FeatureID: "a", StepID: "step-a2", Status: "queued"}, {ID: "a3", ProjectID: "project", FeatureID: "a", StepID: "step-a1", Status: "queued"}, {ID: "b1", ProjectID: "project", FeatureID: "b", StepID: "step-b1", Status: "queued"}}
	s.kick("a")
	s.kick("a")
	s.kick("b")
	first := map[string]bool{}
	for range 2 {
		select {
		case id := <-started:
			first[id] = true
		case <-time.After(time.Second):
			t.Fatal("independent features did not start")
		}
	}
	if !first["a1"] || !first["b1"] || first["a2"] {
		t.Fatal("same-feature queue order violated", first)
	}
	close(release["a1"])
	select {
	case id := <-started:
		if id != "a2" {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("next step in the same feature did not start")
	}
	close(release["a2"])
	select {
	case id := <-started:
		if id != "a3" {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("repeated step execution did not start")
	}
	close(release["a3"])
	close(release["b1"])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, id := range []string{"a1", "a2", "a3", "b1"} {
		result, err := s.WaitExecution(ctx, id)
		if err != nil || result.Status != "completed" || result.Result != id {
			t.Fatal(result, err)
		}
	}
}

// TestRestartInterruptsRunningAndLeavesQueuedPaused 验证重启后中断活动执行并暂停保留的排队项
func TestRestartInterruptsRunningAndLeavesQueuedPaused(t *testing.T) {
	data := testData(t)
	if err := appdata.WriteJSON(filepath.Join(data.GetInfo().DataDirectory, "tasks.json"), tasksmodel.State{Executions: []tasksmodel.Execution{{ID: "running", Status: "running"}, {ID: "queued", Status: "queued"}, {ID: "done", Status: "completed"}}}); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(data, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	state := s.GetState()
	if !state.Paused || state.Executions[0].Status != "interrupted" || state.Executions[1].Status != "queued" || state.Executions[2].Status != "completed" {
		t.Fatal(state)
	}
}

// TestPartialTurnCompletionKeepsFinalAgentText 验证轮次完成事件不丢失此前收到的最终助手正文
func TestPartialTurnCompletionKeepsFinalAgentText(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	waiter := &turnWaiter{turnID: "turn", items: map[string]string{}, done: make(chan turnOutcome, 1)}
	s.turns["thread"] = waiter
	s.OnCodexEvent(codexmodel.Event{Method: "item/completed", Params: `{"threadId":"thread","turnId":"turn","item":{"type":"agentMessage","id":"answer","text":"FINAL_RESULT","phase":"final_answer"}}`})
	s.OnCodexEvent(codexmodel.Event{Method: "turn/completed", Params: `{"threadId":"thread","turn":{"id":"turn","status":"completed","items":[{"type":"userMessage","id":"user","content":[{"type":"text","text":"task"}]}]}}`})
	select {
	case result := <-waiter.done:
		if result.result != "FINAL_RESULT" || result.status != "completed" {
			t.Fatal(result)
		}
	default:
		t.Fatal("no completion")
	}
	if _, err = json.Marshal(s.GetState()); err != nil {
		t.Fatal(err)
	}
}

// testData 创建使用隔离数据目录的应用设置服务
func testData(t *testing.T) *appdata.Service {
	t.Helper()
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

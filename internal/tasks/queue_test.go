package tasks

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/codex"
	"agentor/internal/workspace"
)

func testData(t *testing.T) *appdata.Service {
	t.Helper()
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDeletingStepStopsItsRunningAndQueuedExecutionsFirst(t *testing.T) {
	data := testData(t)
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspace.Project{Name: "delete check", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspace.Version{Name: "v"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspace.Feature{Title: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveStep(projectID, state.SelectedID, workspace.Step{Title: "step"})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID
	s, err := NewService(data, nil, work, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	work.SetDeleteGuard(s.PrepareDelete)
	started := make(chan struct{})
	stoppedBeforeRemoval := make(chan bool, 1)
	s.runExecution = func(ctx context.Context, execution Execution) (string, string, error) {
		close(started)
		<-ctx.Done()
		_, err := work.GetStepContext(stepID)
		stoppedBeforeRemoval <- err == nil
		return "interrupted", "", ctx.Err()
	}
	s.state.Executions = []Execution{{ID: "running", StepID: stepID, Status: "queued"}, {ID: "queued", StepID: stepID, Status: "queued"}}
	s.kick(stepID)
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

func TestStepQueueIsSerialAndDifferentStepsRunTogether(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.ServiceShutdown() })
	started := make(chan string, 3)
	release := map[string]chan struct{}{"a1": make(chan struct{}), "a2": make(chan struct{}), "b1": make(chan struct{})}
	s.runExecution = func(ctx context.Context, execution Execution) (string, string, error) {
		started <- execution.ID
		select {
		case <-release[execution.ID]:
			return "completed", execution.ID, nil
		case <-ctx.Done():
			return "interrupted", "", ctx.Err()
		}
	}
	s.state.Executions = []Execution{{ID: "a1", StepID: "a", Status: "queued"}, {ID: "a2", StepID: "a", Status: "queued"}, {ID: "b1", StepID: "b", Status: "queued"}}
	s.kick("a")
	s.kick("b")
	first := map[string]bool{}
	for range 2 {
		select {
		case id := <-started:
			first[id] = true
		case <-time.After(time.Second):
			t.Fatal("independent steps did not start")
		}
	}
	if !first["a1"] || !first["b1"] || first["a2"] {
		t.Fatal("same-step queue order violated", first)
	}
	close(release["a1"])
	select {
	case id := <-started:
		if id != "a2" {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("next same-step execution did not start")
	}
	close(release["a2"])
	close(release["b1"])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, id := range []string{"a1", "a2", "b1"} {
		result, err := s.WaitExecution(ctx, id)
		if err != nil || result.Status != "completed" || result.Result != id {
			t.Fatal(result, err)
		}
	}
}

func TestRestartInterruptsRunningAndLeavesQueuedPaused(t *testing.T) {
	data := testData(t)
	if err := appdata.WriteJSON(filepath.Join(data.GetInfo().DataDirectory, "tasks.json"), State{Executions: []Execution{{ID: "running", Status: "running"}, {ID: "queued", Status: "queued"}, {ID: "done", Status: "completed"}}}); err != nil {
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

func TestPartialTurnCompletionKeepsFinalAgentText(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	waiter := &turnWaiter{turnID: "turn", items: map[string]string{}, done: make(chan turnOutcome, 1)}
	s.turns["thread"] = waiter
	s.OnCodexEvent(codex.Event{Method: "item/completed", Params: `{"threadId":"thread","turnId":"turn","item":{"type":"agentMessage","id":"answer","text":"FINAL_RESULT","phase":"final_answer"}}`})
	s.OnCodexEvent(codex.Event{Method: "turn/completed", Params: `{"threadId":"thread","turn":{"id":"turn","status":"completed","items":[{"type":"userMessage","id":"user","content":[{"type":"text","text":"task"}]}]}}`})
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

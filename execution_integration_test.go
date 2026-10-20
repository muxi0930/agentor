package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
)

func TestNativeStepKnowledgeTool(t *testing.T) {
	if os.Getenv("AGENTOR_NATIVE_EXECUTION_SMOKE") != "1" {
		t.Skip("opt-in native Codex execution")
	}
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = data.SetModelSelection(os.Getenv("AGENTOR_SMOKE_MODEL"), "medium"); err != nil {
		t.Fatal(err)
	}
	files, err := artifact.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	runtime := codex.NewService(data, files)
	executions, err := tasks.NewService(data, runtime, work, files)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.ServiceShutdown()
	defer executions.ServiceShutdown()
	router := &toolRouter{tasks: executions, work: work}
	executions.ConfigureTools(router.specs)
	var reads atomic.Int32
	runtime.SetToolHandler(func(call codex.ToolCall) (any, error) {
		value, err := router.call(call)
		if call.Tool == "read_knowledge" && err == nil {
			reads.Add(1)
		}
		return value, err
	})
	runtime.SetEventHandler(executions.OnCodexEvent)
	if _, err = runtime.Connect(); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	state, err := work.SaveProject(workspace.Project{Name: "Native execution check", Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspace.Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspace.Feature{Title: "Knowledge tool"})
	if err != nil {
		t.Fatal(err)
	}
	featureID := state.SelectedID
	state, err = work.SaveStep(projectID, featureID, workspace.Step{Title: "Read check word", Prompt: "Use read_knowledge to read the bound document. Reply with only the verification word from it."})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID
	path := filepath.Join(directory, "check.md")
	if err = os.WriteFile(path, []byte("# Native knowledge check\nThe verification word is SAFFRON.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = work.ImportKnowledge(projectID, featureID, path); err != nil {
		t.Fatal(err)
	}
	execution, err := executions.Dispatch(tasks.DispatchInput{StepID: stepID, Source: tasks.Source{Type: "desktop", MessageID: "native-smoke"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := executions.WaitExecution(ctx, execution.ID)
	if err != nil {
		executions.StopExecution(execution.ID)
		t.Fatal(err)
	}
	if result.Status != "completed" || !strings.Contains(result.Result, "SAFFRON") || reads.Load() == 0 {
		t.Fatalf("status=%s result=%q error=%s knowledgeReads=%d", result.Status, result.Result, result.Error, reads.Load())
	}
	t.Logf("native execution %s thread %s turn %s completed via %d read_knowledge call(s)", result.ID, result.ThreadID, result.TurnID, reads.Load())
}

func TestNativeControllerDispatchAndReport(t *testing.T) {
	if os.Getenv("AGENTOR_NATIVE_CONTROLLER_SMOKE") != "1" {
		t.Skip("opt-in native controller execution")
	}
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = data.SetModelSelection(os.Getenv("AGENTOR_SMOKE_MODEL"), "medium"); err != nil {
		t.Fatal(err)
	}
	files, err := artifact.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	runtime := codex.NewService(data, files)
	executions, err := tasks.NewService(data, runtime, work, files)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.ServiceShutdown()
	defer executions.ServiceShutdown()
	router := &toolRouter{tasks: executions, work: work}
	executions.ConfigureTools(router.specs)
	var dispatches atomic.Int32
	runtime.SetToolHandler(func(call codex.ToolCall) (any, error) {
		value, err := router.call(call)
		if call.Tool == "dispatch_step" && err == nil {
			dispatches.Add(1)
		}
		return value, err
	})
	runtime.SetEventHandler(executions.OnCodexEvent)
	work.SetDeleteGuard(executions.PrepareDelete)
	if _, err = runtime.Connect(); err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspace.Project{Name: "Controller dispatch check", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspace.Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspace.Feature{Title: "Controller workflow"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveStep(projectID, state.SelectedID, workspace.Step{Title: "Return marker", Prompt: "Reply only CONTROLLER_STEP_DONE."})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID
	request, err := executions.SubmitRequest(tasks.ControllerInput{ProjectID: projectID, StepID: stepID, Text: "Use dispatch_step exactly once to run the explicitly targeted step. Set instruction to 'Reply only CONTROLLER_STEP_DONE.' and completionCriteria to 'The reply contains CONTROLLER_STEP_DONE'. Then briefly acknowledge it was queued. Do not create definitions or run shell commands.", Source: tasks.Source{Type: "desktop", MessageID: "controller-native-smoke"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := executions.GetState()
		var result *tasks.Execution
		var report *tasks.Request
		for i := range snapshot.Executions {
			if snapshot.Executions[i].RequestID == request.ID {
				result = &snapshot.Executions[i]
			}
		}
		for i := range snapshot.Requests {
			if snapshot.Requests[i].ID == request.ID && snapshot.Requests[i].Status == "error" {
				t.Fatal(snapshot.Requests[i].Error)
			}
			if snapshot.Requests[i].CompletionExecutionID != "" {
				report = &snapshot.Requests[i]
			}
		}
		if result != nil && result.Status == "error" {
			t.Fatal(result.Error)
		}
		if result != nil && result.Status == "completed" && report != nil && report.Status == "completed" {
			if dispatches.Load() != 1 || !strings.Contains(result.Result, "CONTROLLER_STEP_DONE") || !strings.Contains(report.Result, "CONTROLLER_STEP_DONE") || result.Model != request.Model || result.Effort != request.Effort {
				t.Fatalf("dispatches=%d execution=%#v report=%#v", dispatches.Load(), result, report)
			}
			t.Logf("controller request %s dispatched execution %s once; completion report %s received", request.ID, result.ID, report.ID)
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("controller workflow timed out: %#v", snapshot)
		}
	}
}

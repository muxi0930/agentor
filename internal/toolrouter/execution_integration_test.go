package toolrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/tasks"
	tasksmodel "agentor/internal/tasks/model"
	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"
)

// TestNativeStepKnowledgeTool 验证原生步骤会话调用知识工具
func TestNativeStepKnowledgeTool(t *testing.T) {
	if os.Getenv("AGENTOR_NATIVE_EXECUTION_SMOKE") != "1" {
		t.Skip("opt-in native Codex execution")
	}
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService("test")
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
	router := &Router{tasks: executions, work: work}
	executions.ConfigureTools(router.Specs)
	var reads atomic.Int32
	runtime.SetToolHandler(func(call codexmodel.ToolCall) (any, error) {
		value, err := router.Call(call)
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
	state, err := work.SaveProject(workspacemodel.Project{Name: "Native execution check", Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspacemodel.Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspacemodel.Feature{Title: "Knowledge tool"})
	if err != nil {
		t.Fatal(err)
	}
	featureID := state.SelectedID
	state, err = work.SaveStep(projectID, featureID, workspacemodel.Step{Title: "Read check word", Prompt: "Use read_knowledge to read the bound document. Reply with only the verification word from it."})
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
	execution, err := executions.Dispatch(tasksmodel.DispatchInput{StepID: stepID, Source: tasksmodel.Source{Type: "desktop", MessageID: "native-smoke"}})
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

// TestNativeControllerDispatchAndReport 验证原生控制中心派发步骤并汇报结果
func TestNativeControllerDispatchAndReport(t *testing.T) {
	if os.Getenv("AGENTOR_NATIVE_CONTROLLER_SMOKE") != "1" {
		t.Skip("opt-in native controller execution")
	}
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService("test")
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
	router := &Router{tasks: executions, work: work}
	executions.ConfigureTools(router.Specs)
	var dispatches atomic.Int32
	runtime.SetToolHandler(func(call codexmodel.ToolCall) (any, error) {
		value, err := router.Call(call)
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
	state, err := work.SaveProject(workspacemodel.Project{Name: "Controller dispatch check", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspacemodel.Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspacemodel.Feature{Title: "Controller workflow"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveStep(projectID, state.SelectedID, workspacemodel.Step{Title: "Return marker", Prompt: "Reply only CONTROLLER_STEP_DONE."})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID
	request, err := executions.SubmitRequest(tasksmodel.ControllerInput{ProjectID: projectID, StepID: stepID, Text: "Use dispatch_step exactly once to run the explicitly targeted step. Set instruction to 'Reply only CONTROLLER_STEP_DONE.' and completionCriteria to 'The reply contains CONTROLLER_STEP_DONE'. Then briefly acknowledge it was queued. Do not create definitions or run shell commands.", Source: tasksmodel.Source{Type: "desktop", MessageID: "controller-native-smoke"}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := executions.GetState()
		var result *tasksmodel.Execution
		var report *tasksmodel.Request
		for i := range snapshot.Executions {
			if snapshot.Executions[i].RequestID == request.ID {
				result = &snapshot.Executions[i]
			}
		}
		for i := range snapshot.Requests {
			if snapshot.Requests[i].ID == request.ID && snapshot.Requests[i].Status == "error" {
				t.Fatal(snapshot.Requests[i].Error)
			}
			if snapshot.Requests[i].CompletionID != "" {
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

// TestNativeControllerCreateVersion 验证原生控制中心创建版本并经现有查询读取及拒绝越界
func TestNativeControllerCreateVersion(t *testing.T) {
	if os.Getenv("AGENTOR_NATIVE_CONTROLLER_SMOKE") != "1" {
		t.Skip("opt-in native controller execution")
	}
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService("test")
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
	state, err := work.SaveProject(workspacemodel.Project{Name: "创建版本验证", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveProject(workspacemodel.Project{Name: "范围外项目", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	otherID := state.SelectedID
	runtime := codex.NewService(data, files)
	executions, err := tasks.NewService(data, runtime, work, files)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.ServiceShutdown()
	defer executions.ServiceShutdown()
	router := New(executions, work, nil, nil)
	executions.ConfigureTools(router.Specs)
	var creates, reads, lists atomic.Int32
	runtime.SetToolHandler(func(call codexmodel.ToolCall) (any, error) {
		if call.Tool == "create_version" {
			for _, target := range []string{otherID, "missing", ""} {
				arguments, _ := json.Marshal(map[string]string{"projectId": target, "name": "禁止创建", "description": ""})
				if _, err := router.Call(codexmodel.ToolCall{ThreadID: call.ThreadID, Tool: "create_version", Arguments: arguments}); err == nil {
					return nil, fmt.Errorf("越界或缺失项目未被拒绝：%q", target)
				}
			}
		}
		value, err := router.Call(call)
		if err == nil {
			switch call.Tool {
			case "create_version":
				creates.Add(1)
			case "get_work_item":
				if version, ok := value.(workspacemodel.Version); ok && version.Name == "0.2.0" {
					reads.Add(1)
				}
			case "list_workspaces":
				payload, _ := json.Marshal(value)
				if strings.Contains(string(payload), "0.2.0") {
					lists.Add(1)
				}
			}
		}
		return value, err
	})
	runtime.SetEventHandler(executions.OnCodexEvent)
	if _, err = runtime.Connect(); err != nil {
		t.Fatal(err)
	}
	request, err := executions.SubmitRequest(tasksmodel.ControllerInput{
		ProjectID: projectID,
		Text:      fmt.Sprintf("请用 create_version 在项目 %s 下创建名称为 0.2.0、说明为宿主工具验证的版本，只创建一次。随后用 get_work_item 按返回的版本 ID 读取，再用 list_workspaces 确认版本存在。不要使用 shell 或派发步骤。", projectID),
		Source:    tasksmodel.Source{Type: "desktop", MessageID: "create-version-smoke"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		for _, current := range executions.GetState().Requests {
			if current.ID != request.ID {
				continue
			}
			if current.Status == "error" || current.Status == "interrupted" {
				t.Fatal(current)
			}
			if current.Status == "completed" {
				state = work.GetState()
				if creates.Load() != 1 || reads.Load() == 0 || lists.Load() == 0 || state.SelectedID != otherID || len(state.Projects[0].Versions) != 1 || len(state.Projects[1].Versions) != 0 {
					t.Fatalf("creates=%d reads=%d lists=%d state=%#v", creates.Load(), reads.Load(), lists.Load(), state)
				}
				t.Logf("控制中心创建版本并通过两项现有查询读取：%s", state.Projects[0].Versions[0].ID)
				return
			}
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("控制中心创建版本超时", executions.GetState())
		}
	}
}

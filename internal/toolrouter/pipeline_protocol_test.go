package toolrouter

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	"agentor/internal/pipeline"
	pipelinemodel "agentor/internal/pipeline/model"
	"agentor/internal/tasks"
	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"
)

// TestPipelineAgentProtocol 验证原生工具请求、正常结束门槛及绑定步骤完成状态
func TestPipelineAgentProtocol(t *testing.T) {
	for _, mode := range []string{"success", "missing", "interrupted", "failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("AGENTOR_ROUTER_PROTOCOL_TEST", "1")
			t.Setenv("AGENTOR_ROUTER_PIPELINE_PROTOCOL", mode)
			t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
			data, err := appdata.NewService("pipeline-protocol")
			if err != nil {
				t.Fatal(err)
			}
			executable, _ := os.Executable()
			if _, err = data.SetCodexPath(executable); err != nil {
				t.Fatal(err)
			}
			data.SetModelSelection("test", "medium")
			root := data.GetInfo().DataDirectory
			files, err := artifact.NewService(root)
			if err != nil {
				t.Fatal(err)
			}
			work, err := workspace.NewService(root)
			if err != nil {
				t.Fatal(err)
			}
			state, err := work.SaveProject(workspacemodel.Project{Name: "bound", Directory: t.TempDir()})
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
			state, err = work.SaveStep(projectID, state.SelectedID, workspacemodel.Step{Title: "bound step"})
			if err != nil {
				t.Fatal(err)
			}
			stepID := state.SelectedID
			runtime := codex.NewService(data, files)
			defer runtime.ServiceShutdown()
			executions, err := tasks.NewService(data, runtime, work, files)
			if err != nil {
				t.Fatal(err)
			}
			defer executions.ServiceShutdown()
			pipelines, err := pipeline.NewService(data, work, executions, files, runtime)
			if err != nil {
				t.Fatal(err)
			}
			defer pipelines.ServiceShutdown()
			router := New(executions, work, pipelines, nil)
			pipelines.ConfigureTools(router.Specs)
			runtime.SetToolHandler(router.Call)
			runtime.SetEventHandler(executions.OnCodexEvent)
			if _, err = runtime.Connect(); err != nil {
				t.Fatal(err)
			}
			task := pipelinemodel.Task{Name: "protocol", Entry: "agent", Actions: []pipelinemodel.Action{
				{ID: "agent", Kind: "agent", Type: "agent.run", StepID: stepID, Prompt: "work", Criteria: "pass", Permission: "read-only", MaxRetries: 1, TimeoutSeconds: 3, Branches: []pipelinemodel.Branch{{Name: "ready", Target: "report"}}},
				{ID: "report", Kind: "static", Type: "report.write", Params: map[string]any{"fileName": "result.md", "content": map[string]any{"result": "agent", "path": "answer"}}, Branches: []pipelinemodel.Branch{{Name: "success", Target: "done"}}},
				{ID: "done", Kind: "end", Type: "end.success"},
			}}
			raw, _ := json.Marshal(task)
			task, err = pipelines.SaveTask(projectID, string(raw))
			if err != nil {
				t.Fatal(err)
			}
			run, err := pipelines.RunTask(pipelinemodel.RunInput{TaskID: task.ID})
			if err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) {
				current := pipelines.GetState().Runs[0]
				if current.Status == "queued" || current.Status == "running" {
					time.Sleep(5 * time.Millisecond)
					continue
				}
				ctx, err := work.GetStepContext(stepID)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "success" {
					if current.Status != "completed" || current.CurrentNode != "done" || !ctx.Step.Completed || len(current.Actions) != 2 {
						t.Fatalf("report handshake failed: %+v step=%+v", current, ctx.Step)
					}
					files := files.GetPipelineArtifacts(run.ID)
					if len(files) != 1 {
						t.Fatal(files)
					}
					content, err := os.ReadFile(files[0].LocalPath)
					if err != nil || string(content) != "protocol output" {
						t.Fatal(string(content), err)
					}
				} else {
					if current.Status != "error" || ctx.Step.Completed || len(current.Actions) != 2 || current.Actions[1].Attempt != 2 {
						t.Fatalf("missing completion gate/retry: %+v step=%+v", current, ctx.Step)
					}
					if len(files.GetPipelineArtifacts(run.ID)) != 0 {
						t.Fatal("downstream action ran after failed agent")
					}
				}
				return
			}
			t.Fatal("protocol run timed out", pipelines.GetState().Runs)
		})
	}
}

// pipelineProtocolServer 模拟原生进程到宿主的工具汇报，不连接真实模型
func pipelineProtocolServer(mode string) {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("pipeline-protocol-test")
		return
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	number := 0
	threadID, turnID := "", ""
	complete := func(status string) {
		encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": threadID, "turn": map[string]string{"id": turnID, "status": status}}})
	}
	for {
		var request struct {
			ID     json.RawMessage `json:"id"`     // 请求标识
			Method string          `json:"method"` // 协议方法
			Params struct {
				ThreadID string `json:"threadId"` // 会话标识
				Cwd      string `json:"cwd"`      // 工作目录
			} `json:"params"` // 请求参数
			Result map[string]any `json:"result"` // 工具响应
		}
		if decoder.Decode(&request) != nil {
			return
		}
		if len(request.ID) == 0 {
			continue
		}
		if request.Method == "" {
			status := "completed"
			if mode == "interrupted" || request.Result["success"] != true {
				status = "interrupted"
			}
			complete(status)
			continue
		}
		result := map[string]any{}
		switch request.Method {
		case "thread/start":
			number++
			threadID = fmt.Sprintf("pipeline-thread-%d", number)
			result["thread"] = map[string]any{"id": threadID, "cwd": request.Params.Cwd}
		case "thread/read", "thread/resume":
			result["thread"] = map[string]any{"id": request.Params.ThreadID, "cwd": request.Params.Cwd}
		case "turn/start":
			threadID = request.Params.ThreadID
			turnID = fmt.Sprintf("pipeline-turn-%d", number)
			result["turn"] = map[string]any{"id": turnID, "status": "inProgress"}
		case "model/list":
			result["data"] = []any{map[string]any{"model": "test", "supportedReasoningEfforts": []any{map[string]string{"reasoningEffort": "medium"}}}}
		}
		if encoder.Encode(map[string]any{"id": request.ID, "result": result}) != nil {
			return
		}
		if request.Method == "turn/start" {
			time.Sleep(20 * time.Millisecond)
			if mode == "missing" {
				complete("completed")
				continue
			}
			outcome, branch := "success", "ready"
			if mode == "failure" {
				outcome, branch = "failure", ""
			}
			encoder.Encode(map[string]any{"id": "node-report", "method": "item/tool/call", "params": map[string]any{"threadId": threadID, "turnId": turnID, "callId": "report", "tool": "pipeline_complete_node", "arguments": map[string]any{"outcome": outcome, "branch": branch, "summary": "protocol checks", "output": map[string]any{"answer": "protocol output"}}}})
		}
		if request.Method == "turn/interrupt" {
			complete("interrupted")
		}
	}
}

package toolrouter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentor/internal/appdata"
	appdatamodel "agentor/internal/appdata/model"
	"agentor/internal/artifact"
	"agentor/internal/codex"
	codexmodel "agentor/internal/codex/model"
	"agentor/internal/lark"
	"agentor/internal/tasks"
	tasksmodel "agentor/internal/tasks/model"
	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"
)

// TestMain 在测试子进程中提供本地原生协议响应，不连接模型或外部服务
func TestMain(m *testing.M) {
	if os.Getenv("AGENTOR_ROUTER_PROTOCOL_TEST") != "1" {
		os.Exit(m.Run())
	}
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("router-protocol-test")
		return
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	for {
		var request struct {
			ID     json.RawMessage `json:"id"`     // 协议请求标识
			Method string          `json:"method"` // 协议方法
			Params struct {
				ThreadID string `json:"threadId"` // 请求会话
				Cwd      string `json:"cwd"`      // 工作目录
			} `json:"params"` // 请求参数
		}
		if decoder.Decode(&request) != nil {
			return
		}
		if len(request.ID) == 0 {
			continue
		}
		result := map[string]any{}
		switch request.Method {
		case "thread/start", "thread/read", "thread/resume":
			result["thread"] = map[string]any{"id": "router-thread", "cwd": request.Params.Cwd}
		case "turn/start":
			result["turn"] = map[string]any{"id": "router-turn", "status": "inProgress"}
		case "model/list":
			result["data"] = []any{map[string]any{"model": "test", "supportedReasoningEfforts": []any{map[string]string{"reasoningEffort": "medium"}}}}
		}
		if encoder.Encode(map[string]any{"id": request.ID, "result": result}) != nil {
			return
		}
		if request.Method == "turn/interrupt" {
			encoder.Encode(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": request.Params.ThreadID, "turn": map[string]string{"id": "router-turn", "status": "interrupted"}}})
		}
	}
}

// TestControllerRequestPermissions 验证实际活动会话的请求限制和错误检查顺序
func TestControllerRequestPermissions(t *testing.T) {
	for _, test := range []struct {
		name        string // 场景名称
		kind        string // 请求类型
		needsTarget bool   // 是否缺少明确目标
	}{
		{"普通指令", "instruction", false},
		{"缺少目标", "instruction", true},
		{"完成汇报", "execution_result", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			router, _ := newControllerRouter(t, test.kind, test.needsTarget)
			for _, spec := range router.Specs("controller") {
				readOnly := spec.Name == "list_workspaces" || spec.Name == "get_work_item" || spec.Name == "list_executions" || spec.Name == "list_pipeline_tasks"
				_, err := router.Call(codexmodel.ToolCall{ThreadID: "router-thread", Tool: spec.Name, Arguments: json.RawMessage(`{`)})
				if !readOnly && (test.needsTarget || test.kind != "instruction") {
					if err == nil || err.Error() != "此请求需要明确目标或仅用于完成汇报，不能修改或再次派发" {
						t.Fatalf("%s: %v", spec.Name, err)
					}
				} else if _, ok := err.(*json.SyntaxError); !ok {
					t.Fatalf("%s 未进入参数解析：%v", spec.Name, err)
				}
			}
			_, err := router.Call(codexmodel.ToolCall{ThreadID: "router-thread", Tool: "unknown", Arguments: json.RawMessage(`{`)})
			if err == nil || err.Error() != "当前会话角色不能调用此工具" {
				t.Fatal(err)
			}
			_, err = router.Call(codexmodel.ToolCall{ThreadID: "missing", Tool: "unknown", Arguments: json.RawMessage(`{`)})
			if err == nil || err.Error() != "此线程没有活动的工作对象" {
				t.Fatal(err)
			}
		})
	}
}

// TestControllerProjectScope 验证查询与创建沿用请求项目且不改变桌面选择
func TestControllerProjectScope(t *testing.T) {
	router, projectID := newControllerRouter(t, "instruction", false)
	state, err := router.work.SaveProject(workspacemodel.Project{Name: "other", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	otherID := state.SelectedID
	for _, call := range []codexmodel.ToolCall{
		{Tool: "get_work_item", Arguments: json.RawMessage(fmt.Sprintf(`{"kind":"project","id":%q}`, otherID))},
		{Tool: "create_version", Arguments: json.RawMessage(fmt.Sprintf(`{"projectId":%q,"name":"blocked"}`, otherID))},
	} {
		call.ThreadID = "router-thread"
		if _, err = router.Call(call); err == nil || err.Error() != "对象不存在或不属于本次请求范围" {
			t.Fatal(call.Tool, err)
		}
	}
	result, err := router.Call(codexmodel.ToolCall{ThreadID: "router-thread", Tool: "list_workspaces", Arguments: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	projects := result.([]map[string]any)
	if len(projects) != 1 || projects[0]["id"] != projectID {
		t.Fatal(projects)
	}
	_, err = router.Call(codexmodel.ToolCall{ThreadID: "router-thread", Tool: "create_version", Arguments: json.RawMessage(fmt.Sprintf(`{"projectId":%q,"name":"allowed"}`, projectID))})
	if err != nil || router.work.GetState().SelectedID != otherID {
		t.Fatal("创建版本改变了桌面选择", err)
	}
}

// TestStepArtifactScope 验证步骤分享保留当前执行的产物归属限制
func TestStepArtifactScope(t *testing.T) {
	root := t.TempDir()
	files, err := artifact.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "report.md")
	if err := os.WriteFile(path, []byte("report"), 0600); err != nil {
		t.Fatal(err)
	}
	item, err := files.RegisterProduced(artifact.ProducedOwner{ThreadID: "thread", ProjectID: "project", ExecutionID: "other"}, path, root)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := lark.NewService(root, nil, nil, files, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { remote.ServiceShutdown() })
	if err := remote.ShareArtifactFor(item.ID, "project", "current", tasksmodel.Source{}); err == nil || err.Error() != "产物不属于当前任务范围" {
		t.Fatal(err)
	}
}

// newControllerRouter 通过本地协议建立可调用工具的实际控制中心请求
func newControllerRouter(t *testing.T, kind string, needsTarget bool) (*Router, string) {
	t.Helper()
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	t.Setenv("AGENTOR_ROUTER_PROTOCOL_TEST", "1")
	data, err := appdata.NewService("test")
	if err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := data.SetCodexPath(executable); err != nil {
		t.Fatal(err)
	}
	root := data.GetInfo().DataDirectory
	files, err := artifact.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	work, err := workspace.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspacemodel.Project{Name: "allowed", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	request := tasksmodel.Request{ID: "request", ProjectID: projectID, Status: "queued", Kind: kind, Text: "test", Model: "test", Effort: "medium", NeedsTarget: needsTarget, Executor: appdatamodel.ControllerSettings{Models: []string{"test"}, Efforts: []string{"medium"}}}
	if err := appdata.WriteJSON(filepath.Join(root, "tasks.json"), tasksmodel.State{Requests: []tasksmodel.Request{request}}); err != nil {
		t.Fatal(err)
	}
	runtime := codex.NewService(data, files)
	t.Cleanup(func() { runtime.ServiceShutdown() })
	executions, err := tasks.NewService(data, runtime, work, files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { executions.ServiceShutdown() })
	router := New(executions, work, nil, nil)
	executions.ConfigureTools(router.Specs)
	runtime.SetEventHandler(executions.OnCodexEvent)
	if _, err := runtime.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := executions.ResumeQueue(); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := executions.CurrentRequest("router-thread"); err == nil {
			return router, projectID
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("控制中心没有进入活动状态：%+v", executions.GetState())
		}
	}
}

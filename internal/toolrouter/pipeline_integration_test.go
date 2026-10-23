package toolrouter

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// TestNativePipelineWorktree 使用显式指定的测试仓库验证真实模型和完整 Git 流水线
func TestNativePipelineWorktree(t *testing.T) {
	if os.Getenv("AGENTOR_NATIVE_PIPELINE_SMOKE") != "1" {
		t.Skip("opt-in native pipeline smoke")
	}
	repository, root := os.Getenv("AGENTOR_PIPELINE_SMOKE_REPOSITORY"), os.Getenv("AGENTOR_PIPELINE_SMOKE_DATA")
	if repository == "" || root == "" {
		t.Fatal("explicit test repository and isolated data directory required")
	}
	t.Setenv("AGENTOR_DATA_DIR", root)
	data, err := appdata.NewService("pipeline-smoke")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = data.SetModelSelection(os.Getenv("AGENTOR_SMOKE_MODEL"), "low"); err != nil {
		t.Fatal(err)
	}
	files, err := artifact.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	work, err := workspace.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	runtime := codex.NewService(data, files)
	executions, err := tasks.NewService(data, runtime, work, files)
	if err != nil {
		t.Fatal(err)
	}
	pipelines, err := pipeline.NewService(data, work, executions, files, runtime)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.ServiceShutdown()
	defer executions.ServiceShutdown()
	defer pipelines.ServiceShutdown()
	router := New(executions, work, pipelines, nil)
	executions.ConfigureTools(router.Specs)
	pipelines.ConfigureTools(router.Specs)
	runtime.SetToolHandler(router.Call)
	runtime.SetEventHandler(executions.OnCodexEvent)
	if _, err = runtime.Connect(); err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspacemodel.Project{Name: "Feat", Directory: repository})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspacemodel.Version{Name: "0.1"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspacemodel.Feature{Title: "流水线完整链路验证"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveStep(projectID, state.SelectedID, workspacemodel.Step{Title: "写入并验证流水线标记", Prompt: "只在当前 worktree 创建 pipeline-smoke.txt，内容严格为 PIPELINE_OK 加一个换行。读取文件验证内容后，将该文件提交到当前分支。提交标题使用 test: 验证流水线步骤，并添加 Co-authored-by: Codex <codex@openai.com>。不要切换分支、合并或清理工作区。最后调用 report_step_outcome 明确汇报成功与验证说明。若需要命令权限，正常发起权限请求。", PermissionMode: "workspace-write"})
	if err != nil {
		t.Fatal(err)
	}
	task := pipelinemodel.Task{Name: "Feat 工作区完整链路验证", Actions: []pipelinemodel.Action{
		{ID: "create", Type: "worktree.create", Params: map[string]any{}},
		{ID: "step", Type: "step.run", Params: map[string]any{"stepId": state.SelectedID, "instruction": "完成步骤定义中的精确测试，禁止修改其他文件。", "completionCriteria": "文件内容验证通过、已提交，结构化汇报成功。"}},
		{ID: "merge", Type: "worktree.merge", Params: map[string]any{}},
		{ID: "cleanup", Type: "worktree.remove", Params: map[string]any{}},
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
	t.Logf("run=%s data=%s", run.ID, root)
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		// 实测中的命令审批由操作者逐条检查并在隔离目录写入响应
		for _, approval := range runtime.GetApprovals() {
			if approval.Status != "pending" {
				continue
			}
			if err = appdata.WriteJSON(filepath.Join(root, "pending-approval.json"), approval); err != nil {
				t.Fatal(err)
			}
			var decision struct {
				ID       string `json:"id"`
				Decision string `json:"decision"`
			}
			if err = appdata.ReadJSON(filepath.Join(root, "approval-response.json"), &decision); err != nil {
				t.Fatal(err)
			}
			if decision.ID == approval.ID {
				if err = runtime.RespondApproval(decision.ID, decision.Decision); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, current := range pipelines.GetState().Runs {
			if current.ID != run.ID || current.Status == "queued" || current.Status == "running" {
				continue
			}
			if current.Status != "completed" {
				t.Fatalf("status=%s error=%s", current.Status, current.Error)
			}
			content, err := os.ReadFile(filepath.Join(repository, "pipeline-smoke.txt"))
			if err != nil || string(content) != "PIPELINE_OK\n" {
				t.Fatalf("verification failed: %q %v", content, err)
			}
			if _, err = os.Stat(current.Worktree.Directory); !os.IsNotExist(err) {
				t.Fatal("worktree was not removed", err)
			}
			t.Logf("completed run=%s mergedCommit=%s actions=%d", run.ID, current.Worktree.MergedCommit, len(current.Actions))
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("native pipeline timed out")
}

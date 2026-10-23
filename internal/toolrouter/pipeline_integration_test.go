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
	projectID := os.Getenv("AGENTOR_PIPELINE_SMOKE_PROJECT_ID")
	versionID := ""
	if projectID != "" {
		for _, project := range work.GetState().Projects {
			if project.ID == projectID && filepath.Clean(project.Directory) == filepath.Clean(repository) {
				for _, version := range project.Versions {
					if version.Name == "0.1" {
						versionID = version.ID
					}
				}
			}
		}
		if versionID == "" {
			t.Fatal("existing project and version 0.1 must match the explicit repository")
		}
	} else {
		state, err := work.SaveProject(workspacemodel.Project{Name: "Feat", Directory: repository})
		if err != nil {
			t.Fatal(err)
		}
		projectID = state.SelectedID
		state, err = work.SaveVersion(projectID, workspacemodel.Version{Name: "0.1"})
		if err != nil {
			t.Fatal(err)
		}
		versionID = state.SelectedID
	}
	state, err := work.SaveFeature(projectID, versionID, workspacemodel.Feature{Title: "节点图流水线真实验证", Description: "验证模板实例化、Agent 结果汇报、Go 合并和工作区清理。"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveStep(projectID, state.SelectedID, workspacemodel.Step{Title: "写入并验收节点图标记", Prompt: "仅在当前 worktree 创建 pipeline-graph-smoke.txt，内容严格为 PIPELINE_GRAPH_OK 加一个换行。读取验证后，仅提交这个文件到当前分支，提交标题使用 test: 验证节点图流水线步骤，附加 Co-authored-by: Codex <codex@openai.com>。不得修改其他文件，不切换分支、不合并、不清理。若文件已存在且内容正确且已提交，直接验证并汇报。完成后调用 pipeline_complete_node，outcome=success、branch=success，output 包含文件名与提交 ID；summary 提供验收依据。失败时 outcome=failure 且 branch 为空。", PermissionMode: "danger-full-access"})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID

	task := pipelinemodel.Task{Name: "Feat 工作区完整链路验证", Entry: "create", Actions: []pipelinemodel.Action{
		{ID: "create", Kind: "static", Type: "worktree.create", Branches: []pipelinemodel.Branch{{Name: "success", Target: "develop"}}},
		{ID: "develop", Kind: "agent", Type: "agent.run", StepSlot: "development", Prompt: "完成绑定步骤定义中的精确测试，禁止修改其他文件。", Criteria: "文件内容验证通过、已提交，汇报验收通过。", Permission: "danger-full-access", TimeoutSeconds: 420, Branches: []pipelinemodel.Branch{{Name: "success", Target: "merge"}}},
		{ID: "merge", Kind: "static", Type: "worktree.merge", Branches: []pipelinemodel.Branch{{Name: "success", Target: "cleanup"}, {Name: "dirty"}, {Name: "diverged"}}},
		{ID: "cleanup", Kind: "static", Type: "worktree.remove", Branches: []pipelinemodel.Branch{{Name: "success", Target: "done"}}},
		{ID: "done", Kind: "end", Type: "end.success"},
	}}

	for i := range task.Actions {
		task.Actions[i].Position = pipelinemodel.Position{X: float64(i) * 320, Y: 120}
	}
	raw, _ := json.Marshal(task)
	task, err = pipelines.SaveTemplate(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	task, err = pipelines.Instantiate(pipelinemodel.InstantiateInput{TemplateID: task.ID, ProjectID: projectID, Name: "Feat · 节点图完整链路验证", Bindings: map[string]string{"development": stepID}})
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
				ID       string `json:"id"`       // 审批标识
				Decision string `json:"decision"` // 操作者决定
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
			content, err := os.ReadFile(filepath.Join(repository, "pipeline-graph-smoke.txt"))
			if err != nil || string(content) != "PIPELINE_GRAPH_OK\n" {
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

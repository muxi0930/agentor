package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	tasksmodel "agentor/internal/tasks/model"
	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"
)

// TestVersionWorktreeLifecycle 验证跨运行绑定、步骤目录、完成检查与合并后清理
func TestVersionWorktreeLifecycle(t *testing.T) {
	s, projectID, versionID, stepID := versionFixture(t)
	raw, _ := s.work.GetStepContext(stepID)
	resolved, err := s.work.ResolveStepContext(stepID)
	if err != nil || resolved.Directory != raw.Directory {
		t.Fatal("无绑定改变了执行目录", resolved, err)
	}
	if err := s.work.BindStepThread(stepID, "old-project-session"); err != nil {
		t.Fatal(err)
	}
	prepared := runVersion(t, s, projectID, versionID, constant.PipelineSystemPrepare)
	if prepared.Status != constant.TasksStatusCompleted {
		t.Fatal(prepared.Error)
	}
	_, version, _ := s.work.GetVersion(projectID, versionID)
	w := version.Worktree
	if w == nil || w.Status != constant.WorkspaceWorktreeReady {
		t.Fatal(version)
	}
	resolved, err = s.work.ResolveStepContext(stepID)
	if err != nil || resolved.Directory != w.Directory || resolved.Step.ThreadID != "" {
		t.Fatal(resolved, err)
	}
	reopened, reopenErr := workspace.NewService(s.data.GetInfo().DataDirectory)
	if reopenErr != nil {
		t.Fatal(reopenErr)
	}
	restored, restoreErr := reopened.ResolveStepContext(stepID)
	if restoreErr != nil || restored.Directory != w.Directory {
		t.Fatal("重启后绑定丢失", restored, restoreErr)
	}
	again := runVersion(t, s, projectID, versionID, constant.PipelineSystemPrepare)
	if again.Status != constant.TasksStatusCompleted || again.Worktree.Directory != w.Directory {
		t.Fatal(again)
	}
	if _, err := s.RunVersion(pipelinemodel.VersionRunInput{ProjectID: projectID, VersionID: versionID, Operation: constant.PipelineSystemMerge}); !errors.Is(err, constant.ErrPipelineVersionIncomplete) {
		t.Fatal(err)
	}
	if _, err := s.work.CompleteStep(stepID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.Directory, "version.txt"), []byte("version"), 0600); err != nil {
		t.Fatal(err)
	}
	dirty := runVersion(t, s, projectID, versionID, constant.PipelineSystemMerge)
	if dirty.Status != constant.TasksStatusError || !strings.Contains(dirty.Error, constant.ErrPipelineVersionDirty.Error()) {
		t.Fatal(dirty)
	}
	versionGit(t, w.Directory, "add", "version.txt")
	versionGit(t, w.Directory, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "version")
	merged := runVersion(t, s, projectID, versionID, constant.PipelineSystemMerge)
	if merged.Status != constant.TasksStatusCompleted {
		t.Fatal(merged.Error)
	}
	_, version, _ = s.work.GetVersion(projectID, versionID)
	if version.Worktree.Status != constant.WorkspaceWorktreeCleaned || version.Worktree.Directory != "" || version.Worktree.MergedCommit == "" {
		t.Fatal(version.Worktree)
	}
	if _, err := os.Stat(w.Directory); !os.IsNotExist(err) {
		t.Fatal("未清理 worktree", err)
	}
	if _, err := git(context.Background(), raw.Directory, "show-ref", "--verify", "refs/heads/"+w.Branch); err == nil {
		t.Fatal("未删除分支")
	}
	if _, err := os.Stat(filepath.Join(raw.Directory, "version.txt")); err != nil {
		t.Fatal(err)
	}
}

// TestVersionWorktreeConcurrencyAndGuards 验证普通使用可并发、独占期间拒绝派发及修改
func TestVersionWorktreeConcurrencyAndGuards(t *testing.T) {
	s, projectID, versionID, stepID := versionFixture(t)
	first, err := s.work.AcquireVersion(projectID, versionID, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.work.AcquireVersion(projectID, versionID, false)
	if err != nil {
		t.Fatal("错误地串行化普通任务", err)
	}
	if _, err := s.RunVersion(pipelinemodel.VersionRunInput{ProjectID: projectID, VersionID: versionID, Operation: constant.PipelineSystemPrepare}); !errors.Is(err, constant.ErrWorkspaceVersionBusy) {
		t.Fatal(err)
	}
	first()
	second()
	exclusive, err := s.work.AcquireVersion(projectID, versionID, true)
	if err != nil {
		t.Fatal(err)
	}
	defer exclusive()
	if _, err := s.work.AcquireVersion(projectID, versionID, false); !errors.Is(err, constant.ErrWorkspaceVersionBusy) {
		t.Fatal(err)
	}
	if _, err := s.work.CompleteStep(stepID); !errors.Is(err, constant.ErrWorkspaceVersionBusy) {
		t.Fatal(err)
	}
	if _, err := s.work.SaveFeature(projectID, versionID, workspacemodel.Feature{Title: "late"}); !errors.Is(err, constant.ErrWorkspaceVersionBusy) {
		t.Fatal(err)
	}
}

// TestVersionSystemDefinitionsReadOnly 验证内置定义不能保存、删除或复制
func TestVersionSystemDefinitionsReadOnly(t *testing.T) {
	s, projectID, versionID, _ := versionFixture(t)
	for _, task := range s.GetState().Templates {
		if task.System == "" {
			continue
		}
		if err := s.validateDefinition(task, true); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveTemplate(textOf(task)); !errors.Is(err, constant.ErrPipelineSystemReadOnly) {
			t.Fatal(err)
		}
		if err := s.DeleteTemplate(task.ID); !errors.Is(err, constant.ErrPipelineSystemReadOnly) {
			t.Fatal(err)
		}
		if _, err := s.Instantiate(pipelinemodel.InstantiateInput{TemplateID: task.ID}); !errors.Is(err, constant.ErrPipelineSystemReadOnly) {
			t.Fatal(err)
		}
	}
	run := runVersion(t, s, projectID, versionID, constant.PipelineSystemPrepare)
	if _, err := s.RunTask(pipelinemodel.RunInput{TaskID: run.TaskID}); !errors.Is(err, constant.ErrPipelineSystemReadOnly) {
		t.Fatal(err)
	}
	task := run.Task
	task.System, task.VersionID = "", ""
	if _, err := s.SaveTask(projectID, textOf(task)); !errors.Is(err, constant.ErrPipelineSystemReadOnly) {
		t.Fatal(err)
	}
	if err := s.DeleteTask(projectID, task.ID); !errors.Is(err, constant.ErrPipelineSystemReadOnly) {
		t.Fatal(err)
	}
}

// TestVersionWorktreeConflictFailureRetainsWorkspace 验证冲突处理失败保留现场且不合并
func TestVersionWorktreeConflictFailureRetainsWorkspace(t *testing.T) {
	s, projectID, versionID, stepID := versionFixture(t)
	run := runVersion(t, s, projectID, versionID, constant.PipelineSystemPrepare)
	if run.Status != constant.TasksStatusCompleted {
		t.Fatal(run.Error)
	}
	if _, err := s.work.CompleteStep(stepID); err != nil {
		t.Fatal(err)
	}
	project, version, _ := s.work.GetVersion(projectID, versionID)
	for _, dir := range []string{project.Directory, version.Worktree.Directory} {
		if err := os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte(dir), 0600); err != nil {
			t.Fatal(err)
		}
		versionGit(t, dir, "add", "conflict.txt")
		versionGit(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "diverge")
	}
	before := versionGit(t, project.Directory, "rev-parse", "HEAD")
	s.runAgent = func(context.Context, pipelinemodel.Run, pipelinemodel.Action, map[string]any) (pipelinemodel.NodeReport, error) {
		return pipelinemodel.NodeReport{Outcome: "failure", Summary: "验证失败"}, nil
	}
	failed := runVersion(t, s, projectID, versionID, constant.PipelineSystemMerge)
	if failed.Status != constant.TasksStatusError || len(failed.Actions) != 2 {
		t.Fatal(failed)
	}
	if after := versionGit(t, project.Directory, "rev-parse", "HEAD"); after != before {
		t.Fatal("错误地合并")
	}
	if _, err := os.Stat(version.Worktree.Directory); err != nil {
		t.Fatal("丢失工作区", err)
	}
}

// versionFixture 创建真实 Git 仓库及一个未完成步骤
func versionFixture(t *testing.T) (*Service, string, string, string) {
	t.Helper()
	s, projectID := fixture(t)
	project, _ := s.project(projectID)
	versionGit(t, project.Directory, "init", "-b", "main")
	versionGit(t, project.Directory, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "init")
	state, err := s.work.SaveVersion(projectID, workspacemodel.Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	versionID := state.SelectedID
	state, err = s.work.SaveFeature(projectID, versionID, workspacemodel.Feature{Title: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = s.work.SaveStep(projectID, state.SelectedID, workspacemodel.Step{Title: "step"})
	if err != nil {
		t.Fatal(err)
	}
	return s, projectID, versionID, state.SelectedID
}

// runVersion 等待系统运行与独占范围释放
func runVersion(t *testing.T, s *Service, projectID, versionID, operation string) pipelinemodel.Run {
	t.Helper()
	run, err := s.RunVersion(pipelinemodel.VersionRunInput{ProjectID: projectID, VersionID: versionID, Operation: operation})
	if err != nil {
		t.Fatal(err)
	}
	result := waitRun(t, s, run.ID)
	s.workerGroup.Wait()
	return result
}

// versionGit 执行测试仓库命令并保留错误上下文
func versionGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	result, err := git(context.Background(), dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// TestVersionWorktreeMissingNeverFallsBack 验证丢失的绑定不会使用项目目录
func TestVersionWorktreeMissingNeverFallsBack(t *testing.T) {
	s, projectID, versionID, stepID := versionFixture(t)
	run := runVersion(t, s, projectID, versionID, constant.PipelineSystemPrepare)
	if run.Status != constant.TasksStatusCompleted {
		t.Fatal(run.Error)
	}
	_, version, _ := s.work.GetVersion(projectID, versionID)
	version.Worktree.Directory = filepath.Join(t.TempDir(), "missing")
	if err := s.work.SetVersionWorktree(projectID, versionID, version.Worktree); err != nil {
		t.Fatal(err)
	}
	if _, err := s.work.ResolveStepContext(stepID); !errors.Is(err, constant.ErrWorkspaceWorktreeInvalid) {
		t.Fatal(err)
	}
}

// TestVersionWorktreeCleanupRetry 验证合并后清理失败重试不重新运行 Agent
func TestVersionWorktreeCleanupRetry(t *testing.T) {
	s, projectID, versionID, stepID := versionFixture(t)
	run := runVersion(t, s, projectID, versionID, constant.PipelineSystemPrepare)
	if run.Status != constant.TasksStatusCompleted {
		t.Fatal(run.Error)
	}
	if _, err := s.work.CompleteStep(stepID); err != nil {
		t.Fatal(err)
	}
	_, version, _ := s.work.GetVersion(projectID, versionID)
	run.VersionID = versionID
	if _, err := s.performWorktree(context.Background(), run, constant.PipelineActionMergeWorktree); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(version.Worktree.Directory, "uncommitted.txt")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	failed := runVersion(t, s, projectID, versionID, constant.PipelineSystemMerge)
	if failed.Status != constant.TasksStatusError {
		t.Fatal(failed)
	}
	_, current, _ := s.work.GetVersion(projectID, versionID)
	if current.Worktree.Status != constant.WorkspaceWorktreeMerged {
		t.Fatal(current.Worktree)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.runAgent = func(context.Context, pipelinemodel.Run, pipelinemodel.Action, map[string]any) (pipelinemodel.NodeReport, error) {
		t.Error("清理重试启动了 Agent")
		return pipelinemodel.NodeReport{}, nil
	}
	retried := runVersion(t, s, projectID, versionID, constant.PipelineSystemMerge)
	if retried.Status != constant.TasksStatusCompleted {
		t.Fatal(retried.Error)
	}
}

// TestVersionWorktreeDivergenceResolved 验证分叉整合完成后重新检查并清理
func TestVersionWorktreeDivergenceResolved(t *testing.T) {
	s, projectID, versionID, stepID := versionFixture(t)
	prepared := runVersion(t, s, projectID, versionID, constant.PipelineSystemPrepare)
	if prepared.Status != constant.TasksStatusCompleted {
		t.Fatal(prepared.Error)
	}
	if _, err := s.work.CompleteStep(stepID); err != nil {
		t.Fatal(err)
	}
	project, version, _ := s.work.GetVersion(projectID, versionID)
	for i, dir := range []string{project.Directory, version.Worktree.Directory} {
		name := []string{"source.txt", "version.txt"}[i]
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
		versionGit(t, dir, "add", name)
		versionGit(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", name)
	}
	s.runAgent = func(ctx context.Context, run pipelinemodel.Run, node pipelinemodel.Action, params map[string]any) (pipelinemodel.NodeReport, error) {
		if run.Worktree.Directory != version.Worktree.Directory {
			return pipelinemodel.NodeReport{}, errors.New("错误工作区")
		}
		_, err := git(ctx, run.Worktree.Directory, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "merge", "--no-edit", run.Worktree.SourceBranch)
		return pipelinemodel.NodeReport{Outcome: "success", Branch: "success", Summary: "测试整合"}, err
	}
	result := runVersion(t, s, projectID, versionID, constant.PipelineSystemMerge)
	if result.Status != constant.TasksStatusCompleted || len(result.Actions) != 3 {
		t.Fatal(result)
	}
	for _, name := range []string{"source.txt", "version.txt"} {
		if _, err := os.Stat(filepath.Join(project.Directory, name)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestVersionWorktreeRejectsPendingTasks 验证排队步骤和暂停的关联流水线阻止工作区切换
func TestVersionWorktreeRejectsPendingTasks(t *testing.T) {
	for _, status := range []string{"queued", "running", "paused"} {
		t.Run(status, func(t *testing.T) {
			s, projectID, versionID, stepID := versionFixture(t)
			value, err := s.work.GetStepContext(stepID)
			if err != nil {
				t.Fatal(err)
			}
			if status == "paused" {
				s.state.Runs = append(s.state.Runs, pipelinemodel.Run{ID: "pending", Status: status, StepContexts: map[string]workspacemodel.StepContext{"agent": value}})
			} else {
				err = s.tasks.RecordPipelineExecution(tasksmodel.Execution{ID: "pending", PipelineRunID: "pipeline", ProjectID: projectID, FeatureID: value.FeatureID, StepID: stepID, Context: value, Status: status}, nil)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.RunVersion(pipelinemodel.VersionRunInput{ProjectID: projectID, VersionID: versionID, Operation: constant.PipelineSystemPrepare}); !errors.Is(err, constant.ErrPipelineVersionActive) {
				t.Fatal(err)
			}
		})
	}
}

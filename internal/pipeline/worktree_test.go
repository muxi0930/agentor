package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
)

// TestWorktreeLifecycle 验证固定 Git 动作隔离、成功门槛、合并与重复清理
func TestWorktreeLifecycle(t *testing.T) {
	s, projectID := fixture(t)
	project, _ := s.project(projectID)
	ctx := context.Background()
	for _, args := range [][]string{{"init", "-b", "main"}, {"-c", "user.name=Pipeline Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "init"}} {
		if _, err := git(ctx, project.Directory, args...); err != nil {
			t.Fatal(err)
		}
	}
	run := pipelinemodel.Run{ID: "git-test", Directory: project.Directory, ProjectID: projectID, Status: "running"}
	s.state.Runs = append(s.state.Runs, run)
	value, err := s.performWorktree(ctx, run, constant.PipelineActionCreateWorktree)
	if err != nil {
		t.Fatal(err)
	}
	w := value.(*pipelinemodel.Worktree)
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionCreateWorktree); err != nil {
		t.Fatal(err)
	}
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionRemoveWorktree); err == nil {
		t.Fatal("removed before merge")
	}
	if err = os.WriteFile(filepath.Join(w.Directory, "result.txt"), []byte("done"), 0600); err != nil {
		t.Fatal(err)
	}
	value, err = s.performWorktree(ctx, run, constant.PipelineActionMergeWorktree)
	if err != nil || value.(pipelinemodel.NodeReport).Branch != "dirty" {
		t.Fatal("dirty is an expected branch", value, err)
	}

	for _, args := range [][]string{{"add", "result.txt"}, {"-c", "user.name=Pipeline Test", "-c", "user.email=test@example.invalid", "commit", "-m", "result"}} {
		if _, err = git(ctx, w.Directory, args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionMergeWorktree); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(project.Directory, "result.txt")); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = s.performWorktree(ctx, run, constant.PipelineActionRemoveWorktree); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = os.Stat(w.Directory); !os.IsNotExist(err) {
		t.Fatal("worktree directory retained", err)
	}
	if _, err = git(ctx, project.Directory, "show-ref", "--verify", "refs/heads/"+w.Branch); err == nil {
		t.Fatal("worktree branch retained")
	}
}

// TestWorktreeDivergenceIsBusinessOutcome 验证来源分支分歧只返回固定分支而不进入 Git 冲突现场
func TestWorktreeDivergenceIsBusinessOutcome(t *testing.T) {
	s, projectID := fixture(t)
	project, _ := s.project(projectID)
	ctx := context.Background()
	commit := []string{"-c", "user.name=Pipeline Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-m", "test: commit"}
	if _, err := git(ctx, project.Directory, "init", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, project.Directory, commit...); err != nil {
		t.Fatal(err)
	}
	run := pipelinemodel.Run{ID: "diverged", Directory: project.Directory, Status: "running"}
	s.state.Runs = append(s.state.Runs, run)
	value, err := s.performWorktree(ctx, run, constant.PipelineActionCreateWorktree)
	if err != nil {
		t.Fatal(err)
	}
	worktree := value.(*pipelinemodel.Worktree)
	for _, directory := range []string{project.Directory, worktree.Directory} {
		// 使用不同内容，保证两侧产生不同提交
		if err = os.WriteFile(filepath.Join(directory, "side.txt"), []byte(directory), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = git(ctx, directory, "add", "side.txt"); err != nil {
			t.Fatal(err)
		}
		if _, err = git(ctx, directory, commit...); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := git(ctx, project.Directory, "rev-parse", "HEAD")
	value, err = s.performWorktree(ctx, run, constant.PipelineActionMergeWorktree)
	if err != nil {
		t.Fatal(err)
	}
	report, ok := value.(pipelinemodel.NodeReport)
	if !ok || report.Outcome != "success" || report.Branch != "diverged" {
		t.Fatal(value)
	}
	after, _ := git(ctx, project.Directory, "rev-parse", "HEAD")
	if after != before {
		t.Fatal("分歧时仍修改了来源分支")
	}
	if _, err = git(ctx, project.Directory, "rev-parse", "--verify", "MERGE_HEAD"); err == nil {
		t.Fatal("进入了未处理的 Git 合并状态")
	}
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionRemoveWorktree); err == nil {
		t.Fatal("删除了尚未合并的工作区")
	}
}

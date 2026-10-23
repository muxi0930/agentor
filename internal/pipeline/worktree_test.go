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
	run := pipelinemodel.Run{ID: "git-test", ProjectID: projectID, Status: "running"}
	s.state.Runs = append(s.state.Runs, run)
	value, err := s.performWorktree(ctx, run, constant.PipelineActionCreateWorktree)
	if err != nil {
		t.Fatal(err)
	}
	w := value.(*pipelinemodel.Worktree)
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionCreateWorktree); err != nil {
		t.Fatal(err)
	}
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionMergeWorktree); err == nil {
		t.Fatal("merged without successful step")
	}
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionRemoveWorktree); err == nil {
		t.Fatal("removed before merge")
	}
	if err = os.WriteFile(filepath.Join(w.Directory, "result.txt"), []byte("done"), 0600); err != nil {
		t.Fatal(err)
	}
	s.updateRun(run.ID, func(run *pipelinemodel.Run) {
		run.Actions = append(run.Actions, pipelinemodel.ActionResult{Type: constant.PipelineActionRunStep, Status: "completed"})
	})
	if _, err = s.performWorktree(ctx, run, constant.PipelineActionMergeWorktree); err == nil {
		t.Fatal("merged dirty worktree")
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

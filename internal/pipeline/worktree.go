package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
)

// performWorktree 执行仅限本次运行的工作区创建、快进合并与清理
func (s *Service) performWorktree(ctx context.Context, run pipelinemodel.Run, kind string) (any, error) {
	for _, current := range s.GetState().Runs {
		if current.ID == run.ID {
			run = current
			break
		}
	}
	w := run.Worktree
	if kind == constant.PipelineActionCreateWorktree {
		if w == nil {
			project, err := s.project(run.ProjectID)
			if err != nil {
				return nil, err
			}
			branch, err := git(ctx, project.Directory, "symbolic-ref", "--short", "HEAD")
			if err != nil {
				return nil, err
			}
			commit, err := git(ctx, project.Directory, "rev-parse", "HEAD")
			if err != nil {
				return nil, err
			}
			w = &pipelinemodel.Worktree{Repository: project.Directory, SourceBranch: branch, SourceCommit: commit, Branch: "codex/pipeline-" + run.ID, Directory: filepath.Join(s.root, "worktrees", run.ID)}
			if err = s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Worktree = w }); err != nil {
				return nil, err
			}
		}
		if _, err := os.Stat(w.Directory); err == nil {
			branch, err := git(ctx, w.Directory, "symbolic-ref", "--short", "HEAD")
			if err != nil || branch != w.Branch {
				return nil, fmt.Errorf("运行工作区分支不匹配：%s", branch)
			}
			return w, nil
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(w.Directory), 0700); err != nil {
			return nil, err
		}
		if _, err := git(ctx, w.Repository, "worktree", "add", "-b", w.Branch, w.Directory, w.SourceCommit); err != nil {
			return nil, err
		}
		return w, nil
	}
	if w == nil {
		return nil, constant.ErrPipelineWorktreeMissing
	}
	branch, err := git(ctx, w.Repository, "symbolic-ref", "--short", "HEAD")
	if err != nil || branch != w.SourceBranch {
		return nil, fmt.Errorf("来源工作区已切换分支，预期 %s", w.SourceBranch)
	}
	if kind == constant.PipelineActionMergeWorktree {
		// 合并只接受当前运行中已通过结构化成功检查的步骤
		success := false
		for _, action := range run.Actions {
			if action.Type == constant.PipelineActionRunStep && action.Status == constant.TasksStatusCompleted {
				success = true
			}
		}
		if !success {
			return nil, constant.ErrPipelineStepOutcomeMissing
		}
		for _, directory := range []string{w.Repository, w.Directory} {
			status, err := git(ctx, directory, "status", "--porcelain")
			if err != nil {
				return nil, err
			}
			if status != "" {
				return nil, fmt.Errorf("工作区存在未提交变更：%s", directory)
			}
		}
		branch, err = git(ctx, w.Directory, "symbolic-ref", "--short", "HEAD")
		if err != nil || branch != w.Branch {
			return nil, constant.ErrPipelineWorktreeBranchChanged
		}
		commit, err := git(ctx, w.Directory, "rev-parse", "HEAD")
		if err != nil {
			return nil, err
		}
		if _, err = git(ctx, w.Repository, "-c", "core.hooksPath="+os.DevNull, "merge", "--ff-only", "--", w.Branch); err != nil {
			return nil, err
		}
		w.MergedCommit = commit
		if err = s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Worktree = w }); err != nil {
			return nil, err
		}
		return w, nil
	}
	if w.MergedCommit == "" {
		return nil, constant.ErrPipelineWorktreeNotMerged
	}
	if _, err = git(ctx, w.Repository, "merge-base", "--is-ancestor", w.MergedCommit, "HEAD"); err != nil {
		return nil, err
	}
	if _, err = os.Stat(w.Directory); err == nil {
		head, err := git(ctx, w.Directory, "rev-parse", "HEAD")
		if err != nil || head != w.MergedCommit {
			return nil, constant.ErrPipelineWorktreeAdvanced
		}
		if _, err = git(ctx, w.Repository, "worktree", "remove", "--", w.Directory); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if _, err = git(ctx, w.Repository, "show-ref", "--verify", "--quiet", "refs/heads/"+w.Branch); err == nil {
		head, err := git(ctx, w.Repository, "rev-parse", "refs/heads/"+w.Branch)
		if err != nil || head != w.MergedCommit {
			return nil, constant.ErrPipelineWorktreeBranchAdvanced
		}
		if _, err = git(ctx, w.Repository, "branch", "-d", "--", w.Branch); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// git 通过固定参数执行 Git，禁止 shell 字符串解释
func git(ctx context.Context, directory string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s：%w：%s", args[0], err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

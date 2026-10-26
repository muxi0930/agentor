package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
)

// performRunWorktree 执行宿主已确定的工作区创建、快进合并与清理
func (s *Service) performRunWorktree(ctx context.Context, run pipelinemodel.Run, kind string) (any, error) {
	w := run.Worktree
	if run.VersionID != "" && w != nil {
		if _, statErr := os.Stat(w.Directory); statErr == nil {
			common, err := git(ctx, w.Directory, "rev-parse", "--path-format=absolute", "--git-common-dir")
			if err != nil {
				return nil, err
			}
			source, err := git(ctx, w.Repository, "rev-parse", "--path-format=absolute", "--git-common-dir")
			if err != nil || common != source {
				return nil, constant.ErrWorkspaceWorktreeInvalid
			}
			branch, err := git(ctx, w.Directory, "symbolic-ref", "--short", "HEAD")
			if err != nil || branch != w.Branch {
				return nil, constant.ErrPipelineWorktreeBranchChanged
			}
		} else if !os.IsNotExist(statErr) {
			return nil, statErr
		}
	}
	if kind == constant.PipelineActionCreateWorktree {
		if w == nil {
			branch, err := git(ctx, run.Directory, "symbolic-ref", "--short", "HEAD")
			if err != nil {
				return nil, err
			}
			commit, err := git(ctx, run.Directory, "rev-parse", "HEAD")
			if err != nil {
				return nil, err
			}
			w = &pipelinemodel.Worktree{Repository: run.Directory, SourceBranch: branch, SourceCommit: commit, Branch: "codex/pipeline-" + run.ID, Directory: filepath.Join(s.root, "worktrees", run.ID)}
			if err = s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Worktree = clone(w) }); err != nil {
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
		for _, directory := range []string{w.Repository, w.Directory} {
			status, err := git(ctx, directory, "status", "--porcelain")
			if err != nil {
				return nil, err
			}
			if status != "" {
				return pipelinemodel.NodeReport{Outcome: constant.PipelineBranchSuccess, Branch: constant.PipelineBranchDirty, Output: map[string]any{"directory": directory, "reason": status}}, nil
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
		if _, ancestorErr := git(ctx, w.Repository, "merge-base", "--is-ancestor", "HEAD", commit); ancestorErr != nil {
			var exitErr *exec.ExitError
			if errors.As(ancestorErr, &exitErr) && exitErr.ExitCode() == 1 {
				return pipelinemodel.NodeReport{Outcome: constant.PipelineBranchSuccess, Branch: constant.PipelineBranchDiverged, Output: map[string]any{"sourceBranch": w.SourceBranch, "branch": w.Branch}}, nil
			}
			return nil, ancestorErr
		}

		if _, err = git(ctx, w.Repository, "-c", "core.hooksPath="+os.DevNull, "merge", "--ff-only", "--", w.Branch); err != nil {
			return nil, err
		}
		w.MergedCommit = commit
		if err = s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Worktree = clone(w) }); err != nil {
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

package workspace

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"agentor/internal/appdata"
	"agentor/internal/constant"
	workspacemodel "agentor/internal/workspace/model"
)

// AcquireVersion 获取共享执行范围或独占工作区操作，不串行化普通任务
//
//wails:ignore
func (s *Service) AcquireVersion(projectID, versionID string, exclusive bool) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if findVersion(findProject(&s.state, projectID), versionID) == nil {
		return nil, constant.ErrWorkspaceVersionNotFoundInProject
	}
	if s.versionOperations[versionID] || exclusive && s.versionUses[versionID] > 0 {
		return nil, constant.ErrWorkspaceVersionBusy
	}
	if s.versionUses == nil {
		s.versionUses = map[string]int{}
	}
	if s.versionOperations == nil {
		s.versionOperations = map[string]bool{}
	}
	if exclusive {
		s.versionOperations[versionID] = true
	} else {
		s.versionUses[versionID]++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if exclusive {
				delete(s.versionOperations, versionID)
			} else {
				s.versionUses[versionID]--
			}
		})
	}, nil
}

// GetVersion 返回版本和所属项目的独立快照
//
//wails:ignore
func (s *Service) GetVersion(projectID, versionID string) (workspacemodel.Project, workspacemodel.Version, error) {
	state := s.GetState()
	project := findProject(&state, projectID)
	version := findVersion(project, versionID)
	if version == nil {
		return workspacemodel.Project{}, workspacemodel.Version{}, constant.ErrWorkspaceVersionNotFoundInProject
	}
	return *project, *version, nil
}

// SetVersionWorktree 保存宿主工作区记录，目录变化时解除原步骤会话绑定
//
//wails:ignore
func (s *Service) SetVersionWorktree(projectID, versionID string, value *workspacemodel.VersionWorktree) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	version := findVersion(findProject(&next, projectID), versionID)
	if version == nil {
		return constant.ErrWorkspaceVersionNotFoundInProject
	}
	oldDirectory, newDirectory := "", ""
	if version.Worktree != nil {
		oldDirectory = version.Worktree.Directory
	}
	if value != nil {
		copy := *value
		version.Worktree = &copy
		newDirectory = value.Directory
	} else {
		version.Worktree = nil
	}
	if oldDirectory != newDirectory {
		for f := range version.Features {
			for i := range version.Features[f].Steps {
				version.Features[f].Steps[i].ThreadID = ""
			}
		}
	}
	if err := appdata.WriteJSON(filepath.Join(s.root, constant.WorkspaceStateFilename), next); err != nil {
		return err
	}
	s.state = next
	return nil
}

// ResolveStepContext 根据版本绑定确定执行目录，异常绑定不得回退项目目录
//
//wails:ignore
func (s *Service) ResolveStepContext(stepID string) (workspacemodel.StepContext, error) {
	result, err := s.GetStepContext(stepID)
	if err != nil {
		return result, err
	}
	_, version, err := s.GetVersion(result.ProjectID, result.VersionID)
	if err != nil {
		return result, err
	}
	w := version.Worktree
	if w == nil || w.Status == constant.WorkspaceWorktreeCleaned {
		return result, nil
	}
	if w.Status != constant.WorkspaceWorktreeReady || w.Directory == "" {
		return result, constant.ErrWorkspaceWorktreeInvalid
	}
	branch, err := exec.Command("git", "-C", w.Directory, "symbolic-ref", "--short", "HEAD").CombinedOutput()
	if err != nil || strings.TrimSpace(string(branch)) != w.Branch {
		return result, fmt.Errorf("%w：%s", constant.ErrWorkspaceWorktreeInvalid, w.Directory)
	}
	common, err := exec.Command("git", "-C", w.Directory, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return result, err
	}
	source, err := exec.Command("git", "-C", w.Repository, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil || strings.TrimSpace(string(common)) != strings.TrimSpace(string(source)) {
		return result, constant.ErrWorkspaceWorktreeInvalid
	}
	result.Directory = w.Directory
	return result, nil
}

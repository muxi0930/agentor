package pipeline

import (
	"context"
	"fmt"
	"path/filepath"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
	workspacemodel "agentor/internal/workspace/model"
)

// RunVersion 通过固定系统定义运行版本工作区流程，独占范围覆盖整个运行
func (s *Service) RunVersion(input pipelinemodel.VersionRunInput) (pipelinemodel.Run, error) {
	if input.Operation != constant.PipelineSystemPrepare && input.Operation != constant.PipelineSystemMerge {
		return pipelinemodel.Run{}, constant.ErrPipelineSystemReadOnly
	}
	release, err := s.work.AcquireVersion(input.ProjectID, input.VersionID, true)
	if err != nil {
		return pipelinemodel.Run{}, err
	}
	launched := false
	defer func() {
		if !launched {
			release()
		}
	}()
	project, version, err := s.work.GetVersion(input.ProjectID, input.VersionID)
	if err != nil {
		return pipelinemodel.Run{}, err
	}
	for _, execution := range s.tasks.GetState().Executions {
		if execution.Context.VersionID == version.ID && (execution.Status == constant.TasksStatusQueued || execution.Status == constant.TasksStatusRunning) {
			return pipelinemodel.Run{}, constant.ErrPipelineVersionActive
		}
	}
	for _, run := range s.GetState().Runs {
		if !activeRun(run.Status) {
			continue
		}
		if run.ProjectID == project.ID && run.VersionID == version.ID {
			return pipelinemodel.Run{}, constant.ErrPipelineVersionActive
		}
		for _, value := range run.StepContexts {
			if value.VersionID == version.ID {
				return pipelinemodel.Run{}, constant.ErrPipelineVersionActive
			}
		}
	}
	if input.Operation == constant.PipelineSystemMerge {
		if version.Worktree == nil || version.Worktree.Status == constant.WorkspaceWorktreeCleaned {
			return pipelinemodel.Run{}, constant.ErrPipelineWorktreeMissing
		}
		if err := versionCompleted(version); err != nil {
			return pipelinemodel.Run{}, err
		}
	}
	task := systemTask(input.Operation)
	task.ID += "." + version.ID
	task.ProjectID, task.VersionID = project.ID, version.ID
	task.Name += " · " + version.Name
	s.mu.Lock()
	next := clone(s.state)
	found := false
	for i := range next.Tasks {
		if next.Tasks[i].ID == task.ID {
			next.Tasks[i] = task
			found = true
			break
		}
	}
	if !found {
		next.Tasks = append(next.Tasks, task)
	}
	err = s.saveState(next)
	s.mu.Unlock()
	if err != nil {
		return pipelinemodel.Run{}, err
	}
	run, err := s.startRun(pipelinemodel.RunInput{TaskID: task.ID, Model: input.Model, Effort: input.Effort, Source: input.Source, RequestID: input.RequestID}, "", release)
	if err != nil {
		return run, err
	}
	launched = true
	return run, nil
}

// systemTask 返回代码内置的只读节点图，输入无法改变检查与合并顺序
func systemTask(operation string) pipelinemodel.Task {
	success := func(target string) []pipelinemodel.Branch {
		return []pipelinemodel.Branch{{Name: constant.PipelineBranchSuccess, Target: target}}
	}
	end := pipelinemodel.Action{ID: "done", Kind: constant.PipelineKindEnd, Type: constant.PipelineEndSuccess, Name: constant.PipelineSystemDoneName, Position: pipelinemodel.Position{X: 900, Y: 100}}
	task := pipelinemodel.Task{ID: constant.PipelineSystemPrefix + operation, System: operation, Entry: "create", Name: constant.PipelineSystemPrepareName}
	if operation == constant.PipelineSystemPrepare {
		task.Actions = []pipelinemodel.Action{{ID: "create", Kind: constant.PipelineKindStatic, Type: constant.PipelineActionCreateWorktree, Name: constant.PipelineSystemCreateName, Branches: success("done")}, end}
		return task
	}
	task.Name, task.Entry = constant.PipelineSystemMergeName, "merge"
	task.Actions = []pipelinemodel.Action{
		{ID: "merge", Kind: constant.PipelineKindStatic, Type: constant.PipelineActionMergeWorktree, Name: constant.PipelineSystemCheckMergeName, Branches: []pipelinemodel.Branch{{Name: constant.PipelineBranchSuccess, Target: "cleanup"}, {Name: constant.PipelineBranchDiverged, Target: "resolve"}, {Name: constant.PipelineBranchDirty}}},
		{ID: "resolve", Kind: constant.PipelineKindAgent, Type: constant.PipelineActionResolveConflicts, Name: constant.PipelineSystemResolveName, Prompt: constant.PipelineSystemResolvePrompt, Criteria: constant.PipelineSystemResolveCriteria, Permission: constant.AppDataPermissionWorkspaceWrite, TimeoutSeconds: constant.PipelineSystemResolveTimeout, Branches: success("cleanup"), Position: pipelinemodel.Position{X: 300, Y: 100}},
		{ID: "cleanup", Kind: constant.PipelineKindStatic, Type: constant.PipelineActionRemoveWorktree, Name: constant.PipelineSystemCleanupName, Branches: success("done"), Position: pipelinemodel.Position{X: 600, Y: 100}},
		end,
	}
	return task
}

// versionCompleted 检查版本内功能和步骤的实际完成状态，空集合不能通过
func versionCompleted(version workspacemodel.Version) error {
	if len(version.Features) == 0 {
		return constant.ErrPipelineVersionIncomplete
	}
	for _, feature := range version.Features {
		if !feature.Completed || len(feature.Steps) == 0 {
			return constant.ErrPipelineVersionIncomplete
		}
		for _, step := range feature.Steps {
			if !step.Completed {
				return constant.ErrPipelineVersionIncomplete
			}
		}
	}
	return nil
}

// performWorktree 为版本流程加载长期工作区记录，其余流水线保持运行级工作区
func (s *Service) performWorktree(ctx context.Context, run pipelinemodel.Run, kind string) (any, error) {
	for _, current := range s.GetState().Runs {
		if current.ID == run.ID {
			run = current
			break
		}
	}
	if run.VersionID == "" {
		return s.performRunWorktree(ctx, run, kind)
	}
	project, version, err := s.work.GetVersion(run.ProjectID, run.VersionID)
	if err != nil {
		return nil, err
	}
	stored := version.Worktree
	if kind == constant.PipelineActionCreateWorktree && stored != nil && stored.Status == constant.WorkspaceWorktreeMerged {
		return nil, constant.ErrPipelineVersionCleanupPending
	}
	if kind == constant.PipelineActionCreateWorktree && (stored == nil || stored.Status == constant.WorkspaceWorktreeCleaned) {
		branch, err := git(ctx, project.Directory, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return nil, err
		}
		commit, err := git(ctx, project.Directory, "rev-parse", "HEAD")
		if err != nil {
			return nil, err
		}
		stored = &workspacemodel.VersionWorktree{Repository: project.Directory, SourceBranch: branch, SourceCommit: commit, Branch: constant.PipelineVersionBranchPrefix + version.ID, Directory: filepath.Join(s.root, "worktrees", constant.PipelineVersionDirectoryPrefix+version.ID), Status: constant.WorkspaceWorktreePreparing}
	}
	if stored == nil || stored.Directory == "" {
		return nil, constant.ErrPipelineWorktreeMissing
	}
	if kind != constant.PipelineActionCreateWorktree {
		if err := versionCompleted(version); err != nil {
			return nil, err
		}
	}
	stored.RunID = run.ID
	if err := s.work.SetVersionWorktree(run.ProjectID, run.VersionID, stored); err != nil {
		return nil, err
	}
	run.Worktree = &pipelinemodel.Worktree{Directory: stored.Directory, Repository: stored.Repository, SourceBranch: stored.SourceBranch, SourceCommit: stored.SourceCommit, Branch: stored.Branch, MergedCommit: stored.MergedCommit}
	if err := s.updateRun(run.ID, func(value *pipelinemodel.Run) { value.Worktree = clone(run.Worktree) }); err != nil {
		return nil, err
	}
	// 清理失败后重试时只清理，不重新整合或合并
	if kind == constant.PipelineActionMergeWorktree && stored.Status == constant.WorkspaceWorktreeMerged {
		return run.Worktree, nil
	}
	result, err := s.performRunWorktree(ctx, run, kind)
	if err != nil {
		return nil, err
	}
	if report, ok := result.(pipelinemodel.NodeReport); ok {
		if report.Branch == constant.PipelineBranchDirty {
			return nil, fmt.Errorf("%w：%v", constant.ErrPipelineVersionDirty, report.Output)
		}
		return result, nil
	}
	for _, current := range s.GetState().Runs {
		if current.ID == run.ID && current.Worktree != nil {
			stored.MergedCommit = current.Worktree.MergedCommit
			break
		}
	}
	switch kind {
	case constant.PipelineActionCreateWorktree:
		stored.Status = constant.WorkspaceWorktreeReady
	case constant.PipelineActionMergeWorktree:
		stored.Status = constant.WorkspaceWorktreeMerged
	case constant.PipelineActionRemoveWorktree:
		stored.Status, stored.Directory = constant.WorkspaceWorktreeCleaned, ""
	}
	if err := s.work.SetVersionWorktree(run.ProjectID, run.VersionID, stored); err != nil {
		return nil, err
	}
	return result, nil
}

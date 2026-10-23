package pipeline

import (
	"context"
	"path/filepath"

	"agentor/internal/appdata"
	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
)

// ControlRun 校验项目和状态后执行暂停、继续或停止命令
func (s *Service) ControlRun(projectID, id, operation string) (pipelinemodel.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ctx.Err(); err != nil {
		return pipelinemodel.Run{}, err
	}

	next := clone(s.state)
	for i := range next.Runs {
		run := &next.Runs[i]
		if run.ID != id || run.ProjectID != projectID {
			continue
		}

		launch := false
		switch operation {
		case constant.PipelineControlPause:
			if run.Status != constant.TasksStatusQueued && run.Status != constant.TasksStatusRunning {
				return pipelinemodel.Run{}, constant.ErrPipelineTransitionInvalid
			}
			run.PauseRequested = true
		case constant.PipelineControlResume:
			if run.Status != constant.PipelineStatusPaused {
				return pipelinemodel.Run{}, constant.ErrPipelineTransitionInvalid
			}

			if s.cancellations[id] != nil {
				return pipelinemodel.Run{}, constant.ErrPipelineWorkerActive
			}
			for _, other := range next.Runs {
				if other.ID != id && other.TaskID == run.TaskID && activeRun(other.Status) {
					return pipelinemodel.Run{}, constant.ErrPipelineTaskAlreadyRunning
				}
			}
			run.Status, run.Error, run.CompletedAt, run.PauseRequested = constant.TasksStatusQueued, "", "", false
			launch = true
		case constant.PipelineControlStop:
			if !activeRun(run.Status) {
				return pipelinemodel.Run{}, constant.ErrPipelineTransitionInvalid
			}
			run.PauseRequested = false
			if run.Status == constant.PipelineStatusPaused {
				run.Status, run.CompletedAt = constant.TasksStatusCancelled, now()
			}
		default:
			return pipelinemodel.Run{}, constant.ErrPipelineTransitionInvalid
		}

		if err := appdata.WriteJSON(filepath.Join(s.root, constant.PipelineStateFilename), next); err != nil {
			return pipelinemodel.Run{}, err
		}
		s.state = next
		if launch {
			ctx, cancel := context.WithCancel(s.ctx)
			s.cancellations[id] = cancel
			s.workerGroup.Add(1)
			go s.execute(ctx, clone(*run))
		} else if operation == constant.PipelineControlStop && s.cancellations[id] != nil {
			s.cancellations[id]()
		}
		s.changed()
		return clone(*run), nil
	}
	return pipelinemodel.Run{}, constant.ErrPipelineRunNotFound
}

// activeRun 判断运行是否仍占用任务的执行位置
func activeRun(status string) bool {
	return status == constant.TasksStatusQueued || status == constant.TasksStatusRunning || status == constant.PipelineStatusPaused
}

// pauseAtBoundary 在节点边界持久化暂停，执行中的节点不被强行中止
func (s *Service) pauseAtBoundary(id string) (bool, error) {
	paused := false
	err := s.updateRun(id, func(run *pipelinemodel.Run) {
		if run.PauseRequested {
			run.Status, run.PauseRequested = constant.PipelineStatusPaused, false
			paused = true
		}
	})
	return paused, err
}

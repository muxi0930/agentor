package workspace

import (
	"errors"
	"path/filepath"
	"reflect"
	"time"

	"agentor/internal/appdata"
)

type StepCompletion struct {
	Completed    bool   // 是否已完成
	CompletedAt  string // 完成时间，未完成时为空
	DispatchedAt string // 最后一次派发时间
}

// CompleteStep 手动完成步骤并汇总所属功能状态
func (s *Service) CompleteStep(stepID string) (State, error) {
	return s.mutate(func(state *State) error {
		_, _, _, step := findStep(state, stepID)
		if step == nil {
			return errors.New("步骤不存在")
		}
		if !step.Completed {
			step.Completed, step.CompletedAt = true, time.Now().UTC().Format(time.RFC3339Nano)
		}
		return nil
	})
}

// SyncCompletion 根据各步骤最后一次派发的执行记录同步完成状态
//
//wails:ignore
func (s *Service) SyncCompletion(latest map[string]StepCompletion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := clone(s.state)
	for p := range next.Projects {
		for v := range next.Projects[p].Versions {
			for f := range next.Projects[p].Versions[v].Features {
				feature := &next.Projects[p].Versions[v].Features[f]
				for i := range feature.Steps {
					step := &feature.Steps[i]
					if completion, ok := latest[step.ID]; ok {
						// 保留派发之后的完成标记，避免旧执行记录在后续保存时覆盖
						completedAt, _ := time.Parse(time.RFC3339Nano, step.CompletedAt)
						dispatchedAt, err := time.Parse(time.RFC3339Nano, completion.DispatchedAt)
						if step.Completed && err == nil && completedAt.After(dispatchedAt) {
							continue
						}
						step.Completed, step.CompletedAt = completion.Completed, completion.CompletedAt
					}
				}
			}
		}
	}
	summarizeCompletion(&next)
	if reflect.DeepEqual(s.state, next) {
		return nil
	}
	if err := appdata.WriteJSON(filepath.Join(s.root, "workspace.json"), next); err != nil {
		return err
	}
	s.state = next
	return nil
}

// summarizeCompletion 按全部步骤的完成状态汇总功能及其最晚完成时间
func summarizeCompletion(state *State) {
	for p := range state.Projects {
		for v := range state.Projects[p].Versions {
			for f := range state.Projects[p].Versions[v].Features {
				feature := &state.Projects[p].Versions[v].Features[f]
				feature.Completed, feature.CompletedAt = len(feature.Steps) > 0, ""
				var latest time.Time
				for _, step := range feature.Steps {
					if !step.Completed {
						feature.Completed = false
						break
					}
					completedAt, _ := time.Parse(time.RFC3339Nano, step.CompletedAt)
					if completedAt.After(latest) {
						latest, feature.CompletedAt = completedAt, step.CompletedAt
					}
				}
				if !feature.Completed {
					feature.CompletedAt = ""
				}
			}
		}
	}
}

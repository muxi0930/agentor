package workspace

import (
	"slices"

	"agentor/internal/constant"
	workspacemodel "agentor/internal/workspace/model"
)

// MoveNode 调整项目标签版本功能或步骤的顺序
func (s *Service) MoveNode(kind, id string, offset int) (workspacemodel.State, error) {
	if offset == 0 {
		return workspacemodel.State{}, constant.ErrWorkspaceMoveOffsetZero
	}

	return s.mutate(func(state *workspacemodel.State) error {
		if kind == constant.WorkspaceKindProject {
			return move(state.Projects, id, offset, func(v workspacemodel.Project) string { return v.ID })
		}
		for p := range state.Projects {
			project := &state.Projects[p]
			if kind == constant.WorkspaceKindTag && slices.ContainsFunc(project.Tags, func(v workspacemodel.Tag) bool { return v.ID == id }) {
				return move(project.Tags, id, offset, func(v workspacemodel.Tag) string { return v.ID })
			}
			if kind == constant.WorkspaceKindVersion && findVersion(project, id) != nil {
				return move(project.Versions, id, offset, func(v workspacemodel.Version) string { return v.ID })
			}
			for v := range project.Versions {
				version := &project.Versions[v]
				if kind == constant.WorkspaceKindFeature && slices.ContainsFunc(version.Features, func(v workspacemodel.Feature) bool { return v.ID == id }) {
					return move(version.Features, id, offset, func(v workspacemodel.Feature) string { return v.ID })
				}
				for f := range version.Features {
					feature := &version.Features[f]
					if kind == constant.WorkspaceKindStep && slices.ContainsFunc(feature.Steps, func(v workspacemodel.Step) bool { return v.ID == id }) {
						return move(feature.Steps, id, offset, func(v workspacemodel.Step) string { return v.ID })
					}
				}
			}
		}
		return constant.ErrWorkspaceSortTargetNotFound
	})
}

// MoveFeature 将功能移动到同一项目内的另一版本
func (s *Service) MoveFeature(projectID, featureID, targetVersionID string) (workspacemodel.State, error) {
	return s.mutate(func(state *workspacemodel.State) error {
		project := findProject(state, projectID)
		source, feature := findFeature(project, featureID)
		target := findVersion(project, targetVersionID)
		if feature == nil || target == nil {
			return constant.ErrWorkspaceMoveFeatureOutsideProject
		}
		if source.ID == target.ID {
			return nil
		}
		for i := range feature.Steps {
			feature.Steps[i].ThreadID = ""
		}
		target.Features = append(target.Features, *feature)
		source.Features = slices.DeleteFunc(source.Features, func(v workspacemodel.Feature) bool { return v.ID == featureID })
		return nil
	})
}

// DeleteNode 删除节点前检查关联任务并移除节点
func (s *Service) DeleteNode(kind, id string) (workspacemodel.State, error) {
	if s.deleteGuard != nil && kind != constant.WorkspaceKindTag {
		steps := []string{}
		for _, project := range s.GetState().Projects {
			for _, version := range project.Versions {
				for _, feature := range version.Features {
					for _, step := range feature.Steps {
						if kind == constant.WorkspaceKindProject && project.ID == id || kind == constant.WorkspaceKindVersion && version.ID == id || kind == constant.WorkspaceKindFeature && feature.ID == id || kind == constant.WorkspaceKindStep && step.ID == id {
							steps = append(steps, step.ID)
						}
					}
				}
			}
		}
		if len(steps) > 0 {
			release, err := s.deleteGuard(steps)
			if err != nil {
				return workspacemodel.State{}, err
			}
			if release != nil {
				defer release()
			}
		}
	}

	return s.mutate(func(state *workspacemodel.State) error {
		found := false
		if kind == constant.WorkspaceKindProject {
			state.Projects = slices.DeleteFunc(state.Projects, func(v workspacemodel.Project) bool {
				if v.ID == id {
					found = true
					return true
				}
				return false
			})
			state.Documents = slices.DeleteFunc(state.Documents, func(v workspacemodel.Document) bool { return v.ProjectID == id })
		}
		for p := range state.Projects {
			project := &state.Projects[p]
			if kind == constant.WorkspaceKindTag {
				project.Tags = slices.DeleteFunc(project.Tags, func(v workspacemodel.Tag) bool {
					if v.ID == id {
						found = true
						return true
					}
					return false
				})
				for i := range project.Versions {
					project.Versions[i].TagIDs = slices.DeleteFunc(project.Versions[i].TagIDs, func(value string) bool { return value == id })
				}
			}
			if kind == constant.WorkspaceKindVersion {
				project.Versions = slices.DeleteFunc(project.Versions, func(v workspacemodel.Version) bool {
					if v.ID == id {
						found = true
						return true
					}
					return false
				})
			}
			for v := range project.Versions {
				version := &project.Versions[v]
				if kind == constant.WorkspaceKindFeature {
					version.Features = slices.DeleteFunc(version.Features, func(v workspacemodel.Feature) bool {
						if v.ID == id {
							found = true
							return true
						}
						return false
					})
				}
				for f := range version.Features {
					feature := &version.Features[f]
					if kind == constant.WorkspaceKindStep {
						feature.Steps = slices.DeleteFunc(feature.Steps, func(v workspacemodel.Step) bool {
							if v.ID == id {
								found = true
								return true
							}
							return false
						})
					}
				}
			}
		}
		if !found {
			return constant.ErrWorkspaceObjectNotFound
		}
		if kind != constant.WorkspaceKindTag {
			state.SelectedID = ""
		}
		return nil
	})
}

// move 将节点插入目标位置并保持其他节点的相对顺序
func move[T any](items []T, id string, offset int, key func(T) string) error {
	index := slices.IndexFunc(items, func(v T) bool { return key(v) == id })
	if index < 0 {
		return constant.ErrWorkspaceSortTargetNotFound
	}
	target := index + offset
	if target >= 0 && target < len(items) {
		item := items[index]
		if target > index {
			copy(items[index:target], items[index+1:target+1])
		} else {
			copy(items[target+1:index+1], items[target:index])
		}
		items[target] = item
	}
	return nil
}

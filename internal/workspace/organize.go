package workspace

import (
	"errors"
	"slices"
)

func (s *Service) MoveNode(kind, id string, offset int) (State, error) {
	if offset != -1 && offset != 1 {
		return State{}, errors.New("请选择上移或下移")
	}
	return s.mutate(func(state *State) error {
		if kind == "project" {
			return move(state.Projects, id, offset, func(v Project) string { return v.ID })
		}
		for p := range state.Projects {
			project := &state.Projects[p]
			if kind == "tag" && slices.ContainsFunc(project.Tags, func(v Tag) bool { return v.ID == id }) {
				return move(project.Tags, id, offset, func(v Tag) string { return v.ID })
			}
			if kind == "version" && findVersion(project, id) != nil {
				return move(project.Versions, id, offset, func(v Version) string { return v.ID })
			}
			for v := range project.Versions {
				version := &project.Versions[v]
				if kind == "feature" && slices.ContainsFunc(version.Features, func(v Feature) bool { return v.ID == id }) {
					return move(version.Features, id, offset, func(v Feature) string { return v.ID })
				}
				for f := range version.Features {
					feature := &version.Features[f]
					if kind == "step" && slices.ContainsFunc(feature.Steps, func(v Step) bool { return v.ID == id }) {
						return move(feature.Steps, id, offset, func(v Step) string { return v.ID })
					}
				}
			}
		}
		return errors.New("排序对象不存在")
	})
}

func move[T any](items []T, id string, offset int, key func(T) string) error {
	index := slices.IndexFunc(items, func(v T) bool { return key(v) == id })
	if index < 0 {
		return errors.New("排序对象不存在")
	}
	target := index + offset
	if target >= 0 && target < len(items) {
		items[index], items[target] = items[target], items[index]
	}
	return nil
}

func (s *Service) MoveFeature(projectID, featureID, targetVersionID string) (State, error) {
	return s.mutate(func(state *State) error {
		project := findProject(state, projectID)
		source, feature := findFeature(project, featureID)
		target := findVersion(project, targetVersionID)
		if feature == nil || target == nil {
			return errors.New("功能和目标版本必须属于同一项目")
		}
		if source.ID == target.ID {
			return nil
		}
		target.Features = append(target.Features, *feature)
		source.Features = slices.DeleteFunc(source.Features, func(v Feature) bool { return v.ID == featureID })
		return nil
	})
}

func (s *Service) DeleteNode(kind, id string) (State, error) {
	if s.deleteGuard != nil && kind != "tag" {
		steps := []string{}
		for _, project := range s.GetState().Projects {
			for _, version := range project.Versions {
				for _, feature := range version.Features {
					for _, step := range feature.Steps {
						if kind == "project" && project.ID == id || kind == "version" && version.ID == id || kind == "feature" && feature.ID == id || kind == "step" && step.ID == id {
							steps = append(steps, step.ID)
						}
					}
				}
			}
		}
		if len(steps) > 0 {
			release, err := s.deleteGuard(steps)
			if err != nil {
				return State{}, err
			}
			if release != nil {
				defer release()
			}
		}
	}
	return s.mutate(func(state *State) error {
		found := false
		if kind == "project" {
			state.Projects = slices.DeleteFunc(state.Projects, func(v Project) bool {
				if v.ID == id {
					found = true
					return true
				}
				return false
			})
			state.Documents = slices.DeleteFunc(state.Documents, func(v Document) bool { return v.ProjectID == id })
		}
		for p := range state.Projects {
			project := &state.Projects[p]
			if kind == "tag" {
				project.Tags = slices.DeleteFunc(project.Tags, func(v Tag) bool {
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
			if kind == "version" {
				project.Versions = slices.DeleteFunc(project.Versions, func(v Version) bool {
					if v.ID == id {
						found = true
						return true
					}
					return false
				})
			}
			for v := range project.Versions {
				version := &project.Versions[v]
				if kind == "feature" {
					version.Features = slices.DeleteFunc(version.Features, func(v Feature) bool {
						if v.ID == id {
							found = true
							return true
						}
						return false
					})
				}
				for f := range version.Features {
					feature := &version.Features[f]
					if kind == "step" {
						feature.Steps = slices.DeleteFunc(feature.Steps, func(v Step) bool {
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
			return errors.New("对象不存在")
		}
		if kind != "tag" {
			state.SelectedID = ""
		}
		return nil
	})
}

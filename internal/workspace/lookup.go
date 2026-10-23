package workspace

import (
	workspacemodel "agentor/internal/workspace/model"
)

// findProject 按标识查找工作区项目
func findProject(state *workspacemodel.State, id string) *workspacemodel.Project {
	for i := range state.Projects {
		if state.Projects[i].ID == id {
			return &state.Projects[i]
		}
	}
	return nil
}

// findVersion 按标识查找项目内的版本
func findVersion(project *workspacemodel.Project, id string) *workspacemodel.Version {
	if project != nil {
		for i := range project.Versions {
			if project.Versions[i].ID == id {
				return &project.Versions[i]
			}
		}
	}
	return nil
}

// findFeature 查找项目内的功能及其所属版本
func findFeature(project *workspacemodel.Project, id string) (*workspacemodel.Version, *workspacemodel.Feature) {
	if project != nil {
		for i := range project.Versions {
			for j := range project.Versions[i].Features {
				if project.Versions[i].Features[j].ID == id {
					return &project.Versions[i], &project.Versions[i].Features[j]
				}
			}
		}
	}
	return nil, nil
}

// findStep 查找步骤及其所属项目版本和功能
func findStep(state *workspacemodel.State, id string) (*workspacemodel.Project, *workspacemodel.Version, *workspacemodel.Feature, *workspacemodel.Step) {
	for i := range state.Projects {
		project := &state.Projects[i]
		for j := range project.Versions {
			version := &project.Versions[j]
			for k := range version.Features {
				feature := &version.Features[k]
				for l := range feature.Steps {
					if feature.Steps[l].ID == id {
						return project, version, feature, &feature.Steps[l]
					}
				}
			}
		}
	}
	return nil, nil, nil, nil
}

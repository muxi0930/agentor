package workspace

import (
	"crypto/rand"
	"strings"
	"time"

	"agentor/internal/constant"
	workspacemodel "agentor/internal/workspace/model"
)

// BindStepThread 将工作区步骤绑定到业务会话
//
//wails:ignore
func (s *Service) BindStepThread(stepID, threadID string) error {
	_, err := s.mutate(func(state *workspacemodel.State) error {
		_, _, _, step := findStep(state, stepID)
		if step == nil {
			return constant.ErrWorkspaceStepNotFound
		}
		step.ThreadID = threadID
		return nil
	})
	return err
}

// CreateVersion 在指定项目内创建版本并保留桌面选择
//
//wails:ignore
func (s *Service) CreateVersion(projectID string, input workspacemodel.Version) (workspacemodel.Version, error) {
	input.ID = ""
	state, err := s.saveVersion(projectID, input, false)
	if err != nil {
		return workspacemodel.Version{}, err
	}
	project := findProject(&state, projectID)
	return project.Versions[len(project.Versions)-1], nil
}

// CreateFeature 在指定项目版本内创建功能
//
//wails:ignore
func (s *Service) CreateFeature(projectID, versionID string, input workspacemodel.Feature) (workspacemodel.Feature, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return workspacemodel.Feature{}, constant.ErrWorkspaceFeatureTitleRequired
	}

	input.ID = rand.Text()
	input.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	input.Steps = []workspacemodel.Step{}
	input.Completed, input.CompletedAt = false, ""
	input.DocumentIDs = []string{}
	_, err := s.mutate(func(state *workspacemodel.State) error {
		version := findVersion(findProject(state, projectID), versionID)
		if version == nil {
			return constant.ErrWorkspaceVersionOutsideProject
		}
		version.Features = append(version.Features, input)
		return nil
	})
	return input, err
}

// CreateStep 在指定项目功能内创建步骤
//
//wails:ignore
func (s *Service) CreateStep(projectID, featureID string, input workspacemodel.Step) (workspacemodel.Step, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return workspacemodel.Step{}, constant.ErrWorkspaceStepTitleRequired
	}

	input.ID = rand.Text()
	input.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	input.ThreadID = ""
	input.Completed, input.CompletedAt = false, ""
	input.PermissionMode = ""
	_, err := s.mutate(func(state *workspacemodel.State) error {
		_, feature := findFeature(findProject(state, projectID), featureID)
		if feature == nil {
			return constant.ErrWorkspaceFeatureOutsideProject
		}
		feature.Steps = append(feature.Steps, input)
		return nil
	})
	return input, err
}

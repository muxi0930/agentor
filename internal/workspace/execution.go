package workspace

import (
	"crypto/rand"
	"errors"
	"strings"
	"time"
)

// BindStepThread 将工作区步骤绑定到业务会话
//
//wails:ignore
func (s *Service) BindStepThread(stepID, threadID string) error {
	_, err := s.mutate(func(state *State) error {
		_, _, _, step := findStep(state, stepID)
		if step == nil {
			return errors.New("步骤不存在")
		}
		step.ThreadID = threadID
		return nil
	})
	return err
}

// CreateVersion 在指定项目内创建版本并保留桌面选择
//
//wails:ignore
func (s *Service) CreateVersion(projectID string, input Version) (Version, error) {
	input.ID = ""
	state, err := s.saveVersion(projectID, input, false)
	if err != nil {
		return Version{}, err
	}
	project := findProject(&state, projectID)
	return project.Versions[len(project.Versions)-1], nil
}

// CreateFeature 在指定项目版本内创建功能
//
//wails:ignore
func (s *Service) CreateFeature(projectID, versionID string, input Feature) (Feature, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Feature{}, errors.New("请填写功能标题")
	}

	input.ID = rand.Text()
	input.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	input.Steps = []Step{}
	input.Completed, input.CompletedAt = false, ""
	input.DocumentIDs = []string{}
	_, err := s.mutate(func(state *State) error {
		version := findVersion(findProject(state, projectID), versionID)
		if version == nil {
			return errors.New("版本不属于当前项目")
		}
		version.Features = append(version.Features, input)
		return nil
	})
	return input, err
}

// CreateStep 在指定项目功能内创建步骤
//
//wails:ignore
func (s *Service) CreateStep(projectID, featureID string, input Step) (Step, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Step{}, errors.New("请填写步骤标题")
	}

	input.ID = rand.Text()
	input.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	input.ThreadID = ""
	input.Completed, input.CompletedAt = false, ""
	input.PermissionMode = ""
	_, err := s.mutate(func(state *State) error {
		_, feature := findFeature(findProject(state, projectID), featureID)
		if feature == nil {
			return errors.New("功能不属于当前项目")
		}
		feature.Steps = append(feature.Steps, input)
		return nil
	})
	return input, err
}

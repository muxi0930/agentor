package workspace

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"agentor/internal/appdata"
)

type Service struct {
	mu    sync.Mutex
	root  string
	state State
}

func NewService(root string) (*Service, error) {
	s := &Service{root: root, state: State{Projects: []Project{}, Documents: []Document{}}}
	if err := os.MkdirAll(filepath.Join(root, "knowledge"), 0700); err != nil {
		return nil, err
	}
	if err := appdata.ReadJSON(filepath.Join(root, "workspace.json"), &s.state); err != nil {
		return nil, err
	}
	return s, nil
}

func clone(state State) State {
	raw, _ := json.Marshal(state)
	var result State
	json.Unmarshal(raw, &result)
	return result
}

func (s *Service) GetState() State { s.mu.Lock(); defer s.mu.Unlock(); return clone(s.state) }

func (s *Service) mutate(change func(*State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clone(s.state)
	if err := change(&next); err != nil {
		return State{}, err
	}
	if err := appdata.WriteJSON(filepath.Join(s.root, "workspace.json"), next); err != nil {
		return State{}, err
	}
	s.state = next
	return clone(next), nil
}

func (s *Service) Select(id string) (State, error) {
	return s.mutate(func(state *State) error { state.SelectedID = id; return nil })
}

func (s *Service) SaveProject(input Project) (State, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return State{}, errors.New("请填写项目名称")
	}
	if input.Directory == "" {
		return State{}, errors.New("请选择项目目录")
	}
	directory, err := filepath.Abs(input.Directory)
	if err != nil {
		return State{}, err
	}
	info, err := os.Stat(directory)
	if err != nil {
		return State{}, err
	}
	if !info.IsDir() {
		return State{}, errors.New("项目路径不是目录")
	}
	return s.mutate(func(state *State) error {
		project := findProject(state, input.ID)
		if input.ID == "" {
			for _, existing := range state.Projects {
				if existing.Directory == directory {
					return errors.New("此目录已关联一个工作区项目")
				}
			}
			state.Projects = append(state.Projects, Project{ID: rand.Text(), Versions: []Version{}, Tags: []Tag{}, DocumentIDs: []string{}})
			project = &state.Projects[len(state.Projects)-1]
		}
		if project == nil {
			return errors.New("项目不存在")
		}
		project.Name, project.Directory, project.SharedContext, project.Prompt, project.Structure = input.Name, directory, input.SharedContext, input.Prompt, input.Structure
		state.SelectedID = project.ID
		return nil
	})
}

func (s *Service) SaveVersion(projectID string, input Version) (State, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return State{}, errors.New("请填写版本名称")
	}
	return s.mutate(func(state *State) error {
		project := findProject(state, projectID)
		if project == nil {
			return errors.New("项目不存在")
		}
		for _, id := range input.TagIDs {
			if !slices.ContainsFunc(project.Tags, func(tag Tag) bool { return tag.ID == id }) {
				return errors.New("标签不属于当前项目")
			}
		}
		version := findVersion(project, input.ID)
		if input.ID == "" {
			project.Versions = append(project.Versions, Version{ID: rand.Text(), Features: []Feature{}})
			version = &project.Versions[len(project.Versions)-1]
		}
		if version == nil {
			return errors.New("版本不存在")
		}
		version.Name, version.Description, version.TagIDs = input.Name, input.Description, input.TagIDs
		state.SelectedID = version.ID
		return nil
	})
}

func (s *Service) SaveFeature(projectID, versionID string, input Feature) (State, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return State{}, errors.New("请填写功能标题")
	}
	return s.mutate(func(state *State) error {
		version := findVersion(findProject(state, projectID), versionID)
		if version == nil {
			return errors.New("版本不存在或不属于此项目")
		}
		var feature *Feature
		for i := range version.Features {
			if version.Features[i].ID == input.ID {
				feature = &version.Features[i]
			}
		}
		if input.ID == "" {
			version.Features = append(version.Features, Feature{ID: rand.Text(), Steps: []Step{}, DocumentIDs: []string{}})
			feature = &version.Features[len(version.Features)-1]
		}
		if feature == nil {
			return errors.New("功能不存在")
		}
		feature.Title, feature.Description, feature.Context, feature.Prompt = input.Title, input.Description, input.Context, input.Prompt
		state.SelectedID = feature.ID
		return nil
	})
}

func (s *Service) SaveStep(projectID, featureID string, input Step) (State, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return State{}, errors.New("请填写步骤标题")
	}
	if input.PermissionMode != "" && input.PermissionMode != "read-only" && input.PermissionMode != "workspace-write" && input.PermissionMode != "danger-full-access" {
		return State{}, errors.New("未知步骤权限")
	}
	return s.mutate(func(state *State) error {
		_, feature := findFeature(findProject(state, projectID), featureID)
		if feature == nil {
			return errors.New("功能不存在或不属于此项目")
		}
		var step *Step
		for i := range feature.Steps {
			if feature.Steps[i].ID == input.ID {
				step = &feature.Steps[i]
			}
		}
		if input.ID == "" {
			feature.Steps = append(feature.Steps, Step{ID: rand.Text()})
			step = &feature.Steps[len(feature.Steps)-1]
		}
		if step == nil {
			return errors.New("步骤不存在")
		}
		step.Title, step.Description, step.Prompt, step.PermissionMode = input.Title, input.Description, input.Prompt, input.PermissionMode
		state.SelectedID = step.ID
		return nil
	})
}

func (s *Service) SaveTag(projectID string, input Tag) (State, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return State{}, errors.New("请填写标签名称")
	}
	return s.mutate(func(state *State) error {
		project := findProject(state, projectID)
		if project == nil {
			return errors.New("项目不存在")
		}
		if input.ID == "" {
			input.ID = rand.Text()
			project.Tags = append(project.Tags, input)
			return nil
		}
		for i := range project.Tags {
			if project.Tags[i].ID == input.ID {
				project.Tags[i] = input
				return nil
			}
		}
		return errors.New("标签不存在")
	})
}

func (s *Service) GetStepContext(stepID string) (StepContext, error) {
	state := s.GetState()
	project, version, feature, step := findStep(&state, stepID)
	if step == nil {
		return StepContext{}, errors.New("步骤不存在")
	}
	return StepContext{ProjectID: project.ID, ProjectName: project.Name, Directory: project.Directory, SharedContext: project.SharedContext, ProjectPrompt: project.Prompt, Structure: project.Structure, VersionID: version.ID, VersionName: version.Name, FeatureID: feature.ID, FeatureTitle: feature.Title, FeatureDescription: feature.Description, FeatureContext: feature.Context, FeaturePrompt: feature.Prompt, Step: *step, Documents: documentsFor(state, project, feature)}, nil
}

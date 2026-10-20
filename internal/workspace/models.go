package workspace

type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Project struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Directory     string    `json:"directory"`
	SharedContext string    `json:"sharedContext"`
	Prompt        string    `json:"prompt"`
	Structure     string    `json:"structure"`
	Tags          []Tag     `json:"tags"`
	Versions      []Version `json:"versions"`
	DocumentIDs   []string  `json:"documentIds"`
}

type Version struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TagIDs      []string  `json:"tagIds"`
	Features    []Feature `json:"features"`
}

type Feature struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Context     string   `json:"context"`
	Prompt      string   `json:"prompt"`
	DocumentIDs []string `json:"documentIds"`
	Steps       []Step   `json:"steps"`
}

type Step struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Prompt         string `json:"prompt"`
	ThreadID       string `json:"threadId"`
	PermissionMode string `json:"permissionMode"`
}

type Document struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Title       string `json:"title"`
	SourcePath  string `json:"sourcePath"`
	ManagedPath string `json:"managedPath"`
	UpdatedAt   string `json:"updatedAt"`
}

type State struct {
	Projects   []Project  `json:"projects"`
	Documents  []Document `json:"documents"`
	SelectedID string     `json:"selectedId"`
}

type StepContext struct {
	ProjectID          string     `json:"projectId"`
	ProjectName        string     `json:"projectName"`
	Directory          string     `json:"directory"`
	SharedContext      string     `json:"sharedContext"`
	ProjectPrompt      string     `json:"projectPrompt"`
	Structure          string     `json:"structure"`
	VersionID          string     `json:"versionId"`
	VersionName        string     `json:"versionName"`
	FeatureID          string     `json:"featureId"`
	FeatureTitle       string     `json:"featureTitle"`
	FeatureDescription string     `json:"featureDescription"`
	FeatureContext     string     `json:"featureContext"`
	FeaturePrompt      string     `json:"featurePrompt"`
	Step               Step       `json:"step"`
	Documents          []Document `json:"documents"`
}

func findProject(state *State, id string) *Project {
	for i := range state.Projects {
		if state.Projects[i].ID == id {
			return &state.Projects[i]
		}
	}
	return nil
}

func findVersion(project *Project, id string) *Version {
	if project != nil {
		for i := range project.Versions {
			if project.Versions[i].ID == id {
				return &project.Versions[i]
			}
		}
	}
	return nil
}

func findFeature(project *Project, id string) (*Version, *Feature) {
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

func findStep(state *State, id string) (*Project, *Version, *Feature, *Step) {
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

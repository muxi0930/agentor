package workspace

type Tag struct {
	ID    string `json:"id"`    // 稳定标识
	Name  string `json:"name"`  // 显示名称
	Color string `json:"color"` // 标签颜色
}

type Project struct {
	CreatedAt     string    `json:"createdAt,omitempty"` // 创建时间
	ID            string    `json:"id"`                  // 稳定标识
	Name          string    `json:"name"`                // 显示名称
	Directory     string    `json:"directory"`           // 项目工作目录
	SharedContext string    `json:"sharedContext"`       // 项目共享上下文
	Prompt        string    `json:"prompt"`              // 提示词
	Structure     string    `json:"structure"`           // 项目结构说明
	Tags          []Tag     `json:"tags"`                // 项目标签
	Versions      []Version `json:"versions"`            // 项目版本
	DocumentIDs   []string  `json:"documentIds"`         // 绑定的知识文档
}

type Version struct {
	CreatedAt   string    `json:"createdAt,omitempty"` // 创建时间
	DocumentIDs []string  `json:"documentIds"`         // 绑定的知识文档
	ID          string    `json:"id"`                  // 稳定标识
	Name        string    `json:"name"`                // 显示名称
	Description string    `json:"description"`         // 描述
	TagIDs      []string  `json:"tagIds"`              // 关联的项目标签
	Features    []Feature `json:"features"`            // 版本内的功能
}

type Feature struct {
	Completed   bool     `json:"completed"`           // 是否已完成
	CompletedAt string   `json:"completedAt"`         // 完成时间，未完成时为空
	CreatedAt   string   `json:"createdAt,omitempty"` // 创建时间
	ID          string   `json:"id"`                  // 稳定标识
	Title       string   `json:"title"`               // 显示标题
	Description string   `json:"description"`         // 描述
	Context     string   `json:"context"`             // 功能上下文
	Prompt      string   `json:"prompt"`              // 提示词
	DocumentIDs []string `json:"documentIds"`         // 绑定的知识文档
	Steps       []Step   `json:"steps"`               // 功能内的步骤
}

type Step struct {
	Completed      bool   `json:"completed"`           // 是否已完成
	CompletedAt    string `json:"completedAt"`         // 完成时间，未完成时为空
	CreatedAt      string `json:"createdAt,omitempty"` // 创建时间
	ID             string `json:"id"`                  // 稳定标识
	Title          string `json:"title"`               // 显示标题
	Description    string `json:"description"`         // 描述
	Prompt         string `json:"prompt"`              // 提示词
	ThreadID       string `json:"threadId"`            // 绑定的会话
	PermissionMode string `json:"permissionMode"`      // 步骤权限覆盖值
}

type Document struct {
	ID          string `json:"id"`          // 稳定标识
	ProjectID   string `json:"projectId"`   // 所属项目
	Title       string `json:"title"`       // 显示标题
	SourcePath  string `json:"sourcePath"`  // 原始导入路径
	ManagedPath string `json:"managedPath"` // 应用托管文档路径
	UpdatedAt   string `json:"updatedAt"`   // 最近更新时间
}

type State struct {
	Projects   []Project  `json:"projects"`   // 工作区项目
	Documents  []Document `json:"documents"`  // 知识文档
	SelectedID string     `json:"selectedId"` // 当前选中的节点
}

type StepContext struct {
	ProjectID          string     `json:"projectId"`          // 所属项目
	ProjectName        string     `json:"projectName"`        // 项目名称
	Directory          string     `json:"directory"`          // 项目工作目录
	SharedContext      string     `json:"sharedContext"`      // 项目共享上下文
	ProjectPrompt      string     `json:"projectPrompt"`      // 项目提示词
	Structure          string     `json:"structure"`          // 项目结构说明
	VersionID          string     `json:"versionId"`          // 所属版本
	VersionName        string     `json:"versionName"`        // 版本名称
	FeatureID          string     `json:"featureId"`          // 所属功能
	FeatureTitle       string     `json:"featureTitle"`       // 功能标题
	FeatureDescription string     `json:"featureDescription"` // 功能描述
	FeatureContext     string     `json:"featureContext"`     // 功能上下文
	FeaturePrompt      string     `json:"featurePrompt"`      // 功能提示词
	Step               Step       `json:"step"`               // 当前步骤
	Documents          []Document `json:"documents"`          // 知识文档
}

// findProject 按标识查找工作区项目
func findProject(state *State, id string) *Project {
	for i := range state.Projects {
		if state.Projects[i].ID == id {
			return &state.Projects[i]
		}
	}
	return nil
}

// findVersion 按标识查找项目内的版本
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

// findFeature 查找项目内的功能及其所属版本
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

// findStep 查找步骤及其所属项目版本和功能
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

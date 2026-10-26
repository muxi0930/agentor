package workspace

import (
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/constant"
	workspacemodel "agentor/internal/workspace/model"
)

// API 按职责列出服务的全部公开方法
type API interface {
	AcquireVersion(projectID, versionID string, exclusive bool) (func(), error)                     // 获取版本操作范围，普通任务允许并发
	GetVersion(projectID, versionID string) (workspacemodel.Project, workspacemodel.Version, error) // 获取版本与所属项目
	SetVersionWorktree(projectID, versionID string, value *workspacemodel.VersionWorktree) error    // 保存宿主管理的版本工作区
	ResolveStepContext(stepID string) (workspacemodel.StepContext, error)                           // 解析并验证实际步骤执行目录

	SetDeleteGuard(guard func([]string) (func(), error))                                                     // 设置删除节点前的关联任务检查
	GetState() workspacemodel.State                                                                          // 返回与内部状态隔离的工作区快照
	Select(id string) (workspacemodel.State, error)                                                          // 保存当前选中的工作区节点
	SaveProject(input workspacemodel.Project) (workspacemodel.State, error)                                  // 创建或更新项目及其工作目录
	SaveVersion(projectID string, input workspacemodel.Version) (workspacemodel.State, error)                // 创建或更新项目版本
	SaveFeature(projectID, versionID string, input workspacemodel.Feature) (workspacemodel.State, error)     // 创建或更新版本内的功能
	SaveStep(projectID, featureID string, input workspacemodel.Step) (workspacemodel.State, error)           // 创建或更新功能内的步骤
	SaveTag(projectID string, input workspacemodel.Tag) (workspacemodel.State, error)                        // 创建或更新项目标签
	GetStepContext(stepID string) (workspacemodel.StepContext, error)                                        // 汇总步骤所属项目版本功能及知识上下文
	CompleteStep(stepID string) (workspacemodel.State, error)                                                // 手动完成步骤并汇总所属功能状态
	SyncCompletion(latest map[string]workspacemodel.StepCompletion) error                                    // 根据各步骤最后一次派发的执行记录同步完成状态
	BindStepThread(stepID, threadID string) error                                                            // 将工作区步骤绑定到业务会话
	CreateVersion(projectID string, input workspacemodel.Version) (workspacemodel.Version, error)            // 在指定项目内创建版本并保留桌面选择
	CreateFeature(projectID, versionID string, input workspacemodel.Feature) (workspacemodel.Feature, error) // 在指定项目版本内创建功能
	CreateStep(projectID, featureID string, input workspacemodel.Step) (workspacemodel.Step, error)          // 在指定项目功能内创建步骤
	ImportMarkdown(projectID, nodeID string) (workspacemodel.State, error)                                   // 通过原生对话框导入 Markdown 知识文档
	ImportKnowledge(projectID, nodeID, path string) (workspacemodel.State, error)                            // 复制 Markdown 文档并绑定至项目、版本或功能
	BindKnowledge(projectID, nodeID, documentID string, bound bool) (workspacemodel.State, error)            // 调整知识文档与项目、版本或功能的绑定
	ReadKnowledge(projectID, nodeID, documentID string) (workspacemodel.KnowledgeContent, error)             // 读取当前项目、版本或功能可见的知识文档正文
	SearchKnowledge(projectID, nodeID, query string) ([]workspacemodel.SearchMatch, error)                   // 在当前项目、版本或功能范围内搜索知识文档
	DeleteKnowledge(projectID, documentID string) (workspacemodel.State, error)                              // 删除知识文档并移除所有绑定
	PrepareKnowledge(projectID, nodeID, title string) (workspacemodel.Document, error)                       // 创建并绑定用于后续编辑的知识文档
	SyncKnowledge(projectID, nodeID, documentID, content string) (workspacemodel.Document, error)            // 替换知识文档正文并保存更新时间
	MoveNode(kind, id string, offset int) (workspacemodel.State, error)                                      // 调整项目标签版本功能或步骤的顺序
	MoveFeature(projectID, featureID, targetVersionID string) (workspacemodel.State, error)                  // 将功能移动到同一项目内的另一版本
	DeleteNode(kind, id string) (workspacemodel.State, error)                                                // 删除节点前检查关联任务并移除节点
}

var _ API = (*Service)(nil)

type Service struct {
	versionUses       map[string]int                 // 普通任务共享的版本使用计数
	versionOperations map[string]bool                // 工作区准备及合并期间的独占操作
	mu                sync.Mutex                     // 工作区状态读写锁
	root              string                         // 应用数据目录
	state             workspacemodel.State           // 当前工作区状态
	deleteGuard       func([]string) (func(), error) // 删除前核验关联任务并返回释放函数
}

// SetDeleteGuard 设置删除节点前的关联任务检查
//
//wails:ignore
func (s *Service) SetDeleteGuard(guard func([]string) (func(), error)) {
	s.deleteGuard = guard
}

// NewService 初始化工作区并读取本地记录
func NewService(root string) (*Service, error) {
	s := &Service{root: root, state: workspacemodel.State{Projects: []workspacemodel.Project{}, Documents: []workspacemodel.Document{}}}
	if err := os.MkdirAll(filepath.Join(root, constant.WorkspaceKnowledgeDirectory), 0700); err != nil {
		return nil, err
	}
	if err := appdata.ReadJSON(filepath.Join(root, constant.WorkspaceStateFilename), &s.state); err != nil {
		return nil, err
	}
	return s, nil
}

// GetState 返回与内部状态隔离的工作区快照
func (s *Service) GetState() workspacemodel.State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return clone(s.state)
}

// Select 保存当前选中的工作区节点
func (s *Service) Select(id string) (workspacemodel.State, error) {
	return s.mutate(func(state *workspacemodel.State) error {
		state.SelectedID = id
		return nil
	})
}

// SaveProject 创建或更新项目及其工作目录
func (s *Service) SaveProject(input workspacemodel.Project) (workspacemodel.State, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return workspacemodel.State{}, constant.ErrWorkspaceProjectNameRequired
	}
	if input.Directory == "" {
		return workspacemodel.State{}, constant.ErrWorkspaceProjectDirectoryRequired
	}
	directory, err := filepath.Abs(input.Directory)
	if err != nil {
		return workspacemodel.State{}, err
	}
	info, err := os.Stat(directory)
	if err != nil {
		return workspacemodel.State{}, err
	}
	if !info.IsDir() {
		return workspacemodel.State{}, constant.ErrWorkspaceProjectPathNotDirectory
	}

	return s.mutate(func(state *workspacemodel.State) error {
		project := findProject(state, input.ID)
		if input.ID == "" {
			for _, existing := range state.Projects {
				if existing.Directory == directory {
					return constant.ErrWorkspaceProjectDirectoryAlreadyLinked
				}
			}
			state.Projects = append(state.Projects, workspacemodel.Project{ID: rand.Text(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Versions: []workspacemodel.Version{}, Tags: []workspacemodel.Tag{}, DocumentIDs: []string{}})
			project = &state.Projects[len(state.Projects)-1]
		}
		if project == nil {
			return constant.ErrWorkspaceProjectNotFound
		}
		project.Name, project.Directory, project.SharedContext, project.Prompt, project.Structure = input.Name, directory, input.SharedContext, input.Prompt, input.Structure
		state.SelectedID = project.ID
		return nil
	})
}

// SaveVersion 创建或更新项目版本
func (s *Service) SaveVersion(projectID string, input workspacemodel.Version) (workspacemodel.State, error) {
	return s.saveVersion(projectID, input, true)
}

// SaveFeature 创建或更新版本内的功能
func (s *Service) SaveFeature(projectID, versionID string, input workspacemodel.Feature) (workspacemodel.State, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return workspacemodel.State{}, constant.ErrWorkspaceFeatureTitleRequired
	}

	return s.mutate(func(state *workspacemodel.State) error {
		version := findVersion(findProject(state, projectID), versionID)
		if version == nil {
			return constant.ErrWorkspaceVersionNotFoundInProject
		}
		var feature *workspacemodel.Feature
		for i := range version.Features {
			if version.Features[i].ID == input.ID {
				feature = &version.Features[i]
			}
		}
		if input.ID == "" {
			version.Features = append(version.Features, workspacemodel.Feature{ID: rand.Text(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Steps: []workspacemodel.Step{}, DocumentIDs: []string{}})
			feature = &version.Features[len(version.Features)-1]
		}
		if feature == nil {
			return constant.ErrWorkspaceFeatureNotFound
		}
		feature.Title, feature.Description, feature.Context, feature.Prompt = input.Title, input.Description, input.Context, input.Prompt
		state.SelectedID = feature.ID
		return nil
	})
}

// SaveStep 创建或更新功能内的步骤
func (s *Service) SaveStep(projectID, featureID string, input workspacemodel.Step) (workspacemodel.State, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return workspacemodel.State{}, constant.ErrWorkspaceStepTitleRequired
	}
	if input.PermissionMode != "" && input.PermissionMode != constant.AppDataPermissionReadOnly && input.PermissionMode != constant.AppDataPermissionWorkspaceWrite && input.PermissionMode != constant.AppDataPermissionDangerFullAccess {
		return workspacemodel.State{}, constant.ErrWorkspaceStepPermissionUnknown
	}

	return s.mutate(func(state *workspacemodel.State) error {
		_, feature := findFeature(findProject(state, projectID), featureID)
		if feature == nil {
			return constant.ErrWorkspaceFeatureNotFoundInProject
		}
		var step *workspacemodel.Step
		for i := range feature.Steps {
			if feature.Steps[i].ID == input.ID {
				step = &feature.Steps[i]
			}
		}
		if input.ID == "" {
			feature.Steps = append(feature.Steps, workspacemodel.Step{ID: rand.Text(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			step = &feature.Steps[len(feature.Steps)-1]
		}
		if step == nil {
			return constant.ErrWorkspaceStepNotFound
		}
		step.Title, step.Description, step.Prompt, step.PermissionMode = input.Title, input.Description, input.Prompt, input.PermissionMode
		state.SelectedID = step.ID
		return nil
	})
}

// SaveTag 创建或更新项目标签
func (s *Service) SaveTag(projectID string, input workspacemodel.Tag) (workspacemodel.State, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return workspacemodel.State{}, constant.ErrWorkspaceTagNameRequired
	}

	return s.mutate(func(state *workspacemodel.State) error {
		project := findProject(state, projectID)
		if project == nil {
			return constant.ErrWorkspaceProjectNotFound
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
		return constant.ErrWorkspaceTagNotFound
	})
}

// GetStepContext 汇总步骤所属项目版本功能及知识上下文
func (s *Service) GetStepContext(stepID string) (workspacemodel.StepContext, error) {
	state := s.GetState()
	project, version, feature, step := findStep(&state, stepID)
	if step == nil {
		return workspacemodel.StepContext{}, constant.ErrWorkspaceStepNotFound
	}

	return workspacemodel.StepContext{
		ProjectID:          project.ID,
		ProjectName:        project.Name,
		Directory:          project.Directory,
		SharedContext:      project.SharedContext,
		ProjectPrompt:      project.Prompt,
		Structure:          project.Structure,
		VersionID:          version.ID,
		VersionName:        version.Name,
		FeatureID:          feature.ID,
		FeatureTitle:       feature.Title,
		FeatureDescription: feature.Description,
		FeatureContext:     feature.Context,
		FeaturePrompt:      feature.Prompt,
		Step:               *step,
		Documents:          documentsFor(state, project, feature),
	}, nil
}

// clone 复制工作区状态以隔离调用方
func clone(state workspacemodel.State) workspacemodel.State {
	raw, _ := json.Marshal(state)
	var result workspacemodel.State
	json.Unmarshal(raw, &result)
	return result
}

// mutate 保存变更后的工作区状态并返回快照
func (s *Service) mutate(change func(*workspacemodel.State) error) (workspacemodel.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := clone(s.state)
	if err := change(&next); err != nil {
		return workspacemodel.State{}, err
	}
	summarizeCompletion(&next)
	for _, project := range s.state.Projects {
		for _, version := range project.Versions {
			if s.versionOperations[version.ID] {
				updated := findProject(&next, project.ID)
				if updated == nil || updated.Directory != project.Directory || !reflect.DeepEqual(findVersion(updated, version.ID), &version) {
					return workspacemodel.State{}, constant.ErrWorkspaceVersionBusy
				}
			}
			if version.Worktree != nil && version.Worktree.Directory != "" {
				updated := findProject(&next, project.ID)
				if updated == nil || findVersion(updated, version.ID) == nil || updated.Directory != project.Directory {
					return workspacemodel.State{}, constant.ErrWorkspaceVersionWorktreeBound
				}
			}
		}
	}
	if err := appdata.WriteJSON(filepath.Join(s.root, constant.WorkspaceStateFilename), next); err != nil {
		return workspacemodel.State{}, err
	}
	s.state = next
	return clone(next), nil
}

// saveVersion 保存项目版本并按调用入口决定是否更新桌面选择
func (s *Service) saveVersion(projectID string, input workspacemodel.Version, selectVersion bool) (workspacemodel.State, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return workspacemodel.State{}, constant.ErrWorkspaceVersionNameRequired
	}

	return s.mutate(func(state *workspacemodel.State) error {
		project := findProject(state, projectID)
		if project == nil {
			return constant.ErrWorkspaceProjectNotFound
		}
		for _, id := range input.TagIDs {
			if !slices.ContainsFunc(project.Tags, func(tag workspacemodel.Tag) bool { return tag.ID == id }) {
				return constant.ErrWorkspaceTagOutsideProject
			}
		}
		version := findVersion(project, input.ID)
		if input.ID == "" {
			project.Versions = append(project.Versions, workspacemodel.Version{ID: rand.Text(), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Features: []workspacemodel.Feature{}})
			version = &project.Versions[len(project.Versions)-1]
		}
		if version == nil {
			return constant.ErrWorkspaceVersionNotFound
		}
		version.Name, version.Description, version.TagIDs = input.Name, input.Description, input.TagIDs
		if selectVersion {
			state.SelectedID = version.ID
		}
		return nil
	})
}

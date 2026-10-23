package workspace

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"agentor/internal/constant"
	workspacemodel "agentor/internal/workspace/model"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ImportMarkdown 通过原生对话框导入 Markdown 知识文档
func (s *Service) ImportMarkdown(projectID, nodeID string) (workspacemodel.State, error) {
	app := application.Get()
	path, err := app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle(constant.WorkspaceImportMarkdownTitle).AddFilter("Markdown", "*.md;*.markdown").CanChooseFiles(true).CanChooseDirectories(false).PromptForSingleSelection()
	if err != nil {
		return workspacemodel.State{}, err
	}
	if path == "" {
		return s.GetState(), nil
	}

	return s.ImportKnowledge(projectID, nodeID, path)
}

// ImportKnowledge 复制 Markdown 文档并绑定至项目、版本或功能
//
//wails:ignore
func (s *Service) ImportKnowledge(projectID, nodeID, path string) (workspacemodel.State, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return workspacemodel.State{}, constant.ErrWorkspaceMarkdownRequired
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return workspacemodel.State{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return workspacemodel.State{}, err
	}

	id := rand.Text()
	managed := filepath.Join(s.root, constant.WorkspaceKnowledgeDirectory, id+".md")
	return s.mutate(func(state *workspacemodel.State) error {
		project := findProject(state, projectID)
		if project == nil {
			return constant.ErrWorkspaceProjectNotFound
		}
		ids, err := knowledgeBinding(project, nodeID)
		if err != nil {
			return err
		}
		if err := os.WriteFile(managed, content, 0600); err != nil {
			return err
		}
		state.Documents = append(state.Documents, workspacemodel.Document{ID: id, ProjectID: projectID, Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), SourcePath: path, ManagedPath: managed, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		*ids = append(*ids, id)
		return nil
	})
}

// BindKnowledge 调整知识文档与项目、版本或功能的绑定
func (s *Service) BindKnowledge(projectID, nodeID, documentID string, bound bool) (workspacemodel.State, error) {
	return s.mutate(func(state *workspacemodel.State) error {
		project := findProject(state, projectID)
		if project == nil || !slices.ContainsFunc(state.Documents, func(v workspacemodel.Document) bool { return v.ID == documentID && v.ProjectID == projectID }) {
			return constant.ErrWorkspaceDocumentOutsideProject
		}
		ids, err := knowledgeBinding(project, nodeID)
		if err != nil {
			return err
		}
		if bound {
			if !slices.Contains(*ids, documentID) {
				*ids = append(*ids, documentID)
			}
		} else {
			*ids = slices.DeleteFunc(*ids, func(id string) bool { return id == documentID })
		}
		return nil
	})
}

// ReadKnowledge 读取当前项目、版本或功能可见的知识文档正文
func (s *Service) ReadKnowledge(projectID, nodeID, documentID string) (workspacemodel.KnowledgeContent, error) {
	state := s.GetState()
	project := findProject(&state, projectID)
	if project == nil {
		return workspacemodel.KnowledgeContent{}, constant.ErrWorkspaceProjectNotFound
	}

	for _, document := range state.Documents {
		if document.ID != documentID || document.ProjectID != projectID {
			continue
		}
		if nodeID != "" {
			documents, err := knowledgeForNode(state, project, nodeID)
			if err != nil {
				return workspacemodel.KnowledgeContent{}, err
			}
			if !slices.ContainsFunc(documents, func(v workspacemodel.Document) bool { return v.ID == documentID }) {
				return workspacemodel.KnowledgeContent{}, constant.ErrWorkspaceDocumentNotBound
			}
		}

		content, err := os.ReadFile(document.ManagedPath)
		return workspacemodel.KnowledgeContent{Document: document, Content: string(content)}, err
	}
	return workspacemodel.KnowledgeContent{}, constant.ErrWorkspaceDocumentNotFound
}

// SearchKnowledge 在当前项目、版本或功能范围内搜索知识文档
func (s *Service) SearchKnowledge(projectID, nodeID, query string) ([]workspacemodel.SearchMatch, error) {
	state := s.GetState()
	project := findProject(&state, projectID)
	if project == nil {
		return nil, constant.ErrWorkspaceProjectNotFound
	}
	documents := state.Documents
	if nodeID != "" {
		var err error
		documents, err = knowledgeForNode(state, project, nodeID)
		if err != nil {
			return nil, err
		}
	}

	query = strings.ToLower(strings.TrimSpace(query))
	result := []workspacemodel.SearchMatch{}
	for _, document := range documents {
		if document.ProjectID != projectID {
			continue
		}
		content, err := os.ReadFile(document.ManagedPath)
		if err != nil {
			return nil, err
		}
		if query != "" && !strings.Contains(strings.ToLower(document.Title+"\n"+string(content)), query) {
			continue
		}
		snippet := ""
		for _, line := range strings.Split(string(content), "\n") {
			if query == "" || strings.Contains(strings.ToLower(line), query) {
				snippet = line
				break
			}
		}
		letters := []rune(snippet)
		if len(letters) > 240 {
			snippet = string(letters[:240]) + "…"
		}
		result = append(result, workspacemodel.SearchMatch{DocumentID: document.ID, Title: document.Title, Snippet: snippet})
	}
	return result, nil
}

// DeleteKnowledge 删除知识文档并移除所有绑定
func (s *Service) DeleteKnowledge(projectID, documentID string) (workspacemodel.State, error) {
	return s.mutate(func(state *workspacemodel.State) error {
		index := slices.IndexFunc(state.Documents, func(v workspacemodel.Document) bool { return v.ID == documentID && v.ProjectID == projectID })
		if index < 0 {
			return constant.ErrWorkspaceDocumentOutsideProject
		}
		if err := os.Remove(state.Documents[index].ManagedPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		state.Documents = slices.Delete(state.Documents, index, index+1)
		project := findProject(state, projectID)
		if project != nil {
			project.DocumentIDs = slices.DeleteFunc(project.DocumentIDs, func(id string) bool { return id == documentID })
			for v := range project.Versions {
				version := &project.Versions[v]
				version.DocumentIDs = slices.DeleteFunc(version.DocumentIDs, func(id string) bool { return id == documentID })
				for f := range project.Versions[v].Features {
					feature := &project.Versions[v].Features[f]
					feature.DocumentIDs = slices.DeleteFunc(feature.DocumentIDs, func(id string) bool { return id == documentID })
				}
			}
		}
		return nil
	})
}

// PrepareKnowledge 创建并绑定用于后续编辑的知识文档
//
//wails:ignore
func (s *Service) PrepareKnowledge(projectID, nodeID, title string) (workspacemodel.Document, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return workspacemodel.Document{}, constant.ErrWorkspaceDocumentTitleRequired
	}

	id := rand.Text()
	document := workspacemodel.Document{ID: id, ProjectID: projectID, Title: title, ManagedPath: filepath.Join(s.root, constant.WorkspaceKnowledgeDirectory, id+".md"), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, err := s.mutate(func(state *workspacemodel.State) error {
		project := findProject(state, projectID)
		if project == nil {
			return constant.ErrWorkspaceProjectNotFound
		}
		ids, err := knowledgeBinding(project, nodeID)
		if err != nil {
			return err
		}
		if err := os.WriteFile(document.ManagedPath, []byte("# "+title+"\n"), 0600); err != nil {
			return err
		}
		state.Documents = append(state.Documents, document)
		*ids = append(*ids, id)
		return nil
	})
	return document, err
}

// SyncKnowledge 替换知识文档正文并保存更新时间
//
//wails:ignore
func (s *Service) SyncKnowledge(projectID, nodeID, documentID, content string) (workspacemodel.Document, error) {
	current, err := s.ReadKnowledge(projectID, nodeID, documentID)
	if err != nil {
		return workspacemodel.Document{}, err
	}
	document := current.Document
	_, err = s.mutate(func(state *workspacemodel.State) error {
		index := slices.IndexFunc(state.Documents, func(v workspacemodel.Document) bool { return v.ID == documentID && v.ProjectID == projectID })
		if index < 0 {
			return constant.ErrWorkspaceDocumentNotFound
		}
		file, err := os.CreateTemp(filepath.Dir(document.ManagedPath), ".knowledge-*.tmp")
		if err != nil {
			return err
		}
		defer os.Remove(file.Name())

		if _, err = file.WriteString(content); err != nil {
			file.Close()
			return err
		}
		if err = file.Close(); err != nil {
			return err
		}
		if err = os.Rename(file.Name(), document.ManagedPath); err != nil {
			return err
		}
		document.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		state.Documents[index] = document
		return nil
	})
	return document, err
}

// documentsFor 汇总项目、所属版本与功能绑定的知识文档
func documentsFor(state workspacemodel.State, project *workspacemodel.Project, feature *workspacemodel.Feature) []workspacemodel.Document {
	ids := append([]string{}, project.DocumentIDs...)
	if feature != nil {
		version, _ := findFeature(project, feature.ID)
		if version != nil {
			ids = append(ids, version.DocumentIDs...)
		}
		ids = append(ids, feature.DocumentIDs...)
	}

	result := []workspacemodel.Document{}
	for _, document := range state.Documents {
		if document.ProjectID == project.ID && slices.Contains(ids, document.ID) {
			result = append(result, document)
		}
	}
	return result
}

// knowledgeBinding 定位当前节点自身的知识绑定
func knowledgeBinding(project *workspacemodel.Project, nodeID string) (*[]string, error) {
	if nodeID == "" {
		return &project.DocumentIDs, nil
	}
	if version := findVersion(project, nodeID); version != nil {
		return &version.DocumentIDs, nil
	}
	if _, feature := findFeature(project, nodeID); feature != nil {
		return &feature.DocumentIDs, nil
	}
	return nil, constant.ErrWorkspaceNodeOutsideProject
}

// knowledgeForNode 汇总当前节点及其上级绑定的知识
func knowledgeForNode(state workspacemodel.State, project *workspacemodel.Project, nodeID string) ([]workspacemodel.Document, error) {
	if version := findVersion(project, nodeID); version != nil {
		ids := append(append([]string{}, project.DocumentIDs...), version.DocumentIDs...)
		result := []workspacemodel.Document{}
		for _, document := range state.Documents {
			if document.ProjectID == project.ID && slices.Contains(ids, document.ID) {
				result = append(result, document)
			}
		}
		return result, nil
	}
	if _, feature := findFeature(project, nodeID); feature != nil {
		return documentsFor(state, project, feature), nil
	}
	return nil, constant.ErrWorkspaceNodeOutsideProject
}

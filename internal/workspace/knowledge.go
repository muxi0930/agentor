package workspace

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type KnowledgeContent struct {
	Document Document `json:"document"`
	Content  string   `json:"content"`
}

type SearchMatch struct {
	DocumentID string `json:"documentId"`
	Title      string `json:"title"`
	Snippet    string `json:"snippet"`
}

func documentsFor(state State, project *Project, feature *Feature) []Document {
	ids := append([]string{}, project.DocumentIDs...)
	if feature != nil {
		ids = append(ids, feature.DocumentIDs...)
	}
	result := []Document{}
	for _, document := range state.Documents {
		if document.ProjectID == project.ID && slices.Contains(ids, document.ID) {
			result = append(result, document)
		}
	}
	return result
}

func (s *Service) ImportMarkdown(projectID, featureID string) (State, error) {
	app := application.Get()
	path, err := app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).SetTitle("导入 Markdown 知识文档").AddFilter("Markdown", "*.md;*.markdown").CanChooseFiles(true).CanChooseDirectories(false).PromptForSingleSelection()
	if err != nil {
		return State{}, err
	}
	if path == "" {
		return s.GetState(), nil
	}
	return s.ImportKnowledge(projectID, featureID, path)
}

//wails:ignore
func (s *Service) ImportKnowledge(projectID, featureID, path string) (State, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return State{}, errors.New("请选择 Markdown 文档")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return State{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	id := rand.Text()
	managed := filepath.Join(s.root, "knowledge", id+".md")
	return s.mutate(func(state *State) error {
		project := findProject(state, projectID)
		if project == nil {
			return errors.New("项目不存在")
		}
		var feature *Feature
		if featureID != "" {
			_, feature = findFeature(project, featureID)
			if feature == nil {
				return errors.New("功能不属于此项目")
			}
		}
		if err := os.WriteFile(managed, content, 0600); err != nil {
			return err
		}
		state.Documents = append(state.Documents, Document{ID: id, ProjectID: projectID, Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), SourcePath: path, ManagedPath: managed, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		if feature == nil {
			project.DocumentIDs = append(project.DocumentIDs, id)
		} else {
			feature.DocumentIDs = append(feature.DocumentIDs, id)
		}
		return nil
	})
}

func (s *Service) BindKnowledge(projectID, featureID, documentID string, bound bool) (State, error) {
	return s.mutate(func(state *State) error {
		project := findProject(state, projectID)
		if project == nil || !slices.ContainsFunc(state.Documents, func(v Document) bool { return v.ID == documentID && v.ProjectID == projectID }) {
			return errors.New("文档不属于当前项目")
		}
		ids := &project.DocumentIDs
		if featureID != "" {
			_, feature := findFeature(project, featureID)
			if feature == nil {
				return errors.New("功能不属于此项目")
			}
			ids = &feature.DocumentIDs
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

func (s *Service) ReadKnowledge(projectID, featureID, documentID string) (KnowledgeContent, error) {
	state := s.GetState()
	project := findProject(&state, projectID)
	if project == nil {
		return KnowledgeContent{}, errors.New("项目不存在")
	}
	for _, document := range state.Documents {
		if document.ID != documentID || document.ProjectID != projectID {
			continue
		}
		if featureID != "" {
			_, feature := findFeature(project, featureID)
			if feature == nil || (!slices.Contains(project.DocumentIDs, documentID) && !slices.Contains(feature.DocumentIDs, documentID)) {
				return KnowledgeContent{}, errors.New("文档未绑定至当前功能或项目")
			}
		}
		content, err := os.ReadFile(document.ManagedPath)
		return KnowledgeContent{Document: document, Content: string(content)}, err
	}
	return KnowledgeContent{}, errors.New("文档不存在")
}

func (s *Service) SearchKnowledge(projectID, featureID, query string) ([]SearchMatch, error) {
	state := s.GetState()
	project := findProject(&state, projectID)
	if project == nil {
		return nil, errors.New("项目不存在")
	}
	documents := state.Documents
	if featureID != "" {
		_, feature := findFeature(project, featureID)
		if feature == nil {
			return nil, errors.New("功能不属于此项目")
		}
		documents = documentsFor(state, project, feature)
	}
	query = strings.ToLower(strings.TrimSpace(query))
	result := []SearchMatch{}
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
		result = append(result, SearchMatch{DocumentID: document.ID, Title: document.Title, Snippet: snippet})
	}
	return result, nil
}

func (s *Service) DeleteKnowledge(projectID, documentID string) (State, error) {
	return s.mutate(func(state *State) error {
		index := slices.IndexFunc(state.Documents, func(v Document) bool { return v.ID == documentID && v.ProjectID == projectID })
		if index < 0 {
			return errors.New("文档不属于当前项目")
		}
		if err := os.Remove(state.Documents[index].ManagedPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		state.Documents = slices.Delete(state.Documents, index, index+1)
		project := findProject(state, projectID)
		if project != nil {
			project.DocumentIDs = slices.DeleteFunc(project.DocumentIDs, func(id string) bool { return id == documentID })
			for v := range project.Versions {
				for f := range project.Versions[v].Features {
					feature := &project.Versions[v].Features[f]
					feature.DocumentIDs = slices.DeleteFunc(feature.DocumentIDs, func(id string) bool { return id == documentID })
				}
			}
		}
		return nil
	})
}

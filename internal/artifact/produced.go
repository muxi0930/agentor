package artifact

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"
)

type ProducedOwner struct {
	ThreadID    string
	ExecutionID string
	StepID      string
	ProjectID   string
	RequestID   string
}

//wails:ignore
func (s *Service) RegisterProduced(owner ProducedOwner, path, directory string) (Attachment, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(directory, path)
	}
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return Attachment{}, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Attachment{}, err
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return Attachment{}, errors.New("请登记当前项目目录内的任务产物")
	}
	item, err := s.ImportFile(owner.ThreadID, resolved)
	if err != nil {
		return item, err
	}
	item.Kind, item.Source = "artifact", "agent"
	item.ExecutionID, item.StepID, item.ProjectID, item.RequestID = owner.ExecutionID, owner.StepID, owner.ProjectID, owner.RequestID
	s.mu.Lock()
	s.items[item.ID] = item
	err = s.persist()
	s.mu.Unlock()
	return item, err
}

func (s *Service) GetExecutionArtifacts(executionID string) []Attachment {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := []Attachment{}
	for _, item := range s.items {
		if item.Kind == "artifact" && item.ExecutionID == executionID {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt < result[j].CreatedAt })
	return result
}

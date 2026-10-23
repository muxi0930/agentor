package artifact

import (
	"strings"
)

// SavePipelineReport 将 Markdown 报告保存为指定流水线运行的产物
//
//wails:ignore
func (s *Service) SavePipelineReport(runID, projectID, requestID, name, content string) (Attachment, error) {
	item, err := s.save("pipeline:"+runID, name, "pipeline", "text/markdown; charset=utf-8", strings.NewReader(content))
	if err != nil {
		return item, err
	}

	item.Kind, item.PipelineRunID, item.ProjectID, item.RequestID = "artifact", runID, projectID, requestID
	s.mu.Lock()
	s.items[item.ID] = item
	err = s.persist()
	s.mu.Unlock()
	return item, err
}

// GetPipelineArtifacts 读取指定流水线运行的产物
func (s *Service) GetPipelineArtifacts(runID string) []Attachment {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := []Attachment{}
	for _, item := range s.items {
		if item.PipelineRunID == runID && item.Kind == "artifact" {
			result = append(result, item)
		}
	}
	return result
}

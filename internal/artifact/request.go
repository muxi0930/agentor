package artifact

import (
	"crypto/rand"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SaveRequestInput 保存远程请求的独立输入附件
//
//wails:ignore
func (s *Service) SaveRequestInput(requestID, name, mediaType string, input io.Reader) (Attachment, error) {
	return s.saveOwned(Attachment{RequestID: requestID, Kind: "input", Source: "lark"}, filepath.Base(strings.ReplaceAll(name, "\\", "/")), mediaType, input)
}

// RecordFailedInput 保留未下载成功的附件及原因
//
//wails:ignore
func (s *Service) RecordFailedInput(requestID, name, reason string) error {
	item := Attachment{ID: rand.Text(), Name: name, RequestID: requestID, Kind: "input", Source: "lark", Status: "failed", Error: reason, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	s.mu.Lock()
	s.items[item.ID] = item
	err := s.persist()
	s.mu.Unlock()
	return err
}

// GetRequestAttachments 按接收顺序读取请求原始附件
func (s *Service) GetRequestAttachments(requestID string) []Attachment {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := []Attachment{}
	for _, item := range s.items {
		if item.RequestID == requestID && item.ThreadID == "" && item.Kind == "input" {
			result = append(result, item)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt < result[j].CreatedAt })
	return result
}

// CopyRequestInputs 为每次执行复制输入以保留独立归属
//
//wails:ignore
func (s *Service) CopyRequestInputs(requestID, threadID, executionID, stepID string) ([]string, error) {
	ids := []string{}
	if requestID == "" {
		return ids, nil
	}

	for _, item := range s.GetRequestAttachments(requestID) {
		if item.Status != "ready" {
			continue
		}
		copy, err := s.ImportFile(threadID, item.LocalPath)
		if err != nil {
			return nil, err
		}
		copy.RequestID, copy.ExecutionID, copy.StepID, copy.Source = requestID, executionID, stepID, item.Source
		s.mu.Lock()
		s.items[copy.ID] = copy
		err = s.persist()
		s.mu.Unlock()
		if err != nil {
			return nil, err
		}
		ids = append(ids, copy.ID)
	}
	return ids, nil
}

// GetAttachment 按稳定标识读取附件记录
//
//wails:ignore
func (s *Service) GetAttachment(id string) (Attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return Attachment{}, errors.New("附件不存在")
	}
	return item, nil
}

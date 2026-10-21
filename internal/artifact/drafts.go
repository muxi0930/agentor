package artifact

import (
	"errors"
	"os"
	"path/filepath"
)

// DiscardDraftAttachment 只清理尚未发送的应用托管附件副本
func (s *Service) DiscardDraftAttachment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return nil
	}
	if item.Kind != "input" || item.TurnID != "" || item.MessageID != "" || item.RequestID != "" || item.ExecutionID != "" || item.PipelineRunID != "" {
		return errors.New("附件已被消息或任务引用，不能清理")
	}

	if err := os.RemoveAll(filepath.Join(s.root, item.ID)); err != nil {
		return err
	}
	delete(s.items, id)
	return s.persist()
}

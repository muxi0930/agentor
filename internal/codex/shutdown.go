package codex

import (
	"slices"
	"strings"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

// GetActiveTurns 返回活动轮次的本地快照
//
//wails:ignore
func (s *Service) GetActiveTurns() []codexmodel.ActiveTurn {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]codexmodel.ActiveTurn, 0, len(s.active))
	for threadID := range s.active {
		summary := strings.TrimSpace(s.activeText[threadID])
		if summary == "" {
			summary = constant.CodexThreadSummaryPrefix + threadID
		}
		result = append(result, codexmodel.ActiveTurn{ThreadID: threadID, Summary: summary})
	}

	slices.SortFunc(result, func(a, b codexmodel.ActiveTurn) int {
		return strings.Compare(a.ThreadID, b.ThreadID)
	})
	return result
}

// PrepareShutdown 阻止新轮次并同时中断所有已启动的轮次
//
//wails:ignore
func (s *Service) PrepareShutdown() {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return
	}

	s.closing = true
	threadIDs := make([]string, 0, len(s.active))
	for threadID, turnID := range s.active {
		if turnID != constant.CodexPhaseStarting {
			threadIDs = append(threadIDs, threadID)
		}
	}

	s.shutdownRequests.Add(len(threadIDs))
	s.mu.Unlock()

	for _, threadID := range threadIDs {
		go func() {
			defer s.shutdownRequests.Done()

			s.StopTurn(threadID)
		}()
	}
}

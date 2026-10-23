package codex

import (
	"testing"

	codexmodel "agentor/internal/codex/model"
)

// TestGetTokenUsageSnapshot 验证令牌读取按会话隔离且返回独立快照
func TestGetTokenUsageSnapshot(t *testing.T) {
	window := int64(100000)
	s := &Service{usage: map[string]*codexmodel.TokenUsage{"thread": {Last: codexmodel.TokenBreakdown{TotalTokens: 40000}, ModelContextWindow: &window}}}
	if s.GetTokenUsage("other") != nil {
		t.Fatal("未知会话不应借用其他会话用量")
	}
	snapshot := s.GetTokenUsage("thread")
	snapshot.Last.TotalTokens = 1
	*snapshot.ModelContextWindow = 2
	got := s.GetTokenUsage("thread")
	if got.Last.TotalTokens != 40000 || *got.ModelContextWindow != 100000 {
		t.Fatal(got)
	}
}

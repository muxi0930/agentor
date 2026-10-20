package codex

import (
	"encoding/json"
	"os"
	"testing"

	"agentor/internal/appdata"
)

// Opt in with the id of a conversation created for agentor development.
func TestNativeThreadHistory(t *testing.T) {
	threadID := os.Getenv("AGENTOR_CODEX_SMOKE_THREAD")
	if threadID == "" {
		t.Skip("set AGENTOR_CODEX_SMOKE_THREAD for the native app-server check")
	}
	t.Setenv("AGENTOR_DATA_DIR", t.TempDir())
	data, err := appdata.NewService()
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(data, nil)
	defer s.ServiceShutdown()
	if _, err := s.Connect(); err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Thread struct {
			Source     json.RawMessage `json:"source"`
			Originator string          `json:"originator"`
		} `json:"thread"`
	}
	if err := s.call("thread/read", map[string]any{"threadId": threadID}, &metadata); err != nil {
		t.Fatal(err)
	}
	t.Logf("source=%s originator=%s", metadata.Thread.Source, metadata.Thread.Originator)
	conversation, err := s.OpenThread(threadID)
	if err != nil {
		t.Fatal(err)
	}
	userMessages, agentMessages := 0, 0
	for _, turn := range conversation.Turns.Data {
		for _, item := range turn.Items {
			if item.Type == "userMessage" {
				userMessages++
				t.Logf("user message parts=%d", len(item.Content))
			}
			if item.Type == "agentMessage" {
				agentMessages++
			}
		}
	}
	if userMessages == 0 || agentMessages == 0 {
		t.Fatalf("history missing messages: user=%d agent=%d", userMessages, agentMessages)
	}
}

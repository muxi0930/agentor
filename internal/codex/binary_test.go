package codex

import (
	"slices"
	"strings"
	"testing"
)

// TestCLIAuthenticationDoesNotInheritHostTaskIdentity 验证沿用 CLI 认证时隔离宿主任务身份
func TestCLIAuthenticationDoesNotInheritHostTaskIdentity(t *testing.T) {
	t.Setenv("CODEX_INTERNAL_ORIGINATOR_OVERRIDE", "parent-app")
	t.Setenv("CODEX_THREAD_ID", "parent-thread")
	t.Setenv("CODEX_APP_TOOLS_PIPE_PATH", "/parent-tools")
	t.Setenv("OPENAI_API_KEY", "test-key")
	env := processEnvironment("/agentor-home", true)
	for _, entry := range env {
		if strings.HasPrefix(entry, "CODEX_INTERNAL_ORIGINATOR_OVERRIDE=") || strings.HasPrefix(entry, "CODEX_THREAD_ID=") || strings.HasPrefix(entry, "CODEX_APP_TOOLS_PIPE_PATH=") {
			t.Fatal("host identity leaked into child")
		}
	}

	if !slices.Contains(env, "CODEX_HOME=/agentor-home") || !slices.Contains(env, "OPENAI_API_KEY=test-key") {
		t.Fatal("CLI authentication configuration was lost")
	}
	if slices.Contains(processEnvironment("/agentor-home", false), "OPENAI_API_KEY=test-key") {
		t.Fatal("independent configuration inherited CLI credentials")
	}
}

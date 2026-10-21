package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProducedArtifactKeepsOwnershipAndCannotEscapeProject 验证产物归属及项目目录边界
func TestProducedArtifactKeepsOwnershipAndCannotEscapeProject(t *testing.T) {
	s, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	path := filepath.Join(project, "result.md")
	if err = os.WriteFile(path, []byte("# Result"), 0600); err != nil {
		t.Fatal(err)
	}
	owner := ProducedOwner{ThreadID: "thread", ExecutionID: "execution", StepID: "step", ProjectID: "project"}
	item, err := s.RegisterProduced(owner, "result.md", project)
	if err != nil {
		t.Fatal(err)
	}
	if item.Kind != "artifact" || item.ExecutionID != "execution" || len(s.GetExecutionArtifacts("execution")) != 1 || len(s.GetExecutionArtifacts("other")) != 0 {
		t.Fatal(item)
	}
	if raw, err := os.ReadFile(item.LocalPath); err != nil || string(raw) != "# Result" {
		t.Fatal("managed copy missing", err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err = os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RegisterProduced(owner, outside, project); err == nil {
		t.Fatal("outside file registered as project artifact")
	}
	link := filepath.Join(project, "link.md")
	if err = os.Symlink(outside, link); err == nil {
		if _, err = s.RegisterProduced(owner, link, project); err == nil {
			t.Fatal("symlink escaped artifact scope")
		}
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatal("original result file was changed")
	}
}

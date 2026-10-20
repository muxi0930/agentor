package artifact

import (
	"os"
	"strings"
	"testing"
)

// TestRequestInputCopiesKeepIndependentExecutionOwnership 验证附件副本归属及草稿清理范围
func TestRequestInputCopiesKeepIndependentExecutionOwnership(t *testing.T) {
	s, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base, err := s.SaveRequestInput("request", "check.md", "text/markdown", strings.NewReader("ORCHID"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.CopyRequestInputs("request", "thread-a", "execution-a", "step-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CopyRequestInputs("request", "thread-b", "execution-b", "step-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || len(b) != 1 || a[0] == b[0] {
		t.Fatal(a, b)
	}
	if err = s.BindTurn("thread-a", "turn-a", "message-a", a); err != nil {
		t.Fatal(err)
	}
	if err = s.DiscardDraftAttachment(a[0]); err == nil {
		t.Fatal("referenced input was removed")
	}
	if _, err = s.ResolveInputs("thread-b", a); err == nil {
		t.Fatal("another execution reused the attachment ID")
	}
	if originals := s.GetRequestAttachments("request"); len(originals) != 1 || originals[0].ID != base.ID {
		t.Fatal(originals)
	}
	draft, err := s.save("thread-c", "draft.txt", "local", "text/plain", strings.NewReader("draft"))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DiscardDraftAttachment(draft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(draft.LocalPath); !os.IsNotExist(err) {
		t.Fatal("unreferenced app draft copy remains", err)
	}
	if _, err = os.Stat(base.LocalPath); err != nil {
		t.Fatal("request source was removed", err)
	}
}

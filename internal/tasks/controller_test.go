package tasks

import (
	"context"
	"testing"
	"time"

	"agentor/internal/workspace"
)

// TestControllerQueueUsesFIFO 验证控制中心请求按入队顺序串行执行
func TestControllerQueueUsesFIFO(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	started := make(chan string, 3)
	release := map[string]chan struct{}{"one": make(chan struct{}), "two": make(chan struct{}), "three": make(chan struct{})}
	s.runRequest = func(ctx context.Context, request Request) (string, string, error) {
		started <- request.ID
		select {
		case <-release[request.ID]:
			return "completed", request.ID, nil
		case <-ctx.Done():
			return "interrupted", "", ctx.Err()
		}
	}
	s.state.Requests = []Request{{ID: "one", Status: "queued"}, {ID: "two", Status: "queued"}, {ID: "three", Status: "queued"}}
	s.kickController()
	for _, expected := range []string{"one", "two", "three"} {
		select {
		case actual := <-started:
			if actual != expected {
				t.Fatalf("got %s before %s", actual, expected)
			}
			close(release[actual])
		case <-time.After(time.Second):
			t.Fatal("controller did not advance")
		}
	}
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		done := true
		for _, request := range s.GetState().Requests {
			if request.Status != "completed" {
				done = false
			}
		}
		if done {
			break
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatal("controller did not finish the queue")
		}
	}
}

// TestDuplicateSourceMessageReturnsExistingRequest 验证同一来源消息重复提交时复用原请求
func TestDuplicateSourceMessageReturnsExistingRequest(t *testing.T) {
	s, err := NewService(testData(t), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	source := Source{Type: "lark", AppID: "app", ChatID: "chat", SenderID: "sender", MessageID: "message"}
	s.state.Requests = []Request{{ID: "original", Kind: "instruction", Source: source, Status: "completed", Result: "done"}}
	request, err := s.SubmitRequest(ControllerInput{Text: "same delivery", Source: source})
	if err != nil || request.ID != "original" || len(s.GetState().Requests) != 1 {
		t.Fatal(request, err)
	}
}

// TestContinueUsesItsSourceAndQuotedMultipleTargetsNeedClarification 验证继续请求的来源归属和引用多目标时的澄清
func TestContinueUsesItsSourceAndQuotedMultipleTargetsNeedClarification(t *testing.T) {
	data := testData(t)
	work, err := workspace.NewService(data.GetInfo().DataDirectory)
	if err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspace.Project{Name: "project", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspace.Version{Name: "v"})
	if err != nil {
		t.Fatal(err)
	}
	state, err = work.SaveFeature(projectID, state.SelectedID, workspace.Feature{Title: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	featureID := state.SelectedID
	state, err = work.SaveStep(projectID, featureID, workspace.Step{Title: "first"})
	if err != nil {
		t.Fatal(err)
	}
	firstID := state.SelectedID
	state, err = work.SaveStep(projectID, featureID, workspace.Step{Title: "second"})
	if err != nil {
		t.Fatal(err)
	}
	secondID := state.SelectedID
	s, err := NewService(data, nil, work, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.ServiceShutdown()
	a := Source{Type: "lark", AppID: "app", ChatID: "chat", SenderID: "a"}
	b := a
	b.SenderID = "b"
	s.state.LastSteps[sourceKey(a)] = firstID
	resolved, needs, err := s.resolveTarget(ControllerInput{Text: "继续", Source: a})
	if err != nil || needs || resolved.StepID != firstID {
		t.Fatal(resolved, needs, err)
	}
	resolved, needs, err = s.resolveTarget(ControllerInput{Text: "继续", Source: b})
	if err != nil || !needs || resolved.StepID != "" {
		t.Fatal("another sender inherited the target", resolved, needs, err)
	}
	message := a
	message.MessageID = "multi"
	s.state.Executions = []Execution{{StepID: firstID, Source: message}, {StepID: secondID, Source: message}}
	quoted := a
	quoted.ReplyToMessageID = "multi"
	resolved, needs, err = s.resolveTarget(ControllerInput{Text: "继续", Source: quoted})
	if err != nil || !needs || resolved.StepID != "" {
		t.Fatal("ambiguous quoted message picked a step", resolved, needs, err)
	}
	resolved, needs, err = s.resolveTarget(ControllerInput{Text: "继续", Source: quoted, StepID: secondID})
	if err != nil || needs || resolved.StepID != secondID {
		t.Fatal("explicit target lost precedence", resolved, needs, err)
	}
	other, err := work.SaveProject(workspace.Project{Name: "other", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	resolved, needs, err = s.resolveTarget(ControllerInput{Text: "继续", ProjectID: other.SelectedID, Source: a})
	if err != nil || !needs || resolved.StepID != "" || resolved.ProjectID != other.SelectedID {
		t.Fatal("source target escaped selected project", resolved, needs, err)
	}
}

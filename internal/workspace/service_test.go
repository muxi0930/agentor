package workspace

import (
	"os"
	"path/filepath"
	"testing"

	workspacemodel "agentor/internal/workspace/model"
)

// TestMoveKeepsStepAndKnowledgeContextAcrossRestart 验证功能移动与重启后步骤知识上下文一致
func TestMoveKeepsStepAndKnowledgeContextAcrossRestart(t *testing.T) {
	root := t.TempDir()
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.SaveProject(workspacemodel.Project{Name: "Project", Directory: t.TempDir(), SharedContext: "shared", Prompt: "project prompt"})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = s.SaveVersion(projectID, workspacemodel.Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	versionID := state.SelectedID
	state, err = s.SaveFeature(projectID, versionID, workspacemodel.Feature{Title: "Feature", Context: "feature context"})
	if err != nil {
		t.Fatal(err)
	}
	featureID := state.SelectedID
	state, err = s.SaveStep(projectID, featureID, workspacemodel.Step{Title: "Step", Prompt: "step prompt"})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID
	source := filepath.Join(t.TempDir(), "knowledge.md")
	if err = os.WriteFile(source, []byte("# Knowledge\nneedle\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state, err = s.ImportKnowledge(projectID, featureID, source)
	if err != nil {
		t.Fatal(err)
	}
	documentID := state.Documents[0].ID
	state, err = s.SaveVersion(projectID, workspacemodel.Version{Name: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	secondVersionID := state.SelectedID
	if _, err = s.MoveFeature(projectID, featureID, secondVersionID); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	context, err := reopened.GetStepContext(stepID)
	if err != nil {
		t.Fatal(err)
	}
	if context.VersionID != secondVersionID || context.FeatureID != featureID || context.SharedContext != "shared" || context.FeatureContext != "feature context" || context.Step.Prompt != "step prompt" || len(context.Documents) != 1 || context.Documents[0].ID != documentID {
		t.Fatalf("incorrect context: %#v", context)
	}
	read, err := reopened.ReadKnowledge(projectID, featureID, documentID)
	if err != nil || read.Content != "# Knowledge\nneedle\n" {
		t.Fatal(read, err)
	}
	if original, err := os.ReadFile(source); err != nil || string(original) != read.Content {
		t.Fatal("original Markdown was modified", err)
	}
	copy := reopened.GetState()
	copy.Projects[0].Versions[1].Features[0].Steps[0].Title = "changed outside service"
	if context, _ := reopened.GetStepContext(stepID); context.Step.Title != "Step" {
		t.Fatal("snapshot mutated service state")
	}
}

// TestKnowledgeScopeAndCrossProjectMove 验证知识可见范围和跨项目操作限制
func TestKnowledgeScopeAndCrossProjectMove(t *testing.T) {
	s, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.SaveProject(workspacemodel.Project{Name: "A", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectA := state.SelectedID
	state, err = s.SaveVersion(projectA, workspacemodel.Version{Name: "v"})
	if err != nil {
		t.Fatal(err)
	}
	versionA := state.SelectedID
	state, err = s.SaveFeature(projectA, versionA, workspacemodel.Feature{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	featureA := state.SelectedID
	source := filepath.Join(t.TempDir(), "private.md")
	if err = os.WriteFile(source, []byte("needle"), 0600); err != nil {
		t.Fatal(err)
	}
	state, err = s.ImportKnowledge(projectA, featureA, source)
	if err != nil {
		t.Fatal(err)
	}
	documentID := state.Documents[0].ID
	state, err = s.SaveFeature(projectA, versionA, workspacemodel.Feature{Title: "two"})
	if err != nil {
		t.Fatal(err)
	}
	featureOther := state.SelectedID
	if _, err = s.ReadKnowledge(projectA, featureOther, documentID); err == nil {
		t.Fatal("unbound knowledge leaked into another feature")
	}
	if matches, err := s.SearchKnowledge(projectA, featureOther, "needle"); err != nil || len(matches) != 0 {
		t.Fatal("search escaped feature scope", matches, err)
	}
	if _, err = s.BindKnowledge(projectA, "", documentID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadKnowledge(projectA, featureOther, documentID); err != nil {
		t.Fatal("project knowledge was not inherited", err)
	}
	state, err = s.SaveProject(workspacemodel.Project{Name: "B", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectB := state.SelectedID
	state, err = s.SaveVersion(projectB, workspacemodel.Version{Name: "v"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.MoveFeature(projectB, featureA, state.SelectedID); err == nil {
		t.Fatal("cross-project move accepted")
	}
	if _, err = s.BindKnowledge(projectB, "", documentID, true); err == nil {
		t.Fatal("cross-project binding accepted")
	}
	if _, err = s.ReadKnowledge(projectB, "", documentID); err == nil {
		t.Fatal("cross-project read accepted")
	}
}

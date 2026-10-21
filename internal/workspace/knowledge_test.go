package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestKnowledgeHierarchy 验证逐级继承、版本隔离、移动、去重与持久化
func TestKnowledgeHierarchy(t *testing.T) {
	root := t.TempDir()
	s, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.SaveProject(Project{Name: "项目", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = s.SaveVersion(projectID, Version{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	versionID := state.SelectedID
	state, err = s.SaveVersion(projectID, Version{Name: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	otherVersionID := state.SelectedID
	state, err = s.SaveFeature(projectID, versionID, Feature{Title: "功能"})
	if err != nil {
		t.Fatal(err)
	}
	featureID := state.SelectedID
	state, err = s.SaveStep(projectID, featureID, Step{Title: "步骤"})
	if err != nil {
		t.Fatal(err)
	}
	stepID := state.SelectedID
	source := filepath.Join(t.TempDir(), "knowledge.md")
	if err := os.WriteFile(source, []byte("needle"), 0600); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, nodeID := range []string{"", versionID, featureID, otherVersionID} {
		state, err = s.ImportKnowledge(projectID, nodeID, source)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, state.Documents[len(state.Documents)-1].ID)
	}
	for nodeID, count := range map[string]int{versionID: 2, featureID: 3, otherVersionID: 2} {
		matches, err := s.SearchKnowledge(projectID, nodeID, "needle")
		if err != nil || len(matches) != count {
			t.Fatalf("节点 %s 的知识范围错误: %v, %v", nodeID, matches, err)
		}
	}
	if _, err := s.ReadKnowledge(projectID, featureID, ids[1]); err != nil {
		t.Fatal("功能无法读取版本知识", err)
	}
	if _, err := s.ReadKnowledge(projectID, otherVersionID, ids[1]); err == nil {
		t.Fatal("版本知识泄漏至其他版本")
	}
	if _, err := s.BindKnowledge(projectID, versionID, ids[0], true); err != nil {
		t.Fatal(err)
	}
	s, err = NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	context, err := s.GetStepContext(stepID)
	if err != nil || len(context.Documents) != 3 {
		t.Fatal("继承或重启后的去重错误", context.Documents, err)
	}
	if _, err := s.SaveVersion(projectID, Version{ID: versionID, Name: "更新版本"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MoveFeature(projectID, featureID, otherVersionID); err != nil {
		t.Fatal(err)
	}
	context, err = s.GetStepContext(stepID)
	if err != nil || len(context.Documents) != 3 || slices.ContainsFunc(context.Documents, func(d Document) bool { return d.ID == ids[1] }) {
		t.Fatal("移动后没有切换版本知识", context.Documents, err)
	}
	if _, err := s.BindKnowledge(projectID, otherVersionID, ids[3], false); err != nil {
		t.Fatal(err)
	}
	context, err = s.GetStepContext(stepID)
	if err != nil || len(context.Documents) != 2 {
		t.Fatal("取消版本绑定没有生效", context.Documents, err)
	}
	if _, err := s.DeleteKnowledge(projectID, ids[1]); err != nil {
		t.Fatal(err)
	}
	state = s.GetState()
	if slices.Contains(state.Projects[0].Versions[0].DocumentIDs, ids[1]) {
		t.Fatal("删除文档后残留版本绑定")
	}
	if _, err := s.BindKnowledge(projectID, stepID, ids[0], true); err == nil {
		t.Fatal("步骤不应支持单独绑定")
	}
}

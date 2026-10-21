package workspace_test

import (
	"reflect"
	"testing"
	"time"

	"agentor/internal/workspace"
)

// TestCreateVersionPreservesSelectionAndPersists 验证宿主创建版本保留选择且持久化后可读取
func TestCreateVersionPreservesSelectionAndPersists(t *testing.T) {
	root := t.TempDir()
	work, err := workspace.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspace.Project{Name: "项目", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	state, err = work.SaveVersion(projectID, workspace.Version{Name: "已有版本"})
	if err != nil {
		t.Fatal(err)
	}
	selectedID := state.SelectedID
	created, err := work.CreateVersion(projectID, workspace.Version{ID: selectedID, Name: " 0.2.0 ", Description: "版本说明"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.ID == selectedID || created.Name != "0.2.0" || created.Description != "版本说明" || len(created.Features) != 0 {
		t.Fatal(created)
	}
	if _, err := time.Parse(time.RFC3339Nano, created.CreatedAt); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state = reopened.GetState()
	if state.SelectedID != selectedID || len(state.Projects[0].Versions) != 2 || !reflect.DeepEqual(state.Projects[0].Versions[1], created) {
		t.Fatal(state)
	}
	before := work.GetState()
	for _, input := range []struct {
		projectID string            // 目标项目
		version   workspace.Version // 创建参数
	}{
		{projectID, workspace.Version{Name: "  "}},
		{"missing", workspace.Version{Name: "版本"}},
		{projectID, workspace.Version{Name: "版本", TagIDs: []string{"missing"}}},
	} {
		if _, err := work.CreateVersion(input.projectID, input.version); err == nil {
			t.Fatal("无效创建应失败", input)
		}
		if !reflect.DeepEqual(before, work.GetState()) {
			t.Fatal("失败的创建改变了工作区")
		}
	}
}

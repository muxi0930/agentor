package workspace_test

import (
	"encoding/json"
	"testing"
	"time"

	"agentor/internal/workspace"
	workspacemodel "agentor/internal/workspace/model"
)

// TestControllerCreatedAt 验证飞书控制器创建入口返回真实时间并持久化到列表数据
func TestControllerCreatedAt(t *testing.T) {
	root := t.TempDir()
	work, err := workspace.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := work.SaveProject(workspacemodel.Project{Name: "项目", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	projectID := state.SelectedID
	before := time.Now().UTC()
	state, err = work.SaveVersion(projectID, workspacemodel.Version{Name: "版本"})
	if err != nil {
		t.Fatal(err)
	}
	version := state.Projects[0].Versions[0]
	feature, err := work.CreateFeature(projectID, version.ID, workspacemodel.Feature{Title: "功能"})
	if err != nil {
		t.Fatal(err)
	}
	step, err := work.CreateStep(projectID, feature.ID, workspacemodel.Step{Title: "步骤"})
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()
	expected := []string{version.CreatedAt, feature.CreatedAt, step.CreatedAt}
	for i, value := range expected {
		created, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || created.Before(before) || created.After(after) {
			t.Errorf("层级 %d 创建时间无效：%q，%v", i, value, err)
		}
	}
	reopened, err := workspace.NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.GetState().SelectedID != version.ID {
		t.Fatal("控制器创建改变了桌面选择")
	}
	data, err := json.Marshal(reopened.GetState())
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Projects []struct {
			Versions []struct {
				CreatedAt string `json:"createdAt"`
				Features  []struct {
					CreatedAt string `json:"createdAt"`
					Steps     []struct {
						CreatedAt string `json:"createdAt"`
					} `json:"steps"`
				} `json:"features"`
			} `json:"versions"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	v := payload.Projects[0].Versions[0]
	actual := []string{v.CreatedAt, v.Features[0].CreatedAt, v.Features[0].Steps[0].CreatedAt}
	for i, value := range actual {
		if value != expected[i] {
			t.Fatalf("层级 %d 持久化或序列化丢失创建时间：%q != %q", i, value, expected[i])
		}
	}
}

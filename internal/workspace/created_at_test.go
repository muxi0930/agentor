package workspace

import (
	"path/filepath"
	"testing"
	"time"

	"agentor/internal/appdata"
	workspacemodel "agentor/internal/workspace/model"
)

// TestCreatedAtPersists 验证四级节点创建时间在编辑和重启后保持不变且历史数据不补写时间
func TestCreatedAtPersists(t *testing.T) {
	root := t.TempDir()
	service, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.SaveProject(workspacemodel.Project{Name: "项目", Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	project := state.Projects[0]
	state, err = service.SaveVersion(project.ID, workspacemodel.Version{Name: "版本"})
	if err != nil {
		t.Fatal(err)
	}
	version := state.Projects[0].Versions[0]
	state, err = service.SaveFeature(project.ID, version.ID, workspacemodel.Feature{Title: "功能"})
	if err != nil {
		t.Fatal(err)
	}
	feature := state.Projects[0].Versions[0].Features[0]
	state, err = service.SaveStep(project.ID, feature.ID, workspacemodel.Step{Title: "步骤"})
	if err != nil {
		t.Fatal(err)
	}
	step := state.Projects[0].Versions[0].Features[0].Steps[0]
	expected := []string{project.CreatedAt, version.CreatedAt, feature.CreatedAt, step.CreatedAt}
	for _, value := range expected {
		if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
			t.Fatalf("invalid creation time %q: %v", value, err)
		}
	}

	// 更新请求携带其他时间也不能覆盖服务端记录
	project.CreatedAt, version.CreatedAt, feature.CreatedAt, step.CreatedAt = "changed", "changed", "changed", "changed"
	updates := []func() (workspacemodel.State, error){
		func() (workspacemodel.State, error) { return service.SaveProject(project) },
		func() (workspacemodel.State, error) { return service.SaveVersion(project.ID, version) },
		func() (workspacemodel.State, error) { return service.SaveFeature(project.ID, version.ID, feature) },
		func() (workspacemodel.State, error) { return service.SaveStep(project.ID, feature.ID, step) },
	}
	for _, update := range updates {
		if _, err := update(); err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	state = reopened.GetState()
	project = state.Projects[0]
	version = project.Versions[0]
	feature = version.Features[0]
	step = feature.Steps[0]
	actual := []string{project.CreatedAt, version.CreatedAt, feature.CreatedAt, step.CreatedAt}
	for i, value := range actual {
		if value != expected[i] {
			t.Fatalf("creation time changed: got %q, want %q", value, expected[i])
		}
	}

	state.Projects[0].CreatedAt = ""
	state.Projects[0].Versions[0].CreatedAt = ""
	state.Projects[0].Versions[0].Features[0].CreatedAt = ""
	state.Projects[0].Versions[0].Features[0].Steps[0].CreatedAt = ""
	if err := appdata.WriteJSON(filepath.Join(root, "workspace.json"), state); err != nil {
		t.Fatal(err)
	}
	legacy, err := NewService(root)
	if err != nil {
		t.Fatal(err)
	}
	project.CreatedAt, version.CreatedAt, feature.CreatedAt, step.CreatedAt = "", "", "", ""
	updates = []func() (workspacemodel.State, error){
		func() (workspacemodel.State, error) { return legacy.SaveProject(project) },
		func() (workspacemodel.State, error) { return legacy.SaveVersion(project.ID, version) },
		func() (workspacemodel.State, error) { return legacy.SaveFeature(project.ID, version.ID, feature) },
		func() (workspacemodel.State, error) { return legacy.SaveStep(project.ID, feature.ID, step) },
	}
	for _, update := range updates {
		if _, err := update(); err != nil {
			t.Fatal(err)
		}
	}
	state = legacy.GetState()
	project = state.Projects[0]
	version = project.Versions[0]
	feature = version.Features[0]
	if project.CreatedAt != "" || version.CreatedAt != "" || feature.CreatedAt != "" || feature.Steps[0].CreatedAt != "" {
		t.Fatal("legacy creation time was fabricated")
	}
}

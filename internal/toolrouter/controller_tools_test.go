package toolrouter

import (
	"slices"
	"testing"
)

// TestControllerDispatchToolsRequireExecutionChoice 验证所有派发入口都向控制中心声明模型决策参数
func TestControllerDispatchToolsRequireExecutionChoice(t *testing.T) {
	router := &Router{}
	checked := 0
	for _, spec := range router.Specs("controller") {
		if !slices.Contains([]string{"dispatch_step", "run_pipeline", "request_pipeline_session", "trigger_schedule"}, spec.Name) {
			continue
		}
		properties := spec.InputSchema["properties"].(map[string]any)
		required := spec.InputSchema["required"].([]string)
		if properties["model"] == nil || properties["effort"] == nil || !slices.Contains(required, "model") || !slices.Contains(required, "effort") {
			t.Fatal(spec)
		}
		checked++
	}
	if checked != 4 {
		t.Fatal(checked)
	}
}

// TestControllerCreateVersionSpec 验证版本创建工具仅向控制中心开放并要求明确项目
func TestControllerCreateVersionSpec(t *testing.T) {
	router := &Router{}
	found := false
	for _, spec := range router.Specs("controller") {
		if spec.Name != "create_version" {
			continue
		}
		found = true
		properties := spec.InputSchema["properties"].(map[string]any)
		required := spec.InputSchema["required"].([]string)
		for _, name := range []string{"projectId", "name", "description"} {
			if properties[name] == nil || !slices.Contains(required, name) {
				t.Fatal(spec)
			}
		}
	}
	if !found {
		t.Fatal("控制中心未注册 create_version")
	}
	for _, role := range []string{"step", "pipeline_manager", "pipeline_config"} {
		for _, spec := range router.Specs(role) {
			if spec.Name == "create_version" {
				t.Fatalf("%s 不应拥有创建版本工具", role)
			}
		}
	}
}

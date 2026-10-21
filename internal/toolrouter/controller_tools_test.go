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

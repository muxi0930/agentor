package toolrouter

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
	tasksmodel "agentor/internal/tasks/model"
)

// TestToolCatalogContract 验证角色工具的顺序、说明和完整参数协议保持不变
func TestToolCatalogContract(t *testing.T) {
	if os.Getenv("UPDATE_TOOL_CATALOG") == "1" {
		actual := map[string]any{}
		for _, role := range []string{"controller", "step", "pipeline_node"} {
			actual[role] = (&Router{}).Specs(role)
		}
		raw, _ := json.MarshalIndent(actual, "", "  ")
		if err := os.WriteFile("testdata/catalog.json", append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile("testdata/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]json.RawMessage
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	router := &Router{}
	for role, want := range expected {
		t.Run(role, func(t *testing.T) {
			actual, err := json.Marshal(router.Specs(role))
			if err != nil {
				t.Fatal(err)
			}
			var a, b any
			json.Unmarshal(actual, &a)
			json.Unmarshal(want, &b)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("工具协议改变：%s", actual)
			}
		})
	}
	for _, role := range []string{"", "unknown"} {
		if router.Specs(role) != nil {
			t.Fatalf("未知角色 %q 获得了工具", role)
		}
	}
}

// TestPipelineNodeCannotManageGraph 验证节点 Agent 只能汇报，不能自行编排或修改流水线
func TestPipelineNodeCannotManageGraph(t *testing.T) {
	router := &Router{}
	specs := router.Specs(constant.ToolRouterRolePipelineNode)
	if len(specs) != 1 || specs[0].Name != constant.ToolRouterToolCompleteNode {
		t.Fatal(specs)
	}
	for _, role := range []string{"pipeline_manager", "pipeline_config"} {
		if router.Specs(role) != nil {
			t.Fatal("retired role retains tools", role)
		}
	}
	for _, name := range []string{"save_pipeline_task", "run_pipeline", "control_pipeline_run"} {
		if _, err := authorizeTool(constant.ToolRouterRolePipelineNode, name, false, constant.ErrToolRouterRoleForbidden, nil); err == nil {
			t.Fatal("node received management tool", name)
		}
	}
}

// TestControllerPipelineCannotChangeProject 验证控制中心不能覆盖所属项目
func TestControllerPipelineCannotChangeProject(t *testing.T) {
	_, err := (&Router{}).controllerPipelineCall(tasksmodel.Request{ProjectID: "allowed"}, codexmodel.ToolCall{Tool: "save_pipeline_task", Arguments: json.RawMessage(`{"projectId":"other","json":"{}"}`)})
	if err != constant.ErrToolRouterProjectOutsideScope {
		t.Fatal(err)
	}
}

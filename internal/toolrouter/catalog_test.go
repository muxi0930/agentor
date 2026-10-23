package toolrouter

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	codexmodel "agentor/internal/codex/model"
	pipelinemodel "agentor/internal/pipeline/model"
)

// TestToolCatalogContract 验证角色工具的顺序、说明和完整参数协议保持不变
func TestToolCatalogContract(t *testing.T) {
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

// TestPipelineAuthorizationPrecedesArguments 验证角色和请求类型检查先于参数解析
func TestPipelineAuthorizationPrecedesArguments(t *testing.T) {
	router := &Router{}
	for _, role := range []string{"pipeline_manager", "pipeline_config"} {
		for _, spec := range router.Specs(role) {
			readOnly := spec.Name == "list_actions" || spec.Name == "list_pipeline_tasks" || spec.Name == "validate_pipeline_task"
			for _, kind := range []string{"instruction", "run_result", ""} {
				t.Run(role+"/"+kind+"/"+spec.Name, func(t *testing.T) {
					_, err := router.pipelineCall(pipelinemodel.SessionRequest{Kind: kind, Input: pipelinemodel.SessionInput{Role: role}}, codexmodel.ToolCall{Tool: spec.Name, Arguments: json.RawMessage(`{`)})
					if kind != "instruction" && !readOnly {
						if err == nil || err.Error() != "完成汇报只能查询，不能重新运行或修改配置" {
							t.Fatal(err)
						}
					} else if _, ok := err.(*json.SyntaxError); !ok {
						t.Fatalf("允许的工具未进入参数解析：%v", err)
					}
				})
			}
		}
		_, err := router.pipelineCall(pipelinemodel.SessionRequest{Input: pipelinemodel.SessionInput{Role: role}}, codexmodel.ToolCall{Tool: "unknown", Arguments: json.RawMessage(`{`)})
		if err == nil || err.Error() != "当前流水线角色不能调用此工具" {
			t.Fatal(err)
		}
	}
}

// TestPipelineScheduleCannotChangeProject 验证配置工具不会修改请求范围外的调度
func TestPipelineScheduleCannotChangeProject(t *testing.T) {
	_, err := (&Router{}).pipelineCall(pipelinemodel.SessionRequest{Kind: "instruction", Input: pipelinemodel.SessionInput{Role: "pipeline_config", ProjectID: "allowed"}}, codexmodel.ToolCall{Tool: "save_pipeline_schedule", Arguments: json.RawMessage(`{"json":"{\"projectId\":\"other\"}"}`)})
	if err == nil || err.Error() != "调度不能改变项目范围" {
		t.Fatal(err)
	}
}

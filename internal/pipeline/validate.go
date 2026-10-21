package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// GetActions 读取流水线支持的固定动作目录
func (s *Service) GetActions() []ActionSpec {
	return []ActionSpec{
		{
			"workspace.snapshot",
			"读取当前项目的结构及步骤最新状态。",
			[]Parameter{{"versionId", "string", false}, {"featureId", "string", false}},
			"项目结构和步骤状态对象",
		},
		{
			"step.run",
			"进入正式步骤队列并等待完成。停止流水线只取消本次发起的执行。",
			[]Parameter{{"stepId", "string", true}, {"instruction", "string", true}, {"completionCriteria", "string", false}},
			"executionId、status、result、artifactIds 对象",
		},
		{
			"knowledge.read",
			"读取当前项目共享或指定功能绑定的知识正文。",
			[]Parameter{{"documentId", "string", true}, {"featureId", "string", false}},
			"document 和 content 对象",
		},
		{
			"report.write",
			"将文字或前序完整结果确定性写为 Markdown 报告产物。",
			[]Parameter{{"fileName", "string", true}, {"content", "any", true}},
			"artifactId 和 fileName 对象",
		},
	}
}

// ValidateTask 解析并校验当前项目的任务配置
func (s *Service) ValidateTask(projectID, text string) (Task, error) {
	task, err := parseTask(projectID, text)
	if err != nil {
		return task, err
	}

	return task, s.validateTask(task)
}

// valueType 判断输入参数支持的值类型
func valueType(value any) string {
	switch value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int64, json.Number:
		return "number"
	case map[string]any:
		return "object"
	}

	return "invalid"
}

// reference 识别完整参数值中的输入或结果引用
func reference(value any) (string, string, bool) {
	object, ok := value.(map[string]any)
	if !ok || len(object) != 1 {
		return "", "", false
	}

	for _, kind := range []string{"input", "result"} {
		if name, ok := object[kind].(string); ok {
			return kind, name, true
		}
	}

	return "", "", false
}

// parseTask 解析单个任务并固定所属项目
func parseTask(projectID, text string) (Task, error) {
	var task Task
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&task); err != nil {
		return task, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return task, errors.New("配置只能包含一个 JSON 任务")
	}

	if task.ProjectID != "" && task.ProjectID != projectID {
		return task, errors.New("任务不能改变所属项目")
	}

	task.ProjectID = projectID
	return task, nil
}

// validateTask 校验动作参数、引用顺序与项目归属
func (s *Service) validateTask(task Task) error {
	if _, err := s.project(task.ProjectID); err != nil {
		return err
	}

	if strings.TrimSpace(task.Name) == "" || len(task.Actions) == 0 {
		return errors.New("请填写名称和至少一个固定动作")
	}

	fields := map[string]string{}
	for _, field := range task.Inputs {
		if field.Name == "" || fields[field.Name] != "" {
			return errors.New("输入字段名称为空或重复")
		}

		if field.Type != "string" && field.Type != "number" && field.Type != "boolean" {
			return fmt.Errorf("输入 %s 的类型只支持 string/number/boolean", field.Name)
		}

		fields[field.Name] = field.Type
	}

	previous := map[string]bool{}
	for _, action := range task.Actions {
		if action.ID == "" || previous[action.ID] {
			return errors.New("动作 ID 为空或重复")
		}

		var spec *ActionSpec
		for _, option := range s.GetActions() {
			if option.Name == action.Type {
				copy := option
				spec = &copy
				break
			}
		}

		if spec == nil {
			return fmt.Errorf("不支持固定动作 %s", action.Type)
		}

		parameters := map[string]Parameter{}
		for _, param := range spec.Parameters {
			parameters[param.Name] = param
			if param.Required {
				if _, ok := action.Params[param.Name]; !ok {
					return fmt.Errorf("动作 %s 缺少 %s", action.ID, param.Name)
				}
			}
		}

		for name, value := range action.Params {
			param, ok := parameters[name]
			if !ok {
				return fmt.Errorf("动作 %s 没有参数 %s", action.ID, name)
			}

			kind := valueType(value)
			if ref, id, ok := reference(value); ok {
				if ref == "input" {
					kind = fields[id]
					if kind == "" {
						return fmt.Errorf("输入引用 %s 不存在", id)
					}
				} else {
					if !previous[id] {
						return fmt.Errorf("结果引用 %s 尚未完成或不存在", id)
					}

					kind = "object"
				}
			}

			if kind == "invalid" || kind == "object" && param.Type != "any" || param.Type != "any" && kind != param.Type {
				return fmt.Errorf("动作 %s 参数 %s 应为 %s", action.ID, name, param.Type)
			}

			if kind == "object" {
				if _, _, ok := reference(value); !ok {
					return errors.New("参数对象只支持整个值的 input/result 引用")
				}
			}
		}

		if value, ok := action.Params["stepId"].(string); ok {
			context, err := s.work.GetStepContext(value)
			if err != nil {
				return err
			}

			if context.ProjectID != task.ProjectID {
				return errors.New("步骤不属于当前项目")
			}
		}

		if file, ok := action.Params["fileName"].(string); ok {
			if err := reportName(file); err != nil {
				return err
			}
		}

		previous[action.ID] = true
	}

	return nil
}

// reportName 校验报告使用单个 Markdown 文件名
func reportName(name string) error {
	if name == "" || strings.ContainsAny(name, "/\\") || filepath.Base(name) != name || strings.ToLower(filepath.Ext(name)) != ".md" {
		return errors.New("报告文件名必须是单个 .md 文件名")
	}

	return nil
}

// validateInputs 校验运行输入的必填项与字段类型
func validateInputs(task Task, inputs map[string]any) error {
	fields := map[string]InputField{}
	for _, field := range task.Inputs {
		fields[field.Name] = field
		value, exists := inputs[field.Name]
		if !exists {
			if field.Required {
				return fmt.Errorf("缺少输入 %s", field.Name)
			}

			continue
		}

		if valueType(value) != field.Type {
			return fmt.Errorf("输入 %s 应为 %s", field.Name, field.Type)
		}
	}

	for name := range inputs {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("未知输入 %s", name)
		}
	}

	return nil
}

// resolveParams 将输入和已完成动作结果替换到参数中
func resolveParams(params, inputs, results map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for name, value := range params {
		if kind, id, ok := reference(value); ok {
			source := inputs
			if kind == "result" {
				source = results
			}

			var exists bool
			value, exists = source[id]
			if !exists {
				return nil, fmt.Errorf("引用 %s 的值不存在", id)
			}
		}

		out[name] = value
	}

	return out, nil
}

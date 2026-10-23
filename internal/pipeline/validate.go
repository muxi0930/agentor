package pipeline

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"agentor/internal/constant"
	pipelinemodel "agentor/internal/pipeline/model"
)

// GetActions 读取静态动作契约与固定业务分支
func (s *Service) GetActions() []pipelinemodel.ActionSpec {
	success := []pipelinemodel.Branch{{Name: constant.PipelineBranchSuccess, Description: constant.PipelineBranchSuccessDescription}}
	return []pipelinemodel.ActionSpec{
		{Name: constant.PipelineActionCreateWorktree, Description: constant.PipelineCreateWorktreeDescription, Result: constant.PipelineWorktreeResult, Branches: success},
		{Name: constant.PipelineActionMergeWorktree, Description: constant.PipelineMergeWorktreeDescription, Result: constant.PipelineWorktreeResult, Branches: append(slices.Clone(success), pipelinemodel.Branch{Name: constant.PipelineBranchDirty, Description: constant.PipelineBranchDirtyDescription}, pipelinemodel.Branch{Name: constant.PipelineBranchDiverged, Description: constant.PipelineBranchDivergedDescription})},
		{Name: constant.PipelineActionRemoveWorktree, Description: constant.PipelineRemoveWorktreeDescription, Result: constant.PipelineWorktreeResult, Branches: success},
		{Name: constant.PipelineActionSnapshot, Description: constant.PipelineInspectActionDescription, Parameters: []pipelinemodel.Parameter{{Name: "versionId", Type: "string"}, {Name: "featureId", Type: "string"}}, Result: constant.PipelineInspectActionResult, Branches: success, RequiresProject: true},
		{Name: constant.PipelineActionReadKnowledge, Description: constant.PipelineKnowledgeActionDescription, Parameters: []pipelinemodel.Parameter{{Name: "documentId", Type: "string", Required: true}, {Name: "featureId", Type: "string"}}, Result: constant.PipelineKnowledgeActionResult, Branches: success, RequiresProject: true},
		{Name: constant.PipelineActionWriteReport, Description: constant.PipelineReportActionDescription, Parameters: []pipelinemodel.Parameter{{Name: "fileName", Type: "string", Required: true}, {Name: "content", Type: "any", Required: true}}, Result: constant.PipelineReportActionResult, Branches: success},
	}
}

// ValidateTask 校验独立或项目流水线的完整节点图
func (s *Service) ValidateTask(projectID, text string) (pipelinemodel.Task, error) {
	task, err := parseTask(projectID, text)
	if err != nil {
		return task, err
	}
	return task, s.validateTask(task)
}

// ValidateTemplate 校验不绑定项目与实际步骤的流水线模板
func (s *Service) ValidateTemplate(text string) (pipelinemodel.Task, error) {
	task, err := parseTask("", text)
	if err != nil {
		return task, err
	}
	return task, s.validateDefinition(task, true)
}

// parseTask 解析单个定义并固定所属项目
func parseTask(projectID, text string) (pipelinemodel.Task, error) {
	var task pipelinemodel.Task
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&task); err != nil {
		return task, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return task, constant.ErrPipelineMultipleTaskDefinitions
	}
	if task.ProjectID != "" && task.ProjectID != projectID {
		return task, constant.ErrPipelineTaskProjectChanged
	}
	task.ProjectID = projectID
	return task, nil
}

// validateTask 检查实例图与项目绑定
func (s *Service) validateTask(task pipelinemodel.Task) error {
	return s.validateDefinition(task, false)
}

// validateDefinition 检查节点、连线、业务分支和所有路径上的数据依赖
func (s *Service) validateDefinition(task pipelinemodel.Task, template bool) error {
	if task.ProjectID != "" {
		if _, err := s.project(task.ProjectID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(task.Name) == "" || len(task.Actions) == 0 {
		return constant.ErrPipelineTaskDefinitionRequired
	}
	fields := map[string]string{}
	for _, field := range task.Inputs {
		if strings.TrimSpace(field.Name) == "" || fields[field.Name] != "" {
			return constant.ErrPipelineInputNameInvalid
		}
		if !slices.Contains([]string{"string", "number", "boolean", "any"}, field.Type) {
			return fmt.Errorf("输入 %s 的类型不受支持", field.Name)
		}
		fields[field.Name] = field.Type
	}
	nodes := map[string]pipelinemodel.Action{}
	specs := map[string]pipelinemodel.ActionSpec{}
	for _, spec := range s.GetActions() {
		specs[spec.Name] = spec
	}
	for _, node := range task.Actions {
		if strings.TrimSpace(node.ID) == "" || nodes[node.ID].ID != "" {
			return constant.ErrPipelineActionIDInvalid
		}
		nodes[node.ID] = node
		if node.Kind != constant.PipelineKindAgent && (node.MaxRetries != 0 || node.FailureTarget != "" || node.StepID != "" || node.StepSlot != "") {
			return fmt.Errorf("节点 %s：只有 Agent 节点可以设置重试、失败处理或绑定步骤", node.ID)
		}
		switch node.Kind {
		case constant.PipelineKindStatic:
			spec, ok := specs[node.Type]
			if !ok {
				return fmt.Errorf("不支持静态动作 %s", node.Type)
			}
			if !template && spec.RequiresProject && task.ProjectID == "" {
				return fmt.Errorf("动作 %s 需要项目，请在项目内应用模板", node.Type)
			}

			for _, parameter := range spec.Parameters {
				if kind, id, path, ok := reference(node.Params[parameter.Name]); ok && kind == "input" && path == "" && fields[id] != "any" && parameter.Type != "any" && fields[id] != "" && fields[id] != parameter.Type {
					return fmt.Errorf("节点 %s 参数 %s 的输入引用类型不匹配", node.ID, parameter.Name)
				}
			}
			if err := validateParameters(spec, node.Params, true); err != nil {
				return fmt.Errorf("节点 %s：%w", node.ID, err)
			}
			if len(node.Branches) != len(spec.Branches) {
				return fmt.Errorf("节点 %s 的分支必须遵循动作契约", node.ID)
			}
			for _, branch := range node.Branches {
				if !slices.ContainsFunc(spec.Branches, func(b pipelinemodel.Branch) bool { return b.Name == branch.Name }) {
					return fmt.Errorf("动作 %s 不支持分支 %s", node.Type, branch.Name)
				}
			}
		case constant.PipelineKindAgent:
			if (node.Type != constant.PipelineActionAgent && node.Type != constant.PipelineActionResolveConflicts) || strings.TrimSpace(node.Prompt) == "" || strings.TrimSpace(node.Criteria) == "" {
				return fmt.Errorf("Agent 节点 %s 必须填写提示词和验收标准", node.ID)
			}
			if node.MaxRetries < 0 || node.MaxRetries > constant.PipelineAgentMaxRetries {
				return fmt.Errorf("Agent 节点 %s 重试次数需在 0 至 %d 之间", node.ID, constant.PipelineAgentMaxRetries)
			}
			if node.TimeoutSeconds < 1 || int64(node.TimeoutSeconds) > int64((1<<63-1)/int64(time.Second)) {
				return fmt.Errorf("Agent 节点 %s 的超时秒数超出有效范围", node.ID)
			}
			if node.Permission != constant.AppDataPermissionReadOnly && node.Permission != constant.AppDataPermissionWorkspaceWrite && node.Permission != constant.AppDataPermissionDangerFullAccess {
				return fmt.Errorf("Agent 节点 %s 的执行权限不受支持", node.ID)
			}
			if len(node.Branches) == 0 {
				return fmt.Errorf("Agent 节点 %s 至少声明一个业务分支", node.ID)
			}
			if template && node.StepID != "" {
				return fmt.Errorf("模板节点 %s 应配置步骤插槽，不能绑定实际步骤", node.ID)
			}
			if !template && node.StepSlot != "" && node.StepID == "" {
				return fmt.Errorf("请绑定步骤插槽 %s", node.StepSlot)
			}
			if node.StepID != "" {
				ctx, err := s.work.GetStepContext(node.StepID)
				if err != nil {
					return err
				}
				if ctx.ProjectID != task.ProjectID {
					return constant.ErrPipelineStepOutsideProject
				}
			}
		case constant.PipelineKindEnd:
			if node.Type != constant.PipelineEndSuccess && node.Type != constant.PipelineEndFailure {
				return fmt.Errorf("终点 %s 需指定成功或失败", node.ID)
			}
			if len(node.Branches) > 0 || len(node.Params) > 0 {
				return fmt.Errorf("终点 %s 不可配置输入或后续节点", node.ID)
			}
		default:
			return fmt.Errorf("节点 %s 的类型不受支持", node.ID)
		}
		names := map[string]bool{}
		for _, branch := range node.Branches {
			if strings.TrimSpace(branch.Name) == "" || branch.Name == "__failure" || names[branch.Name] {
				return fmt.Errorf("节点 %s 分支名称为空、保留名称或重复", node.ID)
			}
			names[branch.Name] = true
		}
	}
	if nodes[task.Entry].ID == "" {
		return fmt.Errorf("入口节点 %s 不存在", task.Entry)
	}
	predecessors := map[string][]string{}
	for _, node := range task.Actions {
		targets := []string{node.FailureTarget}
		for _, b := range node.Branches {
			targets = append(targets, b.Target)
		}
		for _, target := range targets {
			if target == "" {
				continue
			}
			if nodes[target].ID == "" {
				return fmt.Errorf("节点 %s 的目标 %s 不存在", node.ID, target)
			}
			predecessors[target] = append(predecessors[target], node.ID)
		}
	}
	// 后序遍历保证无环，逆序用于计算每个节点必经的前驱
	colors := map[string]int{}
	order := []string{}
	var visit func(string) error
	visit = func(id string) error {
		if colors[id] == 1 {
			return fmt.Errorf("节点图包含循环：%s；请使用 Agent 节点的有限重试", id)
		}
		if colors[id] == 2 {
			return nil
		}
		colors[id] = 1
		node := nodes[id]
		targets := []string{node.FailureTarget}
		for _, b := range node.Branches {
			targets = append(targets, b.Target)
		}
		for _, target := range targets {
			if target != "" {
				if err := visit(target); err != nil {
					return err
				}
			}
		}
		colors[id] = 2
		order = append(order, id)
		return nil
	}
	if err := visit(task.Entry); err != nil {
		return err
	}
	if len(order) != len(nodes) {
		return constant.ErrPipelineUnreachableNode
	}
	slices.Reverse(order)
	dominates := map[string]map[string]bool{}
	for _, id := range order {
		common := map[string]bool{}
		for i, parent := range predecessors[id] {
			if i == 0 {
				for ancestor := range dominates[parent] {
					common[ancestor] = true
				}
				common[parent] = true
			} else {
				for ancestor := range common {
					if ancestor != parent && !dominates[parent][ancestor] {
						delete(common, ancestor)
					}
				}
			}
		}
		dominates[id] = common
		if err := walkReferences(nodes[id].Params, func(kind, ref string) error {
			if kind == "input" {
				if fields[ref] == "" {
					return fmt.Errorf("输入 %s 不存在", ref)
				}
			} else if !common[ref] {
				return fmt.Errorf("结果 %s 不是节点 %s 在所有路径上的前序节点", ref, id)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// validateParameters 按静态契约检查必填字段和解析后的值类型
func validateParameters(spec pipelinemodel.ActionSpec, params map[string]any, allowRefs bool) error {
	known := map[string]pipelinemodel.Parameter{}
	for _, p := range spec.Parameters {
		known[p.Name] = p
		if _, ok := params[p.Name]; p.Required && !ok {
			return fmt.Errorf("缺少参数 %s", p.Name)
		}
	}
	for name, value := range params {
		p, ok := known[name]
		if !ok {
			return fmt.Errorf("未知参数 %s", name)
		}
		if _, _, _, ref := reference(value); allowRefs && ref {
			continue
		}
		if p.Type != "any" && valueType(value) != p.Type {
			return fmt.Errorf("参数 %s 应为 %s", name, p.Type)
		}
	}
	if file, ok := params["fileName"].(string); ok {
		return reportName(file)
	}
	return nil
}

// valueType 判断 JSON 参数类型
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
	case []any:
		return "array"
	case nil:
		return "null"
	}
	return "invalid"
}

// reference 识别完整值引用，path 是以点分隔的对象字段路径
func reference(value any) (string, string, string, bool) {
	object, ok := value.(map[string]any)
	if !ok || len(object) < 1 || len(object) > 2 {
		return "", "", "", false
	}
	path, _ := object["path"].(string)
	for _, kind := range []string{"input", "result"} {
		if id, ok := object[kind].(string); ok && (len(object) == 1 || object["path"] != nil) {
			return kind, id, path, true
		}
	}
	return "", "", "", false
}

// walkReferences 递归检查嵌套 JSON 中的依赖
func walkReferences(value any, check func(string, string) error) error {
	if kind, id, _, ok := reference(value); ok {
		return check(kind, id)
	}
	switch v := value.(type) {
	case map[string]any:
		for _, item := range v {
			if err := walkReferences(item, check); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := walkReferences(item, check); err != nil {
				return err
			}
		}
	}
	return nil
}

// reportName 校验报告使用单个 Markdown 文件名
func reportName(name string) error {
	if name == "" || strings.ContainsAny(name, "/\\") || filepath.Base(name) != name || strings.ToLower(filepath.Ext(name)) != ".md" {
		return constant.ErrPipelineReportFilenameInvalid
	}
	return nil
}

// validateInputs 校验运行输入的必填项与字段类型
func validateInputs(task pipelinemodel.Task, inputs map[string]any) error {
	fields := map[string]bool{}
	for _, field := range task.Inputs {
		fields[field.Name] = true
		value, ok := inputs[field.Name]
		if !ok {
			if field.Required {
				return fmt.Errorf("缺少输入 %s", field.Name)
			}
			continue
		}
		if field.Type != "any" && valueType(value) != field.Type {
			return fmt.Errorf("输入 %s 应为 %s", field.Name, field.Type)
		}
	}
	for name := range inputs {
		if !fields[name] {
			return fmt.Errorf("未知输入 %s", name)
		}
	}
	return nil
}

// resolveParams 将嵌套输入和节点输出引用解析为本次实际参数
func resolveParams(params, inputs, results map[string]any) (map[string]any, error) {
	var resolve func(any) (any, error)
	resolve = func(value any) (any, error) {
		if kind, id, path, ok := reference(value); ok {
			source := inputs
			if kind == "result" {
				source = results
			}
			result, exists := source[id]
			if !exists {
				return nil, fmt.Errorf("引用 %s 的值不存在", id)
			}
			if path != "" {
				for _, part := range strings.Split(path, ".") {
					object, ok := result.(map[string]any)
					if !ok {
						return nil, fmt.Errorf("引用 %s.%s 不是对象路径", id, path)
					}
					result, exists = object[part]
					if !exists {
						return nil, fmt.Errorf("引用 %s.%s 的字段不存在", id, path)
					}
				}
			}
			return result, nil
		}
		switch v := value.(type) {
		case map[string]any:
			out := map[string]any{}
			for key, item := range v {
				next, err := resolve(item)
				if err != nil {
					return nil, err
				}
				out[key] = next
			}
			return out, nil
		case []any:
			out := make([]any, len(v))
			for i, item := range v {
				next, err := resolve(item)
				if err != nil {
					return nil, err
				}
				out[i] = next
			}
			return out, nil
		}
		return value, nil
	}
	value, err := resolve(params)
	if err != nil {
		return nil, err
	}
	return value.(map[string]any), nil
}

package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"unicode/utf8"
)

type ElicitationOption struct {
	Value string `json:"value"` // 选项值
	Label string `json:"label"` // 选项显示文字
}

type ElicitationField struct {
	Name        string              `json:"name"`        // 字段名称
	Type        string              `json:"type"`        // 类型标识
	Title       string              `json:"title"`       // 显示标题
	Description string              `json:"description"` // 说明文字
	Format      string              `json:"format"`      // 值格式约束
	Required    bool                `json:"required"`    // 必填字段
	Minimum     *float64            `json:"minimum"`     // 数值下限
	Maximum     *float64            `json:"maximum"`     // 数值上限
	MinLength   *int                `json:"minLength"`   // 文字长度下限
	MaxLength   *int                `json:"maxLength"`   // 文字长度上限
	MinItems    *int                `json:"minItems"`    // 选项数量下限
	MaxItems    *int                `json:"maxItems"`    // 选项数量上限
	DefaultJSON string              `json:"defaultJSON"` // 默认值的 JSON 表示
	Options     []ElicitationOption `json:"options"`     // 可用选项
}

// AnswerElicitation 校验并回复当前 MCP 问答请求
func (s *Service) AnswerElicitation(id, action, contentJSON string) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	s.mu.Unlock()
	if !found || approval.Method != "mcpServer/elicitation/request" {
		return errors.New("该请求不是 MCP 问答")
	}
	if !slices.Contains([]string{"accept", "decline", "cancel"}, action) {
		return errors.New("不支持的响应")
	}

	var content any
	if action == "accept" {
		if approval.Mode == "openai/userVerification" {
			return errors.New("当前桌面不支持设备身份验证，请在原服务完成验证或取消")
		}
		if approval.Mode != "url" {
			var values map[string]any
			if json.Unmarshal([]byte(contentJSON), &values) != nil || values == nil {
				return errors.New("请填写有效的 JSON 对象")
			}

			for _, field := range approval.Fields {
				if err := validateElicitationField(field, values[field.Name]); err != nil {
					return fmt.Errorf("%s：%w", field.Title, err)
				}
			}

			content = values
		}
	}
	return s.resolveApproval(id, map[string]any{"action": action, "content": content})
}

// elicitationFields 将 MCP 表单结构转换为可展示字段
func elicitationFields(raw json.RawMessage) []ElicitationField {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"` // 按名称索引的字段定义
		Required   []string                   `json:"required"`   // 必填字段
	}
	if json.Unmarshal(raw, &schema) != nil {
		return nil
	}

	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}

	sort.Strings(names)
	fields := []ElicitationField{}
	for _, name := range names {
		var property struct {
			ElicitationField                 // 当前字段定义
			Default          json.RawMessage `json:"default"` // 原始默认值
			Enum             []string        `json:"enum"`    // 允许的选项值
			OneOf            []struct {
				Const string `json:"const"` // 选项固定值
				Title string `json:"title"` // 显示标题
			} `json:"oneOf"` // 单项可选定义
			Items struct {
				Enum  []string `json:"enum"` // 允许的选项值
				AnyOf []struct {
					Const string `json:"const"` // 选项固定值
					Title string `json:"title"` // 显示标题
				} `json:"anyOf"` // 数组可选定义
			} `json:"items"` // 数组元素约束
		}
		if json.Unmarshal(schema.Properties[name], &property) != nil {
			return nil
		}

		field := property.ElicitationField
		if !slices.Contains([]string{"string", "number", "integer", "boolean", "array"}, field.Type) {
			return nil
		}

		field.Name, field.Required, field.DefaultJSON = name, slices.Contains(schema.Required, name), string(property.Default)
		if field.Title == "" {
			field.Title = name
		}
		options := property.Enum
		if field.Type == "array" {
			options = property.Items.Enum
		}
		for _, value := range options {
			field.Options = append(field.Options, ElicitationOption{Value: value, Label: value})
		}

		for _, option := range property.OneOf {
			field.Options = append(field.Options, ElicitationOption{Value: option.Const, Label: option.Title})
		}

		for _, option := range property.Items.AnyOf {
			field.Options = append(field.Options, ElicitationOption{Value: option.Const, Label: option.Title})
		}

		if field.Type == "array" && len(field.Options) == 0 {
			return nil
		}

		fields = append(fields, field)
	}

	return fields
}

// validateElicitationField 按字段定义校验 MCP 表单答案
func validateElicitationField(field ElicitationField, value any) error {
	if value == nil {
		if field.Required {
			return errors.New("请填写此项")
		}

		return nil
	}

	validOption := func(value string) bool {
		for _, option := range field.Options {
			if option.Value == value {
				return true
			}
		}

		return len(field.Options) == 0
	}
	switch field.Type {
	case "string":
		text, ok := value.(string)
		if !ok || !validOption(text) {
			return errors.New("请填写有效文字或选项")
		}
		length := utf8.RuneCountInString(text)
		if field.MinLength != nil && length < *field.MinLength || field.MaxLength != nil && length > *field.MaxLength {
			return errors.New("文字长度不符合要求")
		}
	case "number", "integer":
		number, ok := value.(float64)
		if !ok || field.Type == "integer" && math.Trunc(number) != number {
			return errors.New("请填写有效数字")
		}
		if field.Minimum != nil && number < *field.Minimum || field.Maximum != nil && number > *field.Maximum {
			return errors.New("数字超出允许范围")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("请选择是或否")
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return errors.New("请选择选项")
		}
		if field.MinItems != nil && len(items) < *field.MinItems || field.MaxItems != nil && len(items) > *field.MaxItems {
			return errors.New("选择数量不符合要求")
		}
		for _, item := range items {
			if text, ok := item.(string); !ok || !validOption(text) {
				return errors.New("无效选项")
			}
		}
	}

	return nil
}

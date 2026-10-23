package codex

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"unicode/utf8"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

// AnswerElicitation 校验并回复当前 MCP 问答请求
func (s *Service) AnswerElicitation(id, action, contentJSON string) error {
	s.mu.Lock()
	approval, found := s.approvals[id]
	s.mu.Unlock()
	if !found || approval.Method != constant.CodexMethodMcpServerElicitationRequest {
		return constant.ErrCodexElicitationRequired
	}
	if !slices.Contains([]string{constant.CodexApprovalAccept, constant.CodexApprovalDecline, constant.CodexApprovalCancel}, action) {
		return constant.ErrCodexElicitationActionUnsupported
	}

	var content any
	if action == constant.CodexApprovalAccept {
		if approval.Mode == "openai/userVerification" {
			return constant.ErrCodexDeviceAuthenticationUnsupported
		}
		if approval.Mode != "url" {
			var values map[string]any
			if json.Unmarshal([]byte(contentJSON), &values) != nil || values == nil {
				return constant.ErrCodexJSONObjectRequired
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
func elicitationFields(raw json.RawMessage) []codexmodel.ElicitationField {
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
	fields := []codexmodel.ElicitationField{}
	for _, name := range names {
		var property struct {
			codexmodel.ElicitationField                 // 当前字段定义
			Default                     json.RawMessage `json:"default"` // 原始默认值
			Enum                        []string        `json:"enum"`    // 允许的选项值
			OneOf                       []struct {
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
			field.Options = append(field.Options, codexmodel.ElicitationOption{Value: value, Label: value})
		}

		for _, option := range property.OneOf {
			field.Options = append(field.Options, codexmodel.ElicitationOption{Value: option.Const, Label: option.Title})
		}

		for _, option := range property.Items.AnyOf {
			field.Options = append(field.Options, codexmodel.ElicitationOption{Value: option.Const, Label: option.Title})
		}

		if field.Type == "array" && len(field.Options) == 0 {
			return nil
		}

		fields = append(fields, field)
	}

	return fields
}

// validateElicitationField 按字段定义校验 MCP 表单答案
func validateElicitationField(field codexmodel.ElicitationField, value any) error {
	if value == nil {
		if field.Required {
			return constant.ErrCodexFieldRequired
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
			return constant.ErrCodexTextInvalid
		}
		length := utf8.RuneCountInString(text)
		if field.MinLength != nil && length < *field.MinLength || field.MaxLength != nil && length > *field.MaxLength {
			return constant.ErrCodexTextLengthInvalid
		}
	case "number", "integer":
		number, ok := value.(float64)
		if !ok || field.Type == "integer" && math.Trunc(number) != number {
			return constant.ErrCodexNumberInvalid
		}
		if field.Minimum != nil && number < *field.Minimum || field.Maximum != nil && number > *field.Maximum {
			return constant.ErrCodexNumberRangeInvalid
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return constant.ErrCodexBooleanRequired
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return constant.ErrCodexOptionRequired
		}
		if field.MinItems != nil && len(items) < *field.MinItems || field.MaxItems != nil && len(items) > *field.MaxItems {
			return constant.ErrCodexSelectionCountInvalid
		}
		for _, item := range items {
			if text, ok := item.(string); !ok || !validOption(text) {
				return constant.ErrCodexOptionInvalid
			}
		}
	}

	return nil
}

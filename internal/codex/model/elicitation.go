package model

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

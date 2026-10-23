package model

import (
	"encoding/json"
)

type DynamicToolSpec struct {
	Type        string         `json:"type"`        // 动态工具类型
	Name        string         `json:"name"`        // 动态工具名称
	Description string         `json:"description"` // 工具用途说明
	InputSchema map[string]any `json:"inputSchema"` // 宿主工具的输入结构
}

type ToolCall struct {
	ThreadID  string          `json:"threadId"`  // 所属会话标识
	TurnID    string          `json:"turnId"`    // 所属轮次标识
	CallID    string          `json:"callId"`    // 工具调用标识
	Namespace string          `json:"namespace"` // 工具命名空间
	Tool      string          `json:"tool"`      // 宿主工具名称
	Arguments json.RawMessage `json:"arguments"` // 原始工具参数
}

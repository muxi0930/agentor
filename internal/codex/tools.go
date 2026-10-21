package codex

import (
	"encoding/json"
	"errors"
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

// handleToolRequest 异步执行宿主工具并回复原始请求
func (s *Service) handleToolRequest(generation uint64, message packet, c *client) {
	var call ToolCall
	if err := json.Unmarshal(message.Params, &call); err != nil {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
		return
	}

	s.mu.Lock()
	handler := s.toolHandler
	s.mu.Unlock()

	go func() {
		var result any
		err := errors.New("当前会话没有可用的宿主工具")
		if handler != nil {
			result, err = handler(call)
		}

		text := ""
		if err != nil {
			text = err.Error()
		} else {
			var raw []byte
			raw, err = json.Marshal(result)
			if err != nil {
				text = err.Error()
			} else {
				text = string(raw)
			}
		}

		s.mu.Lock()
		current := s.status.Generation == generation && s.client == c
		s.mu.Unlock()
		if current {
			c.write(map[string]any{
				"id": message.ID,
				"result": map[string]any{
					"success":      err == nil,
					"contentItems": []map[string]string{{"type": "inputText", "text": text}},
				},
			})
		}
	}()
}

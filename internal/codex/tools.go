package codex

import (
	"encoding/json"

	codexmodel "agentor/internal/codex/model"
	"agentor/internal/constant"
)

// handleToolRequest 异步执行宿主工具并回复原始请求
func (s *Service) handleToolRequest(generation uint64, message packet, c *client) {
	var call codexmodel.ToolCall
	if err := json.Unmarshal(message.Params, &call); err != nil {
		c.write(packet{ID: message.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
		return
	}

	s.mu.Lock()
	handler := s.toolHandler
	s.mu.Unlock()

	go func() {
		var result any
		err := constant.ErrCodexToolHandlerUnavailable
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
					"contentItems": []map[string]string{{"type": constant.CodexItemInputText, "text": text}},
				},
			})
		}
	}()
}

package codex

import (
	"encoding/json"
	"errors"
)

type DynamicToolSpec struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type ToolCall struct {
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
	CallID    string          `json:"callId"`
	Namespace string          `json:"namespace"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

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
			c.write(map[string]any{"id": message.ID, "result": map[string]any{"success": err == nil, "contentItems": []map[string]string{{"type": "inputText", "text": text}}}})
		}
	}()
}

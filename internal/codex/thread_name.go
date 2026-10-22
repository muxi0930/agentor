package codex

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

type managedThreadName struct {
	mu     sync.Mutex // 串行处理同一会话的标题读取与更新
	prefix string     // 新建业务会话的角色前缀
}

// apply 沿用原生标题或预览，仅在存在有效标题时添加一次角色前缀
func (n *managedThreadName) apply(ctx context.Context, c *client, thread *Thread) error {
	title := strings.TrimSpace(thread.Name)
	if title == "" || title == n.prefix {
		title = strings.TrimSpace(thread.Preview)
	}
	if title == "" || strings.HasPrefix(title, n.prefix) {
		return nil
	}
	if c == nil {
		return errors.New("Codex 尚未就绪，请先连接")
	}

	name := n.prefix + " " + title
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := c.request(ctx, "thread/name/set", map[string]string{"threadId": thread.ID, "name": name}, nil); err != nil {
		return err
	}
	thread.Name = name
	return nil
}

// trackManagedThreadName 等待原生标题生成后异步补充前缀，避免阻塞协议响应读取
func (s *Service) trackManagedThreadName(generation uint64, message packet) {
	if message.Method != "thread/name/updated" && message.Method != "turn/completed" && message.Method != "item/completed" {
		return
	}
	var params struct {
		ThreadID string `json:"threadId"` // 所属会话标识
		Item     Item   `json:"item"`     // 上游消息节点
	}
	if json.Unmarshal(message.Params, &params) != nil || (message.Method == "item/completed" && params.Item.Type != "userMessage") {
		return
	}

	s.mu.Lock()
	n := s.threadNames[params.ThreadID]
	c := s.client
	current := s.status.Generation == generation && s.status.Phase == "ready"
	s.mu.Unlock()
	if n == nil || c == nil || !current {
		return
	}

	go func() {
		n.mu.Lock()
		defer n.mu.Unlock()

		s.mu.Lock()
		current := s.status.Generation == generation && s.client == c && !s.closing
		s.mu.Unlock()
		if !current {
			return
		}

		ctx, cancel := context.WithTimeout(s.ctx, 60*time.Second)
		defer cancel()
		var result struct {
			Thread Thread `json:"thread"` // 最新原生标题与预览
		}
		if c.request(ctx, "thread/read", map[string]string{"threadId": params.ThreadID}, &result) == nil {
			n.apply(ctx, c, &result.Thread)
		}
	}()
}

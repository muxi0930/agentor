package artifact

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"strings"
)

// CreateHTMLPreview 创建随消息视图挂载的临时预览，源码仍保留在 Codex 历史中
func (s *Service) CreateHTMLPreview(threadID, source string) (string, error) {
	if threadID == "" || strings.TrimSpace(source) == "" {
		return "", errors.New("预览需要会话和 HTML 内容")
	}

	id := rand.Text()
	s.mu.Lock()
	s.previews[id] = previewBridge + source
	s.mu.Unlock()
	return "/agentor/previews/" + id, nil
}

// ReleaseHTMLPreview 释放消息视图不再使用的临时预览
func (s *Service) ReleaseHTMLPreview(url string) {
	s.mu.Lock()
	delete(s.previews, strings.TrimPrefix(url, "/agentor/previews/"))
	s.mu.Unlock()
}

// servePreview 返回带沙箱约束的临时 HTML 预览
func (s *Service) servePreview(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	source, found := s.previews[strings.TrimPrefix(r.URL.Path, "/agentor/previews/")]
	s.mu.Unlock()
	if !found {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// 单独打开预览地址时仍保留与内嵌视图相同的沙箱边界
	w.Header().Set("Content-Security-Policy", "sandbox allow-scripts")
	io.WriteString(w, source)
}

const previewBridge = `<!doctype html><meta charset="utf-8"><script>
(() => {
  const send = value => parent.postMessage({channel:"agentor-preview", ...value}, "*");
  window.agentor = {followup: text => { if (typeof text === "string") send({type:"followup", text}); }};
  addEventListener("DOMContentLoaded", () => {
    const resize = () => send({type:"resize", height:document.documentElement.scrollHeight});
    new ResizeObserver(resize).observe(document.body); resize();
  });
  addEventListener("error", event => send({type:"error", text:event.message}));
})();
</script>`

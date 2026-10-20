package artifact

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Preview resources exist only while their message view is mounted. The original
// source remains in Codex history; no second HTML history is stored here.
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

func (s *Service) ReleaseHTMLPreview(url string) {
	s.mu.Lock()
	delete(s.previews, strings.TrimPrefix(url, "/agentor/previews/"))
	s.mu.Unlock()
}

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
	// Also enforce the iframe boundary if a preview URL is opened on its own.
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

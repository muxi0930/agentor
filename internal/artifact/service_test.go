package artifact

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestAttachmentOwnershipAndResourceRouting 验证附件归属及资源路由范围
func TestAttachmentOwnershipAndResourceRouting(t *testing.T) {
	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{R: 20, G: 110, B: 220, A: 255})
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "input.png")
	if err := os.WriteFile(source, imageData.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file, err := s.ImportFile("thread-a", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveInputs("thread-b", []string{file.ID}); err == nil {
		t.Fatal("attachment leaked into another thread")
	}
	if err := s.BindTurn("thread-a", "turn-a", "message-a", []string{file.ID}); err != nil {
		t.Fatal(err)
	}
	if s.GetAttachments("thread-a")[0].MessageID != "message-a" {
		t.Fatal("message ownership missing")
	}
	handler := s.Middleware(http.NotFoundHandler())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", file.URL, nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || !bytes.Equal(response.Body.Bytes(), imageData.Bytes()) {
		t.Fatalf("wrong resource response: %d", response.Code)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/agentor/attachments/../settings.json", nil))
	if response.Code != http.StatusNotFound {
		t.Fatal("resource path escaped the attachment registry")
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatal("original file was changed")
	}
}

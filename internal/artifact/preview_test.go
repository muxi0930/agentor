package artifact

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreviewIsIsolatedAndReleased(t *testing.T) {
	s, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	url, err := s.CreateHTMLPreview("thread", `<button onclick="agentor.followup('next')">Next</button>`)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.Middleware(http.NotFoundHandler())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", url, nil))
	if response.Code != 200 || response.Header().Get("Content-Security-Policy") != "sandbox allow-scripts" {
		t.Fatal("preview not sandboxed", response.Code, response.Header())
	}
	if !strings.Contains(response.Body.String(), "Next</button>") || strings.Contains(response.Body.String(), "wails/runtime") {
		t.Fatal("incorrect preview source")
	}
	s.ReleaseHTMLPreview(url)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", url, nil))
	if response.Code != 404 {
		t.Fatal("released resource is still available")
	}
}

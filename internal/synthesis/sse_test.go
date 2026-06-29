package synthesis

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// flushRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

// nonFlushWriter wraps an http.ResponseWriter but deliberately does NOT implement Flusher.
// By embedding http.ResponseWriter (not *httptest.ResponseRecorder), we avoid
// inheriting the Flush method that newer Go versions added to ResponseRecorder.
type nonFlushWriter struct {
	http.ResponseWriter
}

func TestSSEWriter_Write(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	sse, err := NewSSEWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := sse.Write("delta", map[string]string{"text": "hi"}); err != nil {
		t.Fatal(err)
	}
	if err := sse.Write("complete", map[string]bool{"done": true}); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, "event: delta") {
		t.Errorf("missing delta event: %s", body)
	}
	if !strings.Contains(body, `"text":"hi"`) {
		t.Errorf("missing data: %s", body)
	}
	if !strings.Contains(body, "event: complete") {
		t.Errorf("missing complete event: %s", body)
	}
}

func TestSSEWriter_NotSupported(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &nonFlushWriter{ResponseWriter: rec}
	_, err := NewSSEWriter(w)
	if err == nil {
		t.Fatal("expected error for non-flusher")
	}
}

func TestSSEWriter_Headers(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	_, err := NewSSEWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	h := w.Header()
	if h.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", h.Get("Content-Type"))
	}
	if h.Get("Cache-Control") != "no-cache" {
		t.Errorf("expected no-cache, got %s", h.Get("Cache-Control"))
	}
	if h.Get("Connection") != "keep-alive" {
		t.Errorf("expected keep-alive, got %s", h.Get("Connection"))
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

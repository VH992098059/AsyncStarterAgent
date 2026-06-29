package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/synthesis"
)

func TestDraftStreamHandler_SSEUnsupported(t *testing.T) {
	// Test the SSE unsupported path directly: when the underlying writer
	// does not implement http.Flusher, NewSSEWriter returns an error.
	// We cannot easily simulate this through gin.ServeHTTP because gin's
	// test writer always implements Flusher. Instead, test the handler's
	// error path by calling the SSEWriter constructor with a non-flusher.
	w := httptest.NewRecorder()
	// Strip Flusher by wrapping with a plain http.ResponseWriter embed.
	type noFlush struct{ http.ResponseWriter }
	_, err := synthesis.NewSSEWriter(noFlush{ResponseWriter: w})
	if err == nil {
		t.Fatal("expected error for non-flusher writer")
	}
}

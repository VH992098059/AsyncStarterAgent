package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asyncstarter/agent/internal/synthesis"
)

func TestDraftStreamHandler_A2UIUnsupported(t *testing.T) {
	w := httptest.NewRecorder()
	type noFlush struct{ http.ResponseWriter }
	_, err := synthesis.NewA2UIWriter(noFlush{ResponseWriter: w})
	if err == nil {
		t.Fatal("expected error for non-flusher writer")
	}
}

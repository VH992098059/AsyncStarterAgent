package synthesis

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestA2UIWriter_Headers(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	_, err := NewA2UIWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	h := w.Header()
	if h.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", h.Get("Content-Type"))
	}
}

func TestA2UIWriter_WriteProgress(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, err := NewA2UIWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := aw.WriteProgress("retrieve", "running"); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"progress"`) {
		t.Errorf("missing type:progress in: %s", body)
	}
	if !strings.Contains(body, `"phase":"retrieve"`) {
		t.Errorf("missing phase:retrieve in: %s", body)
	}
	if !strings.Contains(body, `"status":"running"`) {
		t.Errorf("missing status:running in: %s", body)
	}
}

func TestA2UIWriter_WriteDelta(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, err := NewA2UIWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := aw.WriteDelta("hello"); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"delta"`) {
		t.Errorf("missing type:delta in: %s", body)
	}
	if !strings.Contains(body, `"text":"hello"`) {
		t.Errorf("missing text:hello in: %s", body)
	}
}

func TestA2UIWriter_WriteComplete(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, err := NewA2UIWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	marks := []Mark{{ID: "m1", Hint: "check", Position: 5, Resolved: false}}
	if err := aw.WriteComplete(marks, 0.9); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"complete"`) {
		t.Errorf("missing type:complete in: %s", body)
	}
	if !strings.Contains(body, `"completeness":0.9`) {
		t.Errorf("missing completeness in: %s", body)
	}
}

func TestA2UIWriter_WriteError(t *testing.T) {
	w := &flushRecorder{httptest.NewRecorder()}
	aw, err := NewA2UIWriter(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := aw.WriteError("something failed"); err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"error"`) {
		t.Errorf("missing type:error in: %s", body)
	}
}

func TestA2UIWriter_NotSupported(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &nonFlushWriter{ResponseWriter: rec}
	_, err := NewA2UIWriter(w)
	if err == nil {
		t.Fatal("expected error for non-flusher")
	}
}

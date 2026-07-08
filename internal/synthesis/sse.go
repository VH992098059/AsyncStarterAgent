package synthesis

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter writes Server-Sent Events to an HTTP response (FR-C05, NFR-04).
//
// Deprecated: use A2UIWriter instead. SSEWriter 使用 `event:` 前缀的 SSE 格式，
// 而 A2UIWriter 使用无 event 前缀的 `data: {json}\n\n` 格式，前端 EventSource
// onmessage 统一监听。新代码请用 internal/synthesis/a2ui.go 中的 A2UIWriter。
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates an SSEWriter. Returns error if the ResponseWriter doesn't support flushing.
//
// Deprecated: use NewA2UIWriter instead.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	return &SSEWriter{w: w, flusher: f}, nil
}

// Write sends an SSE event with the given event type and JSON-encoded data.
// Event types: delta / complete / error / mark
func (s *SSEWriter) Write(eventType string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", eventType, body); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

package synthesis

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSEWriter writes Server-Sent Events to an HTTP response (FR-C05, NFR-04).
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates an SSEWriter. Returns error if the ResponseWriter doesn't support flushing.
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

package synthesis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// A2UIWriter is not safe for concurrent use.
type A2UIWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewA2UIWriter(w http.ResponseWriter) (*A2UIWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	return &A2UIWriter{w: w, flusher: f}, nil
}

func (a *A2UIWriter) write(payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(a.w, "data: %s\n\n", body); err != nil {
		return err
	}
	a.flusher.Flush()
	return nil
}

func (a *A2UIWriter) WriteProgress(phase, status string) error {
	return a.write(map[string]string{
		"type":   "progress",
		"phase":  phase,
		"status": status,
	})
}

func (a *A2UIWriter) WriteDelta(text string) error {
	return a.write(map[string]string{
		"type": "delta",
		"text": text,
	})
}

func (a *A2UIWriter) WriteComplete(marks []Mark, completeness float32) error {
	return a.write(map[string]interface{}{
		"type":         "complete",
		"marks":        marks,
		"completeness": completeness,
	})
}

func (a *A2UIWriter) WriteError(message string) error {
	return a.write(map[string]string{
		"type":    "error",
		"message": message,
	})
}

// WriteChatDelta 写入 chat 流式 token，附 message_id 让前端定位对应的 assistant 气泡。
// 区别于 WriteDelta（draft 流式），chat 事件独立 type 避免前端混用。
func (a *A2UIWriter) WriteChatDelta(messageID, text string) error {
	return a.write(map[string]string{
		"type":       "chat_delta",
		"message_id": messageID,
		"text":       text,
	})
}

// WriteChatReasoning 写入 chat 推理内容（thinking），前端可折叠显示。
func (a *A2UIWriter) WriteChatReasoning(messageID, text string) error {
	return a.write(map[string]string{
		"type":       "chat_reasoning",
		"message_id": messageID,
		"text":       text,
	})
}

// WriteChatComplete 标记某条 assistant 消息流式结束。
func (a *A2UIWriter) WriteChatComplete(messageID string) error {
	return a.write(map[string]string{
		"type":       "chat_complete",
		"message_id": messageID,
	})
}

// WriteChatError 标记某条 assistant 消息流式失败。
func (a *A2UIWriter) WriteChatError(messageID, message string) error {
	return a.write(map[string]string{
		"type":       "chat_error",
		"message_id": messageID,
		"message":    message,
	})
}

type a2uiCtxKey struct{}

func WithA2UIWriter(ctx context.Context, w *A2UIWriter) context.Context {
	return context.WithValue(ctx, a2uiCtxKey{}, w)
}

func writerFromCtx(ctx context.Context) *A2UIWriter {
	v, _ := ctx.Value(a2uiCtxKey{}).(*A2UIWriter)
	return v
}

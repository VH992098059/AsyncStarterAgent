package synthesis

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestService_StreamMarkdown(t *testing.T) {
	rec := &flushRecorder{httptest.NewRecorder()}
	aw, err := NewA2UIWriter(rec)
	if err != nil {
		t.Fatal(err)
	}

	svc := &Service{pool: nil}
	md := "这是测试内容"
	if err := svc.streamMarkdown(context.Background(), md, aw); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"delta"`) {
		t.Errorf("missing delta events in: %s", body)
	}
	if !strings.Contains(body, `"type":"complete"`) {
		t.Errorf("missing complete event in: %s", body)
	}
}

func TestService_StreamMarkdown_MultiByte(t *testing.T) {
	rec := &flushRecorder{httptest.NewRecorder()}
	aw, err := NewA2UIWriter(rec)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{pool: nil}
	// 50 个汉字，每个3字节=150字节，超过 chunkSize=30 runes
	md := "这是一段较长的中文测试内容用于验证多字节字符切割不会产生乱码这里需要超过三十个汉字才能触发分块"
	if err := svc.streamMarkdown(context.Background(), md, aw); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	// 验证有多个 delta 事件
	count := strings.Count(body, `"type":"delta"`)
	if count < 2 {
		t.Errorf("expected multiple delta chunks, got %d in: %s", count, body)
	}
	// 验证 complete 事件存在
	if !strings.Contains(body, `"type":"complete"`) {
		t.Errorf("missing complete event in: %s", body)
	}
}


package synthesis

import (
	"context"
	"errors"
	"testing"

	"github.com/asyncstarter/agent/internal/harvesting"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type mockLLM struct {
	chunks []string
	fail   bool
}

func (m *mockLLM) Chat(_ context.Context, _ ChatRequest) (<-chan ChatChunk, error) {
	if m.fail {
		return nil, errors.New("llm unavailable")
	}
	out := make(chan ChatChunk, len(m.chunks)+1)
	for _, c := range m.chunks {
		out <- ChatChunk{Content: c}
	}
	out <- ChatChunk{Done: true}
	close(out)
	return out, nil
}

func TestLLM_Chat_Stream(t *testing.T) {
	cli := &mockLLM{chunks: []string{"Hello", " world", "!"}}
	ch, err := cli.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	got := ""
	for c := range ch {
		if c.Err != nil {
			t.Fatal(c.Err)
		}
		got += c.Content
		if c.Done {
			break
		}
	}
	if got != "Hello world!" {
		t.Errorf("got: %q", got)
	}
}

func TestPromptBuilder(t *testing.T) {
	pb := PromptBuilder{}
	msgs := pb.Build("weekly_report", "draft", []harvesting.ContextItem{{Title: "task A", Source: "github"}})
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first role: %s", msgs[0].Role)
	}
	if msgs[1].Role != "user" {
		t.Errorf("second role: %s", msgs[1].Role)
	}
	// Verify weekly_report specific system prompt
	if !contains(msgs[0].Content, "周报") {
		t.Errorf("system prompt should mention 周报 for weekly_report type")
	}
}

func TestLLM_Chat_Error(t *testing.T) {
	cli := &mockLLM{fail: true}
	_, err := cli.Chat(context.Background(), ChatRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

// mockChatModel implements model.BaseChatModel for testing EinoLLM.
type mockChatModel struct {
	messages []*schema.Message
	fail     bool
}

func (m *mockChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if m.fail {
		return nil, errors.New("model error")
	}
	if len(m.messages) > 0 {
		return m.messages[0], nil
	}
	return &schema.Message{}, nil
}

func (m *mockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m.fail {
		return nil, errors.New("model stream error")
	}
	return schema.StreamReaderFromArray(m.messages), nil
}

func TestEinoLLM_Chat(t *testing.T) {
	msgs := []*schema.Message{
		{Role: schema.Assistant, Content: "Hello"},
		{Role: schema.Assistant, Content: " world"},
		{Role: schema.Assistant, Content: "!"},
	}
	chatModel := &mockChatModel{messages: msgs}
	einoLLM := NewEinoLLM(chatModel)

	ch, err := einoLLM.Chat(context.Background(), ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "test"},
			{Role: "user", Content: "hello"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := ""
	for c := range ch {
		if c.Err != nil {
			t.Fatal(c.Err)
		}
		got += c.Content
		if c.Done {
			break
		}
	}
	if got != "Hello world!" {
		t.Errorf("got: %q", got)
	}
}

func TestEinoLLM_Chat_StreamError(t *testing.T) {
	chatModel := &mockChatModel{fail: true}
	einoLLM := NewEinoLLM(chatModel)

	_, err := einoLLM.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error from stream")
	}
}



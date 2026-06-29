package synthesis

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/asyncstarter/agent/internal/harvesting"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Message represents a chat message (FR-C03).
type Message struct {
	Role    string // system / user / assistant
	Content string
}

// ChatRequest is the input for LLMClient.Chat (FR-C03).
type ChatRequest struct {
	Messages    []Message
	Temperature float32
	MaxTokens   int
}

// ChatChunk is a streaming chunk from the LLM (FR-C03).
type ChatChunk struct {
	Content string
	Done    bool
	Err     error
}

// LLMClient is the interface for LLM chat with streaming (FR-C03).
type LLMClient interface {
	Chat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
}

// PromptBuilder constructs LLM prompts from task data (FR-C03).
type PromptBuilder struct{}

// Build creates chat messages for the LLM from task type, draft, and context items.
func (PromptBuilder) Build(taskType, draft string, items []harvesting.ContextItem) []Message {
	itemSummary := ""
	for _, it := range items {
		itemSummary += "- " + it.Title + " (" + it.Source + ")\n"
	}

	system := "你是一个专业的文档润色助手。请保持原意，修正语法，提升可读性。对不确定的内容使用 [待补充:说明] 标记。"
	if taskType == "weekly_report" {
		system += "这是一份周报，重点突出本周完成的工作。"
	}
	return []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: "素材：\n" + itemSummary + "\n\n草稿：\n" + draft + "\n\n请输出润色后的 markdown。"},
	}
}

// EinoLLM wraps Eino's model.BaseChatModel to implement LLMClient.
// Per user decision, all LLM calls use Eino framework (no hand-written HTTP client).
type EinoLLM struct {
	chatModel model.BaseChatModel
}

// NewEinoLLM creates an EinoLLM from an Eino ChatModel instance.
func NewEinoLLM(chatModel model.BaseChatModel) *EinoLLM {
	return &EinoLLM{chatModel: chatModel}
}

// Chat calls the LLM via Eino and returns streaming chunks (FR-C03).
func (e *EinoLLM) Chat(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error) {
	messages := make([]*schema.Message, len(req.Messages))
	for i, m := range req.Messages {
		switch m.Role {
		case "system":
			messages[i] = schema.SystemMessage(m.Content)
		case "assistant":
			messages[i] = schema.AssistantMessage(m.Content, nil)
		default:
			messages[i] = schema.UserMessage(m.Content)
		}
	}

	// Build Eino options from ChatRequest
	var opts []model.Option
	if req.Temperature > 0 {
		opts = append(opts, model.WithTemperature(req.Temperature))
	}
	if req.MaxTokens > 0 {
		opts = append(opts, model.WithMaxTokens(req.MaxTokens))
	}

	stream, err := e.chatModel.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("eino llm stream: %w", err)
	}

	out := make(chan ChatChunk)
	go e.readStream(ctx, stream, out)
	return out, nil
}

// readStream reads from Eino StreamReader and sends ChatChunk to the output channel.
// Uses sendChunk to avoid goroutine leaks when the consumer stops reading (e.g. ctx cancelled).
func (e *EinoLLM) readStream(ctx context.Context, stream *schema.StreamReader[*schema.Message], out chan<- ChatChunk) {
	defer close(out)
	defer stream.Close()

	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			e.sendChunk(ctx, out, ChatChunk{Done: true})
			return
		}
		if err != nil {
			e.sendChunk(ctx, out, ChatChunk{Err: err})
			return
		}
		if msg.Content != "" {
			e.sendChunk(ctx, out, ChatChunk{Content: msg.Content})
		}
	}
}

// sendChunk sends a ChatChunk to out, respecting context cancellation to prevent goroutine leaks.
func (e *EinoLLM) sendChunk(ctx context.Context, out chan<- ChatChunk, chunk ChatChunk) {
	select {
	case out <- chunk:
	case <-ctx.Done():
	}
}

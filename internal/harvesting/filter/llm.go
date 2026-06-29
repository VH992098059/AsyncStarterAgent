package filter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// LLMFilter uses a harvesting.NoiseClassifier to perform a second-pass noise
// filter on items that survived the rule-based filter.
type LLMFilter struct {
	classifier harvesting.NoiseClassifier
}

// NewLLMFilter creates an LLMFilter with the given NoiseClassifier.
func NewLLMFilter(classifier harvesting.NoiseClassifier) *LLMFilter {
	return &LLMFilter{classifier: classifier}
}

// Apply filters items using the classifier. On classifier error the item is
// conservatively kept (no data loss).
func (f *LLMFilter) Apply(ctx context.Context, items []harvesting.ContextItem) (kept []harvesting.ContextItem, rejected int) {
	for _, it := range items {
		noise, reason, err := f.classifier.IsNoise(ctx, it)
		if err != nil {
			kept = append(kept, it)
			continue
		}
		if noise {
			rejected++
			_ = reason
			continue
		}
		kept = append(kept, it)
	}
	return kept, rejected
}

// EinoClassifier implements NoiseClassifier using Eino's ChatModel to call
// an LLM (e.g. OpenAI) for noise classification.
type EinoClassifier struct {
	chatModel model.BaseChatModel
	promptTmpl string
}

// EinoClassifierConfig holds configuration for creating an EinoClassifier.
type EinoClassifierConfig struct {
	ChatModel  model.BaseChatModel // Eino ChatModel instance (e.g. openai.NewChatModel)
	PromptTmpl string             // Optional custom prompt template; defaults to DefaultPromptTemplate
}

// DefaultPromptTemplate is the default prompt sent to the LLM for noise classification.
const DefaultPromptTemplate = `判断以下内容是否是值得汇报的工作内容。返回 JSON {"is_noise": true/false, "reason": "..."}。

内容: %s

如果只是 merge、版本号变更、格式调整、空消息，则 is_noise=true。
如果是功能开发、bug 修复、讨论、设计、决策，则 is_noise=false。`

// NewEinoClassifier creates an EinoClassifier with the given config.
func NewEinoClassifier(cfg EinoClassifierConfig) (*EinoClassifier, error) {
	if cfg.ChatModel == nil {
		return nil, fmt.Errorf("eino classifier: ChatModel is required")
	}
	tmpl := cfg.PromptTmpl
	if tmpl == "" {
		tmpl = DefaultPromptTemplate
	}
	return &EinoClassifier{
		chatModel:  cfg.ChatModel,
		promptTmpl: tmpl,
	}, nil
}

// IsNoise calls the LLM via Eino ChatModel to classify the item.
func (c *EinoClassifier) IsNoise(ctx context.Context, item harvesting.ContextItem) (bool, string, error) {
	content := item.Title
	if item.Content != "" {
		content = item.Title + "\n" + item.Content
	}
	prompt := fmt.Sprintf(c.promptTmpl, content)

	messages := []*schema.Message{
		schema.SystemMessage("你是一个工作内容噪音过滤器。只返回 JSON，不要其他文字。"),
		schema.UserMessage(prompt),
	}

	resp, err := c.chatModel.Generate(ctx, messages)
	if err != nil {
		return false, "", fmt.Errorf("eino classifier generate: %w", err)
	}

	isNoise, reason, err := ParseResponse(resp.Content)
	if err != nil {
		return false, "", fmt.Errorf("eino classifier parse: %w", err)
	}
	return isNoise, reason, nil
}

// ParseResponse parses the LLM JSON response into noise classification result.
func ParseResponse(raw string) (bool, string, error) {
	var resp struct {
		IsNoise bool   `json:"is_noise"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return false, "", fmt.Errorf("parse: %w", err)
	}
	return resp.IsNoise, resp.Reason, nil
}

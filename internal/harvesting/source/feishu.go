package source

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuProvider abstracts the Feishu/Lark message provider (FR-B03)
type FeishuProvider interface {
	ListMessages(ctx context.Context, chatID string, from, to time.Time) ([]harvesting.ContextItem, error)
}

// FeishuAdapter is the unified entry point, implements sourceAdapter interface
type FeishuAdapter struct {
	Provider FeishuProvider
	Source   string   // feishu / lark
	ChatIDs  []string // chat IDs to fetch messages from
}

func (a *FeishuAdapter) Name() string { return a.Source }

// Fetch implements sourceAdapter interface — fetches messages since the given time
func (a *FeishuAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	to := time.Now()
	var out []harvesting.ContextItem
	for _, chatID := range a.ChatIDs {
		items, err := a.Provider.ListMessages(ctx, chatID, since, to)
		if err != nil {
			return nil, fmt.Errorf("feishu adapter fetch: %w", err)
		}
		for i := range items {
			items[i].Source = a.Source
			if items[i].Type == "" {
				items[i].Type = "message"
			}
		}
		out = append(out, items...)
	}
	return out, nil
}

// --- Lark SDK Provider (concrete implementation) ---

// LarkConfig holds configuration for the Lark SDK provider
type LarkConfig struct {
	AppID     string
	AppSecret string
}

// LarkProvider implements FeishuProvider using the Lark SDK
type LarkProvider struct {
	client *lark.Client
}

// NewLarkProvider creates a new Lark SDK provider with validation
func NewLarkProvider(cfg LarkConfig) (*LarkProvider, error) {
	if cfg.AppID == "" {
		return nil, fmt.Errorf("lark provider: app id is required")
	}
	if cfg.AppSecret == "" {
		return nil, fmt.Errorf("lark provider: app secret is required")
	}
	client := lark.NewClient(cfg.AppID, cfg.AppSecret)
	return &LarkProvider{client: client}, nil
}

// ListMessages fetches messages from a specific chat within the time range
// Uses iterator for automatic pagination
func (p *LarkProvider) ListMessages(ctx context.Context, chatID string, from, to time.Time) ([]harvesting.ContextItem, error) {
	req := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		StartTime(strconv.FormatInt(from.Unix(), 10)).
		EndTime(strconv.FormatInt(to.Unix(), 10)).
		SortType("ByCreateTimeAsc").
		PageSize(50).
		Build()

	iter, err := p.client.Im.V1.Message.ListByIterator(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("lark list messages: %w", err)
	}

	var items []harvesting.ContextItem
	for {
		ok, msg, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("lark iterate messages: %w", err)
		}
		if !ok {
			break
		}
		items = append(items, p.convertMessage(msg, chatID))
	}
	return items, nil
}

func (p *LarkProvider) convertMessage(m *larkim.Message, chatID string) harvesting.ContextItem {
	id := derefStr(m.MessageId)
	msgType := derefStr(m.MsgType)
	content := ""
	if m.Body != nil {
		content = derefStr(m.Body.Content)
	}
	senderID := ""
	if m.Sender != nil {
		senderID = derefStr(m.Sender.Id)
	}
	occurredAt := time.Time{}
	if m.CreateTime != nil {
		ts, err := strconv.ParseInt(*m.CreateTime, 10, 64)
		if err == nil {
			occurredAt = time.UnixMilli(ts)
		}
	}
	return harvesting.ContextItem{
		ID:         "feishu:msg:" + id,
		Source:     "feishu",
		Type:       "message",
		Title:      msgType,
		Content:    content,
		OccurredAt: occurredAt,
		Metadata: map[string]string{
			"chat_id":  chatID,
			"sender":   senderID,
			"msg_type": msgType,
		},
	}
}

// NewFeishuAdapter is a convenience constructor that creates a LarkProvider + FeishuAdapter
func NewFeishuAdapter(cfg LarkConfig, chatIDs []string) (*FeishuAdapter, error) {
	provider, err := NewLarkProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &FeishuAdapter{
		Provider: provider,
		Source:   "feishu",
		ChatIDs:  chatIDs,
	}, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

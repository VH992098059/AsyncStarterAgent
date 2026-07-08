package source

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuProvider 抽象飞书消息 provider（决策 #7: 以用户身份调用）
type FeishuProvider interface {
	ListMessages(ctx context.Context, userToken, chatID string, from, to time.Time) ([]harvesting.ContextItem, error)
}

// FeishuAdapter 是统一入口，实现 sourceAdapter 接口
// 决策 #7: 不再持有全局 client，每次 Fetch 时从 ctx 读取 user_access_token
type FeishuAdapter struct {
	Provider FeishuProvider
	Source   string   // feishu / lark
	ChatIDs  []string // chat IDs to fetch messages from
}

func (a *FeishuAdapter) Name() string { return a.Source }

// Fetch 实现 sourceAdapter 接口
// 决策 #7: 通过 ctx 注入 user_access_token（见 WithUserToken）
func (a *FeishuAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	userToken, ok := UserTokenFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("feishu adapter: user access token not found in context")
	}
	to := time.Now()
	var out []harvesting.ContextItem
	for _, chatID := range a.ChatIDs {
		items, err := a.Provider.ListMessages(ctx, userToken, chatID, since, to)
		if err != nil {
			return nil, fmt.Errorf("feishu adapter fetch: %w", err)
		}
		for i := range items {
			items[i].UserID = userID
			items[i].Source = a.Source
			if items[i].Type == "" {
				items[i].Type = "message"
			}
		}
		out = append(out, items...)
	}
	return out, nil
}

// --- Lark SDK Provider ---

// LarkProvider 用 lark SDK 实现 FeishuProvider
type LarkProvider struct {
	cli *lark.Client
}

// NewLarkProvider 创建 provider。cli 是共享的基础 client（由 ClientFactory 构造），
// 实际 API 调用时通过 larkcore.WithUserAccessToken(userToken) 以用户身份调用
func NewLarkProvider(cli *lark.Client) *LarkProvider {
	return &LarkProvider{cli: cli}
}

func (p *LarkProvider) ListMessages(ctx context.Context, userToken, chatID string, from, to time.Time) ([]harvesting.ContextItem, error) {
	req := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		StartTime(strconv.FormatInt(from.Unix(), 10)).
		EndTime(strconv.FormatInt(to.Unix(), 10)).
		SortType("ByCreateTimeAsc").
		PageSize(50).
		Build()

	// 决策 #7: 以用户身份调用（larkcore.WithUserAccessToken 是 per-request option）
	iter, err := p.cli.Im.V1.Message.ListByIterator(ctx, req, larkcore.WithUserAccessToken(userToken))
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
		items = append(items, convertMessage(msg, chatID))
	}
	return items, nil
}

func convertMessage(m *larkim.Message, chatID string) harvesting.ContextItem {
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

// UserTokenFromContext 从 ctx 读取 user_access_token（由调用方注入）
type ctxKey struct{}

var userTokenKey = ctxKey{}

// WithUserToken 把飞书 user_access_token 注入 ctx
func WithUserToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, userTokenKey, token)
}

// UserTokenFromContext 从 ctx 取出 user_access_token
func UserTokenFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(userTokenKey).(string)
	return t, ok
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// 确保 LarkProvider 满足 FeishuProvider 接口
var _ FeishuProvider = (*LarkProvider)(nil)

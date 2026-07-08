package source

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type mockFeishu struct {
	messages []harvesting.ContextItem
	err      error
}

// 签名变更：新增 userToken 参数（决策 #7）
func (m *mockFeishu) ListMessages(_ context.Context, _ string, _ string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.messages, m.err
}

func TestFeishuAdapter_Fetch(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "1", Title: "text", Type: "message", Content: "hello"},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0].Source != "feishu" {
		t.Errorf("source: %s", items[0].Source)
	}
	if items[0].Type != "message" {
		t.Errorf("type: %s", items[0].Type)
	}
}

func TestFeishuAdapter_Name(t *testing.T) {
	a := &FeishuAdapter{Source: "feishu"}
	if a.Name() != "feishu" {
		t.Errorf("expected feishu, got %s", a.Name())
	}
}

func TestFeishuAdapter_Fetch_DefaultType(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "2", Title: "text", Type: ""},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal("expected 1")
	}
	if items[0].Type != "message" {
		t.Errorf("expected default type message, got %s", items[0].Type)
	}
}

func TestFeishuAdapter_Fetch_EmptyResult(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0, got %d", len(items))
	}
}

func TestFeishuAdapter_Fetch_ProviderError(t *testing.T) {
	mock := &mockFeishu{err: errors.New("api unavailable")}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	_, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, mock.err) {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestFeishuAdapter_Fetch_MultipleChats(t *testing.T) {
	callCount := 0
	mock := &mockFeishuFunc{
		fn: func(chatID string) ([]harvesting.ContextItem, error) {
			callCount++
			return []harvesting.ContextItem{
				{ID: "msg-" + chatID, Title: "text", Type: "message"},
			}, nil
		},
	}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-a", "chat-b"}}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestFeishuAdapter_Fetch_NoChatIDs(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "1", Title: "text", Type: "message"},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: nil}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, err := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 with no chat IDs, got %d", len(items))
	}
}

// 新增：缺 token 时 Fetch 应返回错误（决策 #7）
func TestFeishuAdapter_Fetch_NoToken(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "1", Title: "text", Type: "message"},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	_, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected error for missing user token")
	}
}

// Compile-time check: FeishuAdapter satisfies the sourceAdapter interface
var _ interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)
} = (*FeishuAdapter)(nil)

// mockFeishuFunc is a function-based mock for testing multiple chat scenarios
type mockFeishuFunc struct {
	fn func(chatID string) ([]harvesting.ContextItem, error)
}

// 签名变更：新增 userToken 参数（决策 #7）
func (m *mockFeishuFunc) ListMessages(_ context.Context, _ string, chatID string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.fn(chatID)
}

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

func (m *mockFeishu) ListMessages(_ context.Context, _ string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.messages, m.err
}

func TestFeishuAdapter_Fetch(t *testing.T) {
	mock := &mockFeishu{messages: []harvesting.ContextItem{
		{ID: "1", Title: "text", Type: "message", Content: "hello"},
	}}
	a := &FeishuAdapter{Provider: mock, Source: "feishu", ChatIDs: []string{"chat-1"}}
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
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
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
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
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
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
	_, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
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
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
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
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 with no chat IDs, got %d", len(items))
	}
}

func TestNewLarkProvider_MissingAppID(t *testing.T) {
	_, err := NewLarkProvider(LarkConfig{AppID: "", AppSecret: "secret"})
	if err == nil {
		t.Fatal("expected error for missing app id")
	}
}

func TestNewLarkProvider_MissingAppSecret(t *testing.T) {
	_, err := NewLarkProvider(LarkConfig{AppID: "app-id", AppSecret: ""})
	if err == nil {
		t.Fatal("expected error for missing app secret")
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

func (m *mockFeishuFunc) ListMessages(_ context.Context, chatID string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.fn(chatID)
}

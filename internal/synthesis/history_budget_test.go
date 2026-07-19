package synthesis

import (
	"strings"
	"testing"

	"github.com/asyncstarter/agent/internal/repository"
)

// repeatRunes 生成指定 rune 数的内容，用于精确控制 estimateTokens 的估算结果。
func repeatRunes(n int) string {
	return strings.Repeat("字", n)
}

func TestBuildChatHistory_KeepsAllWithinBudget(t *testing.T) {
	history := []repository.AgentRunMessage{
		{Role: "user", Content: repeatRunes(10), Status: "sent"},
		{Role: "assistant", Content: repeatRunes(10), Status: "done"},
		{Role: "user", Content: repeatRunes(10), Status: "sent"},
	}

	got := buildChatHistory(history, defaultHistoryTokenBudget)

	if len(got) != 3 {
		t.Fatalf("expected 3 messages kept, got %d: %+v", len(got), got)
	}
	if got[0].Role != "user" || got[1].Role != "assistant" || got[2].Role != "user" {
		t.Errorf("unexpected role order: %+v", got)
	}
}

func TestBuildChatHistory_TruncatesOldestWhenOverBudget(t *testing.T) {
	// 每条约 100 tokens（200 rune / 2），预算只够放下最近两条 + 强制保留的最后一条。
	history := []repository.AgentRunMessage{
		{Role: "user", Content: repeatRunes(200), Status: "sent"},      // 最旧，应被裁掉
		{Role: "assistant", Content: repeatRunes(200), Status: "done"}, // 应保留
		{Role: "user", Content: repeatRunes(200), Status: "sent"},      // 最新，强制保留
	}

	got := buildChatHistory(history, 250) // 预算刚好够 assistant+最新 user（250），不够再加最旧 user

	if len(got) != 2 {
		t.Fatalf("expected 2 messages kept (oldest truncated), got %d: %+v", len(got), got)
	}
	if got[0].Role != "assistant" || got[1].Role != "user" {
		t.Errorf("expected [assistant, user] kept in order, got %+v", got)
	}
}

func TestBuildChatHistory_SkipsNonDoneAssistantMessages(t *testing.T) {
	history := []repository.AgentRunMessage{
		{Role: "user", Content: "问题1", Status: "sent"},
		{Role: "assistant", Content: "", Status: "streaming"}, // 占位消息，应跳过
		{Role: "assistant", Content: "失败了", Status: "error"}, // 失败消息，应跳过
		{Role: "user", Content: "问题2", Status: "sent"},
	}

	got := buildChatHistory(history, defaultHistoryTokenBudget)

	if len(got) != 2 {
		t.Fatalf("expected 2 messages (only user), got %d: %+v", len(got), got)
	}
	for _, m := range got {
		if m.Role != "user" {
			t.Errorf("unexpected non-user message survived filter: %+v", m)
		}
	}
}

func TestBuildChatHistory_AlwaysKeepsLastUserMessageEvenOverBudget(t *testing.T) {
	history := []repository.AgentRunMessage{
		{Role: "user", Content: repeatRunes(50), Status: "sent"},
		{Role: "user", Content: repeatRunes(5000), Status: "sent"}, // 单条就远超预算，仍须保留
	}

	got := buildChatHistory(history, 10) // 预算极小

	if len(got) != 1 {
		t.Fatalf("expected only the last message kept, got %d: %+v", len(got), got)
	}
	if got[0].Content != repeatRunes(5000) {
		t.Errorf("expected last user message content preserved, got len=%d", len(got[0].Content))
	}
}

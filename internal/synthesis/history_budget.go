package synthesis

import "github.com/asyncstarter/agent/internal/repository"

// defaultHistoryTokenBudget 是 ChatWithRun 拼装历史消息时的默认 token 预算兜底值。
// 项目未引入 tokenizer 库，estimateTokens 用 rune 数保守估算；该值是在扣除
// system prompt + 当前草稿 + 输出预留后，留给历史消息的保守额度。
const defaultHistoryTokenBudget = 6000

// estimateTokens 用 rune 数做保守估算（中英文混合场景下不易低估）。
func estimateTokens(s string) int {
	n := 0
	for range s {
		n++
	}
	return (n + 1) / 2
}

// buildChatHistory 把落库的历史消息过滤 + 按 token 预算裁剪成可发给 LLM 的 Message 列表。
// 过滤规则与原逻辑一致：user 全部纳入，assistant 只取 status=done。
// 裁剪规则：从最新往最旧遍历，累计 token 不超过 budget；一旦超出立即停止，
// 但无论如何都强制保留最后一条 user 消息（哪怕它单独就超预算），避免当前这轮提问丢失。
func buildChatHistory(history []repository.AgentRunMessage, budget int) []Message {
	// 第一步：按现有规则过滤出合法消息，同时记录最后一条 user 消息的下标
	var filtered []repository.AgentRunMessage
	lastUserIdx := -1
	for _, m := range history {
		switch {
		case m.Role == "user":
			filtered = append(filtered, m)
			lastUserIdx = len(filtered) - 1
		case m.Role == "assistant" && m.Status == "done":
			filtered = append(filtered, m)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	// 第二步：从最新往最旧遍历，按预算反向选取
	kept := make([]bool, len(filtered))
	remaining := budget
	for i := len(filtered) - 1; i >= 0; i-- {
		cost := estimateTokens(filtered[i].Content)
		if cost > remaining && kept[lastUserIdx] {
			// 已经保留了强制项，且当前消息会超预算 -> 停止继续向旧消息回溯
			break
		}
		kept[i] = true
		remaining -= cost
	}
	// 强制保留最后一条 user 消息，即使它单独超预算也不能丢
	if lastUserIdx >= 0 {
		kept[lastUserIdx] = true
	}

	out := make([]Message, 0, len(filtered))
	for i, m := range filtered {
		if !kept[i] {
			continue
		}
		out = append(out, Message{Role: m.Role, Content: m.Content})
	}
	return out
}

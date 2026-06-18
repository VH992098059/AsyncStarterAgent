package queue_test

import (
	"testing"

	"github.com/asyncstarter/agent/internal/queue"
)

// TestClientAndServer_EmptyURL 校验：空 Redis URL 必须返回 error。
// 这是 TDD 的 RED 阶段用测试：写测试时 queue 包还不存在 / API 未实现，
// 确认编译失败 / 失败信息正确。
func TestClientAndServer_EmptyURL(t *testing.T) {
	if _, err := queue.NewClient(""); err == nil {
		t.Fatal("expected error for empty redis url")
	}
}

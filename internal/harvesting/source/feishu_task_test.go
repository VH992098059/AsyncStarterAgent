package source

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"

	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

type mockTaskProvider struct {
	items []harvesting.ContextItem
	err   error
}

func (m *mockTaskProvider) ListTasks(_ context.Context, _ string, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.items, m.err
}

func TestFeishuTaskAdapter_Fetch(t *testing.T) {
	mock := &mockTaskProvider{items: []harvesting.ContextItem{
		{ID: "t1", Title: "写周报", Type: "task", Content: "本周工作总结"},
	}}
	a := &FeishuTaskAdapter{Provider: mock, Source: "feishu"}
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
	if items[0].Type != "task" {
		t.Errorf("type: %s", items[0].Type)
	}
	if items[0].UserID != "user-1" {
		t.Errorf("user id: %s", items[0].UserID)
	}
}

func TestFeishuTaskAdapter_Fetch_NoToken(t *testing.T) {
	mock := &mockTaskProvider{}
	a := &FeishuTaskAdapter{Provider: mock, Source: "feishu"}
	_, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestFeishuTaskAdapter_Fetch_DefaultType(t *testing.T) {
	mock := &mockTaskProvider{items: []harvesting.ContextItem{
		{ID: "t1", Title: "task", Type: ""},
	}}
	a := &FeishuTaskAdapter{Provider: mock, Source: "feishu"}
	ctx := WithUserToken(context.Background(), "fake-token")
	items, _ := a.Fetch(ctx, "user-1", time.Now().Add(-24*time.Hour))
	if items[0].Type != "task" {
		t.Errorf("expected default type task, got %s", items[0].Type)
	}
}

func TestFeishuTaskAdapter_Name(t *testing.T) {
	a := &FeishuTaskAdapter{Source: "feishu"}
	if a.Name() != "feishu" {
		t.Errorf("expected feishu, got %s", a.Name())
	}
}

// 编译期断言
var _ interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)
} = (*FeishuTaskAdapter)(nil)

// --- convertTask unit tests ---

func ptr(s string) *string { return &s }

func TestConvertTask_IncrementalFilter_Excluded(t *testing.T) {
	// updated_at 在 from 之前 → 被过滤掉
	task := &larktask.Task{
		Guid:      ptr("g1"),
		Summary:   ptr("旧任务"),
		UpdatedAt: ptr("1000"), // 1970-01-01 00:00:01 UTC
	}
	from := time.Now()
	_, include := convertTask(task, from)
	if include {
		t.Error("task updated before 'from' should be excluded")
	}
}

func TestConvertTask_IncrementalFilter_Included(t *testing.T) {
	// updated_at 在 from 之后 → 包含
	futureMs := time.Now().Add(1 * time.Hour).UnixMilli()
	task := &larktask.Task{
		Guid:      ptr("g2"),
		Summary:   ptr("新任务"),
		UpdatedAt: ptr(strconv.FormatInt(futureMs, 10)),
	}
	item, include := convertTask(task, time.Now().Add(-24*time.Hour))
	if !include {
		t.Fatal("task updated after 'from' should be included")
	}
	if item.ID != "feishu:task:g2" {
		t.Errorf("ID: %s", item.ID)
	}
	if item.Title != "新任务" {
		t.Errorf("Title: %s", item.Title)
	}
}

func TestConvertTask_NoUpdatedAt_PassThrough(t *testing.T) {
	// UpdatedAt 缺失 → 不过滤，保守包含
	task := &larktask.Task{
		Guid:    ptr("g3"),
		Summary: ptr("无时间戳任务"),
	}
	item, include := convertTask(task, time.Now())
	if !include {
		t.Fatal("task with nil UpdatedAt should be included (no filter)")
	}
	if item.OccurredAt != (time.Time{}) {
		t.Errorf("OccurredAt should be zero, got %v", item.OccurredAt)
	}
}

func TestConvertTask_MalformedTimestamp_PassThrough(t *testing.T) {
	// UpdatedAt 非数字 → 解析失败，updated 保持零值，不过滤
	task := &larktask.Task{
		Guid:      ptr("g4"),
		Summary:   ptr("畸形时间戳"),
		UpdatedAt: ptr("not-a-number"),
	}
	_, include := convertTask(task, time.Now())
	if !include {
		t.Error("task with malformed UpdatedAt should be included (conservative)")
	}
}

func TestConvertTask_Completed_Status(t *testing.T) {
	task := &larktask.Task{
		Guid:   ptr("g5"),
		Status: ptr("done"),
	}
	item, _ := convertTask(task, time.Now().Add(-24*time.Hour))
	if item.Metadata["completed"] != "true" {
		t.Errorf("expected completed=true, got %s", item.Metadata["completed"])
	}
	if !strings.Contains(item.Content, "已完成") {
		t.Errorf("content should contain 已完成, got: %s", item.Content)
	}
}

func TestConvertTask_InProgress_Status(t *testing.T) {
	task := &larktask.Task{
		Guid:   ptr("g6"),
		Status: ptr("todo"),
	}
	item, _ := convertTask(task, time.Now().Add(-24*time.Hour))
	if item.Metadata["completed"] != "false" {
		t.Errorf("expected completed=false, got %s", item.Metadata["completed"])
	}
	if !strings.Contains(item.Content, "进行中") {
		t.Errorf("content should contain 进行中, got: %s", item.Content)
	}
}

func TestConvertTask_DueDate(t *testing.T) {
	dueMs := time.Now().Add(48 * time.Hour).UnixMilli()
	task := &larktask.Task{
		Guid: ptr("g7"),
		Due: &larktask.Due{
			Timestamp: ptr(strconv.FormatInt(dueMs, 10)),
		},
	}
	item, _ := convertTask(task, time.Now().Add(-24*time.Hour))
	if item.Metadata["due"] == "" {
		t.Error("due metadata should be non-empty")
	}
	if !strings.Contains(item.Content, "截止时间") {
		t.Errorf("content should contain 截止时间, got: %s", item.Content)
	}
}

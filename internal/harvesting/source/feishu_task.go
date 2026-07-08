package source

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larktask "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
)

// TaskProvider 抽象飞书任务拉取
type TaskProvider interface {
	ListTasks(ctx context.Context, userToken string, from, to time.Time) ([]harvesting.ContextItem, error)
}

// FeishuTaskAdapter 拉取用户负责的飞书任务作为上下文
type FeishuTaskAdapter struct {
	Provider TaskProvider
	Source   string
}

func (a *FeishuTaskAdapter) Name() string { return a.Source }

func (a *FeishuTaskAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	userToken, ok := UserTokenFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("feishu task adapter: user access token not found in context")
	}
	items, err := a.Provider.ListTasks(ctx, userToken, since, time.Now())
	if err != nil {
		return nil, fmt.Errorf("feishu task fetch: %w", err)
	}
	for i := range items {
		items[i].UserID = userID
		items[i].Source = a.Source
		if items[i].Type == "" {
			items[i].Type = "task"
		}
	}
	return items, nil
}

// LarkTaskProvider 用 lark SDK 实现 TaskProvider
type LarkTaskProvider struct {
	cli *lark.Client
}

func NewLarkTaskProvider(cli *lark.Client) *LarkTaskProvider {
	return &LarkTaskProvider{cli: cli}
}

// ListTasks 调用 task/v2 接口拉取任务
// 飞书 task/v2 的 List 接口不支持按时间范围过滤，只能列出当前用户负责的任务
// 增量过滤在客户端做（按 updated_at 过滤 since）
// 参数 to 保留用于接口一致性（FeishuProvider.ListMessages 也有 to），但 task/v2 API 不支持按结束时间过滤
func (p *LarkTaskProvider) ListTasks(ctx context.Context, userToken string, from, to time.Time) ([]harvesting.ContextItem, error) {
	req := larktask.NewListTaskReqBuilder().
		PageSize(50).
		Build()

	iter, err := p.cli.Task.V2.Task.ListByIterator(ctx, req, larkcore.WithUserAccessToken(userToken))
	if err != nil {
		return nil, fmt.Errorf("lark list tasks: %w", err)
	}

	var items []harvesting.ContextItem
	for {
		ok, task, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("lark iterate tasks: %w", err)
		}
		if !ok {
			break
		}
		item, include := convertTask(task, from)
		if !include {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func convertTask(t *larktask.Task, from time.Time) (harvesting.ContextItem, bool) {
	guid := derefStr(t.Guid)
	summary := derefStr(t.Summary)
	desc := derefStr(t.Description)

	// 解析更新时间用于增量过滤
	updated := time.Time{}
	if t.UpdatedAt != nil {
		// 飞书时间戳是毫秒
		if ms, err := strconv.ParseInt(*t.UpdatedAt, 10, 64); err == nil {
			updated = time.UnixMilli(ms)
		}
		// 解析失败时 updated 保持零值，任务不会被增量过滤丢弃（视为无可靠时间戳，保守包含）
	}

	// 增量过滤：只保留 since 之后更新的任务
	// 注意：updated 为零值（UpdatedAt 缺失或解析失败）时不过滤，保守包含
	if !updated.IsZero() && updated.Before(from) {
		return harvesting.ContextItem{}, false
	}

	// 解析截止时间
	dueStr := ""
	if t.Due != nil && t.Due.Timestamp != nil {
		if dueMs, err := strconv.ParseInt(*t.Due.Timestamp, 10, 64); err == nil {
			dueStr = time.UnixMilli(dueMs).Format(time.RFC3339)
		}
	}

	// 完成状态：优先用 Status 字段（"done" 表示已完成），回退到 CompletedAt
	completed := false
	if s := derefStr(t.Status); s == "done" {
		completed = true
	} else if t.CompletedAt != nil && *t.CompletedAt != "0" {
		completed = true
	}

	content := desc
	if dueStr != "" {
		content += fmt.Sprintf("\n\n截止时间: %s", dueStr)
	}
	if completed {
		content += "\n状态: 已完成"
	} else {
		content += "\n状态: 进行中"
	}

	return harvesting.ContextItem{
		ID:         "feishu:task:" + guid,
		Source:     "feishu",
		Type:       "task",
		Title:      summary,
		Content:    content,
		OccurredAt: updated,
		Metadata: map[string]string{
			"task_guid": guid,
			"due":       dueStr,
			"completed": strconv.FormatBool(completed),
		},
	}, true
}

var _ TaskProvider = (*LarkTaskProvider)(nil)

package delivery

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/asyncstarter/agent/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Notifier interface {
	Notify(ctx context.Context, title, body string) error
}

type Service struct {
	pool    *pgxpool.Pool
	factory AdapterFactory
	notif   Notifier
}

func NewService(pool *pgxpool.Pool, factory AdapterFactory, notif Notifier) *Service {
	return &Service{pool: pool, factory: factory, notif: notif}
}

type DeliverResult struct {
	DeliveryID string `json:"delivery_id"`
	TargetURL  string `json:"target_url"`
	Status     string `json:"status"`
}

func (s *Service) Deliver(ctx context.Context, userID, runID, targetType string) (*DeliverResult, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("database not configured")
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	var draftID uuid.UUID
	var title, md string
	err = s.pool.QueryRow(ctx,
		`SELECT id, title, markdown_content FROM drafts WHERE agent_run_id = $1 LIMIT 1`,
		runID,
	).Scan(&draftID, &title, &md)
	if err != nil {
		return nil, fmt.Errorf("load draft: %w", err)
	}

	deliveryID := uuid.New()
	_, err = s.pool.Exec(ctx,
		`INSERT INTO deliveries (id, draft_id, target_type, status) VALUES ($1, $2, $3, 'pending')`,
		deliveryID, draftID, targetType,
	)
	if err != nil {
		return nil, fmt.Errorf("create delivery: %w", err)
	}

	var targetURL string
	var adapterErr error
	switch targetType {
	case "notion":
		notion, ferr := s.factory.GetNotionAdapter(ctx, uid)
		if ferr != nil {
			adapterErr = ferr
			break
		}
		targetURL, err = notion.CreatePage(ctx, title, md)
	case "obsidian":
		obs, ferr := s.factory.GetObsidianAdapter(ctx, uid)
		if ferr != nil {
			adapterErr = ferr
			break
		}
		targetURL, err = obs.WriteFile(ctx, title, md)
	case "feishu":
		// 决策 #7: 飞书云文档交付
		feishu, ferr := s.factory.GetFeishuAdapter(ctx, uid)
		if ferr != nil {
			adapterErr = ferr
			break
		}
		targetURL, err = feishu.CreateDoc(ctx, title, md)
	default:
		return nil, fmt.Errorf("unsupported target type: %s", targetType)
	}

	status := "success"
	if adapterErr != nil {
		s.markRunStatus(ctx, runID, "failed", "delivery", adapterErr.Error())
		return nil, adapterErr
	}
	if err != nil {
		status = "failed"
		// 部分成功场景（如飞书文档已创建但 block 写入失败）：targetURL 非空时
		// 一并落库，保证用户仍能拿到已创建文档的链接。
		if targetURL != "" {
			_, _ = s.pool.Exec(ctx,
				`UPDATE deliveries SET status = $1, error_message = $2, target_url = $3, updated_at = $4 WHERE id = $5`,
				status, err.Error(), targetURL, time.Now(), deliveryID,
			)
		} else {
			_, _ = s.pool.Exec(ctx,
				`UPDATE deliveries SET status = $1, error_message = $2, updated_at = $3 WHERE id = $4`,
				status, err.Error(), time.Now(), deliveryID,
			)
		}
		s.markRunStatus(ctx, runID, "failed", "delivery", err.Error())
		return &DeliverResult{DeliveryID: deliveryID.String(), TargetURL: targetURL, Status: status}, err
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE deliveries SET target_url = $1, status = $2, updated_at = $3 WHERE id = $4`,
		targetURL, status, time.Now(), deliveryID,
	)
	if err != nil {
		s.markRunStatus(ctx, runID, "failed", "delivery", err.Error())
		return nil, fmt.Errorf("update delivery: %w", err)
	}

	if commentErr := s.updateSourceComment(ctx, uid, runID, targetType, targetURL, title); commentErr != nil {
		log.Printf("[delivery] update source comment: %v", commentErr)
	}

	if s.notif != nil {
		_ = s.notif.Notify(ctx, "草稿已交付", title)
	}

	s.markRunStatus(ctx, runID, "completed", "delivery", "")
	return &DeliverResult{DeliveryID: deliveryID.String(), TargetURL: targetURL, Status: status}, nil
}

// markRunStatus 回写 agent_runs 状态。失败仅记日志，不阻断交付主流程。
func (s *Service) markRunStatus(ctx context.Context, runID, status, stage, errMsg string) {
	runUUID, parseErr := uuid.Parse(runID)
	if parseErr != nil {
		log.Printf("[delivery] invalid run id %q: %v", runID, parseErr)
		return
	}
	if err := repository.UpdateAgentRunStatus(ctx, s.pool, runUUID, status, stage, errMsg); err != nil {
		log.Printf("[delivery] update run status to %s/%s: %v", status, stage, err)
	}
}

func (s *Service) updateSourceComment(ctx context.Context, userID uuid.UUID, runID, targetType, targetURL, title string) error {
	switch targetType {
	case "notion":
		notion, err := s.factory.GetNotionAdapter(ctx, userID)
		if err != nil {
			return nil
		}
		pageID := extractPageID(targetURL)
		return notion.UpdateTaskComment(ctx, pageID, fmt.Sprintf("已生成: %s", title))
	case "obsidian":
		return nil
	case "feishu":
		// 决策 #7: 回写草稿链接到原始飞书任务备注
		// 需要从 agent_run 的 trigger_source 取 task_guid
		taskGUID, err := s.getFeishuTaskGUID(ctx, runID)
		if err != nil {
			return nil // 没有关联的飞书任务，静默跳过
		}
		adapter, err := s.factory.GetFeishuAdapter(ctx, userID)
		if err != nil {
			// 适配器获取失败，静默跳过（评论回写为非关键副作用，与 notion 分支保持一致）
			return nil
		}
		return adapter.CreateTaskComment(ctx, taskGUID, fmt.Sprintf("起跑器草稿: %s\n%s", title, targetURL))
	}
	return nil
}

// parseFeishuTaskGUID 从 trigger_source 字符串解析飞书任务 GUID。
// 预期格式 "feishu:task:<guid>"；不匹配或 guid 为空时返回 ("", false)。
func parseFeishuTaskGUID(src string) (string, bool) {
	if !strings.HasPrefix(src, "feishu:task:") {
		return "", false
	}
	guid := strings.TrimPrefix(src, "feishu:task:")
	if guid == "" {
		return "", false
	}
	return guid, true
}

// getFeishuTaskGUID 从 agent_run.trigger_source 解析飞书任务 GUID。
// trigger_source 格式假设为 "feishu:task:<guid>"；不匹配时返回错误（调用方静默跳过）。
func (s *Service) getFeishuTaskGUID(ctx context.Context, runID string) (string, error) {
	var src string
	err := s.pool.QueryRow(ctx, `SELECT trigger_source FROM agent_runs WHERE id = $1`, runID).Scan(&src)
	if err != nil {
		return "", fmt.Errorf("get feishu task guid: %w", err)
	}
	guid, ok := parseFeishuTaskGUID(src)
	if !ok {
		return "", fmt.Errorf("not a feishu task trigger")
	}
	return guid, nil
}

func extractPageID(url string) string {
	var last string
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			last = url[i+1:]
			break
		}
	}
	if last == "" {
		last = url
	}
	for i := len(last) - 1; i >= 0; i-- {
		if last[i] == '-' {
			return last[i+1:]
		}
	}
	return last
}

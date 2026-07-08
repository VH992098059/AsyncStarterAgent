package delivery

import (
	"context"
	"fmt"
	"log"
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
		_, _ = s.pool.Exec(ctx,
			`UPDATE deliveries SET status = $1, error_message = $2, updated_at = $3 WHERE id = $4`,
			status, err.Error(), time.Now(), deliveryID,
		)
		s.markRunStatus(ctx, runID, "failed", "delivery", err.Error())
		return &DeliverResult{DeliveryID: deliveryID.String(), Status: status}, err
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
	}
	return nil
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

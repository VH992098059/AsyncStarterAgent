package trigger

import (
	"context"
	"fmt"

	"github.com/asyncstarter/agent/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool    *pgxpool.Pool
	matcher *Matcher
}

func NewService(pool *pgxpool.Pool, m *Matcher) *Service {
	return &Service{pool: pool, matcher: m}
}

// ProcessKeyword 处理关键词触发事件
func (s *Service) ProcessKeyword(ctx context.Context, userID uuid.UUID, text string) (uuid.UUID, error) {
	taskType, ok := s.matcher.Match(text)
	if !ok {
		return uuid.Nil, fmt.Errorf("no rule matched")
	}
	return s.createRun(ctx, userID, taskType, SourceKeyword, text)
}

func (s *Service) createRun(ctx context.Context, userID uuid.UUID, taskType string, src Source, srcDetail string) (uuid.UUID, error) {
	run := &repository.AgentRun{
		UserID:        userID,
		TaskType:      taskType,
		Status:        "pending",
		CurrentStage:  "ingestion",
		TriggerType:   string(src),
		TriggerSource: srcDetail,
	}
	return repository.CreateAgentRun(ctx, s.pool, run)
}

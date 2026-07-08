package trigger

import (
	"context"
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

// ProcessKeyword 处理关键词触发事件。
// 当没有规则匹配时，回退为 message 类型，确保任意用户输入都能创建任务。
func (s *Service) ProcessKeyword(ctx context.Context, userID uuid.UUID, text string) (uuid.UUID, error) {
	taskType, ok := s.matcher.Match(text)
	if !ok {
		taskType = "message"
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

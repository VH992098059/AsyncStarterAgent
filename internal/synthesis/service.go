package synthesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DraftResult struct {
	Draft        string  `json:"draft"`
	Marks        []Mark  `json:"marks"`
	Completeness float32 `json:"completeness"`
	Template     string  `json:"template"`
}

type DraftGenerator interface {
	GenerateDraft(ctx context.Context, runID, userID, taskType string) (*DraftResult, error)
}

type Service struct {
	pool        *pgxpool.Pool
	factory     ClientFactory
	templateDir string
	vecStore    *PGVectorStore
}

func NewService(pool *pgxpool.Pool, factory ClientFactory, templateDir string) *Service {
	vecStore := NewPGVectorStore(pool, 1536)
	return &Service{
		pool:        pool,
		factory:     factory,
		templateDir: templateDir,
		vecStore:    vecStore,
	}
}

func (s *Service) GenerateDraft(ctx context.Context, runID, userID string, taskType string) (*DraftResult, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	llm, err := s.factory.GetLLM(ctx, uid)
	if err != nil {
		return nil, err
	}
	embedder, err := s.factory.GetEmbedder(ctx, uid)
	if err != nil {
		return nil, err
	}

	rag := NewRAG(*embedder, s.vecStore)

	temp := s.factory.GetLLMTemperature(ctx, uid)
	maxTokens := s.factory.GetLLMMaxTokens(ctx, uid)

	wfCfg := workflowConfig{
		TemplateDir: s.templateDir,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}
	wf, err := newDraftWorkflow(ctx, rag, llm, wfCfg)
	if err != nil {
		return nil, fmt.Errorf("build workflow: %w", err)
	}

	result, err := wf.generate(ctx, runID, userID, taskType)
	if err != nil {
		return nil, fmt.Errorf("generate draft: %w", err)
	}

	if s.pool != nil {
		marksJSON, _ := marshalMarks(result.Marks)
		_, err = s.pool.Exec(ctx,
			`INSERT INTO drafts (agent_run_id, title, markdown_content, completeness, marks, status)
			 VALUES ($1, $2, $3, $4, $5, 'draft')
			 ON CONFLICT (agent_run_id) DO UPDATE SET markdown_content = $3, completeness = $4, marks = $5, updated_at = NOW()`,
			runID, taskType, result.Draft, result.Completeness, marksJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("save draft: %w", err)
		}
	}

	return result, nil
}

func (s *Service) DraftExists(ctx context.Context, runID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM drafts WHERE agent_run_id = $1)",
		runID,
	).Scan(&exists)
	return exists, err
}

func (s *Service) QueryRunInfo(ctx context.Context, runID string, userID, taskType *string) error {
	return s.pool.QueryRow(ctx,
		"SELECT user_id::text, task_type FROM agent_runs WHERE id = $1",
		runID,
	).Scan(userID, taskType)
}

// streamMarkdown 将已有 markdown 分块写为 A2UI delta + complete。
// 按 rune 切割，避免多字节字符乱码。
func (s *Service) streamMarkdown(ctx context.Context, md string, w *A2UIWriter) error {
	const chunkSize = 30
	runes := []rune(md)
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		if err := w.WriteDelta(string(runes[i:end])); err != nil {
			return err
		}
	}
	marks := ExtractMarks(md)
	return w.WriteComplete(marks, Completeness(md))
}

func (s *Service) StreamDraft(ctx context.Context, runID string, w *A2UIWriter) error {
	row := s.pool.QueryRow(ctx,
		"SELECT markdown_content FROM drafts WHERE agent_run_id = $1 LIMIT 1",
		runID,
	)
	var md string
	if err := row.Scan(&md); err != nil {
		return fmt.Errorf("draft not found: %w", err)
	}
	return s.streamMarkdown(ctx, md, w)
}

// GenerateDraftStream 执行完整 workflow 并将 LLM token 实时写入 w，
// workflow 结束后将结果存库并写 complete 事件。
func (s *Service) GenerateDraftStream(ctx context.Context, runID, userID, taskType string, w *A2UIWriter) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	llm, err := s.factory.GetLLM(ctx, uid)
	if err != nil {
		return err
	}
	embedder, err := s.factory.GetEmbedder(ctx, uid)
	if err != nil {
		return err
	}

	rag := NewRAG(*embedder, s.vecStore)
	temp := s.factory.GetLLMTemperature(ctx, uid)
	maxTokens := s.factory.GetLLMMaxTokens(ctx, uid)

	wfCfg := workflowConfig{
		TemplateDir: s.templateDir,
		Temperature: temp,
		MaxTokens:   maxTokens,
	}
	wf, err := newDraftWorkflow(ctx, rag, llm, wfCfg)
	if err != nil {
		return fmt.Errorf("build workflow: %w", err)
	}

	streamCtx := WithA2UIWriter(ctx, w)
	result, err := wf.generate(streamCtx, runID, userID, taskType)
	if err != nil {
		return fmt.Errorf("generate draft: %w", err)
	}

	if s.pool != nil {
		marksJSON, _ := marshalMarks(result.Marks)
		_, saveErr := s.pool.Exec(ctx,
			`INSERT INTO drafts (agent_run_id, title, markdown_content, completeness, marks, status)
             VALUES ($1, $2, $3, $4, $5, 'draft')
             ON CONFLICT (agent_run_id) DO UPDATE SET markdown_content = $3, completeness = $4, marks = $5, updated_at = NOW()`,
			runID, taskType, result.Draft, result.Completeness, marksJSON,
		)
		if saveErr != nil {
			_ = w.WriteComplete(result.Marks, result.Completeness)
			return fmt.Errorf("save draft: %w", saveErr)
		}
	}

	return w.WriteComplete(result.Marks, result.Completeness)
}

func (s *Service) TestLLM(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	llm, err := s.factory.GetLLM(ctx, uid)
	if err != nil {
		return err
	}
	ch, err := llm.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
		Temperature: 0.3,
		MaxTokens:   10,
	})
	if err != nil {
		return fmt.Errorf("test connection: %w", err)
	}
	for c := range ch {
		if c.Err != nil {
			return fmt.Errorf("test stream: %w", c.Err)
		}
		if c.Done {
			break
		}
	}
	return nil
}

func marshalMarks(marks []Mark) ([]byte, error) {
	if len(marks) == 0 {
		return []byte("[]"), nil
	}
	type markJSON struct {
		ID       string `json:"id"`
		Hint     string `json:"hint"`
		Position int    `json:"position"`
		Resolved bool   `json:"resolved"`
	}
	out := make([]markJSON, len(marks))
	for i, m := range marks {
		out[i] = markJSON(m)
	}
	return json.Marshal(out)
}

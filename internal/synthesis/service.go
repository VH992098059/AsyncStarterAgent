package synthesis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/asyncstarter/agent/internal/repository"
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

// GenerateDraftAsync 是 worker 端调用的非流式草稿生成入口（问题 #11：asynq 接入实际触发路径）。
// 先用 ClaimRunForSynthesis 原子性认领 run（status: pending → running/synthesis）；
// 若认领失败（affected rows=0，说明 DraftStreamHandler.Stream 的同步兜底已经在处理这个 run，
// 或 run 已不是 pending 状态），直接返回 nil，不重复调用 LLM。
// 认领成功后调用 GenerateDraft（内部会再次执行 wf.generate，允许失败重试——asynq 默认
// MaxRetry(3)，多次重试不会重复认领，因为 run 状态已经是 running 不再是 pending，
// 但重试仍需要重新生成，故这里认领检查只做首次拦截，不阻止 asynq 自身的重试机制）。
func (s *Service) GenerateDraftAsync(ctx context.Context, runID string) error {
	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return fmt.Errorf("invalid run id: %w", err)
	}
	if s.pool == nil {
		return fmt.Errorf("pool not configured")
	}

	claimed, err := repository.ClaimRunForSynthesis(ctx, s.pool, runUUID)
	if err != nil {
		return fmt.Errorf("claim run: %w", err)
	}
	if !claimed {
		log.Printf("[synthesis] run %s already claimed (not pending), skip", runID)
		return nil
	}

	var userID, taskType string
	if err := s.QueryRunInfo(ctx, runID, &userID, &taskType); err != nil {
		_ = repository.UpdateAgentRunStatus(ctx, s.pool, runUUID, "failed", "synthesis", err.Error())
		return fmt.Errorf("query run info: %w", err)
	}

	if _, err := s.GenerateDraft(ctx, runID, userID, taskType); err != nil {
		if updErr := repository.UpdateAgentRunStatus(ctx, s.pool, runUUID, "failed", "synthesis", err.Error()); updErr != nil {
			log.Printf("[synthesis] update run status to failed/synthesis: %v", updErr)
		}
		return fmt.Errorf("generate draft: %w", err)
	}

	if err := repository.UpdateAgentRunStatus(ctx, s.pool, runUUID, "completed", "synthesis", ""); err != nil {
		log.Printf("[synthesis] update run status to completed/synthesis: %v", err)
	}
	return nil
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

	// 状态回写：草稿生成成功，run 标记为 running/synthesis（等待交付）。
	// 状态回写失败不阻断主流程（草稿已生成），仅记日志。
	if runUUID, parseErr := uuid.Parse(runID); parseErr == nil && s.pool != nil {
		if err := repository.UpdateAgentRunStatus(ctx, s.pool, runUUID, "running", "synthesis", ""); err != nil {
			log.Printf("[synthesis] update run status to running/synthesis: %v", err)
		}
	} else if parseErr != nil {
		log.Printf("[synthesis] invalid run id %q: %v", runID, parseErr)
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

// MarshalMarks 导出版本，供 handler 层更新草稿时序列化 marks 用。
func MarshalMarks(marks []Mark) ([]byte, error) {
	return marshalMarks(marks)
}

// ResolveMark 解析草稿中的某个 [待补充:xxx] 占位符，用 value 替换并落库。
// 返回替换后的新 markdown。归属校验由调用方（handler）负责。
func (s *Service) ResolveMark(ctx context.Context, runID, markID, value string) (string, error) {
	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return "", fmt.Errorf("invalid run id: %w", err)
	}
	draft, err := repository.GetDraftByRunID(ctx, s.pool, runUUID)
	if err != nil {
		return "", fmt.Errorf("load draft: %w", err)
	}
	repoMarks, err := repository.UnmarshalMarks(draft.Marks)
	if err != nil {
		return "", fmt.Errorf("unmarshal marks: %w", err)
	}
	marks := make([]Mark, len(repoMarks))
	for i, m := range repoMarks {
		marks[i] = Mark{ID: m.ID, Hint: m.Hint, Position: m.Position, Resolved: m.Resolved}
	}
	newMd, err := ReplaceMark(draft.MarkdownContent, markID, marks, value)
	if err != nil {
		return "", fmt.Errorf("replace mark: %w", err)
	}
	newMarks := ExtractMarks(newMd)
	comp := Completeness(newMd)
	marksJSON, err := marshalMarks(newMarks)
	if err != nil {
		return "", fmt.Errorf("marshal marks: %w", err)
	}
	if err := repository.UpdateDraftMarkdown(ctx, s.pool, runUUID, newMd, marksJSON, comp); err != nil {
		return "", fmt.Errorf("update draft: %w", err)
	}
	return newMd, nil
}

// ChatWithRun 对指定 run 执行单轮 LLM 对话（不跑完整 RAG workflow）。
// 流程：归属校验 → 落库 user message(sent) → 拉历史(含刚落的 user msg) →
// 落库 assistant 占位(streaming) → 加载草稿作为 context → 构造 system+历史 →
// LLM 流式 → WriteChatDelta 累积 → 结束 UpdateMessageStatus(done) / 失败 (error)。
//
// 草稿不存在时不阻断（用空串），允许用户在草稿生成前就开 chat。
func (s *Service) ChatWithRun(ctx context.Context, runID, userID, userMessage string, w *A2UIWriter) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return fmt.Errorf("invalid run id: %w", err)
	}

	// 1. 归属校验 + 查 task_type
	run, err := repository.GetAgentRunByID(ctx, s.pool, uid, runUUID)
	if err != nil {
		return fmt.Errorf("load run: %w", err)
	}

	// 2. 落库 user message（status='sent'）
	userMsg := &repository.AgentRunMessage{
		AgentRunID: runUUID,
		UserID:     uid,
		Role:       "user",
		Content:    userMessage,
		Status:     "sent",
	}
	if _, err := repository.CreateMessage(ctx, s.pool, userMsg); err != nil {
		return fmt.Errorf("save user message: %w", err)
	}

	// 3. 拉历史消息（最近 20 条，含刚落的 user message）
	history, err := repository.ListMessagesByRun(ctx, s.pool, runUUID, 20)
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	// 4/5/6. 并发执行：存 assistant 占位 + 加载草稿 + 获取 LLM 客户端
	// 这三个步骤互不依赖，并行执行压缩耗时
	type step4Result struct {
		id    uuid.UUID
		idStr string
		err   error
	}
	type step5Result struct {
		content string
	}
	type step6Result struct {
		client    LLMClient
		temp      float32
		maxTokens int
		err       error
	}

	ch4 := make(chan step4Result, 1)
	ch5 := make(chan step5Result, 1)
	ch6 := make(chan step6Result, 1)

	// step4: 存 assistant 占位
	go func() {
		assistantMsg := &repository.AgentRunMessage{
			AgentRunID: runUUID,
			UserID:     uid,
			Role:       "assistant",
			Content:    "",
			Status:     "streaming",
		}
		id, err := repository.CreateMessage(ctx, s.pool, assistantMsg)
		ch4 <- step4Result{id: id, idStr: id.String(), err: err}
	}()

	// step5: 加载草稿
	go func() {
		draftMd := ""
		if draft, dErr := repository.GetDraftByRunID(ctx, s.pool, runUUID); dErr == nil {
			draftMd = draft.MarkdownContent
		}
		ch5 <- step5Result{content: draftMd}
	}()

	// step6: 获取 LLM 客户端
	go func() {
		llm, err := s.factory.GetLLM(ctx, uid)
		if err != nil {
			ch6 <- step6Result{err: err}
			return
		}
		ch6 <- step6Result{
			client:    llm,
			temp:      s.factory.GetLLMTemperature(ctx, uid),
			maxTokens: s.factory.GetLLMMaxTokens(ctx, uid),
		}
	}()

	// 等待三个步骤完成
	r4 := <-ch4
	r5 := <-ch5
	r6 := <-ch6

	if r4.err != nil {
		return fmt.Errorf("save assistant placeholder: %w", r4.err)
	}
	assistantID := r4.id
	assistantIDStr := r4.idStr
	draftMd := r5.content

	if r6.err != nil {
		_ = repository.UpdateMessageStatus(ctx, s.pool, assistantID, "error", "", "", r6.err.Error())
		_ = w.WriteChatError(assistantIDStr, r6.err.Error())
		return fmt.Errorf("get llm: %w", r6.err)
	}
	llm := r6.client
	temp := r6.temp
	maxTokens := r6.maxTokens

	// 7. 构造 LLM messages：system（含 task_type + 草稿）+ 历史
	// 历史按 token 预算裁剪：user 全部纳入、assistant 只取 status=done（跳过 error/streaming
	// 避免干扰上下文），超预算时从最旧消息开始裁，但强制保留本轮刚发的最后一条 user 消息。
	systemContent := fmt.Sprintf(
		"你是一个专业的文档助手。当前任务类型：%s。你可以基于已生成的草稿回答用户问题或修改草稿内容。",
		run.TaskType,
	)
	if draftMd != "" {
		systemContent += "\n\n当前草稿内容：\n" + draftMd
	} else {
		systemContent += "\n\n（草稿尚未生成，可直接回答用户问题。）"
	}

	msgs := []Message{{Role: "system", Content: systemContent}}
	msgs = append(msgs, buildChatHistory(history, defaultHistoryTokenBudget)...)

	// 8. LLM 流式调用
	ch, err := llm.Chat(ctx, ChatRequest{
		Messages:    msgs,
		Temperature: temp,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		_ = repository.UpdateMessageStatus(ctx, s.pool, assistantID, "error", "", "", err.Error())
		_ = w.WriteChatError(assistantIDStr, err.Error())
		return fmt.Errorf("llm chat: %w", err)
	}

	// 9. 消费流式 channel，写 chat delta + 累积 fullContent + fullReasoning
	var fullContent strings.Builder
	var fullReasoning strings.Builder
	streamErr := error(nil)
	for chunk := range ch {
		if chunk.Err != nil {
			streamErr = chunk.Err
			break
		}
		if chunk.Done {
			break
		}
		// 推理内容（DeepSeek thinking）- 实时流式发送 + 累积存储
		if chunk.Reasoning != "" {
			fullReasoning.WriteString(chunk.Reasoning)
			if wErr := w.WriteChatReasoning(assistantIDStr, chunk.Reasoning); wErr != nil {
				streamErr = wErr
				break
			}
		}
		// 正式回复内容
		if chunk.Content == "" {
			continue
		}
		fullContent.WriteString(chunk.Content)
		if wErr := w.WriteChatDelta(assistantIDStr, chunk.Content); wErr != nil {
			streamErr = wErr
			break
		}
	}

	if streamErr != nil {
		errMsg := streamErr.Error()
		_ = repository.UpdateMessageStatus(ctx, s.pool, assistantID, "error", "", fullReasoning.String(), errMsg)
		_ = w.WriteChatError(assistantIDStr, errMsg)
		return fmt.Errorf("llm stream: %w", streamErr)
	}

	// 10. 流式成功结束，落库完整 assistant 内容 + 推理内容
	if err := repository.UpdateMessageStatus(ctx, s.pool, assistantID, "done", fullContent.String(), fullReasoning.String(), ""); err != nil {
		log.Printf("[synthesis] update assistant message status to done: %v", err)
	}
	return w.WriteChatComplete(assistantIDStr)
}

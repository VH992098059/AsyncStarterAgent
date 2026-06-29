package synthesis

import (
	"context"
	"fmt"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	"github.com/cloudwego/eino/compose"
)

type workflowState struct {
	AgentRunID   string                   `json:"agent_run_id"`
	UserID       string                   `json:"user_id"`
	TaskType     string                   `json:"task_type"`
	Context      []harvesting.ContextItem `json:"context"`
	Retrieved    []harvesting.ContextItem `json:"retrieved"`
	Template     string                   `json:"template"`
	Draft        string                   `json:"draft"`
	Completeness float32                  `json:"completeness"`
	Marks        []Mark                   `json:"marks"`
	Errors       []error                  `json:"-"`
}

func newWorkflowState(agentRunID, userID, taskType string) workflowState {
	return workflowState{
		AgentRunID: agentRunID,
		UserID:     userID,
		TaskType:   taskType,
	}
}

func (s *workflowState) appendError(err error) {
	s.Errors = append(s.Errors, err)
}

type nodeDef struct {
	Key string
	Fn  func(ctx context.Context, s workflowState) (workflowState, error)
}

func buildWorkflow(ctx context.Context, nodes []nodeDef) (compose.Runnable[workflowState, workflowState], error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("agent: at least one node required")
	}
	wf := compose.NewWorkflow[workflowState, workflowState]()
	for i, nd := range nodes {
		lambda := compose.InvokableLambda(nd.Fn)
		node := wf.AddLambdaNode(nd.Key, lambda)
		if i == 0 {
			node.AddInput(compose.START)
		} else {
			node.AddInput(nodes[i-1].Key)
		}
	}
	wf.End().AddInput(nodes[len(nodes)-1].Key)
	runner, err := wf.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("agent: compile workflow: %w", err)
	}
	return runner, nil
}

func runDAG(ctx context.Context, runner compose.Runnable[workflowState, workflowState], init workflowState) (workflowState, error) {
	result, err := runner.Invoke(ctx, init)
	if err != nil {
		return init, fmt.Errorf("agent: run dag: %w", err)
	}
	return result, nil
}

type draftWorkflow struct {
	runner      compose.Runnable[workflowState, workflowState]
	templateDir string
}

type workflowConfig struct {
	TemplateDir string
	Temperature float32
	MaxTokens   int
}

func newDraftWorkflow(ctx context.Context, rag *RAG, llm LLMClient, cfg workflowConfig) (*draftWorkflow, error) {
	if rag == nil {
		return nil, fmt.Errorf("workflow: rag is required")
	}
	if llm == nil {
		return nil, fmt.Errorf("workflow: llm is required")
	}
	templateDir := cfg.TemplateDir
	temperature := cfg.Temperature
	if temperature <= 0 {
		temperature = 0.3
	}
	maxTokens := cfg.MaxTokens

	nodes := []nodeDef{
		{Key: "retrieve", Fn: wfRetrieveNode(rag)},
		{Key: "template", Fn: wfTemplateNode(templateDir)},
		{Key: "llm", Fn: wfLLMNode(llm, temperature, maxTokens)},
		{Key: "mark", Fn: wfMarkNode()},
	}
	runner, err := buildWorkflow(ctx, nodes)
	if err != nil {
		return nil, fmt.Errorf("workflow build: %w", err)
	}
	return &draftWorkflow{runner: runner, templateDir: templateDir}, nil
}

func (w *draftWorkflow) generate(ctx context.Context, runID, userID, taskType string) (*DraftResult, error) {
	init := newWorkflowState(runID, userID, taskType)
	result, err := runDAG(ctx, w.runner, init)
	if err != nil {
		return nil, fmt.Errorf("workflow generate: %w", err)
	}
	return &DraftResult{
		Draft:        result.Draft,
		Marks:        result.Marks,
		Completeness: result.Completeness,
		Template:     result.Template,
	}, nil
}

func wfRetrieveNode(rag *RAG) func(ctx context.Context, s workflowState) (workflowState, error) {
	return func(ctx context.Context, s workflowState) (workflowState, error) {
		items, err := rag.Retrieve(ctx, s.TaskType, 20)
		if err != nil {
			return s, fmt.Errorf("retrieve: %w", err)
		}
		for _, it := range items {
			s.Retrieved = append(s.Retrieved, scoredItemToContextItem(it))
		}
		s.Completeness = 0.3
		return s, nil
	}
}

func wfTemplateNode(templateDir string) func(ctx context.Context, s workflowState) (workflowState, error) {
	return func(ctx context.Context, s workflowState) (workflowState, error) {
		tplRelPath := SelectByTaskType(s.TaskType)
		tplPath := tplRelPath
		if templateDir != "" {
			tplPath = templateDir + "/" + tplRelPath
		}
		tpl, err := LoadTemplate(s.TaskType, tplPath)
		if err != nil {
			return s, fmt.Errorf("template load: %w", err)
		}
		data := TemplateData{
			TaskType: s.TaskType,
			Title:    s.TaskType,
			Items:    s.Retrieved,
		}
		s.Template, err = tpl.Render(data)
		if err != nil {
			return s, fmt.Errorf("template render: %w", err)
		}
		s.Completeness = 0.5
		return s, nil
	}
}

func wfLLMNode(llm LLMClient, temperature float32, maxTokens int) func(ctx context.Context, s workflowState) (workflowState, error) {
	return func(ctx context.Context, s workflowState) (workflowState, error) {
		pb := PromptBuilder{}
		msgs := pb.Build(s.TaskType, s.Template, s.Retrieved)
		req := ChatRequest{
			Messages:    msgs,
			Temperature: temperature,
		}
		if maxTokens > 0 {
			req.MaxTokens = maxTokens
		}
		ch, err := llm.Chat(ctx, req)
		if err != nil {
			return s, fmt.Errorf("llm chat: %w", err)
		}
		for c := range ch {
			if c.Err != nil {
				return s, fmt.Errorf("llm stream: %w", c.Err)
			}
			s.Draft += c.Content
			if c.Done {
				break
			}
		}
		s.Completeness = Completeness(s.Draft)
		return s, nil
	}
}

func wfMarkNode() func(ctx context.Context, s workflowState) (workflowState, error) {
	return func(ctx context.Context, s workflowState) (workflowState, error) {
		s.Marks = ExtractMarks(s.Draft)
		return s, nil
	}
}

func scoredItemToContextItem(s ScoredItem) harvesting.ContextItem {
	title := s.Content
	if len(title) > 80 {
		title = title[:80]
	}
	return harvesting.ContextItem{
		ID:         s.ID,
		Source:     s.Source,
		Title:      title,
		Content:    s.Content,
		Metadata:   s.Metadata,
		OccurredAt: time.Now(),
	}
}

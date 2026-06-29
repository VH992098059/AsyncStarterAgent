// T023: Notion API 文档交付 — 直接 HTTP 调用 Notion REST API（FR-D01 P0, FR-D04 P0）
// 不引入第三方 Notion SDK，使用 net/http 直接调用官方 API。
package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// NotionConfig holds Notion API configuration (FR-D01).
type NotionConfig struct {
	APIKey       string
	ParentPageID string
}

// NotionAdapter creates and updates Notion pages via the official REST API (FR-D01, FR-D04).
type NotionAdapter struct {
	cfg    NotionConfig
	client *http.Client
}

// NewNotionAdapter creates a Notion adapter with the given config.
func NewNotionAdapter(cfg NotionConfig) *NotionAdapter {
	return &NotionAdapter{
		cfg:    cfg,
		client: &http.Client{},
	}
}

const notionAPIURL = "https://api.notion.com/v1"

// CreatePage creates a Notion page with the given title and markdown content (FR-D01).
// Returns the URL of the created page.
func (a *NotionAdapter) CreatePage(ctx context.Context, title, markdown string) (string, error) {
	blocks := markdownToBlocks(markdown)

	body := map[string]interface{}{
		"parent": map[string]string{
			"page_id": a.cfg.ParentPageID,
		},
		"properties": map[string]interface{}{
			"title": []map[string]interface{}{
				{
					"text": map[string]string{"content": title},
				},
			},
		},
		"children": blocks,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("notion marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notionAPIURL+"/pages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("notion request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("notion create page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("notion create page: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("notion decode: %w", err)
	}

	if result.URL != "" {
		return result.URL, nil
	}
	return "https://notion.so/" + result.ID, nil
}

// UpdateTaskComment adds a comment to a Notion page (FR-D04).
func (a *NotionAdapter) UpdateTaskComment(ctx context.Context, pageID, comment string) error {
	body := map[string]interface{}{
		"parent": map[string]string{
			"page_id": pageID,
		},
		"rich_text": []map[string]interface{}{
			{
				"text": map[string]string{"content": comment},
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("notion comment marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notionAPIURL+"/comments", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("notion comment request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("notion comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("notion comment: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// markdownToBlocks converts markdown to Notion block objects (FR-D01).
// Simplified: each non-empty line becomes a paragraph block.
// Heading lines (# ) become heading blocks.
func markdownToBlocks(md string) []map[string]interface{} {
	lines := strings.Split(md, "\n")
	blocks := make([]map[string]interface{}, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "### ") {
			blocks = append(blocks, headingBlock("heading_3", trimmed[4:]))
		} else if strings.HasPrefix(trimmed, "## ") {
			blocks = append(blocks, headingBlock("heading_2", trimmed[3:]))
		} else if strings.HasPrefix(trimmed, "# ") {
			blocks = append(blocks, headingBlock("heading_1", trimmed[2:]))
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			blocks = append(blocks, listItemBlock(trimmed[2:]))
		} else {
			blocks = append(blocks, paragraphBlock(trimmed))
		}
	}

	return blocks
}

func paragraphBlock(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"type": "text", "text": map[string]string{"content": text}},
			},
		},
	}
}

func headingBlock(level, text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   level,
		level: map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"type": "text", "text": map[string]string{"content": text}},
			},
		},
	}
}

func listItemBlock(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "bulleted_list_item",
		"bulleted_list_item": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"type": "text", "text": map[string]string{"content": text}},
			},
		},
	}
}

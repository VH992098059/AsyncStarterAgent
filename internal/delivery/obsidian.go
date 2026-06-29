// T024: Obsidian Vault 交付（FR-D02 P1）
// 将草稿写入 Obsidian Vault 目录的 .md 文件，添加 frontmatter。
package delivery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ObsidianConfig holds Obsidian Vault configuration (FR-D02, T024).
type ObsidianConfig struct {
	VaultPath string
	Subdir    string // default "Generated"
}

// ObsidianWriter is the interface for writing drafts to an Obsidian vault (FR-D02, T024).
type ObsidianWriter interface {
	WriteFile(ctx context.Context, title, markdown string) (string, error)
}

// ObsidianAdapter implements ObsidianWriter for local vault file writes (FR-D02, T024).
type ObsidianAdapter struct {
	cfg ObsidianConfig
}

// NewObsidianAdapter creates a new Obsidian adapter.
// If Subdir is empty, defaults to "Generated".
func NewObsidianAdapter(cfg ObsidianConfig) *ObsidianAdapter {
	if cfg.Subdir == "" {
		cfg.Subdir = "Generated"
	}
	return &ObsidianAdapter{cfg: cfg}
}

// WriteFile writes a markdown draft to the Obsidian vault (FR-D02, T024).
// Creates the file at {VaultPath}/{Subdir}/{title}.md with YAML frontmatter.
// Returns a file:// URL pointing to the created file.
func (a *ObsidianAdapter) WriteFile(_ context.Context, title, markdown string) (string, error) {
	dir := filepath.Join(a.cfg.VaultPath, a.cfg.Subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	filename := sanitizeFilename(title) + ".md"
	path := filepath.Join(dir, filename)
	body := "---\ncreated: " + time.Now().Format(time.RFC3339) + "\n---\n\n" + markdown
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	return "file://" + filepath.ToSlash(path), nil
}

// sanitizeFilename removes characters that are invalid in file names (FR-D02, T024).
func sanitizeFilename(s string) string {
	invalid := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	for _, c := range invalid {
		s = strings.ReplaceAll(s, c, "_")
	}
	return strings.TrimSpace(s)
}

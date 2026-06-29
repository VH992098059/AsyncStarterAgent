package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// errObsidianMaxFiles is a sentinel returned from the walk callback once the
// configured MaxFiles limit has been reached, so the walk stops early without
// being treated as a real failure.
var errObsidianMaxFiles = errors.New("obsidian: max files limit reached")

// ObsidianConfig configures the ObsidianAdapter (FR-B04).
type ObsidianConfig struct {
	VaultPath string // Vault 根目录
	MaxDepth  int    // 默认 3
	MaxFiles  int    // 默认 200
}

// ObsidianAdapter reads local Obsidian vault notes and implements the
// sourceAdapter interface (Name + Fetch). It walks the vault directory
// directly — no provider abstraction layer.
type ObsidianAdapter struct {
	cfg ObsidianConfig
}

// NewObsidianAdapter applies defaults (MaxDepth=3, MaxFiles=200) when the
// corresponding zero values are supplied.
func NewObsidianAdapter(cfg ObsidianConfig) *ObsidianAdapter {
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 3
	}
	if cfg.MaxFiles == 0 {
		cfg.MaxFiles = 200
	}
	return &ObsidianAdapter{cfg: cfg}
}

// Name returns the data source name for adapter routing.
func (a *ObsidianAdapter) Name() string { return "obsidian" }

// Fetch walks the vault and returns .md notes whose ModTime is not before
// `since`. It respects MaxDepth (prunes over-deep directories) and MaxFiles
// (stops once the limit is reached). The userID parameter is stored on each
// ContextItem for incremental sync and persistence (FR-B06).
//
// Depth semantics: depth = number of path separators in the path relative to
// VaultPath. The root itself is depth 0. A file is included when its depth <=
// MaxDepth. A directory (other than the root) is pruned via filepath.SkipDir
// when its depth > MaxDepth.
func (a *ObsidianAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	var items []harvesting.ContextItem
	walkErr := filepath.WalkDir(a.cfg.VaultPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip entries we cannot access (e.g. permission errors) without
			// aborting the whole walk.
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		rel, relErr := filepath.Rel(a.cfg.VaultPath, path)
		if relErr != nil {
			return fmt.Errorf("compute relative path for %s: %w", path, relErr)
		}
		depth := strings.Count(rel, string(filepath.Separator))

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		if info.IsDir() {
			if path != a.cfg.VaultPath && depth > a.cfg.MaxDepth {
				return filepath.SkipDir
			}
			return nil
		}

		// File-level filters.
		if depth > a.cfg.MaxDepth {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		if info.ModTime().Before(since) {
			return nil
		}
		if len(items) >= a.cfg.MaxFiles {
			return errObsidianMaxFiles
		}

		body, readErr := os.ReadFile(path)
		if readErr != nil {
			// Skip unreadable files without aborting the walk.
			return nil
		}

		base := filepath.Base(path)
		title := strings.TrimSuffix(base, filepath.Ext(base))
		items = append(items, harvesting.ContextItem{
			ID:         "obsidian:note:" + path,
			UserID:     userID,
			Source:     "obsidian",
			Type:       "note",
			Title:      title,
			Content:    string(body),
			URL:        "file://" + filepath.ToSlash(path),
			OccurredAt: info.ModTime(),
		})
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errObsidianMaxFiles) {
		return nil, fmt.Errorf("obsidian adapter fetch: %w", walkErr)
	}
	return items, nil
}

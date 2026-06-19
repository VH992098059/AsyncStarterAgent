package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// Compile-time check: ObsidianAdapter satisfies the sourceAdapter interface shape.
// (The sourceAdapter interface itself is defined in T014's pipeline.go; until then
// we assert the method signatures directly.)
var _ interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)
} = (*ObsidianAdapter)(nil)

// writeNote creates a file at relpath under dir with the given body and mtime.
func writeNote(t *testing.T, dir, relpath, body string, mtime time.Time) {
	t.Helper()
	full := filepath.Join(dir, relpath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	if err := os.Chtimes(full, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", full, err)
	}
}

func findItemByTitle(items []harvesting.ContextItem, title string) (harvesting.ContextItem, bool) {
	for _, it := range items {
		if it.Title == title {
			return it, true
		}
	}
	return harvesting.ContextItem{}, false
}

func timeApprox(a, b time.Time) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d <= time.Second
}

func TestObsidianAdapter_Name(t *testing.T) {
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: t.TempDir()})
	if a.Name() != "obsidian" {
		t.Errorf("expected obsidian, got %s", a.Name())
	}
}

func TestObsidianAdapter_Fetch_DefaultsApplied(t *testing.T) {
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: t.TempDir()})
	if a.cfg.MaxDepth != 3 {
		t.Errorf("expected default MaxDepth=3, got %d", a.cfg.MaxDepth)
	}
	if a.cfg.MaxFiles != 200 {
		t.Errorf("expected default MaxFiles=200, got %d", a.cfg.MaxFiles)
	}
}

func TestObsidianAdapter_Fetch(t *testing.T) {
	dir := t.TempDir()
	future := time.Now().Add(1 * time.Hour)
	writeNote(t, dir, "note1.md", "# Note 1", future)
	writeNote(t, dir, "note2.md", "# Note 2", future)
	writeNote(t, dir, "ignored.txt", "skip", future)

	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	items, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (only .md), got %d", len(items))
	}

	it, ok := findItemByTitle(items, "note1")
	if !ok {
		t.Fatalf("expected item with title note1, items: %+v", items)
	}
	if it.Source != "obsidian" {
		t.Errorf("Source: expected obsidian, got %s", it.Source)
	}
	if it.Type != "note" {
		t.Errorf("Type: expected note, got %s", it.Type)
	}
	if it.Title != "note1" {
		t.Errorf("Title: expected note1, got %s", it.Title)
	}
	if it.Content != "# Note 1" {
		t.Errorf("Content: expected '# Note 1', got %q", it.Content)
	}
	if !strings.HasPrefix(it.URL, "file://") {
		t.Errorf("URL: expected file:// scheme, got %s", it.URL)
	}
	if it.OccurredAt.IsZero() {
		t.Errorf("OccurredAt should be set")
	}
	if !timeApprox(it.OccurredAt, future) {
		t.Errorf("OccurredAt: expected ~%v, got %v", future, it.OccurredAt)
	}
}

func TestObsidianAdapter_Fetch_FiltersByMtime(t *testing.T) {
	dir := t.TempDir()
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)
	writeNote(t, dir, "old.md", "old", past)
	writeNote(t, dir, "new.md", "new", future)

	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	items, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (only new.md), got %d", len(items))
	}
	if items[0].Title != "new" {
		t.Errorf("expected title new, got %s", items[0].Title)
	}
}

func TestObsidianAdapter_Fetch_EmptyVault(t *testing.T) {
	dir := t.TempDir()
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	items, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestObsidianAdapter_Fetch_RespectsMaxFiles(t *testing.T) {
	dir := t.TempDir()
	future := time.Now().Add(1 * time.Hour)
	for i := 0; i < 5; i++ {
		writeNote(t, dir, fmt.Sprintf("file%d.md", i), "body", future)
	}

	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir, MaxFiles: 3})
	items, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items (MaxFiles=3), got %d", len(items))
	}
}

// Depth semantics (number of path separators in the path relative to VaultPath):
//
//	root               -> rel="."             -> depth 0
//	root/a             -> rel="a"             -> depth 0
//	root/a/b           -> rel="a/b"           -> depth 1
//	root/a/b/c         -> rel="a/b/c"         -> depth 2
//	root/a/shallow.md  -> rel="a/shallow.md"  -> depth 1
//	root/a/b/c/deep.md -> rel="a/b/c/deep.md" -> depth 3
//
// A file is included when depth <= MaxDepth; otherwise skipped.
// A directory (other than the root) is pruned via filepath.SkipDir when its
// depth > MaxDepth. With MaxDepth=2: shallow.md (depth 1) is included and
// deep.md (depth 3) is skipped.
func TestObsidianAdapter_Fetch_RespectsMaxDepth(t *testing.T) {
	dir := t.TempDir()
	future := time.Now().Add(1 * time.Hour)
	writeNote(t, dir, filepath.Join("a", "shallow.md"), "shallow", future)
	writeNote(t, dir, filepath.Join("a", "b", "c", "deep.md"), "deep", future)

	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir, MaxDepth: 2})
	items, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (shallow only), got %d", len(items))
	}
	if items[0].Title != "shallow" {
		t.Errorf("expected title shallow, got %s", items[0].Title)
	}
}

func TestObsidianAdapter_Fetch_OnlyMarkdown(t *testing.T) {
	// Windows filesystems are case-insensitive, so we use distinct base names to
	// guarantee all four files coexist. The test still verifies that ".md" suffix
	// matching is case-insensitive and that ".markdown" is NOT matched.
	dir := t.TempDir()
	future := time.Now().Add(1 * time.Hour)
	writeNote(t, dir, "alpha.md", "a", future)
	writeNote(t, dir, "beta.MD", "b", future)
	writeNote(t, dir, "gamma.txt", "g", future)
	writeNote(t, dir, "delta.markdown", "d", future)

	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	items, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (alpha.md + beta.MD), got %d", len(items))
	}
	titles := map[string]bool{}
	for _, it := range items {
		titles[it.Title] = true
	}
	if !titles["alpha"] {
		t.Errorf("expected alpha (alpha.md) to be included")
	}
	if !titles["beta"] {
		t.Errorf("expected beta (beta.MD) to be included")
	}
}

func TestObsidianAdapter_Fetch_RespectsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	future := time.Now().Add(1 * time.Hour)
	writeNote(t, dir, "note.md", "# Note", future)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	items, err := a.Fetch(ctx, "user-1", time.Now())
	if err == nil {
		t.Fatalf("expected error from cancelled context, got nil (items=%d)", len(items))
	}
}

func TestObsidianAdapter_Fetch_NonexistentVault(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: missing})
	items, err := a.Fetch(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("expected no error for nonexistent vault (graceful degradation), got: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for nonexistent vault, got %d", len(items))
	}
}

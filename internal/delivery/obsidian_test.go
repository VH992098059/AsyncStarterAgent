package delivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestObsidianAdapter_WriteFile tests writing a draft to the vault (FR-D02, T024).
func TestObsidianAdapter_WriteFile(t *testing.T) {
	dir := t.TempDir()
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	url, err := a.WriteFile(context.Background(), "My Draft", "# Hello\n\nContent")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "file://") {
		t.Errorf("expected file:// URL, got %s", url)
	}
	// Verify file exists
	path := filepath.Join(dir, "Generated", "My Draft.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if !strings.Contains(content, "# Hello") {
		t.Errorf("body missing content: %s", content)
	}
	if !strings.Contains(content, "---\ncreated:") {
		t.Errorf("body missing frontmatter: %s", content)
	}
}

// TestObsidianAdapter_WriteFile_CustomSubdir tests custom subdirectory (FR-D02, T024).
func TestObsidianAdapter_WriteFile_CustomSubdir(t *testing.T) {
	dir := t.TempDir()
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir, Subdir: "Reports"})
	url, err := a.WriteFile(context.Background(), "Weekly", "# Report")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "Reports") {
		t.Errorf("URL should contain Reports: %s", url)
	}
	// Verify file in custom subdir
	path := filepath.Join(dir, "Reports", "Weekly.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file not found at %s", path)
	}
}

// TestObsidianAdapter_WriteFile_DefaultSubdir tests default subdir is "Generated" (FR-D02, T024).
func TestObsidianAdapter_WriteFile_DefaultSubdir(t *testing.T) {
	dir := t.TempDir()
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	if a.cfg.Subdir != "Generated" {
		t.Errorf("default subdir should be Generated, got %s", a.cfg.Subdir)
	}
}

// TestObsidianAdapter_WriteFile_CreatesDir tests that missing directories are created (FR-D02, T024).
func TestObsidianAdapter_WriteFile_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	a := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	_, err := a.WriteFile(context.Background(), "Test", "content")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "Generated"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("Generated should be a directory")
	}
}

// TestSanitizeFilename tests illegal character removal (FR-D02, T024).
func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"valid name":    "valid name",
		"with/slash":    "with_slash",
		"with:colon":    "with_colon",
		"with?question": "with_question",
		"with<angle>":   "with_angle_",
		"with*star":     "with_star",
		"with|pipe":     "with_pipe",
		`with"quote`:    "with_quote",
		"with\\back":    "with_back",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("%q: want %q, got %q", in, want, got)
		}
	}
}

// TestSanitizeFilename_TrimsWhitespace tests whitespace trimming (FR-D02, T024).
func TestSanitizeFilename_TrimsWhitespace(t *testing.T) {
	if got := sanitizeFilename("  hello  "); got != "hello" {
		t.Errorf("want 'hello', got %q", got)
	}
}

// TestObsidianAdapter_SatisfiesInterface verifies ObsidianAdapter implements ObsidianWriter (T024).
func TestObsidianAdapter_SatisfiesInterface(t *testing.T) {
	var _ ObsidianWriter = (*ObsidianAdapter)(nil)
}

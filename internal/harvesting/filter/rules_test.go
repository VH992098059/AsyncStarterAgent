package filter

import (
	"regexp"
	"testing"

	"github.com/asyncstarter/agent/internal/harvesting"
)

func TestRuleFilter_Apply(t *testing.T) {
	f := NewRuleFilter(DefaultRules())
	items := []harvesting.ContextItem{
		{Title: "Merge pull request #42 from feature/xx", Type: "commit"},
		{Title: "chore: bump dependencies", Type: "commit"},
		{Title: "feat: add login flow", Type: "commit"},
		{Title: "fix: typo in docs", Type: "pr"},
		{Title: "", Content: "", Type: "commit"},
		{Title: "Team standup", Content: "discussed Q3 OKR", Type: "meeting"},
	}
	kept, rejections := f.Apply(items)
	if len(kept) != 2 {
		t.Errorf("expected 2 kept, got %d", len(kept))
	}
	if rejections["merge-commit"] != 1 {
		t.Errorf("merge-commit: %d", rejections["merge-commit"])
	}
	if rejections["deps-bump"] != 1 {
		t.Errorf("deps-bump: %d", rejections["deps-bump"])
	}
	if rejections["empty"] != 1 {
		t.Errorf("empty: %d", rejections["empty"])
	}
}

func TestRuleFilter_Apply_AllKept(t *testing.T) {
	f := NewRuleFilter(DefaultRules())
	items := []harvesting.ContextItem{
		{Title: "feat: add user auth", Content: "implement JWT auth", Type: "commit"},
		{Title: "Sprint planning", Content: "planned next sprint", Type: "meeting"},
	}
	kept, rejections := f.Apply(items)
	if len(kept) != 2 {
		t.Errorf("expected 2 kept, got %d", len(kept))
	}
	if len(rejections) != 0 {
		t.Errorf("expected 0 rejections, got %d", len(rejections))
	}
}

func TestRuleFilter_Apply_EmptyInput(t *testing.T) {
	f := NewRuleFilter(DefaultRules())
	kept, rejections := f.Apply(nil)
	if len(kept) != 0 {
		t.Errorf("expected 0 kept, got %d", len(kept))
	}
	if len(rejections) != 0 {
		t.Errorf("expected 0 rejections, got %d", len(rejections))
	}
}

func TestRuleFilter_Apply_CustomRules(t *testing.T) {
	custom := []Rule{
		{Name: "auto-generated", Pattern: regexp.MustCompile(`(?i)^auto-generated`), Reason: "auto-generated code"},
	}
	f := NewRuleFilter(custom)
	items := []harvesting.ContextItem{
		{Title: "auto-generated: swagger client", Type: "commit"},
		{Title: "feat: add API endpoint", Type: "commit"},
	}
	kept, rejections := f.Apply(items)
	if len(kept) != 1 {
		t.Errorf("expected 1 kept, got %d", len(kept))
	}
	if rejections["auto-generated"] != 1 {
		t.Errorf("auto-generated: %d", rejections["auto-generated"])
	}
}

func TestRuleFilter_Apply_ContentMatch(t *testing.T) {
	f := NewRuleFilter(DefaultRules())
	items := []harvesting.ContextItem{
		{Title: "Update config", Content: "bump version to 2.0.0", Type: "commit"},
	}
	kept, rejections := f.Apply(items)
	if len(kept) != 0 {
		t.Errorf("expected 0 kept (content matched bump-version), got %d", len(kept))
	}
	if rejections["bump-version"] != 1 {
		t.Errorf("bump-version: %d", rejections["bump-version"])
	}
}

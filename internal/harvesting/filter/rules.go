// Package filter implements noise filtering for harvested context items (FR-B05).
package filter

import (
	"regexp"
	"strings"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// Rule defines a single rejection rule with a name, regex pattern, and reason.
type Rule struct {
	Name    string
	Pattern *regexp.Regexp
	Reason  string
}

// RuleFilter applies a set of rules to filter out noise from context items.
type RuleFilter struct {
	rules []Rule
}

// NewRuleFilter creates a RuleFilter with the given rules.
func NewRuleFilter(rules []Rule) *RuleFilter { return &RuleFilter{rules: rules} }

// DefaultRules returns the built-in noise rejection rules (FR-B05).
func DefaultRules() []Rule {
	return []Rule{
		{Name: "merge-commit", Pattern: regexp.MustCompile(`^Merge (pull request|branch)`), Reason: "merge commit"},
		{Name: "deps-bump", Pattern: regexp.MustCompile(`^(chore|build|ci):.*(bump|upgrade|update)`), Reason: "dependency bump"},
		{Name: "formatting", Pattern: regexp.MustCompile(`^(style|format):`), Reason: "formatting only"},
		{Name: "typo", Pattern: regexp.MustCompile(`(?i)\btypo\b`), Reason: "typo fix"},
		{Name: "bump-version", Pattern: regexp.MustCompile(`bump.*version`), Reason: "version bump"},
	}
}

// Apply filters items using the rule set. Returns kept items and a map of
// rule-name → rejection count.
func (f *RuleFilter) Apply(items []harvesting.ContextItem) (kept []harvesting.ContextItem, rejections map[string]int) {
	rejections = make(map[string]int)
	for _, it := range items {
		matched := ""
		for _, r := range f.rules {
			if r.Pattern.MatchString(it.Title) || r.Pattern.MatchString(it.Content) {
				matched = r.Name
				break
			}
		}
		if matched != "" {
			rejections[matched]++
			continue
		}
		if strings.TrimSpace(it.Content) == "" && strings.TrimSpace(it.Title) == "" {
			rejections["empty"]++
			continue
		}
		kept = append(kept, it)
	}
	return kept, rejections
}

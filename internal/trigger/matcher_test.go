package trigger

import (
	"testing"
)

func TestMatcher_WeeklyReport(t *testing.T) {
	m := NewMatcher(defaultRules())
	cases := []struct {
		text      string
		wantType  string
		wantMatch bool
	}{
		{"写本周周报", "weekly_report", true},
		{"准备周报", "weekly_report", true},
		{"周报", "weekly_report", true},
		{"项目总结", "summary", true},
		{"整理一下会议纪要", "meeting_minutes", true},
		{"规划下季度", "plan", true},
		{"买菜", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := m.Match(c.text)
		if ok != c.wantMatch {
			t.Errorf("text=%q: want match=%v, got %v", c.text, c.wantMatch, ok)
		}
		if ok && got != c.wantType {
			t.Errorf("text=%q: want type=%s, got %s", c.text, c.wantType, got)
		}
	}
}

func TestMatcher_CustomRule(t *testing.T) {
	m := NewMatcher(defaultRules())
	m.AddRule(Rule{Pattern: `(?i)retro`, TaskType: "summary"})
	got, ok := m.Match("today retro")
	if !ok || got != "summary" {
		t.Fatalf("custom rule failed: %s %v", got, ok)
	}
}

func TestMatcher_LongestMatchWins(t *testing.T) {
	m := NewMatcher(defaultRules())
	// "项目总结" 长度 > "总结"；应返回 summary（更具体）
	got, ok := m.Match("项目总结")
	if !ok || got != "summary" {
		t.Fatalf("expected summary, got %s ok=%v", got, ok)
	}
}

// TestMatcher_EqualLengthTiebreak 验证等长命中时按 rule 注册顺序（先注册者优先）。
// 例: "周报总结" 同时被 "周报" (rule 1, len 2) 和 "总结|小结" (rule 2, len 2) 命中，
// 应返回 rule 1 的 weekly_report（先注册者优先）。
func TestMatcher_EqualLengthTiebreak(t *testing.T) {
	m := NewMatcher(defaultRules())
	got, ok := m.Match("周报总结")
	if !ok || got != "weekly_report" {
		t.Fatalf("expected weekly_report (rule 1 wins on tie), got %s ok=%v", got, ok)
	}
}

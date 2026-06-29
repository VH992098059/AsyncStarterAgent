package trigger

import (
	"regexp"
	"sort"
)

type Rule struct {
	Pattern  string
	TaskType string
}

type Matcher struct {
	rules []compiledRule
}

type compiledRule struct {
	re       *regexp.Regexp
	taskType string
	raw      string
}

func NewMatcher(rules []Rule) *Matcher {
	m := &Matcher{}
	for _, r := range rules {
		m.AddRule(r)
	}
	return m
}

func (m *Matcher) AddRule(r Rule) {
	re := regexp.MustCompile(r.Pattern)
	m.rules = append(m.rules, compiledRule{re: re, taskType: r.TaskType, raw: r.Pattern})
}

func (m *Matcher) Match(text string) (string, bool) {
	type hit struct {
		taskType string
		length   int
		raw      string
	}
	var hits []hit
	for _, r := range m.rules {
		if loc := r.re.FindStringIndex(text); loc != nil {
			hits = append(hits, hit{taskType: r.taskType, length: loc[1] - loc[0], raw: r.raw})
		}
	}
	if len(hits) == 0 {
		return "", false
	}
	// 最长匹配优先（按规范：关键词冲突时取最长匹配）。
	// 使用 SliceStable 保证等长命中时按 rule 注册顺序（先注册者优先），行为可预期。
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].length > hits[j].length })
	return hits[0].taskType, true
}

func defaultRules() []Rule {
	return []Rule{
		{Pattern: `周报`, TaskType: "weekly_report"},
		{Pattern: `总结|小结`, TaskType: "summary"},
		{Pattern: `纪要|会议记录`, TaskType: "meeting_minutes"},
		{Pattern: `规划|计划`, TaskType: "plan"},
	}
}

// DefaultMatcherRules is the exported alias used by tests / wire
func DefaultMatcherRules() []Rule { return defaultRules() }

// Rules 返回当前已注册的规则列表（只读快照）。
// 用于 GET /api/v1/keywords 端点暴露给前端展示。
func (m *Matcher) Rules() []Rule {
	out := make([]Rule, len(m.rules))
	for i, r := range m.rules {
		out[i] = Rule{Pattern: r.raw, TaskType: r.taskType}
	}
	return out
}

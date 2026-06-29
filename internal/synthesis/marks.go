package synthesis

import (
	"fmt"
	"regexp"
)

// Mark represents a [待补充:xxx] placeholder in the generated draft.
// This is the synthesis-package representation used for extraction/replacement.
type Mark struct {
	ID       string `json:"id"`
	Hint     string `json:"hint"`
	Position int    `json:"position"`
	Resolved bool   `json:"resolved"`
}

var markRe = regexp.MustCompile(`\[待补充:([^\]]+)\]`)

// ExtractMarks extracts all [待补充:xxx] placeholders from markdown.
func ExtractMarks(md string) []Mark {
	matches := markRe.FindAllStringSubmatchIndex(md, -1)
	out := make([]Mark, 0, len(matches))
	for i, m := range matches {
		hint := md[m[2]:m[3]]
		uid := fmt.Sprintf("mark-%d", i)
		out = append(out, Mark{
			ID:       uid,
			Hint:     hint,
			Position: m[0],
		})
	}
	return out
}

// ReplaceMark replaces the single mark identified by markID using the provided value.
// Items must contain the mark metadata; replacement is performed by locating the
// original placeholder at its recorded position, so duplicate hints are handled
// correctly. The replacement value is treated as literal text (no regex expansion).
func ReplaceMark(md string, markID string, items []Mark, value string) (string, error) {
	for _, m := range items {
		if m.ID != markID {
			continue
		}
		placeholder := "[待补充:" + m.Hint + "]"
		end := m.Position + len(placeholder)
		if end > len(md) || md[m.Position:end] != placeholder {
			return md, fmt.Errorf("mark position mismatch: %s", markID)
		}
		return md[:m.Position] + value + md[end:], nil
	}
	return md, fmt.Errorf("mark not found: %s", markID)
}

// Completeness calculates a 0.0-1.0 completeness score based on the number of
// [待补充:xxx] marks in the markdown. More marks => lower completeness.
func Completeness(md string) float32 {
	total := float32(len(md))
	marks := float32(len(ExtractMarks(md)))
	if total == 0 {
		return 0
	}
	markPenalty := marks * 20.0
	if markPenalty > total {
		return 0
	}
	return 1.0 - markPenalty/total
}

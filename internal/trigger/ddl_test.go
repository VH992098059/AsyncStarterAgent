package trigger

import (
	"testing"
	"time"
)

func TestDDLDetector_WithinLeadTime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		deadline time.Time
		lead     time.Duration
		want     bool
	}{
		{"due_in_12h_with_24h_lead", now.Add(12 * time.Hour), 24 * time.Hour, true},
		{"due_in_48h_with_24h_lead", now.Add(48 * time.Hour), 24 * time.Hour, false},
		{"overdue", now.Add(-1 * time.Hour), 24 * time.Hour, true},
		{"due_in_exactly_lead", now.Add(24 * time.Hour), 24 * time.Hour, true},
	}
	d := NewDDLDetector()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := d.ShouldTrigger(c.deadline, c.lead, now)
			if got != c.want {
				t.Errorf("deadline=%v lead=%v want=%v got=%v", c.deadline, c.lead, c.want, got)
			}
		})
	}
}

func TestDDLDetector_DefaultLeadTime(t *testing.T) {
	d := NewDDLDetector()
	if d.DefaultLead() != 24*time.Hour {
		t.Fatalf("expected 24h default, got %v", d.DefaultLead())
	}
}

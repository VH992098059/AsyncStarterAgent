package synthesis

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

func TestTemplate_LoadAndRender(t *testing.T) {
	dir := t.TempDir()
	tplPath := filepath.Join(dir, "test.tmpl")
	err := os.WriteFile(tplPath, []byte("# {{.Title}}\n{{range .Items}}- {{.Title}}\n{{end}}"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := LoadTemplate("test", tplPath)
	if err != nil {
		t.Fatal(err)
	}
	data := TemplateData{
		Title: "Week",
		Items: []harvesting.ContextItem{{Title: "A"}, {Title: "B"}},
		Now:   time.Now(),
	}
	out, err := tpl.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(out, "Week") || !contains(out, "A") || !contains(out, "B") {
		t.Errorf("render output: %s", out)
	}
}

func TestTemplate_SelectByTaskType(t *testing.T) {
	cases := map[string]string{
		"weekly_report":   "templates/weekly_report.md.tmpl",
		"summary":         "templates/summary.md.tmpl",
		"plan":            "templates/plan.md.tmpl",
		"meeting_minutes": "templates/meeting_minutes.md.tmpl",
		"unknown":         "templates/summary.md.tmpl",
	}
	for in, want := range cases {
		if got := SelectByTaskType(in); got != want {
			t.Errorf("%s: want %s, got %s", in, want, got)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

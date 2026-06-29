package synthesis

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type Template struct {
	tpl *template.Template
}

type TemplateData struct {
	TaskType   string
	Title      string
	Items      []harvesting.ContextItem
	Now        time.Time
	UserID     string
	CustomVars map[string]string
}

func LoadTemplate(name, path string) (*Template, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", path, err)
	}
	tpl, err := template.New(name).Funcs(funcMap()).Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	return &Template{tpl: tpl}, nil
}

func (t *Template) Render(data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := t.tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"upper": strings.ToUpper,
		"join":  strings.Join,
		"date":  func(format string) string { return time.Now().Format(format) },
	}
}

func SelectByTaskType(taskType string) string {
	switch taskType {
	case "weekly_report":
		return "templates/weekly_report.md.tmpl"
	case "summary":
		return "templates/summary.md.tmpl"
	case "plan":
		return "templates/plan.md.tmpl"
	case "meeting_minutes":
		return "templates/meeting_minutes.md.tmpl"
	default:
		return "templates/summary.md.tmpl"
	}
}

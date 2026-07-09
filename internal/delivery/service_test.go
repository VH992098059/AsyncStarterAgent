package delivery

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestExtractPageID(t *testing.T) {
	cases := map[string]string{
		"https://notion.so/Page-abc123":   "abc123",
		"https://notion.so/abc123":        "abc123",
		"https://example.com/x/y/xyz-987": "987",
	}
	for in, want := range cases {
		if got := extractPageID(in); got != want {
			t.Errorf("%s: want %s, got %s", in, want, got)
		}
	}
}

func TestExtractPageID_NoSlash(t *testing.T) {
	got := extractPageID("abc123")
	if got != "abc123" {
		t.Errorf("want abc123, got %s", got)
	}
}

type mockNotifier struct {
	lastTitle string
	lastBody  string
}

func (m *mockNotifier) Notify(_ context.Context, title, body string) error {
	m.lastTitle = title
	m.lastBody = body
	return nil
}

type mockFactory struct {
	notion *NotionAdapter
	obs    *ObsidianAdapter
}

func (m *mockFactory) GetNotionAdapter(_ context.Context, _ uuid.UUID) (*NotionAdapter, error) {
	if m.notion == nil {
		return nil, fmt.Errorf("notion not configured")
	}
	return m.notion, nil
}

func (m *mockFactory) GetObsidianAdapter(_ context.Context, _ uuid.UUID) (*ObsidianAdapter, error) {
	if m.obs == nil {
		return nil, fmt.Errorf("obsidian not configured")
	}
	return m.obs, nil
}

func (m *mockFactory) GetFeishuAdapter(_ context.Context, _ uuid.UUID) (*FeishuAdapter, error) {
	return nil, fmt.Errorf("feishu not configured")
}

func TestNewService(t *testing.T) {
	notif := &mockNotifier{}
	svc := NewService(nil, &mockFactory{}, notif)
	if svc == nil {
		t.Fatal("nil service")
	}
	if svc.notif == nil {
		t.Error("expected notif to be set")
	}
}

func TestNewService_NilNotifier(t *testing.T) {
	svc := NewService(nil, &mockFactory{}, nil)
	if svc == nil {
		t.Fatal("nil service")
	}
	if svc.notif != nil {
		t.Error("expected nil notif")
	}
}

func TestDeliver_UnsupportedTarget(t *testing.T) {
	svc := NewService(nil, &mockFactory{}, nil)
	_, err := svc.Deliver(context.Background(), "user-1", "draft-1", "unsupported")
	if err == nil {
		t.Fatal("expected error for unsupported target")
	}
}

func TestDeliver_NotionNotConfigured(t *testing.T) {
	svc := NewService(nil, &mockFactory{}, nil)
	_, err := svc.Deliver(context.Background(), "user-1", "draft-1", "notion")
	if err == nil {
		t.Fatal("expected error for nil notion adapter")
	}
}

func TestDeliver_ObsidianNotConfigured(t *testing.T) {
	svc := NewService(nil, &mockFactory{}, nil)
	_, err := svc.Deliver(context.Background(), "user-1", "draft-1", "obsidian")
	if err == nil {
		t.Fatal("expected error for nil obsidian adapter")
	}
}

func TestDeliver_ObsidianWriteFile(t *testing.T) {
	dir := t.TempDir()
	obs := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	fac := &mockFactory{obs: obs}
	svc := NewService(nil, fac, nil)

	_, err := svc.Deliver(context.Background(), "user-1", "draft-1", "obsidian")
	if err == nil {
		t.Fatal("expected error due to nil pool")
	}
	if err.Error() == "obsidian adapter not configured" {
		t.Error("obsidian adapter should be configured")
	}
}

func TestNewService_WithObsidian(t *testing.T) {
	dir := t.TempDir()
	obs := NewObsidianAdapter(ObsidianConfig{VaultPath: dir})
	fac := &mockFactory{obs: obs}
	svc := NewService(nil, fac, nil)
	if svc == nil {
		t.Fatal("nil service")
	}
}

func TestParseFeishuTaskGUID(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantGUID string
		wantOK   bool
	}{
		{"valid guid", "feishu:task:abc-123-def", "abc-123-def", true},
		{"valid guid with dashes", "feishu:task:550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000", true},
		{"not feishu", "写周报", "", false},
		{"keyword source", "keyword", "", false},
		{"empty", "", "", false},
		{"partial prefix no colon suffix", "feishu:task", "", false},
		{"feishu:task: empty guid", "feishu:task:", "", false},
		{"only prefix", "feishu:task", "", false},
		{"feishu prefix only", "feishu", "", false},
		{"todoist source", "todoist", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			guid, ok := parseFeishuTaskGUID(tc.src)
			if ok != tc.wantOK {
				t.Errorf("ok: want %v, got %v (src=%q)", tc.wantOK, ok, tc.src)
			}
			if guid != tc.wantGUID {
				t.Errorf("guid: want %q, got %q (src=%q)", tc.wantGUID, guid, tc.src)
			}
		})
	}
}

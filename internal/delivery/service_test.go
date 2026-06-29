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

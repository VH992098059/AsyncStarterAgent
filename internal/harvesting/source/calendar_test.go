package source

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

type mockCalendar struct {
	events []harvesting.ContextItem
	err    error
}

func (m *mockCalendar) ListEvents(_ context.Context, _, _ time.Time) ([]harvesting.ContextItem, error) {
	return m.events, m.err
}

func TestCalendarAdapter_Fetch(t *testing.T) {
	mock := &mockCalendar{events: []harvesting.ContextItem{
		{ID: "1", Title: "Team Standup", Type: "meeting"},
	}}
	a := &CalendarAdapter{Provider: mock, Source: "google_calendar"}
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal("expected 1")
	}
	if items[0].Source != "google_calendar" {
		t.Errorf("source: %s", items[0].Source)
	}
	if items[0].Type != "meeting" {
		t.Errorf("type: %s", items[0].Type)
	}
}

func TestCalendarAdapter_Name(t *testing.T) {
	a := &CalendarAdapter{Source: "google_calendar"}
	if a.Name() != "google_calendar" {
		t.Errorf("expected google_calendar, got %s", a.Name())
	}
}

func TestCalendarAdapter_Fetch_DefaultType(t *testing.T) {
	mock := &mockCalendar{events: []harvesting.ContextItem{
		{ID: "2", Title: "Sprint Planning", Type: ""},
	}}
	a := &CalendarAdapter{Provider: mock, Source: "google_calendar"}
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal("expected 1")
	}
	if items[0].Type != "meeting" {
		t.Errorf("expected default type meeting, got %s", items[0].Type)
	}
}

func TestCalendarAdapter_Fetch_EmptyResult(t *testing.T) {
	mock := &mockCalendar{events: []harvesting.ContextItem{}}
	a := &CalendarAdapter{Provider: mock, Source: "google_calendar"}
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0, got %d", len(items))
	}
}

func TestCalendarAdapter_Fetch_ProviderError(t *testing.T) {
	mock := &mockCalendar{err: errors.New("api unavailable")}
	a := &CalendarAdapter{Provider: mock, Source: "google_calendar"}
	_, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, mock.err) {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

// Compile-time check: CalendarAdapter satisfies the sourceAdapter interface
var _ interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)
} = (*CalendarAdapter)(nil)

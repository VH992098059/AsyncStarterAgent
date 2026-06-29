package source

import (
	"context"
	"fmt"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
)

// CalendarProvider 抽象日历 Provider（Google/Outlook）
type CalendarProvider interface {
	ListEvents(ctx context.Context, from, to time.Time) ([]harvesting.ContextItem, error)
}

// CalendarAdapter 统一入口，实现 sourceAdapter 接口
type CalendarAdapter struct {
	Provider CalendarProvider
	Source   string // google_calendar / outlook
}

func (a *CalendarAdapter) Name() string { return a.Source }

// Fetch implements sourceAdapter interface — fetches events since the given time
func (a *CalendarAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	to := time.Now()
	items, err := a.Provider.ListEvents(ctx, since, to)
	if err != nil {
		return nil, fmt.Errorf("calendar adapter fetch: %w", err)
	}
	for i := range items {
		items[i].UserID = userID
		items[i].Source = a.Source
		if items[i].Type == "" {
			items[i].Type = "meeting"
		}
	}
	return items, nil
}

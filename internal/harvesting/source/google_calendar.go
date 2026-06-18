package source

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/asyncstarter/agent/internal/harvesting"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type GoogleCalendarConfig struct {
	CredentialsFile string // Service Account credentials JSON file path
	CalendarID      string // primary / xxx@group.calendar.google.com
}

type GoogleCalendarProvider struct {
	svc *calendar.Service
	cal string
}

func NewGoogleCalendarProvider(ctx context.Context, cfg GoogleCalendarConfig) (*GoogleCalendarProvider, error) {
	if cfg.CredentialsFile == "" {
		return nil, errors.New("google calendar: credentials file path is required")
	}
	svc, err := calendar.NewService(ctx, option.WithCredentialsFile(cfg.CredentialsFile))
	if err != nil {
		return nil, fmt.Errorf("new calendar service: %w", err)
	}
	cal := cfg.CalendarID
	if cal == "" {
		cal = "primary"
	}
	return &GoogleCalendarProvider{svc: svc, cal: cal}, nil
}

func (p *GoogleCalendarProvider) ListEvents(ctx context.Context, from, to time.Time) ([]harvesting.ContextItem, error) {
	var items []harvesting.ContextItem
	pageToken := ""
	for {
		call := p.svc.Events.List(p.cal).
			TimeMin(from.Format(time.RFC3339)).
			TimeMax(to.Format(time.RFC3339)).
			SingleEvents(true).
			MaxResults(250).
			OrderBy("startTime").
			Context(ctx)
		if pageToken != "" {
			call.PageToken(pageToken)
		}
		events, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("list events: %w", err)
		}
		for _, e := range events.Items {
			start := time.Time{}
			if e.Start != nil {
				if e.Start.DateTime != "" {
					t, parseErr := time.Parse(time.RFC3339, e.Start.DateTime)
					if parseErr != nil {
						start = time.Now()
					} else {
						start = t
					}
				} else if e.Start.Date != "" {
					t, parseErr := time.Parse("2006-01-02", e.Start.Date)
					if parseErr != nil {
						start = time.Now()
					} else {
						start = t
					}
				}
			}
			items = append(items, harvesting.ContextItem{
				ID:         "gcal:" + e.Id,
				Source:     "google_calendar",
				Type:       "meeting",
				Title:      e.Summary,
				Content:    e.Description,
				URL:        e.HtmlLink,
				OccurredAt: start,
				Metadata: map[string]string{
					"attendees": fmt.Sprintf("%d", len(e.Attendees)),
				},
			})
		}
		if events.NextPageToken == "" {
			break
		}
		pageToken = events.NextPageToken
	}
	return items, nil
}

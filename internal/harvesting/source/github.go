package source

import (
	"context"
	"fmt"
	"time"

	gh "github.com/google/go-github/v57/github"
	"github.com/asyncstarter/agent/internal/harvesting"
	"golang.org/x/oauth2"
)

type GitHubConfig struct {
	Token string    // PAT or installation token
	Owner string
	Repo  string
	Since time.Time // 增量同步起点
}

type GitHubAdapter struct {
	cfg GitHubConfig
	cli *gh.Client
}

func NewGitHubAdapter(cfg GitHubConfig) *GitHubAdapter {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	httpClient := oauth2.NewClient(context.Background(), ts)
	return &GitHubAdapter{cfg: cfg, cli: gh.NewClient(httpClient)}
}

// FetchCommits 拉取增量 commits
func (a *GitHubAdapter) FetchCommits(ctx context.Context) ([]harvesting.ContextItem, error) {
	opts := &gh.CommitsListOptions{
		Since:       a.cfg.Since,
		ListOptions: gh.ListOptions{PerPage: 50},
	}
	commits, _, err := a.cli.Repositories.ListCommits(ctx, a.cfg.Owner, a.cfg.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list commits: %w", err)
	}
	items := make([]harvesting.ContextItem, 0, len(commits))
	for _, c := range commits {
		msg := ""
		if c.Commit != nil && c.Commit.Message != nil {
			msg = *c.Commit.Message
		}
		author := ""
		if c.Author != nil && c.Author.Login != nil {
			author = *c.Author.Login
		}
		sha := ""
		if c.SHA != nil {
			sha = *c.SHA
		}
		date := time.Time{}
		if c.Commit != nil && c.Commit.Author != nil && c.Commit.Author.Date != nil {
			date = c.Commit.Author.Date.Time
		}
		items = append(items, harvesting.ContextItem{
			ID:         "github:commit:" + sha,
			Source:     "github",
			Type:       "commit",
			Title:      msg,
			Content:    msg,
			URL:        fmt.Sprintf("https://github.com/%s/%s/commit/%s", a.cfg.Owner, a.cfg.Repo, sha),
			OccurredAt: date,
			Metadata:   map[string]string{"author": author, "sha": sha},
		})
	}
	return items, nil
}

// FetchPullRequests 拉取 PR
func (a *GitHubAdapter) FetchPullRequests(ctx context.Context) ([]harvesting.ContextItem, error) {
	opts := &gh.PullRequestListOptions{
		State:     "all",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: gh.ListOptions{PerPage: 30},
	}
	prs, _, err := a.cli.PullRequests.List(ctx, a.cfg.Owner, a.cfg.Repo, opts)
	if err != nil {
		return nil, fmt.Errorf("list prs: %w", err)
	}
	items := make([]harvesting.ContextItem, 0, len(prs))
	for _, p := range prs {
		items = append(items, harvesting.ContextItem{
			ID:         fmt.Sprintf("github:pr:%d", p.GetNumber()),
			Source:     "github",
			Type:       "pr",
			Title:      p.GetTitle(),
			Content:    p.GetBody(),
			URL:        p.GetHTMLURL(),
			OccurredAt: p.GetUpdatedAt().Time,
				Metadata: map[string]string{
				"state":  p.GetState(),
				"author": p.GetUser().GetLogin(),
			},
		})
	}
	// Client-side incremental filter: drop PRs updated before Since
	if !a.cfg.Since.IsZero() {
		filtered := make([]harvesting.ContextItem, 0, len(items))
		for _, it := range items {
			if it.OccurredAt.After(a.cfg.Since) || it.OccurredAt.Equal(a.cfg.Since) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	return items, nil
}

// Name returns the data source name for adapter routing
func (a *GitHubAdapter) Name() string { return "github" }

// Fetch implements the sourceAdapter interface — merges commits + PRs
func (a *GitHubAdapter) Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error) {
	a.cfg.Since = since
	commits, err := a.FetchCommits(ctx)
	if err != nil {
		return nil, err
	}
	prs, err := a.FetchPullRequests(ctx)
	if err != nil {
		return nil, err
	}
	return append(commits, prs...), nil
}

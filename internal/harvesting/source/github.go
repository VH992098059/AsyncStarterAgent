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

// FetchCommits 拉取增量 commits，翻页直到最后一页（NextPage == 0）
func (a *GitHubAdapter) FetchCommits(ctx context.Context) ([]harvesting.ContextItem, error) {
	opts := &gh.CommitsListOptions{
		Since:       a.cfg.Since,
		ListOptions: gh.ListOptions{PerPage: 50},
	}
	var items []harvesting.ContextItem
	for {
		commits, resp, err := a.cli.Repositories.ListCommits(ctx, a.cfg.Owner, a.cfg.Repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list commits: %w", err)
		}
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
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return items, nil
}

// FetchPullRequests 拉取 PR，翻页直到最后一页或遇到早于 Since 的记录（列表已按 updated desc 排序，
// 一旦遇到早于 Since 的 PR 可提前终止分页）
func (a *GitHubAdapter) FetchPullRequests(ctx context.Context) ([]harvesting.ContextItem, error) {
	opts := &gh.PullRequestListOptions{
		State:       "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: gh.ListOptions{PerPage: 30},
	}
	var items []harvesting.ContextItem
pageLoop:
	for {
		prs, resp, err := a.cli.PullRequests.List(ctx, a.cfg.Owner, a.cfg.Repo, opts)
		if err != nil {
			return nil, fmt.Errorf("list prs: %w", err)
		}
		for _, p := range prs {
			updatedAt := p.GetUpdatedAt().Time
			if !a.cfg.Since.IsZero() && updatedAt.Before(a.cfg.Since) {
				break pageLoop
			}
			items = append(items, harvesting.ContextItem{
				ID:         fmt.Sprintf("github:pr:%d", p.GetNumber()),
				Source:     "github",
				Type:       "pr",
				Title:      p.GetTitle(),
				Content:    p.GetBody(),
				URL:        p.GetHTMLURL(),
				OccurredAt: updatedAt,
				Metadata: map[string]string{
					"state":  p.GetState(),
					"author": p.GetUser().GetLogin(),
				},
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
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
	all := append(commits, prs...)
	for i := range all {
		all[i].UserID = userID
	}
	return all, nil
}

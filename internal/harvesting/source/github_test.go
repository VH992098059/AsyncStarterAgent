package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	gh "github.com/google/go-github/v57/github"
	"github.com/asyncstarter/agent/internal/harvesting"
)

// newMockGitHubClient 构造一个不依赖外部网络的 *gh.Client
func newMockGitHubClient(handler http.HandlerFunc) (*gh.Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	cli := gh.NewClient(nil)
	cli.BaseURL, _ = url.Parse(srv.URL + "/")
	return cli, srv
}

func TestGitHubAdapter_FetchCommits_Mock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/commits", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"sha": "abc123",
				"commit": map[string]interface{}{
					"message": "feat: add login",
					"author": map[string]interface{}{
						"date": time.Now().Format(time.RFC3339),
					},
				},
				"author": map[string]interface{}{"login": "alice"},
			},
		})
	})
	cli, srv := newMockGitHubClient(mux.ServeHTTP)
	defer srv.Close()

	a := &GitHubAdapter{
		cfg: GitHubConfig{Token: "x", Owner: "o", Repo: "r", Since: time.Now().Add(-24 * time.Hour)},
		cli: cli,
	}
	items, err := a.FetchCommits(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	if items[0].Type != "commit" {
		t.Errorf("type: %s", items[0].Type)
	}
	if items[0].Source != "github" {
		t.Errorf("source: %s", items[0].Source)
	}
	if items[0].Metadata["sha"] != "abc123" {
		t.Errorf("sha: %s", items[0].Metadata["sha"])
	}
}

func TestGitHubAdapter_FetchPRs_Mock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"number":     42,
				"title":      "Add dark mode",
				"body":       "Implement dark mode toggle",
				"state":      "open",
				"html_url":   "https://example/pr/42",
				"updated_at": time.Now().Format(time.RFC3339),
				"user":       map[string]interface{}{"login": "bob"},
			},
		})
	})
	cli, srv := newMockGitHubClient(mux.ServeHTTP)
	defer srv.Close()

	a := &GitHubAdapter{cfg: GitHubConfig{Token: "x", Owner: "o", Repo: "r"}, cli: cli}
	items, err := a.FetchPullRequests(context.Background())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 1 || items[0].Type != "pr" {
		t.Fatalf("items: %+v", items)
	}
}

// Compile-time check: GitHubAdapter satisfies the expected interface shape
var _ interface {
	Name() string
	Fetch(ctx context.Context, userID string, since time.Time) ([]harvesting.ContextItem, error)
} = (*GitHubAdapter)(nil)

func TestGitHubAdapter_Fetch_Merged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/commits", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"sha": "abc123",
				"commit": map[string]interface{}{
					"message": "feat: add login",
					"author":  map[string]interface{}{"date": time.Now().Format(time.RFC3339)},
				},
				"author": map[string]interface{}{"login": "alice"},
			},
		})
	})
	mux.HandleFunc("/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"number":     42,
				"title":      "Add dark mode",
				"body":       "Implement dark mode toggle",
				"state":      "open",
				"html_url":   "https://example/pr/42",
				"updated_at": time.Now().Format(time.RFC3339),
				"user":       map[string]interface{}{"login": "bob"},
			},
		})
	})
	cli, srv := newMockGitHubClient(mux.ServeHTTP)
	defer srv.Close()
	a := &GitHubAdapter{
		cfg: GitHubConfig{Token: "x", Owner: "o", Repo: "r"},
		cli: cli,
	}
	items, err := a.Fetch(context.Background(), "user-1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 (1 commit + 1 pr), got %d", len(items))
	}
}

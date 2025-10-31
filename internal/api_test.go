package ghprcomments

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v61/github"
)

// mockGitHubServer creates a test HTTP server that mocks GitHub API responses
func mockGitHubServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *github.Client) {
	server := httptest.NewServer(handler)
	client := github.NewClient(nil)
	client.BaseURL.Scheme = "http"
	client.BaseURL.Host = server.URL[7:] // Remove "http://"
	return server, client
}

func TestNewGitHubClient(t *testing.T) {
	ctx := context.Background()

	t.Run("creates client for github.com", func(t *testing.T) {
		client, err := NewGitHubClient(ctx, "test-token", "github.com")
		if err != nil {
			t.Fatalf("NewGitHubClient failed: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
		if client.BaseURL.Host != "api.github.com" {
			t.Errorf("expected api.github.com, got %s", client.BaseURL.Host)
		}
	})

	t.Run("creates enterprise client for custom host", func(t *testing.T) {
		client, err := NewGitHubClient(ctx, "test-token", "github.example.com")
		if err != nil {
			t.Fatalf("NewGitHubClient failed: %v", err)
		}
		if client == nil {
			t.Fatal("expected non-nil client")
		}
		if client.BaseURL.Host != "github.example.com" {
			t.Errorf("expected github.example.com, got %s", client.BaseURL.Host)
		}
	})
}

func TestFetchComments_Success(t *testing.T) {
	ctx := context.Background()

	// Mock responses for the three parallel API calls
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/repos/owner/repo/issues/1/comments":
			// Issue comments
			comments := []*github.IssueComment{
				{ID: github.Int64(100), Body: github.String("Issue comment 1")},
				{ID: github.Int64(101), Body: github.String("Issue comment 2")},
			}
			json.NewEncoder(w).Encode(comments)

		case r.URL.Path == "/repos/owner/repo/pulls/1/comments":
			// Review comments (code-level)
			comments := []*github.PullRequestComment{
				{ID: github.Int64(200), Body: github.String("Review comment 1")},
			}
			json.NewEncoder(w).Encode(comments)

		case r.URL.Path == "/repos/owner/repo/pulls/1/reviews":
			// PR reviews
			reviews := []*github.PullRequestReview{
				{ID: github.Int64(300), State: github.String("APPROVED")},
			}
			json.NewEncoder(w).Encode(reviews)

		default:
			t.Errorf("unexpected request to %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}

	server, client := mockGitHubServer(t, handler)
	defer server.Close()

	fetcher := NewFetcher(client)
	payload, err := fetcher.FetchComments(ctx, "owner", "repo", 1)

	if err != nil {
		t.Fatalf("FetchComments failed: %v", err)
	}

	if len(payload.issueComments) != 2 {
		t.Errorf("expected 2 issue comments, got %d", len(payload.issueComments))
	}
	if len(payload.reviewComments) != 1 {
		t.Errorf("expected 1 review comment, got %d", len(payload.reviewComments))
	}
	if len(payload.reviews) != 1 {
		t.Errorf("expected 1 review, got %d", len(payload.reviews))
	}
}

func TestFetchComments_Error(t *testing.T) {
	ctx := context.Background()

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Simulate API error
		http.Error(w, "API rate limit exceeded", http.StatusForbidden)
	}

	server, client := mockGitHubServer(t, handler)
	defer server.Close()

	fetcher := NewFetcher(client)
	_, err := fetcher.FetchComments(ctx, "owner", "repo", 1)

	if err == nil {
		t.Fatal("expected error from FetchComments, got nil")
	}
}

func TestGetPullRequestSummary(t *testing.T) {
	ctx := context.Background()

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/pulls/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		updatedAt := time.Date(2025, 10, 31, 12, 0, 0, 0, time.UTC)
		createdAt := time.Date(2025, 10, 30, 10, 0, 0, 0, time.UTC)

		pr := &github.PullRequest{
			Number:    github.Int(42),
			Title:     github.String("Add feature X"),
			State:     github.String("open"),
			HTMLURL:   github.String("https://github.com/owner/repo/pull/42"),
			UpdatedAt: &github.Timestamp{Time: updatedAt},
			CreatedAt: &github.Timestamp{Time: createdAt},
			User: &github.User{
				Login: github.String("octocat"),
			},
			Head: &github.PullRequestBranch{
				Ref: github.String("feature-x"),
			},
			Base: &github.PullRequestBranch{
				Ref: github.String("main"),
				Repo: &github.Repository{
					Name: github.String("repo"),
					Owner: &github.User{
						Login: github.String("owner"),
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pr)
	}

	server, client := mockGitHubServer(t, handler)
	defer server.Close()

	fetcher := NewFetcher(client)
	summary, err := fetcher.GetPullRequestSummary(ctx, "owner", "repo", 42)

	if err != nil {
		t.Fatalf("GetPullRequestSummary failed: %v", err)
	}

	if summary.Number != 42 {
		t.Errorf("expected PR number 42, got %d", summary.Number)
	}
	if summary.Title != "Add feature X" {
		t.Errorf("expected title 'Add feature X', got %q", summary.Title)
	}
	if summary.Author != "octocat" {
		t.Errorf("expected author 'octocat', got %q", summary.Author)
	}
	if summary.HeadRef != "feature-x" {
		t.Errorf("expected head ref 'feature-x', got %q", summary.HeadRef)
	}
	if summary.BaseRef != "main" {
		t.Errorf("expected base ref 'main', got %q", summary.BaseRef)
	}
	if summary.RepoOwner != "owner" {
		t.Errorf("expected repo owner 'owner', got %q", summary.RepoOwner)
	}
	if summary.RepoName != "repo" {
		t.Errorf("expected repo name 'repo', got %q", summary.RepoName)
	}
}

func TestListPullRequestSummaries(t *testing.T) {
	ctx := context.Background()

	t.Run("single page of PRs", func(t *testing.T) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/owner/repo/pulls" {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			prs := []*github.PullRequest{
				{
					Number: github.Int(1),
					Title:  github.String("PR 1"),
					State:  github.String("open"),
					User:   &github.User{Login: github.String("user1")},
					Head:   &github.PullRequestBranch{Ref: github.String("feat1")},
					Base: &github.PullRequestBranch{
						Ref: github.String("main"),
						Repo: &github.Repository{
							Name:  github.String("repo"),
							Owner: &github.User{Login: github.String("owner")},
						},
					},
				},
				{
					Number: github.Int(2),
					Title:  github.String("PR 2"),
					State:  github.String("open"),
					User:   &github.User{Login: github.String("user2")},
					Head:   &github.PullRequestBranch{Ref: github.String("feat2")},
					Base: &github.PullRequestBranch{
						Ref: github.String("main"),
						Repo: &github.Repository{
							Name:  github.String("repo"),
							Owner: &github.User{Login: github.String("owner")},
						},
					},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(prs)
		}

		server, client := mockGitHubServer(t, handler)
		defer server.Close()

		fetcher := NewFetcher(client)
		summaries, err := fetcher.ListPullRequestSummaries(ctx, "owner", "repo")

		if err != nil {
			t.Fatalf("ListPullRequestSummaries failed: %v", err)
		}

		if len(summaries) != 2 {
			t.Fatalf("expected 2 PRs, got %d", len(summaries))
		}

		if summaries[0].Number != 1 {
			t.Errorf("expected first PR number 1, got %d", summaries[0].Number)
		}
		if summaries[1].Number != 2 {
			t.Errorf("expected second PR number 2, got %d", summaries[1].Number)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		callCount := 0
		handler := func(w http.ResponseWriter, r *http.Request) {
			callCount++

			page := r.URL.Query().Get("page")

			w.Header().Set("Content-Type", "application/json")

			if page == "" || page == "1" {
				// First page - set Link header for next page
				w.Header().Set("Link", `<http://example.com/repos/owner/repo/pulls?page=2>; rel="next"`)
				prs := []*github.PullRequest{
					{
						Number: github.Int(1),
						Title:  github.String("PR 1"),
						State:  github.String("open"),
						User:   &github.User{Login: github.String("user1")},
						Head:   &github.PullRequestBranch{Ref: github.String("feat1")},
						Base: &github.PullRequestBranch{
							Ref: github.String("main"),
							Repo: &github.Repository{
								Name:  github.String("repo"),
								Owner: &github.User{Login: github.String("owner")},
							},
						},
					},
				}
				json.NewEncoder(w).Encode(prs)
			} else {
				// Second page - no Link header (last page)
				prs := []*github.PullRequest{
					{
						Number: github.Int(2),
						Title:  github.String("PR 2"),
						State:  github.String("open"),
						User:   &github.User{Login: github.String("user2")},
						Head:   &github.PullRequestBranch{Ref: github.String("feat2")},
						Base: &github.PullRequestBranch{
							Ref: github.String("main"),
							Repo: &github.Repository{
								Name:  github.String("repo"),
								Owner: &github.User{Login: github.String("owner")},
							},
						},
					},
				}
				json.NewEncoder(w).Encode(prs)
			}
		}

		server, client := mockGitHubServer(t, handler)
		defer server.Close()

		fetcher := NewFetcher(client)
		summaries, err := fetcher.ListPullRequestSummaries(ctx, "owner", "repo")

		if err != nil {
			t.Fatalf("ListPullRequestSummaries failed: %v", err)
		}

		if len(summaries) != 2 {
			t.Fatalf("expected 2 PRs from pagination, got %d", len(summaries))
		}

		if callCount != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", callCount)
		}
	})

	t.Run("no PRs returns error", func(t *testing.T) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]*github.PullRequest{})
		}

		server, client := mockGitHubServer(t, handler)
		defer server.Close()

		fetcher := NewFetcher(client)
		_, err := fetcher.ListPullRequestSummaries(ctx, "owner", "repo")

		if err != ErrNoPullRequests {
			t.Errorf("expected ErrNoPullRequests, got %v", err)
		}
	})
}

func TestSummarizePullRequest(t *testing.T) {
	t.Run("nil PR returns nil", func(t *testing.T) {
		summary := summarizePullRequest(nil)
		if summary != nil {
			t.Errorf("expected nil for nil PR, got %+v", summary)
		}
	})

	t.Run("complete PR metadata", func(t *testing.T) {
		updatedAt := time.Date(2025, 10, 31, 12, 0, 0, 0, time.UTC)
		createdAt := time.Date(2025, 10, 30, 10, 0, 0, 0, time.UTC)

		pr := &github.PullRequest{
			Number:    github.Int(123),
			Title:     github.String("Test PR"),
			State:     github.String("open"),
			HTMLURL:   github.String("https://github.com/owner/repo/pull/123"),
			UpdatedAt: &github.Timestamp{Time: updatedAt},
			CreatedAt: &github.Timestamp{Time: createdAt},
			User: &github.User{
				Login: github.String("testuser"),
			},
			Head: &github.PullRequestBranch{
				Ref: github.String("feature-branch"),
			},
			Base: &github.PullRequestBranch{
				Ref: github.String("main"),
				Repo: &github.Repository{
					Name: github.String("testrepo"),
					Owner: &github.User{
						Login: github.String("testowner"),
					},
				},
			},
		}

		summary := summarizePullRequest(pr)

		if summary.Number != 123 {
			t.Errorf("expected number 123, got %d", summary.Number)
		}
		if summary.Title != "Test PR" {
			t.Errorf("expected title 'Test PR', got %q", summary.Title)
		}
		if summary.Author != "testuser" {
			t.Errorf("expected author 'testuser', got %q", summary.Author)
		}
		if summary.State != "open" {
			t.Errorf("expected state 'open', got %q", summary.State)
		}
		if summary.HeadRef != "feature-branch" {
			t.Errorf("expected head ref 'feature-branch', got %q", summary.HeadRef)
		}
		if summary.BaseRef != "main" {
			t.Errorf("expected base ref 'main', got %q", summary.BaseRef)
		}
		if summary.RepoName != "testrepo" {
			t.Errorf("expected repo name 'testrepo', got %q", summary.RepoName)
		}
		if summary.RepoOwner != "testowner" {
			t.Errorf("expected repo owner 'testowner', got %q", summary.RepoOwner)
		}
		if summary.URL != "https://github.com/owner/repo/pull/123" {
			t.Errorf("expected URL 'https://github.com/owner/repo/pull/123', got %q", summary.URL)
		}
		if !summary.Updated.Equal(updatedAt) {
			t.Errorf("expected updated time %v, got %v", updatedAt, summary.Updated)
		}
		if !summary.Created.Equal(createdAt) {
			t.Errorf("expected created time %v, got %v", createdAt, summary.Created)
		}
	})

	t.Run("handles nil fields gracefully", func(t *testing.T) {
		pr := &github.PullRequest{
			Number: github.Int(456),
			Title:  github.String("Minimal PR"),
			// All other fields are nil
		}

		summary := summarizePullRequest(pr)

		if summary == nil {
			t.Fatal("expected non-nil summary")
		}
		if summary.Number != 456 {
			t.Errorf("expected number 456, got %d", summary.Number)
		}
		if summary.Author != "" {
			t.Errorf("expected empty author, got %q", summary.Author)
		}
		if summary.RepoOwner != "" {
			t.Errorf("expected empty repo owner, got %q", summary.RepoOwner)
		}
		if summary.RepoName != "" {
			t.Errorf("expected empty repo name, got %q", summary.RepoName)
		}
		if !summary.Updated.IsZero() {
			t.Errorf("expected zero updated time, got %v", summary.Updated)
		}
	})
}

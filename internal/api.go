package ghprcomments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/v61/github"
	"golang.org/x/oauth2"
)

// Fetcher bundles GitHub operations used by the CLI.
type Fetcher struct {
	client *github.Client
}

// NewGitHubClient constructs an authenticated GitHub REST client.
func NewGitHubClient(ctx context.Context, token, host string) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client := oauth2.NewClient(ctx, ts)

	if host == "github.com" {
		return github.NewClient(client), nil
	}

	base := fmt.Sprintf("https://%s/api/v3/", host)
	upload := fmt.Sprintf("https://%s/uploads/", host)
	return github.NewEnterpriseClient(base, upload, client)
}

// NewFetcher creates a Fetcher instance.
func NewFetcher(client *github.Client) *Fetcher {
	return &Fetcher{client: client}
}

// PullRequestSummary carries the metadata we display and persist.
type PullRequestSummary struct {
	Number    int
	Title     string
	Author    string
	State     string
	Created   time.Time
	Updated   time.Time
	HeadRef   string
	BaseRef   string
	RepoName  string
	RepoOwner string
}

// commentPayload groups the raw GitHub responses.
type commentPayload struct {
	issueComments  []*github.IssueComment
	reviewComments []*github.PullRequestComment
	reviews        []*github.PullRequestReview
}

// FetchComments retrieves every comment category for the pull request.
func (f *Fetcher) FetchComments(ctx context.Context, owner, repo string, number int) (commentPayload, error) {
	issues, err := f.listIssueComments(ctx, owner, repo, number)
	if err != nil {
		return commentPayload{}, err
	}

	reviewComments, err := f.listReviewComments(ctx, owner, repo, number)
	if err != nil {
		return commentPayload{}, err
	}

	reviews, err := f.listReviews(ctx, owner, repo, number)
	if err != nil {
		return commentPayload{}, err
	}

	return commentPayload{
		issueComments:  issues,
		reviewComments: reviewComments,
		reviews:        reviews,
	}, nil
}

// GetPullRequestSummary fetches metadata for a single pull request.
func (f *Fetcher) GetPullRequestSummary(ctx context.Context, owner, repo string, number int) (*PullRequestSummary, error) {
	pr, _, err := f.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, err
	}
	return summarizePullRequest(pr), nil
}

// ListPullRequestSummaries returns a set of pull requests for interactive selection.
func (f *Fetcher) ListPullRequestSummaries(ctx context.Context, owner, repo string) ([]*PullRequestSummary, error) {
	opts := &github.PullRequestListOptions{
		State:     "open",
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	}

	var summaries []*PullRequestSummary

	for {
		prs, resp, err := f.client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			summaries = append(summaries, summarizePullRequest(pr))
		}
		if resp.NextPage == 0 || len(summaries) >= 200 {
			break
		}
		opts.Page = resp.NextPage
	}

	if len(summaries) == 0 {
		return nil, errors.New("no pull requests found")
	}

	return summaries, nil
}

func (f *Fetcher) listIssueComments(ctx context.Context, owner, repo string, number int) ([]*github.IssueComment, error) {
	opts := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	var all []*github.IssueComment
	for {
		items, resp, err := f.client.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if resp.NextPage == 0 {
			return all, nil
		}
		opts.Page = resp.NextPage
	}
}

func (f *Fetcher) listReviewComments(ctx context.Context, owner, repo string, number int) ([]*github.PullRequestComment, error) {
	opts := &github.PullRequestListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	var all []*github.PullRequestComment
	for {
		items, resp, err := f.client.PullRequests.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if resp.NextPage == 0 {
			return all, nil
		}
		opts.Page = resp.NextPage
	}
}

func (f *Fetcher) listReviews(ctx context.Context, owner, repo string, number int) ([]*github.PullRequestReview, error) {
	opts := &github.ListOptions{PerPage: 100}
	var all []*github.PullRequestReview
	for {
		items, resp, err := f.client.PullRequests.ListReviews(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if resp.NextPage == 0 {
			return all, nil
		}
		opts.Page = resp.NextPage
	}
}

func summarizePullRequest(pr *github.PullRequest) *PullRequestSummary {
	if pr == nil {
		return nil
	}

	author := ""
	if pr.User != nil && pr.User.Login != nil {
		author = pr.GetUser().GetLogin()
	}

	repoOwner := ""
	repoName := ""
	if pr.Base != nil && pr.Base.Repo != nil {
		repoOwner = pr.Base.Repo.GetOwner().GetLogin()
		repoName = pr.Base.Repo.GetName()
	}

	headRef := ""
	if pr.Head != nil {
		headRef = pr.Head.GetRef()
	}

	baseRef := ""
	if pr.Base != nil {
		baseRef = pr.Base.GetRef()
	}

	updated := time.Time{}
	if pr.UpdatedAt != nil {
		updated = pr.UpdatedAt.Time
	}

	created := time.Time{}
	if pr.CreatedAt != nil {
		created = pr.CreatedAt.Time
	}

	return &PullRequestSummary{
		Number:    pr.GetNumber(),
		Title:     pr.GetTitle(),
		Author:    author,
		State:     pr.GetState(),
		Created:   created,
		Updated:   updated,
		HeadRef:   headRef,
		BaseRef:   baseRef,
		RepoOwner: repoOwner,
		RepoName:  repoName,
	}
}

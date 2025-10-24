package ghprcomments

import (
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
)

// Output captures the unified payload for downstream use.
type Output struct {
	PR       PullRequestMetadata `json:"pr"`
	Comments []Comment           `json:"comments"`
}

// PullRequestMetadata is serialized as part of the output contract.
type PullRequestMetadata struct {
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Author    string    `json:"author"`
	UpdatedAt time.Time `json:"updated_at"`
	HeadRef   string    `json:"head_ref"`
}

// Comment represents an individual review unit.
type Comment struct {
	Type      string    `json:"type"`
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	IsBot     bool      `json:"is_bot"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"path,omitempty"`
	Line      *int      `json:"line,omitempty"`
	State     string    `json:"state,omitempty"`
	BodyText  string    `json:"body_text"`
	Permalink string    `json:"permalink,omitempty"`
}

// NormalizationOptions controls comment shaping.
type NormalizationOptions struct {
	StripHTML bool
}

// BuildOutput merges PR metadata and comments into the external contract.
func BuildOutput(pr *PullRequestSummary, payload commentPayload, opts NormalizationOptions) Output {
	if pr == nil {
		return Output{}
	}

	comments := make([]Comment, 0, len(payload.issueComments)+len(payload.reviewComments)+len(payload.reviews))

	for _, ic := range payload.issueComments {
		comments = append(comments, normalizeIssueComment(ic, opts))
	}

	for _, rc := range payload.reviewComments {
		comments = append(comments, normalizeReviewComment(rc, opts))
	}

	for _, review := range payload.reviews {
		comments = append(comments, normalizeReview(review, opts))
	}

	sort.Slice(comments, func(i, j int) bool {
		return comments[i].CreatedAt.Before(comments[j].CreatedAt)
	})

	repo := pr.RepoOwner
	if pr.RepoName != "" {
		repo = strings.Trim(pr.RepoOwner+"/"+pr.RepoName, "/")
	}

	meta := PullRequestMetadata{
		Repo:      repo,
		Number:    pr.Number,
		Title:     pr.Title,
		State:     pr.State,
		Author:    pr.Author,
		UpdatedAt: pr.Updated,
		HeadRef:   pr.HeadRef,
	}

	return Output{PR: meta, Comments: comments}
}

func normalizeIssueComment(c *github.IssueComment, opts NormalizationOptions) Comment {
	body := c.GetBody()
	if opts.StripHTML {
		body = StripHTML(body)
	}

	return Comment{
		Type:      "issue",
		ID:        c.GetID(),
		Author:    safeLogin(c.GetUser()),
		IsBot:     IsBotAuthor(c.GetUser()),
		CreatedAt: derefTimestamp(c.CreatedAt),
		BodyText:  body,
		Permalink: c.GetHTMLURL(),
	}
}

func normalizeReviewComment(c *github.PullRequestComment, opts NormalizationOptions) Comment {
	body := c.GetBody()
	if opts.StripHTML {
		body = StripHTML(body)
	}

	var linePtr *int
	if c.Line != nil {
		lineVal := c.GetLine()
		linePtr = &lineVal
	}

	return Comment{
		Type:      "review_comment",
		ID:        c.GetID(),
		Author:    safeLogin(c.GetUser()),
		IsBot:     IsBotAuthor(c.GetUser()),
		CreatedAt: derefTimestamp(c.CreatedAt),
		Path:      c.GetPath(),
		Line:      linePtr,
		BodyText:  body,
		Permalink: c.GetHTMLURL(),
	}
}

func normalizeReview(r *github.PullRequestReview, opts NormalizationOptions) Comment {
	body := r.GetBody()
	if opts.StripHTML {
		body = StripHTML(body)
	}

	return Comment{
		Type:      "review_event",
		ID:        r.GetID(),
		Author:    safeLogin(r.GetUser()),
		IsBot:     IsBotAuthor(r.GetUser()),
		CreatedAt: derefTimestamp(r.SubmittedAt),
		State:     r.GetState(),
		BodyText:  body,
		Permalink: r.GetHTMLURL(),
	}
}

func derefTimestamp(ts *github.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.Time
}

func safeLogin(user *github.User) string {
	if user == nil {
		return ""
	}
	if login := user.GetLogin(); login != "" {
		return login
	}
	return user.GetName()
}

package ghprcomments

import (
	"html"
	"net/url"
	"regexp"
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
	ID        int64     `json:"-"`
	Author    string    `json:"author"`
	IsBot     bool      `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"-"`
	Line      *int      `json:"-"`
	State     string    `json:"-"`
	BodyText  string    `json:"body_text"`
	Permalink string    `json:"permalink"`
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
		comment := normalizeIssueComment(ic, opts)
		comments = append(comments, comment)
	}

	for _, rc := range payload.reviewComments {
		comment := normalizeReviewComment(rc, opts)
		comments = append(comments, comment)
	}

	for _, review := range payload.reviews {
		comment := normalizeReview(review, opts)
		comments = append(comments, comment)
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
	body := cleanCommentBody(c.GetBody(), opts)

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
	body := cleanCommentBody(c.GetBody(), opts)

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
	body := cleanCommentBody(r.GetBody(), opts)

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

var (
	detailsBlockRegex  = regexp.MustCompile(`(?is)<details.*?>.*?</details>`)
	htmlCommentRegex   = regexp.MustCompile(`(?s)<!--.*?-->`)
	codeFenceRegex     = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRegex    = regexp.MustCompile("`([^`]*)`")
	imageMarkdownRegex = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	linkMarkdownRegex  = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	orderedListRegex   = regexp.MustCompile(`^\d+\.\s+`)
	base64BlobRegex    = regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`)
	urlRegex           = regexp.MustCompile(`https?://[^\s)]+`)
)

func cleanCommentBody(body string, opts NormalizationOptions) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}

	// Always normalize to human-readable plain text regardless of incoming flags.
	_ = opts // retained for future expansion and to preserve function signature

	normalized := html.UnescapeString(body)
	normalized = detailsBlockRegex.ReplaceAllString(normalized, " ")
	normalized = htmlCommentRegex.ReplaceAllString(normalized, " ")
	normalized = codeFenceRegex.ReplaceAllString(normalized, " ")

	// Ensure residual HTML is removed before markdown cleanup.
	normalized = StripHTML(normalized)

	normalized = imageMarkdownRegex.ReplaceAllStringFunc(normalized, func(match string) string {
		parts := imageMarkdownRegex.FindStringSubmatch(match)
		alt := strings.TrimSpace(parts[1])
		return alt
	})

	normalized = linkMarkdownRegex.ReplaceAllStringFunc(normalized, func(match string) string {
		parts := linkMarkdownRegex.FindStringSubmatch(match)
		label := strings.TrimSpace(parts[1])
		if label != "" {
			return label
		}
		host := hostFromURL(parts[2])
		return host
	})

	normalized = inlineCodeRegex.ReplaceAllString(normalized, "$1")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")

	lines := strings.Split(normalized, "\n")
	cleanedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "---" || line == "***" || line == "___" {
			continue
		}
		if strings.Count(line, "|") >= 2 {
			// Drop Markdown table rows entirely.
			continue
		}
		for {
			switch {
			case strings.HasPrefix(line, "> "):
				line = strings.TrimPrefix(line, "> ")
			case strings.HasPrefix(line, "- "):
				line = strings.TrimPrefix(line, "- ")
			case strings.HasPrefix(line, "* "):
				line = strings.TrimPrefix(line, "* ")
			case strings.HasPrefix(line, "+ "):
				line = strings.TrimPrefix(line, "+ ")
			case strings.HasPrefix(line, "• "):
				line = strings.TrimPrefix(line, "• ")
			case strings.HasPrefix(line, "#"):
				line = strings.TrimSpace(strings.TrimLeft(line, "#"))
			default:
				line = orderedListRegex.ReplaceAllString(line, "")
				goto cleaned
			}
		}
	cleaned:
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}

	normalized = strings.Join(cleanedLines, " ")
	normalized = urlRegex.ReplaceAllStringFunc(normalized, func(raw string) string {
		host := hostFromURL(raw)
		return host
	})
	normalized = base64BlobRegex.ReplaceAllString(normalized, " ")
	normalized = strings.ReplaceAll(normalized, "\u00a0", " ")

	normalized = strings.Join(strings.Fields(normalized), " ")
	return strings.TrimSpace(normalized)
}

func hostFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Host)
}

package ghprcomments

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v61/github"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no_html", "plain text", "plain text"},
		{"paragraphs", "<p>hello</p>", "hello"},
		{"line_breaks", "first<br>second", "first\nsecond"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripHTML(tt.in)
			if got != tt.want {
				t.Fatalf("StripHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsBotAuthor(t *testing.T) {
	tests := []struct {
		name  string
		login string
		want  bool
	}{
		{"regular_user", "human", false},
		{"dependabot", "dependabot", true},
		{"suffix_bot", "build[bot]", true},
		{"copilot_case", "CoPiLoT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &github.User{Login: github.String(tt.login)}
			if got := IsBotAuthor(user); got != tt.want {
				t.Fatalf("IsBotAuthor(%q) = %v, want %v", tt.login, got, tt.want)
			}
		})
	}
}

func TestFormatCommentType(t *testing.T) {
	tests := map[string]string{
		"issue":          "Issue",
		"review_comment": "Review Comment",
		"review_event":   "Review Event",
		"":               "Comment",
	}

	for input, want := range tests {
		if got := formatCommentType(input); got != want {
			t.Fatalf("formatCommentType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSelectWithPromptRepoQualified(t *testing.T) {
	prs := []*PullRequestSummary{
		{
			Number:    42,
			Title:     "Test PR",
			Author:    "dev1",
			State:     "open",
			Created:   time.Now(),
			Updated:   time.Now(),
			HeadRef:   "feature",
			BaseRef:   "main",
			RepoOwner: "octo",
			RepoName:  "alpha",
		},
	}

	input := strings.NewReader("octo/alpha#42\n")
	var output bytes.Buffer

	got, err := selectWithPrompt(prs, input, &output)
	if err != nil {
		t.Fatalf("selectWithPrompt returned error: %v", err)
	}
	if got != prs[0] {
		t.Fatalf("selectWithPrompt returned unexpected PR: %+v", got)
	}
}

func TestSelectWithPromptDuplicateNumbers(t *testing.T) {
	prs := []*PullRequestSummary{
		{Number: 7, RepoOwner: "octo", RepoName: "alpha", Updated: time.Now()},
		{Number: 7, RepoOwner: "octo", RepoName: "beta", Updated: time.Now()},
	}

	input := strings.NewReader("7\n")
	var output bytes.Buffer

	if _, err := selectWithPrompt(prs, input, &output); err == nil {
		t.Fatalf("selectWithPrompt should have returned an error for duplicate PR numbers")
	}
}

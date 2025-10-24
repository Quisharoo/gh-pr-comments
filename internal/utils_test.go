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

func TestSelectWithPromptFormattingSingleOwner(t *testing.T) {
	prs := []*PullRequestSummary{
		{
			Number:    39,
			Title:     "Add React UI components and services",
			HeadRef:   "feature/shadcn-ui-refactor",
			BaseRef:   "main",
			RepoOwner: "Quisharoo",
			RepoName:  "chatgpt-obsidian-converter",
			Updated:   time.Date(2025, 10, 20, 17, 29, 55, 0, time.UTC),
		},
		{
			Number:    5,
			Title:     "Iterm",
			HeadRef:   "iterm",
			BaseRef:   "main",
			RepoOwner: "Quisharoo",
			RepoName:  "dotfiles",
			Updated:   time.Date(2025, 10, 22, 20, 20, 20, 0, time.UTC),
		},
		{
			Number:    38,
			Title:     "Fix ICS timezone export handling",
			HeadRef:   "merge-old-prs",
			BaseRef:   "main",
			RepoOwner: "Quisharoo",
			RepoName:  "revolut-calendar",
			Updated:   time.Date(2025, 10, 19, 12, 1, 31, 0, time.UTC),
		},
	}

	input := strings.NewReader("1\n")
	var output bytes.Buffer

	if _, err := selectWithPrompt(prs, input, &output); err != nil {
		t.Fatalf("selectWithPrompt returned error: %v", err)
	}

	expected := strings.Join([]string{
		"[1] chatgpt-obsidian-converter#39 - Add React UI components and services [feature/shadcn-ui-refactor\u2192main] updated 2025-10-20 17:29Z",
		"[2] dotfiles#5 - Iterm [iterm\u2192main] updated 2025-10-22 20:20Z",
		"[3] revolut-calendar#38 - Fix ICS timezone export handling [merge-old-prs\u2192main] updated 2025-10-19 12:01Z",
		"Select by index, PR number, or owner/repo#number: ",
	}, "\n")

	if got := output.String(); got != expected {
		t.Fatalf("unexpected prompt output.\nwant: %q\n got: %q", expected, got)
	}
}

func TestSelectWithPromptFormattingMultipleOwners(t *testing.T) {
	prs := []*PullRequestSummary{
		{
			Number:    1,
			Title:     "First",
			HeadRef:   "feature",
			BaseRef:   "main",
			RepoOwner: "octo",
			RepoName:  "alpha",
			Updated:   time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		},
		{
			Number:    2,
			Title:     "Second",
			HeadRef:   "bugfix",
			BaseRef:   "main",
			RepoOwner: "space",
			RepoName:  "beta",
			Updated:   time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC),
		},
	}

	input := strings.NewReader("1\n")
	var output bytes.Buffer

	if _, err := selectWithPrompt(prs, input, &output); err != nil {
		t.Fatalf("selectWithPrompt returned error: %v", err)
	}

	lines := strings.Split(output.String(), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least two lines of output, got %d", len(lines))
	}

	expectedFirst := "[1] octo/alpha#1 - First [feature\u2192main] updated 2024-01-02 03:04Z"
	if lines[0] != expectedFirst {
		t.Fatalf("unexpected first line. want %q got %q", expectedFirst, lines[0])
	}
}

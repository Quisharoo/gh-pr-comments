package ghprcomments

import (
	"testing"

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

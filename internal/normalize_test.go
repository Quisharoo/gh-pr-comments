package ghprcomments

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v61/github"
)

func TestBuildOutputKeepsBotsAndCleansBody(t *testing.T) {
	createdAt := github.Timestamp{Time: time.Date(2025, time.October, 20, 17, 30, 0, 0, time.UTC)}
	later := github.Timestamp{Time: createdAt.Time.Add(2 * time.Minute)}

	payload := commentPayload{
		issueComments: []*github.IssueComment{
			{
				ID:        github.Int64(1),
				Body:      github.String("# Heading\n\nSee [docs](https://docs.github.com/en).\n\n```diff\n- old\n+ new\n```"),
				CreatedAt: &createdAt,
				HTMLURL:   github.String("https://github.com/org/repo/pull/1#issuecomment-1"),
				User:      &github.User{Login: github.String("human")},
			},
			{
				ID:        github.Int64(2),
				Body:      github.String("bot noise"),
				CreatedAt: &later,
				HTMLURL:   github.String("https://github.com/org/repo/pull/1#issuecomment-2"),
				User:      &github.User{Login: github.String("copilot[bot]")},
			},
		},
	}

	pr := &PullRequestSummary{
		Number:    39,
		Title:     "Add React UI",
		Author:    "dev",
		State:     "open",
		Updated:   time.Date(2025, time.October, 20, 17, 29, 55, 0, time.UTC),
		HeadRef:   "feature",
		RepoOwner: "Quisharoo",
		RepoName:  "chatgpt-obsidian-converter",
	}

	out := BuildOutput(pr, payload, NormalizationOptions{})

	if len(out.Comments) != 2 {
		t.Fatalf("expected 2 comments including bots, got %d", len(out.Comments))
	}

	first := out.Comments[0]
	if first.Author != "human" {
		t.Fatalf("expected first comment author 'human', got %q", first.Author)
	}
	if first.Permalink != "https://github.com/org/repo/pull/1#issuecomment-1" {
		t.Fatalf("expected permalink to be preserved, got %q", first.Permalink)
	}
	if strings.Contains(first.BodyText, "http") {
		t.Fatalf("cleaned body should not contain raw URLs, got %q", first.BodyText)
	}
	if strings.Contains(first.BodyText, "```") || strings.Contains(first.BodyText, "# Heading") {
		t.Fatalf("cleaned body should strip markdown artifacts, got %q", first.BodyText)
	}

	second := out.Comments[1]
	if second.Author != "copilot[bot]" {
		t.Fatalf("expected bot comment retained, got %q", second.Author)
	}
	if second.BodyText != "bot noise" {
		t.Fatalf("expected bot comment body preserved, got %q", second.BodyText)
	}
}

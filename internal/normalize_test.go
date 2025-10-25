package ghprcomments

import (
	"os"
	"path/filepath"
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
		t.Fatalf("expected 2 author groups including bots, got %d", len(out.Comments))
	}
	if out.CommentCount != 2 {
		t.Fatalf("expected comment count to be 2, got %d", out.CommentCount)
	}

	firstGroup := out.Comments[0]
	if firstGroup.Author != "copilot[bot]" {
		t.Fatalf("expected newest author group to be 'copilot[bot]', got %q", firstGroup.Author)
	}
	if len(firstGroup.Comments) != 1 {
		t.Fatalf("expected bot to have 1 comment, got %d", len(firstGroup.Comments))
	}
	if firstGroup.Comments[0].BodyText != "bot noise" {
		t.Fatalf("expected bot comment body preserved, got %q", firstGroup.Comments[0].BodyText)
	}

	secondGroup := out.Comments[1]
	if secondGroup.Author != "human" {
		t.Fatalf("expected second author group to be 'human', got %q", secondGroup.Author)
	}
	if len(secondGroup.Comments) != 1 {
		t.Fatalf("expected human to have 1 comment, got %d", len(secondGroup.Comments))
	}

	first := secondGroup.Comments[0]
	if first.Permalink != "https://github.com/org/repo/pull/1#issuecomment-1" {
		t.Fatalf("expected permalink to be preserved, got %q", first.Permalink)
	}
	if strings.Contains(first.BodyText, "http") {
		t.Fatalf("cleaned body should not contain raw URLs, got %q", first.BodyText)
	}
	if strings.Contains(first.BodyText, "```") || strings.Contains(first.BodyText, "# Heading") {
		t.Fatalf("cleaned body should strip markdown artifacts, got %q", first.BodyText)
	}
}

func TestCleanCommentBodyPreservesDetailsContent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "example_bot_feedback.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	body := string(data)
	got := cleanCommentBody(body, NormalizationOptions{})
	checks := []string{
		"Prevent overwriting a generic file",
		"Return an error in SaveOutput",
		"Suggestion importance[1-10]: 8",
		"Why: The suggestion correctly identifies a valid edge case",
	}

	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("cleaned body missing %q in %q", want, got)
		}
	}
}

package ghprcomments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

	got, err := selectWithPrompt(prs, input, &output, SelectPromptOptions{})
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

	if _, err := selectWithPrompt(prs, input, &output, SelectPromptOptions{}); err == nil {
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

	if _, err := selectWithPrompt(prs, input, &output, SelectPromptOptions{}); err != nil {
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

	if _, err := selectWithPrompt(prs, input, &output, SelectPromptOptions{}); err != nil {
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

func TestSelectWithPromptColourizedOutput(t *testing.T) {
	prs := []*PullRequestSummary{
		{
			Number:    10,
			Title:     "Colourful",
			HeadRef:   "feature",
			BaseRef:   "main",
			RepoOwner: "octo",
			RepoName:  "alpha",
			Updated:   time.Date(2025, 10, 24, 12, 0, 0, 0, time.UTC),
		},
	}

	input := strings.NewReader("1\n")
	var output bytes.Buffer

	if _, err := selectWithPrompt(prs, input, &output, SelectPromptOptions{Colorize: true}); err != nil {
		t.Fatalf("selectWithPrompt returned error: %v", err)
	}

	includeOwner := shouldShowRepoOwner(prs)
	repoDisplay := formatRepoDisplay(prs[0], includeOwner)

	expectedLine := fmt.Sprintf("%s %s%s - %s %s %s\n",
		applyStyle(true, ansiDim, "[1]"),
		applyStyle(true, ansiBrightCyan, repoDisplay),
		applyStyle(true, ansiYellow, "#10"),
		"Colourful",
		applyStyle(true, ansiMagenta, "[feature\u2192main]"),
		applyStyle(true, ansiDim, "updated 2025-10-24 12:00Z"),
	)

	expected := expectedLine + "Select by index, PR number, or owner/repo#number: "

	if got := output.String(); got != expected {
		t.Fatalf("unexpected coloured prompt output.\nwant: %q\n got: %q", expected, got)
	}
}

func TestSaveOutputCreatesDirectoryAndFile(t *testing.T) {
	repoRoot := t.TempDir()
	pr := &PullRequestSummary{
		Number:    123,
		Title:     "Add Feature 🚀",
		HeadRef:   "feature/add-feature",
		BaseRef:   "main",
		RepoOwner: "octo",
		RepoName:  "repo",
		Author:    "tester",
		URL:       "https://example.com",
	}
	payload := []byte(`{"ok":true}`)

	path, err := SaveOutput(repoRoot, pr, payload, "")
	if err != nil {
		t.Fatalf("SaveOutput returned error: %v", err)
	}

	dir := filepath.Join(repoRoot, ".pr-comments")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("expected directory %s to exist", dir)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved payload: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("expected YAML front matter, got %q", content)
	}
	if !strings.Contains(content, "pr_number: 123") {
		t.Fatalf("front matter missing pr_number: %q", content)
	}
	if !strings.Contains(content, `pr_title: "Add Feature 🚀"`) {
		t.Fatalf("front matter missing pr_title: %q", content)
	}
	if !strings.Contains(content, `repo_owner: "octo"`) {
		t.Fatalf("front matter missing repo_owner: %q", content)
	}
	if !strings.Contains(content, `repo_name: "repo"`) {
		t.Fatalf("front matter missing repo_name: %q", content)
	}
	if !strings.Contains(content, `head_ref: "feature/add-feature"`) {
		t.Fatalf("front matter missing head_ref: %q", content)
	}
	if !strings.Contains(content, `base_ref: "main"`) {
		t.Fatalf("front matter missing base_ref: %q", content)
	}
	if !strings.Contains(content, `author: "tester"`) {
		t.Fatalf("front matter missing author: %q", content)
	}
	if !strings.Contains(content, `url: "https://example.com"`) {
		t.Fatalf("front matter missing url: %q", content)
	}

	savedAtRe := regexp.MustCompile(`saved_at: "[^"]+"`)
	if !savedAtRe.MatchString(content) {
		t.Fatalf("front matter missing saved_at timestamp: %q", content)
	}

	jsonBlock := "```json\n" + string(payload) + "\n```"
	if !strings.Contains(content, jsonBlock) {
		t.Fatalf("expected JSON block to contain payload; want %q in %q", jsonBlock, content)
	}

	base := filepath.Base(path)
	if base != "pr-123-add-feature.md" {
		t.Fatalf("unexpected filename %q, expected pr-123-add-feature.md", base)
	}
}

func TestSaveOutputOverwritesExistingFile(t *testing.T) {
	repoRoot := t.TempDir()
	pr := &PullRequestSummary{Number: 5, Title: "Improve API", HeadRef: "feature"}
	payload1 := []byte("first payload")
	payload2 := []byte("second payload")

	first, err := SaveOutput(repoRoot, pr, payload1, "")
	if err != nil {
		t.Fatalf("first SaveOutput returned error: %v", err)
	}
	if filepath.Base(first) != "pr-5-improve-api.md" {
		t.Fatalf("unexpected filename %q", first)
	}

	second, err := SaveOutput(repoRoot, pr, payload2, "")
	if err != nil {
		t.Fatalf("second SaveOutput returned error: %v", err)
	}

	if first != second {
		t.Fatalf("expected same filename, got %q and %q", first, second)
	}

	data, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, string(payload1)) {
		t.Fatalf("expected old payload to be overwritten, found in content: %q", content)
	}
	if !strings.Contains(content, string(payload2)) {
		t.Fatalf("file missing new payload: %q", content)
	}
}

func TestSaveOutputRespectsCustomDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	customDir := "codex-artifacts"
	pr := &PullRequestSummary{Number: 42, Title: "Custom Save", HeadRef: "feature"}
	payload := []byte("{}")

	path, err := SaveOutput(repoRoot, pr, payload, customDir)
	if err != nil {
		t.Fatalf("SaveOutput returned error: %v", err)
	}

	expectedDir := filepath.Join(repoRoot, customDir)
	if dirInfo, err := os.Stat(expectedDir); err != nil || !dirInfo.IsDir() {
		t.Fatalf("expected custom directory %s to exist", expectedDir)
	}
	if !strings.HasPrefix(path, expectedDir+string(os.PathSeparator)) {
		t.Fatalf("expected path %q to reside within %s", path, expectedDir)
	}
}

func TestSaveOutputSupportsAbsoluteDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	absoluteDir := filepath.Join(t.TempDir(), "gh-pr-comments-artifacts")
	pr := &PullRequestSummary{
		Number:    7,
		Title:     "Absolute",
		HeadRef:   "feature",
		RepoOwner: "octo",
		RepoName:  "repo",
	}
	payload := []byte("{}")

	path, err := SaveOutput(repoRoot, pr, payload, absoluteDir)
	if err != nil {
		t.Fatalf("SaveOutput returned error: %v", err)
	}

	expectedDir := filepath.Join(absoluteDir, "octo-repo")
	if info, err := os.Stat(expectedDir); err != nil || !info.IsDir() {
		t.Fatalf("expected directory %s to exist", expectedDir)
	}
	if !strings.HasPrefix(path, expectedDir+string(os.PathSeparator)) {
		t.Fatalf("expected path %q to reside within %s", path, expectedDir)
	}
}

func TestSaveOutputRequiresPullRequestNumber(t *testing.T) {
	repoRoot := t.TempDir()
	payload := []byte("{}")

	if _, err := SaveOutput(repoRoot, nil, payload, ""); err == nil {
		t.Fatal("expected error when PR is nil")
	}

	if _, err := SaveOutput(repoRoot, &PullRequestSummary{Number: 0}, payload, ""); err == nil {
		t.Fatal("expected error when PR number is zero")
	}
}

func TestPruneStaleSavedCommentsRemovesClosedFiles(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, ".pr-comments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create comments directory: %v", err)
	}

	openFile := filepath.Join(dir, "pr-7-colourful.md")
	closedFile := filepath.Join(dir, "pr-9-defunct.md")

	if err := os.WriteFile(openFile, []byte("open"), 0o644); err != nil {
		t.Fatalf("write open file: %v", err)
	}
	if err := os.WriteFile(closedFile, []byte("closed"), 0o644); err != nil {
		t.Fatalf("write closed file: %v", err)
	}

	getter := &fakeSummaryGetter{
		summaries: map[int]*PullRequestSummary{
			7: {Number: 7, State: "open"},
			9: {Number: 9, State: "closed"},
		},
	}

	ctx := context.Background()
	removed, err := PruneStaleSavedComments(ctx, getter, repoRoot, "octo", "repo", []*PullRequestSummary{{Number: 7, State: "open"}}, "")
	if err != nil {
		t.Fatalf("PruneStaleSavedComments returned error: %v", err)
	}
	if len(removed) != 1 || removed[0] != closedFile {
		t.Fatalf("expected closed file to be reported as removed, got %v", removed)
	}

	if _, err := os.Stat(openFile); err != nil {
		t.Fatalf("expected open PR file to remain: %v", err)
	}
	if _, err := os.Stat(closedFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected closed PR file to be removed, got err=%v", err)
	}
}

func TestPruneStaleSavedCommentsHonoursCustomDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	customDir := filepath.Join(repoRoot, "codex-artifacts")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("failed to create custom directory: %v", err)
	}

	closed := filepath.Join(customDir, "pr-13-closed.md")
	if err := os.WriteFile(closed, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write closed file: %v", err)
	}

	getter := &fakeSummaryGetter{
		summaries: map[int]*PullRequestSummary{
			13: {Number: 13, State: "closed"},
		},
	}

	removed, err := PruneStaleSavedComments(context.Background(), getter, repoRoot, "octo", "repo", nil, "codex-artifacts")
	if err != nil {
		t.Fatalf("PruneStaleSavedComments returned error: %v", err)
	}
	if len(removed) != 1 || removed[0] != closed {
		t.Fatalf("expected custom directory file to be removed, got %v", removed)
	}
	if _, err := os.Stat(closed); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be removed from custom directory, got %v", err)
	}
}

func TestPruneStaleSavedCommentsIsolatesSharedDirectoryByRepo(t *testing.T) {
	repoRoot := t.TempDir()
	sharedDir := t.TempDir()

	repoADir := filepath.Join(sharedDir, "octo-alpha")
	repoBDir := filepath.Join(sharedDir, "octo-beta")

	if err := os.MkdirAll(repoADir, 0o755); err != nil {
		t.Fatalf("failed to create repo A directory: %v", err)
	}
	if err := os.MkdirAll(repoBDir, 0o755); err != nil {
		t.Fatalf("failed to create repo B directory: %v", err)
	}

	closedInRepoA := filepath.Join(repoADir, "pr-42-alpha.md")
	stillOpenRepoB := filepath.Join(repoBDir, "pr-42-beta.md")

	if err := os.WriteFile(closedInRepoA, []byte("closed"), 0o644); err != nil {
		t.Fatalf("write repo A file: %v", err)
	}
	if err := os.WriteFile(stillOpenRepoB, []byte("open"), 0o644); err != nil {
		t.Fatalf("write repo B file: %v", err)
	}

	getter := &fakeSummaryGetter{
		summaries: map[int]*PullRequestSummary{
			42: {Number: 42, State: "closed"},
		},
	}

	removed, err := PruneStaleSavedComments(context.Background(), getter, repoRoot, "octo", "alpha", nil, sharedDir)
	if err != nil {
		t.Fatalf("PruneStaleSavedComments returned error: %v", err)
	}
	if len(removed) != 1 || removed[0] != closedInRepoA {
		t.Fatalf("expected repo A file to be removed, got %v", removed)
	}

	if _, err := os.Stat(closedInRepoA); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected repo A file to be removed, got %v", err)
	}
	if _, err := os.Stat(stillOpenRepoB); err != nil {
		t.Fatalf("expected repo B file to remain, got %v", err)
	}
}

func TestPruneStaleSavedCommentsRemovesDeletedPRs(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, ".pr-comments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create comments directory: %v", err)
	}

	deleted := filepath.Join(dir, "pr-12-deleted.md")
	if err := os.WriteFile(deleted, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write deleted file: %v", err)
	}

	getter := &fakeSummaryGetter{errors: map[int]error{
		12: &github.ErrorResponse{Response: &http.Response{StatusCode: http.StatusNotFound}, Message: "Not Found"},
	}}

	removed, err := PruneStaleSavedComments(context.Background(), getter, repoRoot, "octo", "repo", nil, "")
	if err != nil {
		t.Fatalf("PruneStaleSavedComments returned error: %v", err)
	}
	if len(removed) != 1 || removed[0] != deleted {
		t.Fatalf("expected deleted file to be reported as removed, got %v", removed)
	}

	if _, err := os.Stat(deleted); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected deleted PR file to be removed, got err=%v", err)
	}
}

func TestPruneStaleSavedCommentsReturnsErrorWhenLookupFails(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, ".pr-comments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create comments directory: %v", err)
	}

	filePath := filepath.Join(dir, "pr-11-failure.md")
	if err := os.WriteFile(filePath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	lookupErr := fmt.Errorf("boom")
	getter := &fakeSummaryGetter{errors: map[int]error{11: lookupErr}}

	removed, err := PruneStaleSavedComments(context.Background(), getter, repoRoot, "octo", "repo", nil, "")
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include lookup failure; got %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("expected no files reported removed on error, got %v", removed)
	}

	if _, statErr := os.Stat(filePath); statErr != nil {
		t.Fatalf("expected file to remain after lookup error: %v", statErr)
	}
}

type fakeSummaryGetter struct {
	summaries map[int]*PullRequestSummary
	errors    map[int]error
}

func (f *fakeSummaryGetter) GetPullRequestSummary(_ context.Context, _ string, _ string, number int) (*PullRequestSummary, error) {
	if err, ok := f.errors[number]; ok {
		return nil, err
	}
	if summary, ok := f.summaries[number]; ok {
		return summary, nil
	}
	return nil, fmt.Errorf("pull request %d not found", number)
}

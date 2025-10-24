package ghprcomments

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
)

var botRegex = regexp.MustCompile(`(?i)(copilot|compliance|security|dependabot|.*\[bot\])`)
var htmlTagRegex = regexp.MustCompile(`(?s)<[^>]+>`)

// DetectRepository determines the owner/name pair for the current context.
func DetectRepository(ctx context.Context) (string, string, error) {
	if repo := os.Getenv("GH_REPO"); repo != "" {
		return splitRepo(repo)
	}

	if HasCommand("gh") {
		if owner, repo, err := detectRepoViaGH(ctx); err == nil {
			return owner, repo, nil
		}
	}

	return detectRepoViaGit(ctx)
}

func detectRepoViaGH(ctx context.Context) (string, string, error) {
	cmd := exec.CommandContext(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	cmd.Stdin = nil
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", "", err
	}
	return splitRepo(strings.TrimSpace(stdout.String()))
}

func detectRepoViaGit(ctx context.Context) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", "remote.origin.url")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", "", errors.New("unable to determine repository; run inside a git repo")
	}

	remote := strings.TrimSpace(stdout.String())
	repo := parseRepoFromRemote(remote)
	if repo == "" {
		return "", "", fmt.Errorf("could not parse repository from remote: %s", remote)
	}
	return splitRepo(repo)
}

func splitRepo(repo string) (string, string, error) {
	repo = strings.TrimSpace(repo)
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo identifier: %s", repo)
	}
	return parts[0], parts[1], nil
}

func parseRepoFromRemote(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	if strings.HasPrefix(remote, "git@") {
		if idx := strings.Index(remote, ":"); idx != -1 {
			return remote[idx+1:]
		}
	}
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "http://") {
		segments := strings.Split(remote, "/")
		if len(segments) >= 2 {
			return strings.Join(segments[len(segments)-2:], "/")
		}
	}
	if strings.Contains(remote, "/") {
		segments := strings.Split(remote, "/")
		return strings.Join(segments[len(segments)-2:], "/")
	}
	return ""
}

// SelectPullRequest chooses a PR either via fzf or a numbered fallback prompt.
func SelectPullRequest(ctx context.Context, prs []*PullRequestSummary, in io.Reader, out io.Writer) (*PullRequestSummary, error) {
	if len(prs) == 0 {
		return nil, errors.New("no pull requests available")
	}

	// Use the simple numbered prompt selection flow only.
	// Historically we attempted to use `fzf` when available and fell
	// back to the prompt on error. That created a secondary interactive
	// prompt (when users escaped the fzf modal). To keep the UX
	// deterministic for non-TTY environments and avoid duplicate
	// prompts, always use the prompt-based selector.
	return selectWithPrompt(prs, in, out)
}

func selectWithFZF(ctx context.Context, prs []*PullRequestSummary) (*PullRequestSummary, error) {
	var input bytes.Buffer
	for _, pr := range prs {
		line := fmt.Sprintf("%s #%d (opened %s)\t@%s\tupdated %s", pr.Title, pr.Number, displayDate(pr.Created), pr.Author, pr.Updated.Format(time.RFC3339))
		input.WriteString(line)
		input.WriteByte('\n')
	}

	cmd := exec.CommandContext(ctx, "fzf", "--prompt", "Select PR> ", "--with-nth", "1,2", "--no-sort")
	cmd.Stdin = &input
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	selection := strings.TrimSpace(stdout.String())
	if selection == "" {
		return nil, errors.New("no pull request selected")
	}

	fields := strings.Fields(selection)
	if len(fields) == 0 {
		return nil, errors.New("unable to parse selection output")
	}
	numberStr := strings.TrimPrefix(fields[0], "#")
	num, err := strconv.Atoi(numberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid selection: %s", selection)
	}

	for _, pr := range prs {
		if pr.Number == num {
			return pr, nil
		}
	}

	return nil, fmt.Errorf("selected PR #%d not found", num)
}

func selectWithPrompt(prs []*PullRequestSummary, in io.Reader, out io.Writer) (*PullRequestSummary, error) {
	fmt.Fprintln(out, "Available pull requests:")
	for idx, pr := range prs {
		fmt.Fprintf(out, "[%d] %s #%d (opened %s) — by @%s, updated %s\n", idx+1, pr.Title, pr.Number, displayDate(pr.Created), pr.Author, pr.Updated.Format(time.RFC3339))
	}
	fmt.Fprint(out, "Select by index or PR number: ")

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return nil, errors.New("no selection provided")
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return nil, errors.New("no selection provided")
	}

	if idx, err := strconv.Atoi(input); err == nil {
		if idx >= 1 && idx <= len(prs) {
			return prs[idx-1], nil
		}
	}

	num, err := strconv.Atoi(strings.TrimPrefix(input, "#"))
	if err != nil {
		return nil, fmt.Errorf("invalid input: %s", input)
	}
	for _, pr := range prs {
		if pr.Number == num {
			return pr, nil
		}
	}
	return nil, fmt.Errorf("pull request #%d not found", num)
}

func displayDate(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("2006-01-02")
}

// StripHTML removes HTML tags in a minimal fashion.
func StripHTML(body string) string {
	if body == "" {
		return body
	}

	replacer := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n")
	body = replacer.Replace(body)
	return htmlTagRegex.ReplaceAllString(body, "")
}

// IsBotAuthor returns true if the author matches the bot regex.
func IsBotAuthor(user *github.User) bool {
	if user == nil {
		return false
	}
	login := strings.ToLower(strings.TrimSpace(user.GetLogin()))
	if login != "" && botRegex.MatchString(login) {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(user.GetName()))
	return name != "" && botRegex.MatchString(name)
}

// HasCommand reports whether a CLI is available on PATH.
func HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// FindRepoRoot discovers the git repository root directory.
func FindRepoRoot(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", errors.New("unable to determine repo root; run inside a git repository")
	}
	return strings.TrimSpace(stdout.String()), nil
}

// SaveOutput persists the rendered payload to the .pr-comments directory.
func SaveOutput(repoRoot string, pr *PullRequestSummary, payload []byte) (string, error) {
	dir := filepath.Join(repoRoot, ".pr-comments")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	branch := sanitizeBranch(pr.HeadRef)
	filename := fmt.Sprintf("PR_%d_%s.json", pr.Number, branch)
	target := filepath.Join(dir, filename)

	if err := os.WriteFile(target, payload, 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func sanitizeBranch(ref string) string {
	if ref == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "_", " ", "_", "\t", "_")
	return replacer.Replace(ref)
}

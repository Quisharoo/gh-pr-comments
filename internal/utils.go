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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
)

var botRegex = regexp.MustCompile(`(?i)(copilot|compliance|security|dependabot|.*\[bot\])`)
var htmlTagRegex = regexp.MustCompile(`(?s)<[^>]+>`)

// Repository represents a local git repository and its remote metadata.
type Repository struct {
	Owner string
	Name  string
	Path  string
}

func (r Repository) fullName() string {
	owner := strings.TrimSpace(r.Owner)
	name := strings.TrimSpace(r.Name)
	switch {
	case owner == "" && name == "":
		return ""
	case owner == "":
		return name
	case name == "":
		return owner
	default:
		return owner + "/" + name
	}
}

// DetectRepository determines the owner/name pair for the current context.
func DetectRepository(ctx context.Context) (string, string, error) {
	repos, err := DetectRepositories(ctx)
	if err != nil {
		return "", "", err
	}
	if len(repos) == 0 {
		return "", "", errors.New("no repositories found")
	}
	if len(repos) > 1 {
		return "", "", errors.New("multiple repositories detected; use DetectRepositories")
	}
	return repos[0].Owner, repos[0].Name, nil
}

// DetectRepositories returns all repositories discoverable from the current directory.
func DetectRepositories(ctx context.Context) ([]Repository, error) {
	if repo := os.Getenv("GH_REPO"); repo != "" {
		owner, name, err := splitRepo(repo)
		if err != nil {
			return nil, err
		}
		root, _ := FindRepoRoot(ctx)
		return []Repository{{Owner: owner, Name: name, Path: root}}, nil
	}

	if HasCommand("gh") {
		if owner, repo, err := detectRepoViaGH(ctx); err == nil {
			root, _ := FindRepoRoot(ctx)
			return []Repository{{Owner: owner, Name: repo, Path: root}}, nil
		}
	}

	if owner, repo, err := detectRepoViaGit(ctx); err == nil {
		root, errRoot := FindRepoRoot(ctx)
		if errRoot != nil {
			root, _ = findRepoRootAt(ctx, ".")
		}
		return []Repository{{Owner: owner, Name: repo, Path: root}}, nil
	}

	repos, err := discoverNestedRepositories(ctx, ".")
	if err != nil {
		return nil, err
	}
	if len(repos) == 0 {
		return nil, errors.New("unable to determine repository; run inside a git repo")
	}
	sort.Slice(repos, func(i, j int) bool {
		a := repos[i].fullName()
		b := repos[j].fullName()
		if a == b {
			return repos[i].Path < repos[j].Path
		}
		return a < b
	})
	return repos, nil
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
	return detectRepoViaGitAt(ctx, ".")
}

func detectRepoViaGitAt(ctx context.Context, path string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "config", "--get", "remote.origin.url")
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

func discoverNestedRepositories(ctx context.Context, root string) ([]Repository, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	const maxDepth = 2
	skipNames := map[string]struct{}{
		".git":         {},
		".hg":          {},
		".svn":         {},
		"node_modules": {},
		"vendor":       {},
		"__pycache__":  {},
	}

	type queueItem struct {
		path  string
		depth int
	}

	queue := []queueItem{{path: rootAbs, depth: 0}}
	repos := make([]Repository, 0)
	seenRoots := make(map[string]struct{})

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(item.path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			if _, skip := skipNames[name]; skip {
				continue
			}

			childPath := filepath.Join(item.path, name)

			rootPath, err := findRepoRootAt(ctx, childPath)
			if err == nil {
				owner, repo, derr := detectRepoViaGitAt(ctx, rootPath)
				if derr != nil {
					continue
				}
				if _, seen := seenRoots[rootPath]; seen {
					continue
				}
				seenRoots[rootPath] = struct{}{}
				repos = append(repos, Repository{Owner: owner, Name: repo, Path: rootPath})
				continue
			}

			if item.depth+1 > maxDepth {
				continue
			}
			queue = append(queue, queueItem{path: childPath, depth: item.depth + 1})
		}
	}

	return repos, nil
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
		repoName := summaryRepoName(pr)
		if repoName == "" {
			repoName = "(unknown repo)"
		}
		line := fmt.Sprintf("%s #%d — %s\tstate %s\tupdated %s", repoName, pr.Number, pr.Title, valueOrFallback(strings.ToLower(pr.State), "unknown"), formatTimestamp(pr.Updated))
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

	repoPart, rest, found := strings.Cut(selection, "#")
	if !found {
		return nil, errors.New("unable to parse selection output")
	}
	repoPart = strings.TrimSpace(repoPart)
	if repoPart == "" {
		return nil, errors.New("unable to parse selection output")
	}

	numberDigits := make([]rune, 0, len(rest))
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		numberDigits = append(numberDigits, r)
	}
	if len(numberDigits) == 0 {
		return nil, errors.New("unable to parse selection output")
	}

	num, err := strconv.Atoi(string(numberDigits))
	if err != nil {
		return nil, fmt.Errorf("invalid selection: %s", selection)
	}

	if match := findByRepoAndNumber(prs, repoPart, num); match != nil {
		return match, nil
	}

	matches := make([]*PullRequestSummary, 0)
	for _, pr := range prs {
		if pr.Number == num {
			matches = append(matches, pr)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("selected PR #%d is present in multiple repositories; specify owner/repo in the prompt", num)
	}
	return nil, fmt.Errorf("selected PR #%d not found", num)
}

func selectWithPrompt(prs []*PullRequestSummary, in io.Reader, out io.Writer) (*PullRequestSummary, error) {
	fmt.Fprintln(out, "Available pull requests:")
	for idx, pr := range prs {
		repoName := summaryRepoName(pr)
		if repoName == "" {
			repoName = "(unknown repo)"
		}
		state := pr.State
		if state == "" {
			state = "unknown"
		}

		headRef := pr.HeadRef
		if headRef == "" {
			headRef = "?"
		}

		baseRef := pr.BaseRef
		if baseRef == "" {
			baseRef = "?"
		}

		fmt.Fprintf(out, "[%d] %s #%d — %s (%s) [%s -> %s]; opened %s; author @%s; updated %s\n",
			idx+1,
			repoName,
			pr.Number,
			pr.Title,
			state,
			headRef,
			baseRef,
			displayDate(pr.Created),
			valueOrFallback(pr.Author, "unknown"),
			formatTimestamp(pr.Updated),
		)
	}
	fmt.Fprint(out, "Select by index, PR number, or owner/repo#number: ")

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

	if before, after, found := strings.Cut(input, "#"); found && strings.TrimSpace(before) != "" {
		num, err := strconv.Atoi(strings.TrimSpace(after))
		if err != nil {
			return nil, fmt.Errorf("invalid input: %s", input)
		}
		match := findByRepoAndNumber(prs, strings.TrimSpace(before), num)
		if match != nil {
			return match, nil
		}
		return nil, fmt.Errorf("pull request %s#%d not found", strings.TrimSpace(before), num)
	}

	num, err := strconv.Atoi(strings.TrimPrefix(input, "#"))
	if err != nil {
		return nil, fmt.Errorf("invalid input: %s", input)
	}
	matches := make([]*PullRequestSummary, 0)
	for _, pr := range prs {
		if pr.Number == num {
			matches = append(matches, pr)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("pull request #%d not found", num)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("pull request #%d is present in multiple repositories; specify as owner/repo#%d", num, num)
	}
	return matches[0], nil
}

func displayDate(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("2006-01-02")
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format(time.RFC3339)
}

func valueOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func summaryRepoName(pr *PullRequestSummary) string {
	if pr == nil {
		return ""
	}
	owner := strings.TrimSpace(pr.RepoOwner)
	name := strings.TrimSpace(pr.RepoName)
	switch {
	case owner == "" && name == "":
		return ""
	case owner == "":
		return name
	case name == "":
		return owner
	default:
		return owner + "/" + name
	}
}

func findByRepoAndNumber(prs []*PullRequestSummary, repo string, number int) *PullRequestSummary {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return nil
	}

	for _, pr := range prs {
		if pr == nil {
			continue
		}
		if strings.EqualFold(summaryRepoName(pr), repo) && pr.Number == number {
			return pr
		}
	}

	if strings.Contains(repo, "/") {
		return nil
	}

	var matches []*PullRequestSummary
	for _, pr := range prs {
		if pr == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(pr.RepoName), repo) && pr.Number == number {
			matches = append(matches, pr)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return nil
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
	return findRepoRootAt(ctx, ".")
}

func findRepoRootAt(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
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

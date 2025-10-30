package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ghprcomments "github.com/Quish-Labs/gh-pr-comments/internal"
	"github.com/Quish-Labs/gh-pr-comments/internal/tui"
	"github.com/google/go-github/v61/github"
	"golang.org/x/term"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, in io.Reader, out, errOut io.Writer) error {
	args = normalizeArgs(args)
	fs := flag.NewFlagSet("gh-pr-comments", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var prNumber int
	var flat bool
	var text bool
	var save bool
	var stripHTML bool
	var noColour bool
	var noColor bool
	var saveDir string
	var noInteractive bool

	fs.IntVar(&prNumber, "p", 0, "pull request number")
	fs.IntVar(&prNumber, "pr", 0, "pull request number")
	fs.BoolVar(&flat, "flat", false, "emit a single JSON array of comments")
	fs.BoolVar(&text, "text", false, "render comments as Markdown")
	fs.BoolVar(&save, "save", false, "persist output (defaults to .pr-comments/; override via --save-dir or GH_PR_COMMENTS_SAVE_DIR)")
	fs.BoolVar(&stripHTML, "strip-html", false, "strip HTML tags from comment bodies")
	fs.BoolVar(&noColour, "no-colour", false, "disable coloured terminal output")
	fs.BoolVar(&noColor, "no-color", false, "disable colored terminal output")
	fs.StringVar(&saveDir, "save-dir", "", "override directory used by --save")
	fs.BoolVar(&noInteractive, "no-interactive", false, "disable interactive TUI (for piping/scripting)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if flat && text {
		return errors.New("cannot use --flat together with --text")
	}

	if text {
		stripHTML = true
	}

	// Determine if we should use interactive mode
	// Interactive is default unless:
	// - --no-interactive is set
	// - --save is set (saving is non-interactive)
	// - --text is set (markdown output is non-interactive)
	// - stdout is not a TTY (piping)
	useInteractive := !noInteractive && !save && !text && isTerminalWriter(out)

	if noColor {
		noColour = true
	}
	if envNoColor := strings.TrimSpace(os.Getenv("NO_COLOR")); envNoColor != "" {
		noColour = true
	}

	if saveDir == "" {
		saveDir = strings.TrimSpace(os.Getenv("GH_PR_COMMENTS_SAVE_DIR"))
	}

	colorEnabled := !noColour && isTerminalWriter(out)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	host := os.Getenv("GH_HOST")
	if host == "" {
		host = "github.com"
	}

	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	// If token not in environment, try to ask `gh` for the token (user already logged in
	// with the GitHub CLI). This keeps UX smooth for users who authenticate via `gh`.
	if token == "" {
		if out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output(); err == nil {
			tok := strings.TrimSpace(string(out))
			if tok != "" {
				token = tok
			}
		}
	}

	if token == "" {
		return errors.New("GH_TOKEN or GITHUB_TOKEN not set; run `gh auth login`")
	}

	repos, err := ghprcomments.DetectRepositories(ctx)
	if err != nil {
		return fmt.Errorf("detect repositories: %w", err)
	}
	if len(repos) == 0 {
		return errors.New("no repositories found; run inside or alongside a git repository")
	}

	client, err := ghprcomments.NewGitHubClient(ctx, token, host)
	if err != nil {
		return fmt.Errorf("create GitHub client: %w", err)
	}

	fetcher := ghprcomments.NewFetcher(client)

	var prSummary *ghprcomments.PullRequestSummary
	var selectedRepo ghprcomments.Repository

	repoLookup := make(map[string]ghprcomments.Repository)
	repoKey := func(owner, name string) string {
		owner = strings.ToLower(strings.TrimSpace(owner))
		name = strings.ToLower(strings.TrimSpace(name))
		if owner == "" {
			return name
		}
		if name == "" {
			return owner
		}
		return owner + "/" + name
	}

	for _, repo := range repos {
		repoLookup[repoKey(repo.Owner, repo.Name)] = repo
	}

	if prNumber > 0 {
		if len(repos) == 1 {
			selectedRepo = repos[0]
			prSummary, err = fetcher.GetPullRequestSummary(ctx, selectedRepo.Owner, selectedRepo.Name, prNumber)
			if err != nil {
				return fmt.Errorf("load pull request: %w", err)
			}
			prSummary.LocalPath = selectedRepo.Path
		} else {
			matches := make([]*ghprcomments.PullRequestSummary, 0)
			var errs []string
			for _, repo := range repos {
				summary, berr := fetcher.GetPullRequestSummary(ctx, repo.Owner, repo.Name, prNumber)
				if berr != nil {
					var ghErr *github.ErrorResponse
					if errors.As(berr, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == 404 {
						continue
					}
					errs = append(errs, fmt.Sprintf("%s/%s: %v", repo.Owner, repo.Name, berr))
					continue
				}
				summary.LocalPath = repo.Path
				matches = append(matches, summary)
			}

			if len(matches) == 0 {
				if len(errs) > 0 {
					return fmt.Errorf("load pull request #%d:\n%s", prNumber, strings.Join(errs, "\n"))
				}
				return fmt.Errorf("pull request #%d not found in discovered repositories", prNumber)
			}
			if len(matches) > 1 {
				return fmt.Errorf("pull request #%d found in multiple repositories; re-run without --pr and select interactively", prNumber)
			}
			prSummary = matches[0]
			selectedRepo = repoLookup[repoKey(prSummary.RepoOwner, prSummary.RepoName)]
			if selectedRepo.Path == "" {
				selectedRepo = ghprcomments.Repository{Owner: prSummary.RepoOwner, Name: prSummary.RepoName, Path: prSummary.LocalPath}
			}
		}

		// If interactive mode and PR was specified, fetch comments and launch JSON explorer directly
		if useInteractive {
			owner := strings.TrimSpace(prSummary.RepoOwner)
			repo := strings.TrimSpace(prSummary.RepoName)
			if owner == "" || repo == "" {
				owner = strings.TrimSpace(selectedRepo.Owner)
				repo = strings.TrimSpace(selectedRepo.Name)
			}

			payloads, err := fetcher.FetchComments(ctx, owner, repo, prNumber)
			if err != nil {
				return fmt.Errorf("fetch comments: %w", err)
			}

			normOpts := ghprcomments.NormalizationOptions{
				StripHTML: stripHTML,
			}

			output := ghprcomments.BuildOutput(prSummary, payloads, normOpts)
			jsonData, err := ghprcomments.MarshalJSON(output, flat)
			if err != nil {
				return fmt.Errorf("marshal JSON: %w", err)
			}

			// Launch JSON explorer directly
			_, err = tui.RunUnifiedFlow(nil, jsonData, nil)
			if err != nil {
				return fmt.Errorf("explore JSON: %w", err)
			}

			return nil
		}
	} else {
		all := make([]*ghprcomments.PullRequestSummary, 0)
		var errs []string
		for _, repo := range repos {
			prs, berr := fetcher.ListPullRequestSummaries(ctx, repo.Owner, repo.Name)
			if berr != nil {
				if errors.Is(berr, ghprcomments.ErrNoPullRequests) {
					continue
				}
				errs = append(errs, fmt.Sprintf("%s/%s: %v", repo.Owner, repo.Name, berr))
				continue
			}
			for _, pr := range prs {
				if pr.RepoOwner == "" {
					pr.RepoOwner = repo.Owner
				}
				if pr.RepoName == "" {
					pr.RepoName = repo.Name
				}
				pr.LocalPath = repo.Path
			}
			all = append(all, prs...)
		}

		var prunedFiles []string
		var pruneAttempted bool
		if len(all) == 0 {
			if save && len(errs) == 0 {
				pruneAttempted = true
				prunedFiles = pruneSavedComments(ctx, fetcher, repos, saveDir, errOut)
			}
			if len(errs) > 0 {
				return fmt.Errorf("list pull requests:\n%s", strings.Join(errs, "\n"))
			}
			if len(repos) == 1 {
				if pruneAttempted {
					if len(prunedFiles) > 0 {
						return fmt.Errorf("%w; removed stale saved comment files:\n%s", ghprcomments.ErrNoPullRequests, strings.Join(prunedFiles, "\n"))
					}
					return fmt.Errorf("%w; no stale saved comment files found", ghprcomments.ErrNoPullRequests)
				}
				return ghprcomments.ErrNoPullRequests
			}
			return errors.New("no open pull requests found across discovered repositories")
		}

		if len(errs) > 0 {
			for _, msg := range errs {
				fmt.Fprintf(errOut, "warning: %s\n", msg)
			}
		}

		// Convert to TUI-compatible format
		tuiPRs := make([]*tui.PullRequestSummary, len(all))
		for i, pr := range all {
			tuiPRs[i] = &tui.PullRequestSummary{
				Number:    pr.Number,
				Title:     pr.Title,
				Author:    pr.Author,
				State:     pr.State,
				Created:   pr.Created,
				Updated:   pr.Updated,
				HeadRef:   pr.HeadRef,
				BaseRef:   pr.BaseRef,
				RepoName:  pr.RepoName,
				RepoOwner: pr.RepoOwner,
				URL:       pr.URL,
				LocalPath: pr.LocalPath,
			}
		}

		// Use interactive TUI by default, fall back to classic prompt only if disabled
		if useInteractive {
			// Create a fetch function for the unified flow
			fetchCommentsFunc := func(selectedPR *tui.PullRequestSummary) ([]byte, error) {
				owner := strings.TrimSpace(selectedPR.RepoOwner)
				repo := strings.TrimSpace(selectedPR.RepoName)

				payloads, err := fetcher.FetchComments(ctx, owner, repo, selectedPR.Number)
				if err != nil {
					return nil, fmt.Errorf("fetch comments: %w", err)
				}

				// Convert TUI PR summary to internal format for BuildOutput
				internalPR := &ghprcomments.PullRequestSummary{
					Number:    selectedPR.Number,
					Title:     selectedPR.Title,
					Author:    selectedPR.Author,
					State:     selectedPR.State,
					Created:   selectedPR.Created,
					Updated:   selectedPR.Updated,
					HeadRef:   selectedPR.HeadRef,
					BaseRef:   selectedPR.BaseRef,
					RepoName:  selectedPR.RepoName,
					RepoOwner: selectedPR.RepoOwner,
					URL:       selectedPR.URL,
					LocalPath: selectedPR.LocalPath,
				}

				normOpts := ghprcomments.NormalizationOptions{
					StripHTML: stripHTML,
				}

				output := ghprcomments.BuildOutput(internalPR, payloads, normOpts)
				return ghprcomments.MarshalJSON(output, flat)
			}

			// Run unified flow: PR selection → loading → JSON explorer
			selectedTUI, err := tui.RunUnifiedFlow(tuiPRs, nil, fetchCommentsFunc)
			if err != nil {
				return fmt.Errorf("interactive flow: %w", err)
			}

			if selectedTUI == nil {
				return errors.New("no pull request selected")
			}

			// Convert back
			prSummary = &ghprcomments.PullRequestSummary{
				Number:    selectedTUI.Number,
				Title:     selectedTUI.Title,
				Author:    selectedTUI.Author,
				State:     selectedTUI.State,
				Created:   selectedTUI.Created,
				Updated:   selectedTUI.Updated,
				HeadRef:   selectedTUI.HeadRef,
				BaseRef:   selectedTUI.BaseRef,
				RepoName:  selectedTUI.RepoName,
				RepoOwner: selectedTUI.RepoOwner,
				URL:       selectedTUI.URL,
				LocalPath: selectedTUI.LocalPath,
			}

			// Interactive flow complete - we're done!
			return nil
		} else {
			prSummary, err = ghprcomments.SelectPullRequestWithOptions(ctx, all, in, out, ghprcomments.SelectPromptOptions{Colorize: colorEnabled})
			if err != nil {
				return fmt.Errorf("select pull request: %w", err)
			}
		}
		selectedRepo = repoLookup[repoKey(prSummary.RepoOwner, prSummary.RepoName)]
		if selectedRepo.Path == "" {
			selectedRepo = ghprcomments.Repository{Owner: prSummary.RepoOwner, Name: prSummary.RepoName, Path: prSummary.LocalPath}
		}
		prNumber = prSummary.Number
	}

	if prSummary == nil {
		return errors.New("no pull request selected")
	}

	owner := strings.TrimSpace(prSummary.RepoOwner)
	repo := strings.TrimSpace(prSummary.RepoName)
	if owner == "" || repo == "" {
		owner = strings.TrimSpace(selectedRepo.Owner)
		repo = strings.TrimSpace(selectedRepo.Name)
	}

	payloads, err := fetcher.FetchComments(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("fetch comments: %w", err)
	}

	normOpts := ghprcomments.NormalizationOptions{
		StripHTML: stripHTML,
	}

	output := ghprcomments.BuildOutput(prSummary, payloads, normOpts)

	if save {
		state := strings.ToLower(strings.TrimSpace(prSummary.State))
		if state != "open" {
			if state == "" {
				state = "unknown"
			}
			return fmt.Errorf("--save only supports open pull requests; #%d is %s", prSummary.Number, state)
		}

		repoRoot := strings.TrimSpace(selectedRepo.Path)
		if repoRoot == "" {
			var err error
			repoRoot, err = ghprcomments.FindRepoRoot(ctx)
			if err != nil {
				return fmt.Errorf("find repo root: %w", err)
			}
		}
		payload, err := ghprcomments.MarshalJSON(output, flat)
		if err != nil {
			return fmt.Errorf("marshal JSON for save: %w", err)
		}
		savePath, err := ghprcomments.SaveOutput(repoRoot, prSummary, payload, saveDir)
		if err != nil {
			return fmt.Errorf("save output: %w", err)
		}
		if _, err := fmt.Fprintf(out, "Comments saved to %s\n", savePath); err != nil {
			return fmt.Errorf("announce save path: %w", err)
		}

		openPRs, listErr := fetcher.ListPullRequestSummaries(ctx, owner, repo)
		if listErr != nil {
			if !errors.Is(listErr, ghprcomments.ErrNoPullRequests) {
				fmt.Fprintf(errOut, "warning: saved output but skipped pruning; unable to list open pull requests: %v\n", listErr)
				return nil
			}
			openPRs = nil
		}
		if _, pruneErr := ghprcomments.PruneStaleSavedComments(ctx, fetcher, repoRoot, owner, repo, openPRs, saveDir); pruneErr != nil {
			fmt.Fprintf(errOut, "warning: prune skipped; %v\n", pruneErr)
		}
		return nil
	}

	if text {
		markup := ghprcomments.RenderMarkdown(output)
		if _, err := fmt.Fprintln(out, markup); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
	} else {
		payload, err := ghprcomments.MarshalJSON(output, flat)
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}

		// Launch interactive JSON explorer by default when interactive mode is enabled
		if useInteractive {
			if err := tui.ExploreJSON(payload); err != nil {
				return fmt.Errorf("explore JSON: %w", err)
			}
			return nil
		}

		// Non-interactive: output to stdout
		display := payload
		if colorEnabled {
			display = ghprcomments.ColouriseJSONComments(colorEnabled, payload)
		}
		if _, err := out.Write(display); err != nil {
			return fmt.Errorf("write JSON: %w", err)
		}
		if len(payload) == 0 || payload[len(payload)-1] != '\n' {
			if _, err := out.Write([]byte("\n")); err != nil {
				return fmt.Errorf("write newline: %w", err)
			}
		}
	}

	return nil
}

func normalizeArgs(args []string) []string {
	cleaned := args
	for len(cleaned) > 0 {
		switch cleaned[0] {
		case "prcomments", "pr-comments", "gh-prcomments", "gh-pr-comments":
			cleaned = cleaned[1:]
			continue
		}
		break
	}
	return cleaned
}

func isTerminalWriter(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func pruneSavedComments(ctx context.Context, fetcher *ghprcomments.Fetcher, repos []ghprcomments.Repository, saveDir string, errOut io.Writer) []string {
	if fetcher == nil || len(repos) == 0 {
		return nil
	}

	removedSet := make(map[string]struct{})
	var removed []string
	seen := make(map[string]struct{})
	for _, repo := range repos {
		owner := strings.TrimSpace(repo.Owner)
		name := strings.TrimSpace(repo.Name)
		if owner == "" || name == "" {
			if errOut != nil {
				fmt.Fprintf(errOut, "warning: prune skipped; repository metadata incomplete for %q\n", strings.TrimSpace(repo.Path))
			}
			continue
		}

		repoRoot := strings.TrimSpace(repo.Path)
		if repoRoot == "" {
			root, err := ghprcomments.FindRepoRoot(ctx)
			if err != nil {
				if errOut != nil {
					fmt.Fprintf(errOut, "warning: prune skipped for %s/%s; %v\n", owner, name, err)
				}
				continue
			}
			repoRoot = root
		}

		key := repoRoot + "|" + owner + "|" + name
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		openPRs, err := fetcher.ListPullRequestSummaries(ctx, owner, name)
		if err != nil {
			if !errors.Is(err, ghprcomments.ErrNoPullRequests) {
				if errOut != nil {
					fmt.Fprintf(errOut, "warning: prune skipped for %s/%s; %v\n", owner, name, err)
				}
				continue
			}
			openPRs = nil
		}

		pruned, err := ghprcomments.PruneStaleSavedComments(ctx, fetcher, repoRoot, owner, name, openPRs, saveDir)
		if err != nil {
			if errOut != nil {
				fmt.Fprintf(errOut, "warning: prune skipped for %s/%s; %v\n", owner, name, err)
			}
			continue
		}

		for _, filePath := range pruned {
			if _, seenFile := removedSet[filePath]; seenFile {
				continue
			}
			removedSet[filePath] = struct{}{}

			display := filePath
			if rel, relErr := filepath.Rel(repoRoot, filePath); relErr == nil {
				display = fmt.Sprintf("%s/%s:%s", owner, name, filepath.ToSlash(rel))
			}
			removed = append(removed, display)
		}
	}

	if removed == nil {
		return []string{}
	}
	return removed
}

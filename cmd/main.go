package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	ghprcomments "github.com/Quish-Labs/gh-pr-comments/internal"
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
	fs := flag.NewFlagSet("gh-pr-comments", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var prNumber int
	var flat bool
	var text bool
	var save bool
	var stripHTML bool
	var noColour bool
	var noColor bool

	fs.IntVar(&prNumber, "p", 0, "pull request number")
	fs.IntVar(&prNumber, "pr", 0, "pull request number")
	fs.BoolVar(&flat, "flat", false, "emit a single JSON array of comments")
	fs.BoolVar(&text, "text", false, "render comments as Markdown")
	fs.BoolVar(&save, "save", false, "persist output to .pr-comments/")
	fs.BoolVar(&stripHTML, "strip-html", false, "strip HTML tags from comment bodies")
	fs.BoolVar(&noColour, "no-colour", false, "disable coloured terminal output")
	fs.BoolVar(&noColor, "no-color", false, "disable colored terminal output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if flat && text {
		return errors.New("cannot use --flat together with --text")
	}

	if text {
		stripHTML = true
	}

	if noColor {
		noColour = true
	}
	if envNoColor := strings.TrimSpace(os.Getenv("NO_COLOR")); envNoColor != "" {
		noColour = true
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

		if len(all) == 0 {
			if len(errs) > 0 {
				return fmt.Errorf("list pull requests:\n%s", strings.Join(errs, "\n"))
			}
			if len(repos) == 1 {
				return ghprcomments.ErrNoPullRequests
			}
			return errors.New("no open pull requests found across discovered repositories")
		}

		if len(errs) > 0 {
			for _, msg := range errs {
				fmt.Fprintf(errOut, "warning: %s\n", msg)
			}
		}

		prSummary, err = ghprcomments.SelectPullRequestWithOptions(ctx, all, in, out, ghprcomments.SelectPromptOptions{Colorize: colorEnabled})
		if err != nil {
			return fmt.Errorf("select pull request: %w", err)
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

	var rendered []byte

	if text {
		markup := ghprcomments.RenderMarkdown(output)
		if _, err := fmt.Fprintln(out, markup); err != nil {
			return fmt.Errorf("write markdown: %w", err)
		}
		payload, err := ghprcomments.MarshalJSON(output, false)
		if err != nil {
			return fmt.Errorf("marshal JSON for save: %w", err)
		}
		rendered = payload
	} else {
		payload, err := ghprcomments.MarshalJSON(output, flat)
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
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
		rendered = payload
	}

	if save {
		repoRoot := strings.TrimSpace(selectedRepo.Path)
		if repoRoot == "" {
			var err error
			repoRoot, err = ghprcomments.FindRepoRoot(ctx)
			if err != nil {
				return fmt.Errorf("find repo root: %w", err)
			}
		}
		savePath, err := ghprcomments.SaveOutput(repoRoot, prSummary, rendered)
		if err != nil {
			return fmt.Errorf("save output: %w", err)
		}
		if _, err := fmt.Fprintf(errOut, "saved output to %s\n", savePath); err != nil {
			return fmt.Errorf("announce save path: %w", err)
		}
	}

	return nil
}

func isTerminalWriter(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

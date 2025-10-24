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

	fs.IntVar(&prNumber, "p", 0, "pull request number")
	fs.IntVar(&prNumber, "pr", 0, "pull request number")
	fs.BoolVar(&flat, "flat", false, "emit a single JSON array of comments")
	fs.BoolVar(&text, "text", false, "render comments as Markdown")
	fs.BoolVar(&save, "save", false, "persist output to .pr-comments/")
	fs.BoolVar(&stripHTML, "strip-html", false, "strip HTML tags from comment bodies")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if flat && text {
		return errors.New("cannot use --flat together with --text")
	}

	if text {
		stripHTML = true
	}

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

	owner, repo, err := ghprcomments.DetectRepository(ctx)
	if err != nil {
		return fmt.Errorf("detect repository: %w", err)
	}

	client, err := ghprcomments.NewGitHubClient(ctx, token, host)
	if err != nil {
		return fmt.Errorf("create GitHub client: %w", err)
	}

	fetcher := ghprcomments.NewFetcher(client)

	var prSummary *ghprcomments.PullRequestSummary

	if prNumber > 0 {
		prSummary, err = fetcher.GetPullRequestSummary(ctx, owner, repo, prNumber)
		if err != nil {
			return fmt.Errorf("load pull request: %w", err)
		}
	} else {
		prs, err := fetcher.ListPullRequestSummaries(ctx, owner, repo)
		if err != nil {
			return fmt.Errorf("list pull requests: %w", err)
		}
		prSummary, err = ghprcomments.SelectPullRequest(ctx, prs, in, out)
		if err != nil {
			return fmt.Errorf("select pull request: %w", err)
		}
		prNumber = prSummary.Number
	}

	if prSummary == nil {
		return errors.New("no pull request selected")
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
		if _, err := out.Write(payload); err != nil {
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
		repoRoot, err := ghprcomments.FindRepoRoot(ctx)
		if err != nil {
			return fmt.Errorf("find repo root: %w", err)
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

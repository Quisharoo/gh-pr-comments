# gh-pr-comments

## Overview
`gh-pr-comments` is a Go-based GitHub CLI extension that fetches every comment, review, and review event on a pull request and emits a normalized export for humans or tooling.

## Key Features
- Detects the current repo via `gh` and supports an interactive PR picker.
- Streams unified JSON with bot tagging, optional flat array, or Markdown text output.
- Can persist snapshots to `.pr-comments/pr-<number>-<slug>.md` (Markdown with embedded JSON) for reuse.

## Prerequisites
- Go 1.22+
- GitHub CLI (`gh`) configured with an access token
- `fzf` for interactive selection (falls back to a basic prompt if absent)
- Optional: `jq` for piping JSON, `make` for project tasks

## Usage
- `gh pr-comments` to pick a PR interactively and print JSON
- Run `gh pr-comments` from a parent workspace with several git repos to browse every open PR across them
- `gh pr-comments -p <number>` to target a specific pull request
- `gh pr-comments --flat` for a single JSON array of comments
- `gh pr-comments --text` for Markdown output with HTML stripped
- `gh pr-comments --save` to write a Markdown snapshot with embedded JSON (defaults to `.pr-comments/`; override via `--save-dir` or `GH_PR_COMMENTS_SAVE_DIR`)
- `gh pr-comments --save --save-dir codex-artifacts` to drop shareable files in a tracked directory
- `gh pr-comments --no-colour` (or `--no-color`) to disable ANSI styling; also respects the `NO_COLOR` environment variable

**zsh auto-correct:** If your shell prompts to correct `pr-comments` to `.pr-comments`, leave it as-is or add `alias gh='nocorrect gh'` (or disable `CORRECT`) in your shell config to silence the prompt.

## Development
- `go test ./...` to run unit tests
- `go vet ./... && staticcheck ./...` for linting
- `go build -o gh-pr-comments ./cmd` to produce the extension binary

## Contributing
Respect the coding guardrails in `AGENTS.md` and keep changes small, tested, and well-documented.

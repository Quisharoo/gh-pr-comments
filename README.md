# gh-pr-comments

## Overview
`gh-pr-comments` is a Go-based GitHub CLI extension that fetches every comment, review, and review event on a pull request and emits a normalized export for humans or tooling.

## Key Features
- Detects the current repo via `gh` and supports an interactive PR picker.
- Streams unified JSON with bot tagging, optional flat array, or Markdown text output.
- Can persist snapshots to `.pr-comments/PR_<number>_<branch>.json` for reuse.

## Prerequisites
- Go 1.22+
- GitHub CLI (`gh`) configured with an access token
- `fzf` for interactive selection (falls back to a basic prompt if absent)
- Optional: `jq` for piping JSON, `make` for project tasks

## Usage
- `gh pr-comments` to pick a PR interactively and print JSON
- `gh pr-comments -p <number>` to target a specific pull request
- `gh pr-comments --flat` for a single JSON array of comments
- `gh pr-comments --text` for Markdown output with HTML stripped
- `gh pr-comments --save` to write the JSON payload under `.pr-comments/`

## Development
- `go test ./...` to run unit tests
- `make lint` for the vet/staticcheck bundle
- `go build ./cmd/...` to produce the extension binary

## Contributing
Respect the coding guardrails in `AGENTS.md` and keep changes small, tested, and well-documented.

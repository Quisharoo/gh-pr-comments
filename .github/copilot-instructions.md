# Copilot Instructions — gh-pr-comments

These notes keep Copilot (and other AI helpers) aligned with how the project really works. Prefer the README for user docs; this file is just build-and-code context.

## Quick Context
- CLI entry point: `cmd/main.go` handles flags, repo detection, and output wiring (including `--save-dir`).
- GitHub integration: `internal/api.go` wraps authenticated REST calls and pagination.
- Comment processing: `internal/normalize.go` merges issue comments, review comments, and reviews.
- Rendering: `internal/render.go` produces JSON (nested or flat) and Markdown output.
- Utilities: `internal/utils.go` covers repo discovery, local persistence, and TTY helpers.

## Build & Test
- `go mod download`
- `go build -o gh-pr-comments ./cmd`
- `go test ./...`
- `go vet ./... && ~/go/bin/staticcheck ./...`

## Development Guardrails
- Go 1.22+; dependencies pinned in `go.mod` (go-github/v61, x/oauth2, x/term).
- Business logic lives in `internal/`; keep `cmd/` focused on CLI orchestration.
- Propagate `context.Context` through API calls; respect the 60s timeout used in `main.go`.
- When persisting output, reuse helpers in `internal/utils.go` (default `.pr-comments/`; allow overrides via `GH_PR_COMMENTS_SAVE_DIR` or `--save-dir`).
- Avoid adding non-standard CLI frameworks; stick with the stdlib `flag` package.

## Manual Smoke Checks
- `./gh-pr-comments --help`
- `./gh-pr-comments` (interactive picker in a repo with PRs)
- `./gh-pr-comments -p <number> --flat`
- `./gh-pr-comments -p <number> --text`
- `./gh-pr-comments -p <number> --save` (default `.pr-comments/`; override with `--save-dir codex-artifacts` for tracked outputs)

Keep this file short—update only when the workflow or key files change.

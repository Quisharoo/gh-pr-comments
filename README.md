# gh-pr-comments

## Overview
`gh-pr-comments` is a Go-based GitHub CLI extension that fetches every comment, review, and review event on a pull request and provides an **interactive terminal UI** for browsing and exploring PR feedback.

## Key Features
- **Interactive TUI by default**: Modern Bubbletea-powered interface with fuzzy search and keyboard navigation
- **fx-inspired JSON explorer**: Navigate nested comment structures with vim keybindings, search, and collapsible trees
- **Multi-repo support**: Detects and lists PRs across all repos in your workspace
- **Flexible output modes**: JSON (structured or flat), Markdown, or interactive exploration
- **Bot detection**: Automatically tags and identifies bot comments
- **Persistent snapshots**: Save Markdown files with embedded JSON for reuse and collaboration

## Prerequisites
- Go 1.22+
- GitHub CLI (`gh`) configured with an access token

## Installation
From the repository root:
1. `go build -o gh-pr-comments ./cmd`
2. `gh extension install .`

`gh extension install .` copies the freshly built binary into the extension’s install directory. After installation you can optionally remove the workspace copy with `rm gh-pr-comments`.

## Usage

### Interactive Mode (Default)
When running in a terminal, `gh-pr-comments` launches an interactive TUI:

```bash
# Interactive flow: PR selector → JSON explorer
gh pr-comments

# Skip PR selection, go straight to JSON explorer
gh pr-comments --pr 123
```

**Interactive PR Selector:**
- `↑`/`↓` or `j`/`k` - Navigate through PRs
- `/` - Fuzzy search by repo, number, or title
- `Enter` - Select PR
- `o` - Open selected PR in browser
- `q` or `Esc` - Quit

**Interactive JSON Explorer (fx-style):**
- `j`/`k` or `↑`/`↓` - Navigate nodes
- `Enter`, `l`, or `→` - Toggle expand/collapse
- `h` or `←` - Collapse (or collapse parent if already collapsed)
- `y` or `c` - Copy highlighted value to clipboard
- `o` - Open URL in browser (for URL values)
- `E` / `C` - Expand all / Collapse all
- `g` / `G` - Go to top/bottom
- `/` - Search for keys or values
- `n` / `N` - Next/previous search match
- `Esc` - Clear search
- `q` - Quit

### Non-Interactive Mode (Piping/Scripting)
For automation, piping, or CI/CD:

```bash
# Pipe to file (auto-detects non-TTY)
gh pr-comments --pr 123 > comments.json

# Explicit non-interactive mode
gh pr-comments --pr 123 --no-interactive

# Pipe to jq
gh pr-comments --pr 123 --no-interactive | jq '.comments[0]'

# Flat JSON array
gh pr-comments --pr 123 --flat --no-interactive

# Markdown output
gh pr-comments --pr 123 --text

# Save to persistent file
gh pr-comments --pr 123 --save
gh pr-comments --pr 123 --save --save-dir codex-artifacts
```

### Multi-Repo Support
Run from a parent workspace with multiple git repos to browse PRs across all of them:

```bash
cd ~/projects
gh pr-comments  # Shows PRs from all repos in subdirectories
```

### Additional Options
- `--strip-html` - Remove HTML tags from comment bodies
- `--no-colour` / `--no-color` - Disable ANSI colors (respects `NO_COLOR` env var)
- `--save-dir <path>` - Override save directory (default: `.pr-comments/`)

**zsh auto-correct:** If your shell prompts to correct `pr-comments` to `.pr-comments`, leave it as-is or add `alias gh='nocorrect gh'` (or disable `CORRECT`) in your shell config to silence the prompt.

## Development
- `go test ./...` to run unit tests
- `go vet ./... && staticcheck ./...` for linting
- `go build -o gh-pr-comments ./cmd` to produce the extension binary

## Architecture

The project consists of:
- **`cmd/main.go`** - CLI entry point with flag parsing and orchestration
- **`internal/api.go`** - GitHub API client for fetching PRs and comments
- **`internal/normalize.go`** - Comment normalization and cleanup
- **`internal/render.go`** - JSON and Markdown output formatting
- **`internal/tui/`** - Interactive TUI components
  - `pr_selector.go` - Bubbletea PR selection interface
  - `json_explorer.go` - fx-inspired JSON tree navigator

## Contributing
Keep changes small, tested, and well-documented.

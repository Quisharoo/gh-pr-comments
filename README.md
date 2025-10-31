# gh-pr-comments

GitHub CLI extension that fetches PR comments, reviews, and events with an interactive terminal UI for exploration.

## Features
- Interactive TUI (Bubbletea) with fuzzy search and vim-style keybindings
- fx-inspired JSON explorer for nested comment structures
- Multi-repo support - detects PRs across workspace repos
- Output modes: JSON (nested/flat), Markdown, or interactive
- Bot detection and persistent Markdown snapshots

## Installation
```bash
go build -trimpath -o gh-pr-comments ./cmd
gh extension install .
```

## Usage

### Interactive Mode (Default)
```bash
gh pr-comments                    # PR selector → JSON explorer
gh pr-comments --pr 123           # Skip selector, explore PR #123
```
Press `?` in the TUI for keyboard shortcuts.

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

### Non-Interactive Mode
```bash
gh pr-comments --pr 123 > comments.json
gh pr-comments --pr 123 --flat --no-interactive
gh pr-comments --pr 123 --text    # Markdown output
gh pr-comments --pr 123 --save    # Save to .pr-comments/
```

### Options
- `--strip-html` - Remove HTML tags from comment bodies
- `--no-color` - Disable ANSI colors
- `--save-dir <path>` - Override save directory (default: `.pr-comments/`)

## Development
```bash
go test ./...
go vet ./... && staticcheck ./...
```

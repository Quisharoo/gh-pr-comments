# Copilot Instructions — gh-pr-comments

## Project Overview

**gh-pr-comments** is a Go-based GitHub CLI extension (~2500 LOC) that fetches and normalizes all comments, reviews, and review events from a GitHub pull request. It outputs unified JSON or Markdown for human/AI consumption.

**Key capabilities:**
- Auto-detects repository via `gh` CLI or git config
- Interactive PR picker using `fzf` (with fallback prompt)
- Aggregates data from three GitHub REST endpoints (issue comments, review comments, reviews)
- Bot detection and tagging
- Multiple output formats: nested JSON (default), flat JSON array (`--flat`), Markdown (`--text`)
- Optional persistence to `.pr-comments/PR_<number>_<branch>.json`
- ANSI colorization with OSC-8 hyperlinks for terminal output

**Tech Stack:**
- Language: Go 1.24+ (go.mod specifies 1.24.0)
- Dependencies: `go-github/v61`, `golang.org/x/oauth2`, `golang.org/x/term`
- Distribution: Built as `gh` extension binary (`gh-pr-comments`)
- Binary target size: <10 MB (currently ~9.7 MB)

## Build & Test Commands

### Prerequisites
- **Go 1.24+** (go 1.24.7 confirmed working)
- **GitHub CLI (`gh`)** with authentication (`gh auth login`)
- **Optional:** `fzf` (for interactive PR selection), `jq` (for JSON processing)

### Build Process

**ALWAYS run these commands in sequence:**

1. **Download dependencies first:**
   ```bash
   go mod download
   ```

2. **Build the binary:**
   ```bash
   go build -o gh-pr-comments ./cmd
   ```
   - **Do NOT use** `go build ./cmd/...` (fails with "output already exists and is a directory")
   - **Always specify** `-o gh-pr-comments` to avoid conflicts
   - Build time: ~5-10 seconds on typical machines
   - Output: `gh-pr-comments` executable (~9.7 MB)

3. **Test the build:**
   ```bash
   ./gh-pr-comments --help
   ```

**Clean build (if needed):**
```bash
rm -f gh-pr-comments
go clean -cache
go build -o gh-pr-comments ./cmd
```

### Running Tests

**Standard test suite:**
```bash
go test ./...
```
- Test time: ~2-3 seconds
- Tests are in `internal/*_test.go` (no tests in `cmd/`)
- All tests should pass before committing

**Specific package tests:**
```bash
go test ./internal -v
```

### Linting

**go vet (required):**
```bash
go vet ./...
```
- Must run clean before commit

**staticcheck (required by AGENTS.md):**
```bash
# Install if not present:
go install honnef.co/go/tools/cmd/staticcheck@latest

# Run linter:
~/go/bin/staticcheck ./...
```
- **Known acceptable warnings:**
  - `internal/api.go:32`: Deprecation warning about `github.NewEnterpriseClient` (affects GitHub Enterprise only)
  - `internal/utils.go`: Unused functions `displayDate` and `formatTimestamp` (legacy code)
- New code must not introduce additional warnings

**Standard check sequence before commit:**
```bash
go vet ./... && ~/go/bin/staticcheck ./... && go test ./...
```

### Runtime Testing

To manually test the tool (requires authenticated `gh`):
```bash
# Test help output
./gh-pr-comments --help

# Test in a repo with PRs (interactive mode)
./gh-pr-comments

# Test specific PR
./gh-pr-comments -p 123

# Test JSON output
./gh-pr-comments -p 123 | jq .

# Test Markdown output
./gh-pr-comments -p 123 --text

# Test save functionality
./gh-pr-comments -p 123 --save
ls .pr-comments/
```

## Project Structure

### Directory Layout
```
.
├── cmd/
│   └── main.go              # CLI entry point, flag parsing, orchestration (304 lines)
├── internal/                # All business logic (must stay in internal/)
│   ├── api.go               # GitHub REST client, pagination (250+ lines)
│   ├── normalize.go         # Comment aggregation, bot detection (180+ lines)
│   ├── render.go            # JSON/Markdown output formatting (150+ lines)
│   ├── utils.go             # Repo detection, file I/O (350+ lines)
│   ├── ansi.go              # Terminal styling primitives (57 lines)
│   ├── colorize.go          # JSON syntax highlighting (216 lines)
│   ├── *_test.go            # Unit tests (5 test files)
├── go.mod                   # Go 1.24.0, dependencies
├── go.sum                   # Dependency checksums
├── extension.yml            # gh extension metadata
├── README.md                # User documentation
├── AGENTS.md                # Coding guardrails (see below)
├── PRD.md                   # Product requirements
└── .gitignore               # Excludes: bin/, dist/, *.test, *.exe, coverage.out, vendor/, gh-pr-comments, .pr-comments/
```

### Key Architecture Patterns

**Separation of concerns (strict):**
- `cmd/main.go`: CLI wiring only (flags, I/O, error handling)
- `internal/`: All business logic, never import from cmd/

**API interaction flow:**
1. `DetectRepositories()` → finds repo via `gh`/git/nested scan
2. `NewGitHubClient()` → creates authenticated client from `GH_TOKEN`
3. `Fetcher.ListPullRequestSummaries()` or `GetPullRequestSummary()` → fetch PR metadata
4. `SelectPullRequestWithOptions()` → interactive picker (uses `fzf` if available, fallback prompt otherwise)
5. `Fetcher.FetchComments()` → aggregates from 3 endpoints with pagination
6. `BuildOutput()` → normalizes, groups by author, sorts by timestamp
7. `MarshalJSON()` or `RenderMarkdown()` → formats output
8. `SaveOutput()` → optional persistence to `.pr-comments/`

**Critical data contracts:**
- `Output` struct: `{ PR: PullRequestMetadata, Comments: []AuthorComments }`
- Never change JSON field names/types without updating PRD.md
- `--flat` mode flattens `[]AuthorComments` into single `[]Comment` array

### Configuration Files

- **go.mod**: Dependency versions (do not upgrade without testing)
- **extension.yml**: gh extension manifest (name, version, bin path)
- **.gitignore**: Build artifacts, `.pr-comments/`, coverage files

### No CI/CD Currently

- No `.github/workflows/` directory exists
- No Makefile (AGENTS.md mentions it but it's not present)
- Validation is manual: `go vet`, `staticcheck`, `go test`

## Coding Guidelines (from AGENTS.md)

**Architecture rules:**
- Keep all logic in `internal/`, `cmd/` only wires flags and I/O
- No hidden concurrency (prefer sequential pagination)
- Always check `resp.NextPage` for completeness
- Return explicit errors, never panic
- Derive `.pr-comments/` from repo root, never hardcode paths

**Quality requirements:**
- Follow `gofmt`, `go vet`, `staticcheck` standards
- All tests must pass: `go test ./...`
- Binary size must stay <10 MB
- No third-party HTML parsers (use minimal regex)
- UTF-8 text I/O only

**Bot detection:**
- Regex: `(?i)(copilot|compliance|security|dependabot|.*\[bot\])`
- See `botRegex` in `internal/utils.go`

**Security:**
- Never log `GH_TOKEN` or comment bodies
- Respect rate limits (handle 403 gracefully)
- No secrets in stdout

## Common Pitfalls & Workarounds

### Build Issues

**Problem:** `go build ./cmd/...` fails with "output already exists"
**Solution:** Use `go build -o gh-pr-comments ./cmd`

**Problem:** Binary committed to git
**Solution:** Already in `.gitignore` as `gh-pr-comments`, but ensure clean with `git rm gh-pr-comments` if needed

### Test Issues

**Problem:** Tests fail on first run
**Solution:** Run `go mod download` first, ensures all dependencies are cached

### Runtime Issues

**Problem:** "GH_TOKEN not set"
**Solution:** User must run `gh auth login` or set `GH_TOKEN`/`GITHUB_TOKEN` env var

**Problem:** "no repositories found"
**Solution:** Must run inside or alongside a git repository with remote configured

**Problem:** fzf not found
**Solution:** Tool falls back to numbered prompt automatically (see `SelectPullRequestWithOptions` in `utils.go`)

## Code Search Shortcuts

Instead of grepping, use these file locations:

- **Main entry point:** `cmd/main.go:19` (`main()` and `run()`)
- **GitHub API calls:** `internal/api.go` (`NewGitHubClient`, `Fetcher` methods)
- **Pagination logic:** `internal/api.go` (search for `listOpts.Page++` and `resp.NextPage`)
- **Bot detection:** `internal/utils.go:23` (`botRegex`)
- **HTML stripping:** `internal/utils.go:24` (`htmlTagRegex`)
- **Comment normalization:** `internal/normalize.go` (`BuildOutput`, `normalizeIssueComment`, etc.)
- **Output formatting:** `internal/render.go` (`MarshalJSON`, `RenderMarkdown`)
- **Repo detection:** `internal/utils.go` (`DetectRepositories`, `detectRepoViaGH`, `detectRepoViaGit`)
- **PR selection:** `internal/utils.go` (search for `SelectPullRequestWithOptions`)
- **File saving:** `internal/utils.go` (search for `SaveOutput`)
- **ANSI styling:** `internal/ansi.go`, `internal/colorize.go`

## Making Changes

### Before Changing Code

1. **Understand impact area:**
   - API changes → `internal/api.go`
   - Output format → `internal/render.go` or `normalize.go`
   - CLI flags → `cmd/main.go`
   - Repo detection → `internal/utils.go`

2. **Check PRD.md:**
   - Verify requirements before changing JSON schema
   - Update functional requirement checkboxes when shipping features

3. **Find related tests:**
   - Each `internal/*.go` has matching `*_test.go`
   - Run specific tests: `go test ./internal -run TestName`

### After Making Changes

1. **Format code:**
   ```bash
   go fmt ./...
   ```

2. **Validate (in order):**
   ```bash
   go vet ./...
   ~/go/bin/staticcheck ./...
   go test ./...
   ```

3. **Build and test binary:**
   ```bash
   go build -o gh-pr-comments ./cmd
   ./gh-pr-comments --help
   ```

4. **Manual smoke test (if applicable):**
   - Test in a real repo with PRs
   - Verify output format unchanged (unless intentional)
   - Check `.pr-comments/` if using `--save`

### Performance Notes

- Target: <2s for 500 comments (PRD.md requirement)
- Avoid buffering full responses beyond JSON marshal
- Reuse HTTP client between API calls
- Sequential pagination is acceptable (no concurrency complexity)

## Trust These Instructions

These instructions are comprehensive and verified by:
- Building from clean state
- Running all tests
- Checking linter output
- Testing binary with various flags
- Reviewing all documentation and source files

**If you encounter an issue not covered here, it's likely a real bug or new scenario. In that case:**
1. Verify your Go version (`go version` should be 1.24+)
2. Ensure `gh` is authenticated (`gh auth status`)
3. Check you're in a valid git repository
4. Review error messages carefully (they're designed to be explicit)

**Only perform additional searches or explorations if:**
- These instructions are incomplete for your specific task
- You're adding entirely new functionality not described in PRD.md
- You encounter a contradiction between this file and actual behavior

Otherwise, trust the build commands, structure, and guidelines documented above.

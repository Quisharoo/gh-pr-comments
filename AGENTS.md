# AGENTS.md — gh-pr-comments

## Purpose
Go-based GitHub CLI extension that retrieves and normalizes all comments on a pull request for human or AI review.

---

## Tech Stack
- **Language:** Go 1.24+
- **Authentication:** `GH_TOKEN` / `GITHUB_TOKEN` environment variables, with fallback to `gh auth token`
- **GitHub API:** `google/go-github/v61` with `golang.org/x/oauth2`
- **Terminal:** `golang.org/x/term` for TTY detection and color support
- **Optional runtime dependencies:**
  - `fzf` for interactive PR selection (graceful fallback to numbered prompt)
  - `jq` for JSON post-processing
- **Distribution:** `gh` extension binary (`gh-pr-comments`)

---

## Architecture

### Directory Structure
```
.
├── cmd/
│   └── main.go              # CLI entry point, flag parsing, I/O wiring
├── internal/
│   ├── api.go               # GitHub REST client, PR/comment fetching
│   ├── normalize.go         # Comment aggregation, grouping, sorting
│   ├── render.go            # JSON/Markdown output formatting
│   ├── colorize.go          # ANSI color application for terminal output
│   ├── ansi.go              # ANSI color code definitions
│   ├── utils.go             # Repository detection, file I/O, helpers
│   ├── *_test.go            # Unit tests for each module
│   └── repo_detect_test.go  # Repository detection integration tests
├── extension.yml            # gh extension metadata
├── go.mod                   # Go module definition
├── go.sum                   # Dependency checksums
├── PRD.md                   # Product requirements document
├── AGENTS.md                # This file: coding guidelines & architecture
└── README.md                # User-facing documentation
```

### Module Responsibilities

#### `cmd/main.go`
- Parse command-line flags (`-p`, `--flat`, `--text`, `--save`, `--strip-html`, `--no-color`)
- Detect repositories via `internal.DetectRepositories`
- Handle multi-repository disambiguation
- Wire stdin/stdout/stderr to internal functions
- Exit cleanly with error codes

#### `internal/api.go`
- `NewGitHubClient`: Create authenticated GitHub REST client (GitHub.com or Enterprise)
- `Fetcher`: Bundle GitHub operations
- `FetchComments`: Paginate through issue comments, review comments, and reviews
- `ListPullRequestSummaries`: List open PRs for repository
- `GetPullRequestSummary`: Retrieve PR metadata

#### `internal/normalize.go`
- `BuildOutput`: Aggregate and group comments by author
- `NormalizationOptions`: Control HTML stripping and other processing
- Group comments: issue comments, review comments, review events
- Sort comments chronologically
- Tag bot accounts via regex matching

#### `internal/render.go`
- `MarshalJSON`: Produce nested or flat JSON output
- `RenderMarkdown`: Generate human-readable review summary
- Escape Markdown special characters
- Format timestamps consistently

#### `internal/colorize.go`
- `ColouriseJSONComments`: Apply ANSI color codes to JSON output
- Terminal detection and `NO_COLOR` environment variable support
- Syntax highlighting for better terminal readability

#### `internal/utils.go`
- `DetectRepositories`: Discover repositories via `GH_REPO`, `gh`, or git remotes
- `FindRepoRoot`: Locate git repository root directory
- `SaveOutput`: Write JSON to `.pr-comments/` directory
- `SelectPullRequestWithOptions`: Interactive or fallback PR selection
- Bot detection regex and HTML stripping utilities

---

## Coding Standards

### General Principles
- **Separation of concerns:** All business logic lives in `internal/`; `cmd/` only handles I/O and CLI orchestration
- **Explicit error handling:** Return errors explicitly; never panic or silently fail
- **No hidden concurrency:** Sequential operations preferred; explicit goroutines only when necessary
- **Context propagation:** Use `context.Context` throughout; respect timeouts (default 60s)
- **Completeness over speed:** Always paginate to completion; check `resp.NextPage`

### Code Organization
- One file per functional area (API, normalization, rendering, utilities)
- Corresponding test file (`*_test.go`) for each module
- Exported types and functions use clear, descriptive names
- Internal helpers remain unexported (lowercase)

### Data Contracts
- **`Output` struct:** Primary contract for PR + comments
  ```go
  type Output struct {
      PR       PullRequestMetadata `json:"pr"`
      Comments []AuthorComments    `json:"comments"`
  }
  ```
- **Schema stability:** Changes to JSON output require PRD update
- **Backward compatibility:** Maintain existing field names and structure

### Dependencies
- Prefer standard library over external packages
- No CLI frameworks (only `flag` package)
- No HTML parsers (use minimal regex for tag stripping)
- Pin major versions in `go.mod`

### File I/O
- Never hardcode paths; derive `.pr-comments/` from repository root
- Use `os.MkdirAll` with `0755` for directory creation
- Write files atomically where possible
- UTF-8 encoding only

### Error Messages
- User-facing errors must be actionable (e.g., "run `gh auth login`")
- Include context in wrapped errors: `fmt.Errorf("fetch comments: %w", err)`
- Distinguish authentication, network, and API errors

---

## Testing & Quality

### Test Coverage
- Unit tests for all exported functions in `internal/`
- Table-driven tests for normalization and rendering logic
- Mock GitHub API responses with fixtures
- Test error paths and edge cases (empty PRs, no comments, pagination)

### Testing Commands
```bash
go test ./...                    # Run all tests
go test -v ./internal            # Verbose test output
go test -race ./...              # Race condition detection
go test -cover ./...             # Coverage report
```

### Code Quality
```bash
gofmt -s -w .                    # Format code
go vet ./...                     # Static analysis
staticcheck ./...                # Advanced linting (if available)
golangci-lint run                # Comprehensive linting (if configured)
```

### Validation Checklist
- [ ] `go test ./...` passes
- [ ] `go vet ./...` produces no warnings
- [ ] `gofmt -d .` shows no diff
- [ ] Binary builds successfully: `go build -o gh-pr-comments ./cmd`
- [ ] Manual smoke test completed (see Contribution Rules)

---

## Performance

### Targets
- PR with 500 comments aggregates in < 2 seconds
- Binary size < 10 MB
- Memory footprint minimal (avoid buffering beyond JSON marshal)

### Optimization Guidelines
- Reuse `github.Client` across API calls
- Batch API requests where possible
- Avoid loading full response bodies into memory
- Use streaming JSON encoding for large outputs (future consideration)

---

## Security & Privacy

### Secrets Management
- Never log tokens, API keys, or comment content
- Avoid printing sensitive data to stdout/stderr
- Use environment variables for credentials only

### Rate Limiting
- Respect GitHub rate limits
- Handle `403` responses gracefully
- Provide clear error messages on rate limit exhaustion

### Data Handling
- Strip or redact secrets if detected in comment bodies (future consideration)
- No telemetry or external data transmission
- All operations local or direct to GitHub API

---

## Contribution Workflow

### Before Starting
1. Review `PRD.md` for current feature status and requirements
2. Check existing issues/PRs for related work
3. Ensure development environment is Go 1.24+ compatible

### Development Process
1. Create feature branch from `main`
2. Make surgical, minimal changes
3. Add/update tests for modified code
4. Run quality checks (test, vet, fmt)
5. Update `PRD.md` checkboxes only when functionality fully ships
6. Commit with clear, descriptive messages

### Manual Testing
Each change should include either:
- **Explicit manual test steps:** Terminal commands demonstrating the feature
- **"Not applicable" note:** If change is internal refactor or documentation-only

Example:
```bash
# Test interactive PR selection with color output
gh pr-comments

# Test flat JSON output for a specific PR
gh pr-comments -p 42 --flat

# Test Markdown rendering with HTML stripped
gh pr-comments -p 42 --text

# Test saving to disk
gh pr-comments -p 42 --save
ls .pr-comments/
```

### Documentation
- Update `README.md` for new user-facing flags or features
- Update `AGENTS.md` (this file) for architectural changes
- Keep `PRD.md` synchronized with implementation status

### Code Review Standards
- Small, focused PRs (< 400 lines preferred)
- Clear PR description explaining motivation and approach
- Self-review checklist completed
- No introduction of non-standard frameworks or heavy dependencies

---

## Extending the Codebase

### Adding New Flags
1. Define flag in `cmd/main.go` with `fs.*Var()`
2. Pass value to appropriate `internal/` function
3. Document in `README.md` usage section
4. Add test case covering the flag behavior

### Adding New Output Formats
1. Add rendering function in `internal/render.go`
2. Export format via new flag (e.g., `--format=yaml`)
3. Update `Output` contract if schema changes
4. Add golden snapshot tests for format stability

### Adding New Comment Sources
1. Add fetch function in `internal/api.go`
2. Integrate into `FetchComments` aggregation logic
3. Update `internal/normalize.go` to handle new comment type
4. Add tests with mock API responses
5. Update `PRD.md` functional requirements

### Adding New Filtering/Sorting
1. Add filter/sort options to `NormalizationOptions` in `internal/normalize.go`
2. Implement logic in `BuildOutput` or helper functions
3. Expose via CLI flag in `cmd/main.go`
4. Add table-driven tests for filter/sort combinations

---

## Operational Guidelines

### Debugging
- Enable verbose output with custom `DEBUG` env var (future consideration)
- Use `--save` to persist output for inspection
- Check `gh auth status` for authentication issues
- Test with `GH_HOST` for GitHub Enterprise scenarios

### Common Issues
| Issue | Diagnosis | Solution |
|-------|-----------|----------|
| "GH_TOKEN not set" | Missing authentication | Run `gh auth login` |
| "no repositories found" | Not in git directory | `cd` into repository |
| "pull request #N not found" | Wrong repo or invalid PR | Omit `-p` flag to list PRs interactively |
| Rate limit exceeded | Too many API calls | Wait or use authenticated token with higher limits |
| Colors not appearing | Terminal not detected or `NO_COLOR` set | Check `isTerminalWriter` logic; unset `NO_COLOR` |

### Performance Profiling
```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./internal
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=. ./internal
go tool pprof mem.prof
```

---

## Future Considerations

### Potential Features (Not Currently Implemented)
- GraphQL API support for richer comment threading
- Unresolved thread detection and filtering
- Inline diff context inclusion
- CI/deployment status aggregation
- Real-time streaming mode
- Custom comment filtering DSL
- Export formats: YAML, CSV, HTML
- Webhook receiver for automated aggregation

### Scalability Notes
- Current architecture supports adding new comment types without refactoring
- Rendering pipeline is modular; new formats can be added independently
- Repository detection is extensible for monorepo scenarios
- API client can be swapped for GraphQL without changing normalization layer

---

## Flexibility Clause
These guidelines ensure clarity, determinism, safety, and maintainability.  
Deviations are permitted only when they:
- Simplify code without affecting correctness
- Improve performance without sacrificing reliability
- Enhance security or error handling
- Are discussed and documented in PR comments

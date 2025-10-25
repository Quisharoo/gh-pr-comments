# AGENTS.md — gh-pr-comments

## Purpose
Go-based GitHub CLI extension that retrieves and normalizes all comments on a pull request for human or AI review.

---

## Tech Stack
- **Language:** Go (modern stable version)
- **Authentication:** GitHub token via environment variables or `gh` CLI
- **GitHub API:** Official Go client library with OAuth2 support
- **Terminal:** Standard library TTY detection and color support
- **Optional runtime dependencies:**
  - `fzf` for interactive selection (graceful fallback to numbered prompts)
  - `jq` for JSON post-processing
- **Distribution:** `gh` extension binary

---

## Architecture

### Directory Structure
```
.
├── cmd/                     # CLI entry point and command-line interface
├── internal/                # Core business logic modules
│   ├── API layer           # GitHub client and data fetching
│   ├── Normalization       # Comment aggregation and processing
│   ├── Rendering           # Output formatting (JSON, Markdown)
│   ├── Colorization        # Terminal color support
│   └── Utilities           # Repository detection, file I/O, helpers
├── extension.yml            # gh extension metadata
├── go.mod/go.sum            # Go module definition
├── PRD.md                   # Product requirements document
├── AGENTS.md                # This file: coding guidelines & architecture
└── README.md                # User-facing documentation
```

### Architectural Layers

#### CLI Layer (`cmd/`)
- Command-line flag parsing and validation
- Repository detection and disambiguation
- I/O orchestration (stdin/stdout/stderr)
- Error code handling and graceful exits

#### API Layer (`internal/`)
- Authenticated GitHub REST client creation (GitHub.com and Enterprise)
- PR and comment data fetching with full pagination
- Rate limit handling and error recovery

#### Normalization Layer (`internal/`)
- Comment aggregation from multiple sources (issue comments, review comments, review events)
- Author-based grouping and chronological sorting
- Bot detection and tagging
- HTML stripping and content processing

#### Rendering Layer (`internal/`)
- JSON output (nested and flat structures)
- Markdown formatting for human readability
- Character escaping and timestamp normalization

#### Colorization Layer (`internal/`)
- ANSI color code application for terminal output
- TTY detection and `NO_COLOR` environment variable support
- Syntax highlighting for enhanced readability

#### Utilities Layer (`internal/`)
- Repository discovery (environment variables, `gh` CLI, git remotes)
- Git repository root detection
- File persistence to standard output directories
- Interactive and fallback PR selection modes

---

## Coding Standards

### General Principles
- **Separation of concerns:** All business logic lives in `internal/`; `cmd/` only handles I/O and CLI orchestration
- **Explicit error handling:** Return errors explicitly; never panic or silently fail
- **No hidden concurrency:** Sequential operations preferred; explicit goroutines only when necessary
- **Context propagation:** Use `context.Context` throughout; respect timeouts (default 60s)
- **Completeness over speed:** Always paginate to completion; check `resp.NextPage`

### Code Organization
- One file per functional area (clear separation of concerns)
- Corresponding test files for each module
- Exported types and functions use clear, descriptive names
- Internal helpers remain unexported

### Data Contracts
- **Primary output structure:** PR metadata + grouped author comments
- **Schema stability:** Changes to JSON output require PRD update
- **Backward compatibility:** Maintain existing field names and structure

### Dependencies
- Prefer standard library over external packages
- Use standard library flag parsing (no CLI frameworks)
- Minimal external dependencies for non-core functionality
- Pin major versions to ensure stability

### File I/O
- Never hardcode paths; derive from repository root
- Use standard directory creation with appropriate permissions
- Write files atomically where possible
- UTF-8 encoding only

### Error Messages
- User-facing errors must be actionable with clear remediation steps
- Include context in wrapped errors for debugging
- Distinguish authentication, network, and API errors

---

## Testing & Quality

### Test Coverage
- Unit tests for all exported functions in `internal/`
- Table-driven tests for normalization and rendering logic
- Mock GitHub API responses with fixtures
- Test error paths and edge cases (empty PRs, no comments, pagination)

### Testing Commands
Standard Go testing tools:
- Run all tests with race detection
- Generate coverage reports
- Verbose output for debugging

### Code Quality
Standard Go toolchain:
- Code formatting with `gofmt`
- Static analysis with `go vet`
- Additional linters as configured

### Validation Checklist
- [ ] All tests pass
- [ ] Static analysis produces no warnings
- [ ] Code is properly formatted
- [ ] Binary builds successfully: `go build -o gh-pr-comments ./cmd`
- [ ] Extension installed and tested: `gh extension install . && gh pr-comments`
- [ ] Manual smoke test completed with real repository

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
5. **Build and install extension:** `go build -o gh-pr-comments ./cmd && gh extension install .`
6. Test in CLI with real workflows
7. Update `PRD.md` checkboxes only when functionality fully ships
8. Commit with clear, descriptive messages

### Manual Testing
Each change should include either:
- **Explicit manual test steps:** Terminal commands demonstrating the feature
- **"Not applicable" note:** If change is internal refactor or documentation-only

Validate key workflows such as interactive selection, output format variations, and file persistence.

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
1. Define flag in CLI layer with appropriate parser
2. Pass value to business logic layer
3. Document in user-facing documentation
4. Add test coverage for flag behavior

### Adding New Output Formats
1. Add rendering function in rendering layer
2. Export format via new flag
3. Update data contracts if schema changes
4. Add golden snapshot tests for format stability

### Adding New Comment Sources
1. Add fetch function in API layer
2. Integrate into comment aggregation pipeline
3. Update normalization layer to handle new comment type
4. Add tests with mock API responses
5. Update PRD functional requirements

### Adding New Filtering/Sorting
1. Add filter/sort options to normalization configuration
2. Implement logic in normalization layer
3. Expose via CLI flag
4. Add table-driven tests for filter/sort combinations

---

## Operational Guidelines

### Debugging
- Use save-to-disk functionality to persist output for inspection
- Verify authentication status with `gh` CLI
- Test GitHub Enterprise scenarios with host environment variable

### Common Issues
| Issue | Diagnosis | Solution |
|-------|-----------|----------|
| Authentication errors | Missing or invalid token | Configure authentication via `gh` CLI |
| Repository not found | Not in git directory | Navigate into repository directory |
| PR not found | Wrong repo or invalid number | Use interactive selection to list available PRs |
| Rate limit exceeded | Too many API calls | Wait for limit reset or use authenticated token |
| Missing colors | Terminal not detected | Check TTY detection or `NO_COLOR` environment variable |

### Performance Profiling
Use standard Go profiling tools for CPU and memory analysis:
- Generate profiles with test benchmarks
- Analyze with `pprof` tooling

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

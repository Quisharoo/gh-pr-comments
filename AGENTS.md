# AGENTS.md — gh-pr-comments

## Purpose
Go-based GitHub CLI extension that retrieves and normalises all comments on a pull request for human or AI review.

---

## Tech Context
- Language: **Go 1.22+**
- Auth: via `GH_TOKEN` automatically managed by `gh`
- External deps: `go-github`, `oauth2`
- Optional runtime deps: `fzf` (for PR picker), `jq` (for manual piping)
- Distribution: built as `gh` extension binary (`gh-pr-comments`)

---

## Directory Structure
.
├── cmd/
│   └── main.go           # CLI entry
├── internal/
│   ├── api.go            # GitHub REST calls
│   ├── normalize.go      # Merge, tag, sort logic
│   ├── render.go         # JSON/text output
│   └── utils.go          # Helpers, error handling
├── go.mod
├── Makefile
└── README.md

---

## Coding Guardrails
- Keep all logic in `internal/`; `cmd/` only wires flags and I/O.
- No hidden concurrency. Prefer simple sequential pagination.
- Use standard `context.Context` and explicit timeouts.
- Always check `resp.NextPage`; completeness over speed.
- Return explicit errors; never panic or silently skip items.
- One `Output` struct as contract `{ PR, Comments }`.
- Never hardcode paths; derive `.pr-comments/` from repo root.
- Strip HTML with minimal regex, not third-party HTML parsers.
- All text I/O UTF-8 only.
- Follow `gofmt`, `go vet`, `staticcheck`.
- `make lint` and `make test` must pass before commit.

---

## Testing & Quality
- Add CLI smoke tests using `os/exec`.
- Mock GitHub API with local fixtures for deterministic runs.
- Validate schema stability with golden JSON snapshots.
- CI: `go test ./...`, `go vet ./...`, `staticcheck ./...`.

---

## Performance
- Single PR aggregation < 2 s for 500 comments typical.
- Minimal memory overhead; avoid full response buffering beyond JSON marshal.
- Reuse client between API calls.

---

## Security & Hygiene
- No logging of tokens or comment bodies.
- Respect rate limits; handle 403 gracefully.
- Strip or redact secrets if ever printed to stdout.

---

## Contribution Rules
- Prefer small, isolated PRs.
- Keep output changes verifiable: each iteration should end with a concise summary and explicit manual test steps (terminal commands) or a clear "not applicable" note.
- Review `PRD.md` after each change; flip the delivery checklist and functional requirement checkboxes from `[ ]` to `[x]` only when functionality ships.
- Do not introduce non-std CLI frameworks.
- Keep binary size < 10 MB.
- Document new flags in README.

---

## Flexibility Clause
These rules ensure clarity, determinism, and safety.  
Deviations allowed only if they simplify code without affecting data correctness or security.

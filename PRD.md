# Product Requirements Document — gh-pr-comments

## Product Summary
`gh-pr-comments` is a lightweight Go-based GitHub CLI extension that retrieves and aggregates all comments and reviews from a pull request.  
It outputs a single, normalised JSON stream for human reading or downstream AI processing (e.g. Codex CLI).  
Goal: eliminate manual copy-paste of feedback and create a reproducible, automation-ready PR review export.

---

## Problem Statement
Engineers frequently receive fragmented feedback across multiple GitHub comment types:
- Issue-level conversation comments
- Inline code review comments
- Review summary events (approve / changes requested)
- Automated bot feedback (Copilot, compliance, Dependabot)

Collecting these requires navigating multiple views or API endpoints.  
This friction blocks automated analysis and slows feedback handling.

---

## Target User
- Engineers reviewing or improving PRs using AI tooling.
- Engineering managers consolidating feedback across multiple contributors.
- Bots or scripts consuming structured PR data.

---

## Goals & Success Criteria
**Primary goal:**  
A single CLI command outputs every accessible PR comment in a unified, structured JSON.

**Success criteria:**
- Works in any authenticated repo with `gh`.
- Completes in <2 s for typical PRs.
- Produces valid, parseable JSON with consistent schema.
- Handles bot detection and tagging automatically.
- Optionally writes JSON to disk for reuse.

---

## Out of Scope (MVP)
- GraphQL or “unresolved thread” logic.
- Inline diff context or code snippets.
- CI/deploy status aggregation beyond minimal PR metadata.
- Real-time or streaming mode.

---

## Core Use Cases
1. **Interactive retrieval**
   - `gh pr-comments`
   - User selects PR via `fzf` (falls back to numbered prompt if `fzf` is unavailable).
   - Output printed as JSON to stdout.
2. **Targeted retrieval**
   - `gh pr-comments -p 42`
   - Directly fetch comments for given PR.
3. **Human-readable review**
   - `gh pr-comments --text`
   - Emits plain Markdown with HTML stripped automatically.
4. **Programmatic consumption**
   - `gh pr-comments --flat | codex evaluate-pr --stdin`
   - Provides a single JSON array of comment objects.
5. **Save for later**
   - `gh pr-comments --save` → `.pr-comments/pr-<number>-<slug>.md`

---

## Functional Requirements
| ID | Requirement | Priority | Status |
|----|--------------|----------|---------|
| FR1 | Auto-detect repo via `gh` | Must | [x] |
| FR2 | Support interactive PR picker via `fzf`, with numbered prompt fallback | Must | [x] |
| FR3 | Aggregate from all three REST endpoints | Must | [x] |
| FR4 | Tag bots via regex `copilot|compliance|security|dependabot|.*\[bot\]` | Must | [x] |
| FR5 | Output JSON (default), Markdown text (`--text` strips HTML), single-array JSON (`--flat`) | Must | [x] |
| FR6 | Optional `--save` flag for local persistence | Should | [x] |
| FR7 | Support `--strip-html` cleaning | Should | [x] |
| FR8 | Handle pagination for completeness | Must | [x] |
| FR9 | Exit clearly on missing auth or no PRs | Must | [x] |
| FR10 | Performance <2 s for 500 comments | Should | [ ] |

---

## Non-Functional Requirements
- **Reliability:** deterministic output; repeatable results for same PR.
- **Security:** never log tokens or comment bodies.
- **Portability:** works on macOS, Linux, Windows (via `gh`).
- **Maintainability:** ≤1000 LOC, modular `internal/` packages.
- **Observability:** human-readable error messages only.

---

## Output Contract (JSON)
```json
{
  "pr": {
    "repo": "owner/name",
    "number": 42,
    "title": "Sync iTerm2 prefs",
    "state": "open",
    "author": "Quisharoo",
    "updated_at": "2025-10-24T06:12:41Z",
    "head_ref": "iterm-sync"
  },
  "comments": [
    {
      "type": "issue|review_comment|review_event",
      "id": 501,
      "author": "harryQA",
      "is_bot": false,
      "created_at": "2025-10-22T12:02:44Z",
      "path": "prefs/iterm2/com.googlecode.iterm2.plist",
      "line": 18,
      "state": "CHANGES_REQUESTED",
      "body_text": "Should this file really be versioned?",
      "permalink": "https://github.com/...#discussion_r501"
    }
  ]
}

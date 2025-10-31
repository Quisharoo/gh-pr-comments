# Spinner and Back Navigation Implementation

## Summary

Added two new user-requested features to the unified TUI flow:
1. **Loading spinner** - Shows a minimal animated spinner during long operations (infrastructure added, not yet used)
2. **Back navigation** - Allows users to go back from JSON comments view to PR list using `q`

## Changes Made

### File: internal/tui/unified_flow.go

#### 1. New Imports
```go
import (
    "github.com/charmbracelet/bubbles/spinner"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)
```

#### 2. Added Fields to UnifiedFlowModel
```go
type UnifiedFlowModel struct {
    state         FlowState
    // ... existing fields ...
    spinner       spinner.Model  // NEW: Spinner for loading state
    loadingMsg    string          // NEW: Optional custom loading message
    allowBack     bool            // NEW: Whether back navigation is allowed
}
```

#### 3. Spinner Initialization

**In NewUnifiedFlowModel()** (when started with PR list):
```go
s := spinner.New()
s.Spinner = spinner.Dot
s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

return UnifiedFlowModel{
    state:      StateSelectingPR,
    prSelector: NewPRSelectorModel(prs),
    spinner:    s,
    allowBack:  true, // Allow back navigation when started with PR list
}
```

**In NewUnifiedFlowWithJSON()** (when started directly with JSON):
```go
s := spinner.New()
s.Spinner = spinner.Dot
s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

return UnifiedFlowModel{
    state:        StateExploringJSON,
    jsonExplorer: explorer,
    jsonData:     jsonData,
    skipPRSelect: true,
    spinner:      s,
    allowBack:    false, // No back navigation when started directly with JSON
}, nil
```

#### 4. Init() Method - Added StateLoading Case
```go
func (m UnifiedFlowModel) Init() tea.Cmd {
    switch m.state {
    case StateSelectingPR:
        return m.prSelector.Init()
    case StateLoading:
        return m.spinner.Tick  // Start spinner animation
    case StateExploringJSON:
        return m.jsonExplorer.Init()
    default:
        return nil
    }
}
```

#### 5. Update() Method - Two Key Changes

**Added StateLoading case:**
```go
case StateLoading:
    // Update spinner
    var cmd tea.Cmd
    m.spinner, cmd = m.spinner.Update(msg)
    return m, cmd
```

**Modified StateExploringJSON case for back navigation:**
```go
case StateExploringJSON:
    // Handle back navigation before passing to JSON explorer
    if msg, ok := msg.(tea.KeyMsg); ok {
        key := msg.String()
        if m.allowBack && key == "q" {
            // Go back to PR selector instead of quitting
            m.state = StateSelectingPR
            m.jsonExplorer = JSONExplorerModel{} // Reset explorer
            // Reset PR selector's quitting state so it doesn't immediately quit
            m.prSelector.quitting = false
            m.prSelector.choice = nil
            return m, nil
        }
    }

    // Update JSON explorer
    updated, cmd := m.jsonExplorer.Update(msg)
    m.jsonExplorer = updated.(JSONExplorerModel)

    // Check if user quit (only happens when back navigation not allowed or ctrl+c)
    if m.jsonExplorer.quitting {
        m.state = StateQuitting
        return m, tea.Quit
    }

    return m, cmd
```

#### 6. View() Method - Added StateLoading Case
```go
func (m UnifiedFlowModel) View() string {
    switch m.state {
    case StateSelectingPR:
        return m.prSelector.View()
    case StateLoading:
        if m.loadingMsg != "" {
            return fmt.Sprintf("\n  %s %s\n", m.spinner.View(), m.loadingMsg)
        }
        return fmt.Sprintf("\n  %s Loading...\n", m.spinner.View())
    case StateExploringJSON:
        return m.jsonExplorer.View()
    case StateQuitting:
        return ""
    default:
        return ""
    }
}
```

## Feature Details

### 1. Loading Spinner

**Design:**
- Uses `bubbles/spinner` package (already a dependency via bubbletea)
- Dot style spinner for minimal, non-intrusive indication
- Pink/magenta color (`lipgloss.Color("205")`) to match project aesthetic
- Supports optional custom loading message via `loadingMsg` field

**Usage:**
To show a loading spinner, transition to `StateLoading`:
```go
m.state = StateLoading
m.loadingMsg = "Fetching PR comments..." // Optional
return m, m.spinner.Tick
```

**Visual output:**
```
  ⠋ Loading...
```
or with custom message:
```
  ⠋ Fetching PR comments...
```

### 2. Back Navigation

**Design:**
- Controlled by `allowBack` field
- When `allowBack` is true: `q` in JSON view returns to PR list (ctrl+c still force quits)
- When `allowBack` is false: `q` quits the application (original behavior)

**Behavior:**
- **Started with PR list** (`NewUnifiedFlowModel`): `allowBack = true`
  - User can navigate: PR list → JSON view → (q) → PR list
  - Supports exploring multiple PRs without restarting the app
  - ctrl+c still force quits at any time

- **Started with JSON directly** (`NewUnifiedFlowWithJSON`): `allowBack = false`
  - No PR list to return to
  - `q` quits as before

**Key intercept:**
Back navigation intercepts the `q` key *before* passing it to the JSON explorer, preventing the explorer from setting its `quitting` flag. Instead, the unified flow transitions back to `StateSelectingPR`. The ctrl+c key is NOT intercepted, allowing force quit at any time.

**State reset:**
When going back, both the JSON explorer and PR selector states are reset:
- `m.jsonExplorer = JSONExplorerModel{}` - Clears JSON tree to free memory
- `m.prSelector.quitting = false` - Resets quit flag so selector doesn't immediately exit
- `m.prSelector.choice = nil` - Clears previous selection

## Usage Scenarios

### Scenario 1: Normal PR browsing flow with back navigation
```
1. User runs: gh pr-comments
2. State: StateSelectingPR → Shows PR list
3. User selects PR #42
4. State: StateExploringJSON → Shows PR #42 comments (allowBack=true)
5. User presses 'q'
6. State: StateSelectingPR → Back to PR list
7. User selects PR #99
8. State: StateExploringJSON → Shows PR #99 comments (allowBack=true)
9. User presses 'q'
10. State: StateSelectingPR → Back to PR list
11. User presses 'q' in PR list
12. State: StateQuitting → Exit
```

### Scenario 2: Direct JSON view (no back navigation)
```
1. User runs: gh pr-comments --pr 42
2. State: StateExploringJSON → Shows PR #42 comments (allowBack=false)
3. User presses 'q'
4. State: StateQuitting → Exit (can't go back, no PR list)
```

### Scenario 3: Loading state (future use)
```
1. User runs: gh pr-comments
2. State: StateLoading → Shows spinner "Loading PRs..."
3. PRs fetched
4. State: StateSelectingPR → Shows PR list
5. User selects PR #42
6. State: StateLoading → Shows spinner "Fetching comments..." (if needed)
7. Comments ready
8. State: StateExploringJSON → Shows comments
```

## Testing

**Build:** ✅ Passes
```
go build ./...
```

**Unit tests:** ✅ All pass
```
go test ./...
ok  	github.com/Quish-Labs/gh-pr-comments/cmd	0.308s
ok  	github.com/Quish-Labs/gh-pr-comments/internal	(cached)
ok  	github.com/Quish-Labs/gh-pr-comments/internal/tui	(cached)
```

## Implementation Notes

1. **Spinner style choice:** Used `spinner.Dot` instead of `spinner.Line` or other styles because:
   - Minimal visual noise
   - Works well in all terminal sizes
   - Consistent with modern CLI tool aesthetics

2. **Color choice:** Pink/magenta (`205`) chosen to:
   - Match the project's existing color palette
   - Provide good visibility without being jarring
   - Stand out from JSON content colors

3. **Back navigation intercept:** Implemented *before* JSON explorer Update() to:
   - Prevent explorer from processing quit keys when back navigation is allowed
   - Maintain clean separation of concerns
   - Allow ctrl+c to still force quit

4. **Explorer reset:** When going back, we reset the JSON explorer to ensure:
   - No memory leaks from large JSON trees
   - Clean state for next PR view
   - No visual artifacts from previous view

## Future Enhancements

1. **Loading state integration:** Currently StateLoading is defined but not used. Future work could:
   - Show spinner during PR list fetching
   - Show spinner during comment prefetching
   - Add custom messages for different loading operations

2. **Loading indicators in PR selector:** Could show per-PR loading indicators as comments are prefetched in parallel

3. **Transition animations:** Could add smooth transitions between states (fade in/out)

4. **Back navigation breadcrumbs:** Could show "Press q to go back" hint in footer when back navigation is available

## Related Files

- [internal/tui/unified_flow.go](internal/tui/unified_flow.go) - Main implementation
- [internal/tui/json_explorer.go](internal/tui/json_explorer.go) - JSON explorer (quit handling modified indirectly)
- [internal/tui/pr_selector.go](internal/tui/pr_selector.go) - PR selector (unchanged, works with back navigation)

## Dependencies

- `github.com/charmbracelet/bubbles/spinner` - Already in go.mod via bubbletea
- `github.com/charmbracelet/lipgloss` - Already in use throughout project

No new dependencies added.

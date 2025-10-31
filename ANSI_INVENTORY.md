# ANSI Code Inventory

## Summary
- **Total ANSI constants**: 11 (in `internal/ansi.go`)
- **Files using ANSI**: 5 files
- **Total usage count**: 37 occurrences

## ANSI Constants (internal/ansi.go)

| Constant | ANSI Code | Usage | Proposed lipgloss Equivalent |
|----------|-----------|-------|------------------------------|
| `ansiReset` | `\u001b[0m` | Reset all styles | `lipgloss.NewStyle()` (implicit) |
| `ansiDim` | `\u001b[2m` | Dim/faint text | `.Faint(true)` |
| `ansiFaint` | `\u001b[90m` | Faint (gray) | `.Foreground(lipgloss.Color("8"))` |
| `ansiBrightCyan` | `\u001b[96m` | Bright cyan | `.Foreground(lipgloss.Color("14"))` |
| `ansiYellow` | `\u001b[33m` | Yellow | `.Foreground(lipgloss.Color("3"))` |
| `ansiMagenta` | `\u001b[35m` | Magenta | `.Foreground(lipgloss.Color("5"))` |
| `ansiBlue` | `\u001b[94m` | Bright blue | `.Foreground(lipgloss.Color("12"))` |
| `ansiGreen` | `\u001b[92m` | Bright green | `.Foreground(lipgloss.Color("10"))` |
| `ansiUnderline` | `\u001b[4m` | Underline | `.Underline(true)` |
| `oscHyperlinkPrefix` | `\u001b]8;;` | OSC hyperlink start | (Keep for now - lipgloss may not support) |
| `oscHyperlinkClosure` | `\u001b]8;;\u0007` | OSC hyperlink end | (Keep for now) |

## Helper Functions (internal/ansi.go)

1. **`applyStyle(enabled, code, text)`** - Applies single ANSI code
   - Used 8 times across colorize.go
   - Lipgloss equivalent: `style.Render(text)` with conditional

2. **`applyStyles(enabled, text, ...codes)`** - Applies multiple ANSI codes
   - Used 2 times (for hyperlinks with underline+blue)
   - Lipgloss equivalent: Chain style methods

3. **`applyHyperlink(enabled, url, text)`** - Wraps text in OSC-8 hyperlink
   - Used 2 times (permalink, PR URL)
   - Keep as-is (lipgloss doesn't directly support OSC-8, but can be added later)

## Usage by File

### internal/colorize.go (PRIMARY TARGET)
- **Line 31**: `applyStyle(true, ansiYellow, value)` - PR number
- **Line 35**: `applyStyle(true, ansiBrightCyan, value)` - Repo name
- **Line 39**: `applyStyle(true, ansiGreen, value)` - Comment type
- **Line 43**: `applyStyle(true, ansiBrightCyan, value)` - Author
- **Line 47**: `applyStyle(true, ansiFaint, value)` - Created timestamp
- **Line 51**: `applyStyle(true, ansiFaint, value)` - Updated timestamp
- **Line 55**: `applyStyle(true, ansiMagenta, value)` - Head ref (branch)
- **Line 59**: `applyStyle(true, ansiMagenta, value)` - Base ref (branch)
- **Line 67-68**: `applyStyles(true, value, ansiUnderline, ansiBlue)` + hyperlink - Permalink
- **Line 72-73**: `applyStyles(true, value, ansiUnderline, ansiBlue)` + hyperlink - PR URL
- **Line 77**: `applyStyle(true, ansiDim, key)` - JSON keys
- **Line 173**: `ansiYellow` - Inline code highlighting
- **Line 176**: `ansiReset` - Reset after inline code

### internal/utils.go
- Import only, no direct usage (colorize functions used indirectly)

### internal/colorize_test.go
- Uses ANSI constants for assertion checking
- Uses helper functions to construct expected output
- Tests will need updating after lipgloss migration

### internal/utils_test.go
- Import only for test setup

## Refactoring Plan

### Phase 1: Define lipgloss Styles
Create style definitions in colorize.go:
```go
var (
    dimStyle         = lipgloss.NewStyle().Faint(true)
    faintStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
    brightCyanStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
    yellowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
    magentaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
    blueStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
    greenStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
    linkStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Underline(true)
)
```

### Phase 2: Replace applyStyle() Calls
Replace each `applyStyle(enabled, code, text)` with:
```go
func renderStyle(enabled bool, style lipgloss.Style, text string) string {
    if !enabled || text == "" {
        return text
    }
    return style.Render(text)
}
```

### Phase 3: Inline Code Highlighting
Replace direct ANSI code concatenation in `highlightInlineCode()` with:
```go
styled := yellowStyle.Render(codeContent)
```

### Phase 4: Keep Hyperlinks As-Is (for now)
- `applyHyperlink()` will remain unchanged
- OSC-8 hyperlinks are terminal-specific and lipgloss doesn't abstract them
- Can be migrated later if needed

### Phase 5: Update Tests
- Golden file test will catch any visual differences
- Update test assertions to use lipgloss styles if needed

## Benefits of Lipgloss Migration

1. **Type safety**: Compile-time style validation vs raw strings
2. **Composability**: Chain styles easily (`.Faint(true).Foreground(...)`)
3. **Automatic ANSI handling**: Lipgloss handles stripping, width calculation, etc.
4. **Consistency**: Uses same library as TUI (bubbletea/bubbles)
5. **Maintainability**: Declarative style definitions vs ANSI constants

## Risks

1. **Output changes**: Lipgloss may generate slightly different ANSI sequences
   - Mitigation: Golden file test will catch this
2. **Performance**: Lipgloss might be slower than raw ANSI
   - Mitigation: Negligible for JSON colorization (not a hot path)
3. **Hyperlinks**: OSC-8 hyperlinks need special handling
   - Mitigation: Keep `applyHyperlink()` unchanged for now

## Estimated Impact

- **Lines removed**: ~40-50 (ansi.go constants + helper functions)
- **Lines modified**: ~20-30 (colorize.go function calls)
- **Lines added**: ~10-15 (lipgloss style definitions)
- **Net reduction**: ~15-25 lines
- **Complexity reduction**: Significant (declarative styles vs imperative ANSI)

package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PullRequestSummary carries PR metadata needed for display.
// This is aliased from the main package to avoid circular dependencies.
type PullRequestSummary struct {
	Number       int
	Title        string
	Author       string
	State        string
	Created      time.Time
	Updated      time.Time
	HeadRef      string
	BaseRef      string
	RepoName     string
	RepoOwner    string
	URL          string
	LocalPath    string
	CommentsJSON []byte // Prefetched JSON comments data
}

// PRSelectorModel is the Bubbletea model for interactive PR selection.
type PRSelectorModel struct {
	list     list.Model
	choice   *PullRequestSummary
	quitting bool
}

// prItem wraps a PullRequestSummary for use with the bubbles list component.
type prItem struct {
	pr PullRequestSummary
}

func (i prItem) FilterValue() string {
	return fmt.Sprintf("%s #%d %s", i.pr.RepoName, i.pr.Number, i.pr.Title)
}

func (i prItem) Title() string {
	return fmt.Sprintf("%s#%d: %s", i.pr.RepoName, i.pr.Number, i.pr.Title)
}

func (i prItem) Description() string {
	arrow := "\u2192"
	updated := formatTimestamp(i.pr.Updated)
	return fmt.Sprintf("[%s%s%s] %s by @%s",
		i.pr.HeadRef,
		arrow,
		i.pr.BaseRef,
		updated,
		i.pr.Author,
	)
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.UTC().Truncate(time.Minute).Format("2006-01-02 15:04Z")
}

// NewPRSelectorModel creates a new PR selector model.
func NewPRSelectorModel(prs []*PullRequestSummary) PRSelectorModel {
	items := make([]list.Item, len(prs))
	for i, pr := range prs {
		if pr != nil {
			items[i] = prItem{pr: *pr}
		}
	}

	// Create custom key bindings
	delegate := list.NewDefaultDelegate()

	// Customize styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("170")).
		Bold(true)

	itemStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	selectedItemStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		Foreground(lipgloss.Color("170")).
		Bold(true)

	delegate.Styles.NormalTitle = itemStyle
	delegate.Styles.SelectedTitle = selectedItemStyle
	delegate.Styles.SelectedDesc = selectedItemStyle.Copy().Foreground(lipgloss.Color("241"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select a Pull Request"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	// Add additional help keys
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select"),
			),
			key.NewBinding(
				key.WithKeys("o"),
				key.WithHelp("o", "open in browser"),
			),
		}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "select PR"),
			),
			key.NewBinding(
				key.WithKeys("o"),
				key.WithHelp("o", "open PR in browser"),
			),
		}
	}

	return PRSelectorModel{
		list: l,
	}
}

// Init implements tea.Model.
func (m PRSelectorModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m PRSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Adjust list dimensions to fit window
		h, v := lipgloss.NewStyle().GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit

		case "o":
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				if item, ok := selectedItem.(prItem); ok && item.pr.URL != "" {
					go openBrowser(item.pr.URL)
				}
			}

		case "enter":
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				if item, ok := selectedItem.(prItem); ok {
					m.choice = &item.pr
					m.quitting = true
					return m, tea.Quit
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m PRSelectorModel) View() string {
	if m.quitting && m.choice != nil {
		return ""
	}
	if m.quitting {
		return "Selection cancelled.\n"
	}
	return m.list.View()
}

// GetChoice returns the selected PR, or nil if none was selected.
func (m PRSelectorModel) GetChoice() *PullRequestSummary {
	return m.choice
}

// SelectPullRequestInteractive launches an interactive TUI for PR selection.
// Returns the selected PR or nil if cancelled.
func SelectPullRequestInteractive(prs []*PullRequestSummary) (*PullRequestSummary, error) {
	if len(prs) == 0 {
		return nil, fmt.Errorf("no pull requests available")
	}

	model := NewPRSelectorModel(prs)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running interactive selector: %w", err)
	}

	if m, ok := finalModel.(PRSelectorModel); ok {
		if m.GetChoice() != nil {
			return m.GetChoice(), nil
		}
	}

	return nil, fmt.Errorf("selection cancelled")
}

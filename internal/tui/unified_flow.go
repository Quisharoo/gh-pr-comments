package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FlowState represents the current state of the interactive flow.
type FlowState int

const (
	StateSelectingPR FlowState = iota
	StateLoading
	StateExploringJSON
	StateQuitting
)

// UnifiedFlowModel manages the entire interactive flow without screen flashing.
type UnifiedFlowModel struct {
	state         FlowState
	prSelector    PRSelectorModel
	jsonExplorer  JSONExplorerModel
	selectedPR    *PullRequestSummary
	jsonData      []byte
	err           error
	skipPRSelect  bool
	fetchComments func(*PullRequestSummary) ([]byte, error)
	width         int
	height        int
}

// NewUnifiedFlowModel creates a new unified flow starting with PR selection.
func NewUnifiedFlowModel(prs []*PullRequestSummary, fetchComments func(*PullRequestSummary) ([]byte, error)) UnifiedFlowModel {
	return UnifiedFlowModel{
		state:         StateSelectingPR,
		prSelector:    NewPRSelectorModel(prs),
		fetchComments: fetchComments,
	}
}

// NewUnifiedFlowWithJSON creates a flow that skips PR selection and goes straight to JSON.
func NewUnifiedFlowWithJSON(jsonData []byte) (UnifiedFlowModel, error) {
	explorer, err := NewJSONExplorerModel(jsonData)
	if err != nil {
		return UnifiedFlowModel{}, err
	}

	return UnifiedFlowModel{
		state:        StateExploringJSON,
		jsonExplorer: explorer,
		jsonData:     jsonData,
		skipPRSelect: true,
	}, nil
}

// Init implements tea.Model.
func (m UnifiedFlowModel) Init() tea.Cmd {
	switch m.state {
	case StateSelectingPR:
		return m.prSelector.Init()
	case StateLoading:
		// Start fetching comments
		return m.fetchCommentsCmd()
	case StateExploringJSON:
		return m.jsonExplorer.Init()
	default:
		return nil
	}
}

// fetchCommentsCmd creates a command that fetches comments asynchronously.
func (m UnifiedFlowModel) fetchCommentsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.fetchComments == nil || m.selectedPR == nil {
			return commentsFetchedMsg{err: fmt.Errorf("no fetch function or PR")}
		}

		jsonData, err := m.fetchComments(m.selectedPR)
		return commentsFetchedMsg{
			data: jsonData,
			err:  err,
		}
	}
}

// commentsFetchedMsg is sent when comments have been fetched.
type commentsFetchedMsg struct {
	data []byte
	err  error
}

// Update implements tea.Model.
func (m UnifiedFlowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle window size for all states
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
	}

	switch m.state {
	case StateSelectingPR:
		// Handle quit keys even before PR selector processes them
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.String() == "ctrl+c" {
				m.state = StateQuitting
				return m, tea.Quit
			}
		}

		// Update PR selector
		updated, cmd := m.prSelector.Update(msg)
		m.prSelector = updated.(PRSelectorModel)

		// Check if PR was selected
		if m.prSelector.quitting {
			if m.prSelector.choice != nil {
				// PR selected - transition to loading state
				m.selectedPR = m.prSelector.choice
				m.state = StateLoading
				return m, m.fetchCommentsCmd()
			}
			// Cancelled - quit
			m.state = StateQuitting
			return m, tea.Quit
		}

		return m, cmd

	case StateLoading:
		// Handle the fetched comments
		if msg, ok := msg.(commentsFetchedMsg); ok {
			if msg.err != nil {
				m.err = msg.err
				m.state = StateQuitting
				return m, tea.Quit
			}

			// Successfully fetched - transition to JSON explorer
			explorer, err := NewJSONExplorerModel(msg.data)
			if err != nil {
				m.err = err
				m.state = StateQuitting
				return m, tea.Quit
			}

			m.jsonExplorer = explorer
			m.jsonData = msg.data
			m.state = StateExploringJSON
			return m, nil
		}

		// Allow quitting during loading
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				m.state = StateQuitting
				return m, tea.Quit
			}
		}

		return m, nil

	case StateExploringJSON:
		// Update JSON explorer
		updated, cmd := m.jsonExplorer.Update(msg)
		m.jsonExplorer = updated.(JSONExplorerModel)

		// Check if user quit
		if m.jsonExplorer.quitting {
			m.state = StateQuitting
			return m, tea.Quit
		}

		return m, cmd

	case StateQuitting:
		return m, tea.Quit

	default:
		return m, nil
	}
}

// View implements tea.Model.
func (m UnifiedFlowModel) View() string {
	switch m.state {
	case StateSelectingPR:
		return m.prSelector.View()
	case StateLoading:
		return m.renderLoadingScreen()
	case StateExploringJSON:
		return m.jsonExplorer.View()
	case StateQuitting:
		return ""
	default:
		return ""
	}
}

// renderLoadingScreen shows a loading message while fetching comments.
func (m UnifiedFlowModel) renderLoadingScreen() string {
	if m.selectedPR == nil {
		return "Loading..."
	}

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Padding(1, 2)

	message := fmt.Sprintf("Fetching comments for %s/%s#%d...",
		m.selectedPR.RepoOwner,
		m.selectedPR.RepoName,
		m.selectedPR.Number)

	return style.Render(message)
}

// SetJSONData transitions to the JSON explorer state with the given data.
func (m *UnifiedFlowModel) SetJSONData(jsonData []byte) error {
	explorer, err := NewJSONExplorerModel(jsonData)
	if err != nil {
		m.err = err
		m.state = StateQuitting
		return err
	}

	m.jsonData = jsonData
	m.jsonExplorer = explorer
	m.state = StateExploringJSON
	return nil
}

// GetSelectedPR returns the selected PR (if any).
func (m UnifiedFlowModel) GetSelectedPR() *PullRequestSummary {
	return m.selectedPR
}

// prSelectedMsg is sent when a PR is selected.
type prSelectedMsg struct {
	pr *PullRequestSummary
}

// RunUnifiedFlow executes the complete interactive flow in a single TUI session.
// If jsonData is provided, it skips PR selection and goes straight to JSON explorer.
// The fetchComments callback is used to fetch comment JSON after PR selection.
func RunUnifiedFlow(prs []*PullRequestSummary, jsonData []byte, fetchComments func(*PullRequestSummary) ([]byte, error)) (*PullRequestSummary, error) {
	var model tea.Model
	var err error

	if jsonData != nil {
		// Skip PR selection, go straight to JSON explorer
		model, err = NewUnifiedFlowWithJSON(jsonData)
		if err != nil {
			return nil, err
		}
	} else {
		// Start with PR selection
		model = NewUnifiedFlowModel(prs, fetchComments)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if m, ok := finalModel.(UnifiedFlowModel); ok {
		if m.err != nil {
			return m.GetSelectedPR(), m.err
		}
		return m.GetSelectedPR(), nil
	}

	return nil, nil
}

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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
	state        FlowState
	prSelector   PRSelectorModel
	jsonExplorer JSONExplorerModel
	selectedPR   *PullRequestSummary
	jsonData     []byte
	err          error
	skipPRSelect bool
	width        int
	height       int
}

// NewUnifiedFlowModel creates a new unified flow starting with PR selection.
// PRs should have CommentsJSON prefetched.
func NewUnifiedFlowModel(prs []*PullRequestSummary) UnifiedFlowModel {
	return UnifiedFlowModel{
		state:      StateSelectingPR,
		prSelector: NewPRSelectorModel(prs),
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
	case StateExploringJSON:
		return m.jsonExplorer.Init()
	default:
		return nil
	}
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
				// PR selected - use prefetched comments
				m.selectedPR = m.prSelector.choice

				// Check if comments were prefetched
				if len(m.selectedPR.CommentsJSON) == 0 {
					m.err = fmt.Errorf("no comments data prefetched for PR #%d", m.selectedPR.Number)
					m.state = StateQuitting
					return m, tea.Quit
				}

				// Transition directly to JSON explorer
				explorer, err := NewJSONExplorerModel(m.selectedPR.CommentsJSON)
				if err != nil {
					m.err = err
					m.state = StateQuitting
					return m, tea.Quit
				}

				m.jsonExplorer = explorer
				m.jsonData = m.selectedPR.CommentsJSON
				m.state = StateExploringJSON

				cmd := m.syncJSONExplorerSize()
				return m, cmd
			}
			// Cancelled - quit
			m.state = StateQuitting
			return m, tea.Quit
		}

		return m, cmd

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
	case StateExploringJSON:
		return m.jsonExplorer.View()
	case StateQuitting:
		return ""
	default:
		return ""
	}
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
	m.syncJSONExplorerSize()
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
// PRs should have CommentsJSON prefetched when prs is provided.
func RunUnifiedFlow(prs []*PullRequestSummary, jsonData []byte) (*PullRequestSummary, error) {
	var model tea.Model
	var err error

	if jsonData != nil {
		// Skip PR selection, go straight to JSON explorer
		model, err = NewUnifiedFlowWithJSON(jsonData)
		if err != nil {
			return nil, err
		}
	} else {
		// Start with PR selection (comments should be prefetched)
		model = NewUnifiedFlowModel(prs)
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

// syncJSONExplorerSize replays the last known window size to the explorer so it
// can fill the available space immediately after the state transition.
func (m *UnifiedFlowModel) syncJSONExplorerSize() tea.Cmd {
	if m.width == 0 || m.height == 0 {
		return nil
	}

	updated, cmd := m.jsonExplorer.Update(tea.WindowSizeMsg{
		Width:  m.width,
		Height: m.height,
	})

	if explorer, ok := updated.(JSONExplorerModel); ok {
		m.jsonExplorer = explorer
	}

	return cmd
}

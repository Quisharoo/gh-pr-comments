package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	ghprcomments "github.com/Quish-Labs/gh-pr-comments/internal"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v61/github"
	"golang.org/x/sync/errgroup"
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
	state           FlowState
	prSelector      PRSelectorModel
	jsonExplorer    JSONExplorerModel
	selectedPR      *PullRequestSummary
	jsonData        []byte
	err             error
	skipPRSelect    bool
	width           int
	height          int
	spinner         spinner.Model
	loadingMsg      string
	allowBack       bool // Whether back navigation from JSON is allowed
	altScreenActive bool // Whether we've entered the terminal alt screen

	// Prefetching state
	prefetchCtx    context.Context
	prefetchCancel context.CancelFunc
	prefetchConfig *PrefetchConfig // Stored config for starting prefetch in Init()
}

// prefetchCompleteMsg is sent when all PRs have been prefetched.
type prefetchCompleteMsg struct {
	prs  []*PullRequestSummary
	errs []error
}

// prefetchErrorMsg is sent when prefetching fails fatally.
type prefetchErrorMsg struct {
	err error
}

// NewUnifiedFlowModel creates a new unified flow starting with PR selection.
// PRs should have CommentsJSON prefetched.
func NewUnifiedFlowModel(prs []*PullRequestSummary) UnifiedFlowModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return UnifiedFlowModel{
		state:      StateSelectingPR,
		prSelector: NewPRSelectorModel(prs),
		spinner:    s,
		allowBack:  true, // Allow back navigation when started with PR list
	}
}

// PrefetchConfig holds the configuration for prefetching PR comments.
type PrefetchConfig struct {
	Ctx                context.Context
	PRs                []*ghprcomments.PullRequestSummary
	Fetcher            *ghprcomments.Fetcher
	Repositories       []ghprcomments.Repository
	RepositoriesLoader func(context.Context) ([]ghprcomments.Repository, error)
	StripHTML          bool
	Flat               bool
}

// NewUnifiedFlowWithPrefetch creates a new unified flow that prefetches PR comments.
// Starts in StateLoading with a spinner, prefetches comments in background,
// then transitions to PR selection.
func NewUnifiedFlowWithPrefetch(config PrefetchConfig) UnifiedFlowModel {
	prefetchCtx, prefetchCancel := context.WithCancel(config.Ctx)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Make a copy of config with the cancellable context
	configCopy := config
	configCopy.Ctx = prefetchCtx

	// Determine loading message
	loadingMsg := "Loading..."
	switch {
	case len(config.PRs) > 0:
		loadingMsg = fmt.Sprintf("Loading comments for %d PRs...", len(config.PRs))
	case len(config.Repositories) == 0 && config.RepositoriesLoader != nil:
		loadingMsg = "Discovering repositories..."
	}

	m := UnifiedFlowModel{
		state:          StateLoading,
		spinner:        s,
		loadingMsg:     loadingMsg,
		allowBack:      true,
		prefetchCtx:    prefetchCtx,
		prefetchCancel: prefetchCancel,
		prefetchConfig: &configCopy,
	}

	return m
}

// startPrefetchCmd returns a command that starts prefetching PR comments.
func startPrefetchCmd(config PrefetchConfig) tea.Cmd {
	return func() tea.Msg {
		// If PRs not provided, fetch them first
		prs := config.PRs
		if prs == nil {
			repos := config.Repositories
			if len(repos) == 0 && config.RepositoriesLoader != nil {
				loaded, err := config.RepositoriesLoader(config.Ctx)
				if err != nil {
					return prefetchErrorMsg{err: fmt.Errorf("detect repositories: %w", err)}
				}
				repos = loaded
			}

			if len(repos) == 0 {
				return prefetchErrorMsg{err: fmt.Errorf("no repositories found")}
			}

			all := make([]*ghprcomments.PullRequestSummary, 0)
			var fatalErr error
			for _, repo := range repos {
				repoPRs, err := config.Fetcher.ListPullRequestSummaries(config.Ctx, repo.Owner, repo.Name)
				if err != nil {
					if errors.Is(err, ghprcomments.ErrNoPullRequests) {
						// Ignore repos with no PRs
						continue
					}
					// Check if it's a 404 - skip repositories that don't exist or are inaccessible
					var ghErr *github.ErrorResponse
					if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound {
						// Skip inaccessible repositories (they may be private or deleted)
						continue
					}
					// Other errors are fatal - but only return if all repos failed
					if fatalErr == nil {
						fatalErr = fmt.Errorf("list PRs for %s/%s: %w", repo.Owner, repo.Name, err)
					}
					continue
				}
				// Clear fatal error if we successfully got PRs from at least one repo
				fatalErr = nil
				for _, pr := range repoPRs {
					if pr.RepoOwner == "" {
						pr.RepoOwner = repo.Owner
					}
					if pr.RepoName == "" {
						pr.RepoName = repo.Name
					}
					pr.LocalPath = repo.Path
				}
				all = append(all, repoPRs...)
			}
			// If we have a fatal error and no PRs, return it
			if fatalErr != nil && len(all) == 0 {
				return prefetchErrorMsg{err: fatalErr}
			}
			prs = all
		}

		if len(prs) == 0 {
			return prefetchErrorMsg{err: fmt.Errorf("no pull requests found")}
		}

		type prefetchResult struct {
			pr    *PullRequestSummary
			warn  error
			index int
		}

		results := make([]prefetchResult, len(prs))
		for i := range results {
			results[i].index = i
		}

		workerLimit := 4
		if len(prs) < workerLimit {
			workerLimit = len(prs)
		}
		if workerLimit == 0 {
			workerLimit = 1
		}

		sem := make(chan struct{}, workerLimit)
		prefetchGroup, groupCtx := errgroup.WithContext(config.Ctx)

		for i, pr := range prs {
			i, pr := i, pr
			prefetchGroup.Go(func() error {
				select {
				case sem <- struct{}{}:
				case <-groupCtx.Done():
					return groupCtx.Err()
				}
				defer func() { <-sem }()

				owner := strings.TrimSpace(pr.RepoOwner)
				repo := strings.TrimSpace(pr.RepoName)

				payloads, err := config.Fetcher.FetchComments(groupCtx, owner, repo, pr.Number)
				if err != nil {
					results[i].warn = fmt.Errorf("failed to fetch comments for %s/%s#%d: %w", owner, repo, pr.Number, err)
					return nil
				}

				normOpts := ghprcomments.NormalizationOptions{
					StripHTML: config.StripHTML,
				}

				output := ghprcomments.BuildOutput(pr, payloads, normOpts)
				jsonData, err := ghprcomments.MarshalJSON(output, config.Flat)
				if err != nil {
					results[i].warn = fmt.Errorf("failed to marshal JSON for %s/%s#%d: %w", owner, repo, pr.Number, err)
					return nil
				}

				results[i].pr = &PullRequestSummary{
					Number:       pr.Number,
					Title:        pr.Title,
					Author:       pr.Author,
					State:        pr.State,
					Created:      pr.Created,
					Updated:      pr.Updated,
					HeadRef:      pr.HeadRef,
					BaseRef:      pr.BaseRef,
					RepoName:     pr.RepoName,
					RepoOwner:    pr.RepoOwner,
					URL:          pr.URL,
					LocalPath:    pr.LocalPath,
					CommentsJSON: jsonData,
				}
				return nil
			})
		}

		if err := prefetchGroup.Wait(); err != nil {
			return prefetchErrorMsg{err: err}
		}

		validPRs := make([]*PullRequestSummary, 0, len(results))
		var errs []error
		for _, res := range results {
			if res.warn != nil {
				errs = append(errs, res.warn)
				continue
			}
			if res.pr != nil {
				validPRs = append(validPRs, res.pr)
			}
		}

		return prefetchCompleteMsg{
			prs:  validPRs,
			errs: errs,
		}
	}
}

func (m UnifiedFlowModel) quitCmd() tea.Cmd {
	if m.altScreenActive {
		return tea.Batch(tea.ExitAltScreen, tea.Quit)
	}
	return tea.Quit
}

// NewUnifiedFlowWithJSON creates a flow that skips PR selection and goes straight to JSON.
func NewUnifiedFlowWithJSON(jsonData []byte) (UnifiedFlowModel, error) {
	explorer, err := NewJSONExplorerModel(jsonData)
	if err != nil {
		return UnifiedFlowModel{}, err
	}

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
}

// Init implements tea.Model.
func (m UnifiedFlowModel) Init() tea.Cmd {
	switch m.state {
	case StateSelectingPR:
		return m.prSelector.Init()
	case StateLoading:
		// Start spinner and prefetch if config is available
		if m.prefetchConfig != nil {
			return tea.Batch(m.spinner.Tick, startPrefetchCmd(*m.prefetchConfig))
		}
		return m.spinner.Tick
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
				return m, m.quitCmd()
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
					return m, m.quitCmd()
				}

				// Transition directly to JSON explorer
				explorer, err := NewJSONExplorerModel(m.selectedPR.CommentsJSON)
				if err != nil {
					m.err = err
					m.state = StateQuitting
					return m, m.quitCmd()
				}

				m.jsonExplorer = explorer
				m.jsonData = m.selectedPR.CommentsJSON
				m.state = StateExploringJSON

				cmd := m.syncJSONExplorerSize()
				return m, cmd
			}
			// Cancelled - quit
			m.state = StateQuitting
			return m, m.quitCmd()
		}

		return m, cmd

	case StateLoading:
		// Handle prefetch completion
		switch msg := msg.(type) {
		case prefetchCompleteMsg:
			if len(msg.prs) == 0 {
				m.err = fmt.Errorf("no PRs with comments available")
				m.state = StateQuitting
				return m, m.quitCmd()
			}
			// Transition to PR selector with prefetched data
			m.prSelector = NewPRSelectorModel(msg.prs)
			m.state = StateSelectingPR
			m.prefetchConfig = nil // Clear config

			// Send the window size to the PR selector so it renders properly
			if m.width > 0 && m.height > 0 {
				updated, _ := m.prSelector.Update(tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.height,
				})
				m.prSelector = updated.(PRSelectorModel)
			}

			// Switch to alt screen now that we have data to show
			m.altScreenActive = true
			return m, tea.Batch(
				tea.EnterAltScreen,
				m.prSelector.Init(),
			)

		case prefetchErrorMsg:
			m.err = msg.err
			m.state = StateQuitting
			return m, m.quitCmd()
		}

		// Update spinner
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

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
			return m, m.quitCmd()
		}

		return m, cmd

	case StateQuitting:
		return m, m.quitCmd()

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

// RunUnifiedFlowWithPrefetch executes the interactive flow with loading spinner
// while prefetching PR comments in the background.
func RunUnifiedFlowWithPrefetch(config PrefetchConfig) (*PullRequestSummary, error) {
	model := NewUnifiedFlowWithPrefetch(config)

	// Start WITHOUT alt screen so spinner shows immediately in terminal
	// We'll switch to alt screen when we transition to PR selector
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if m, ok := finalModel.(UnifiedFlowModel); ok {
		// Clean up context
		if m.prefetchCancel != nil {
			m.prefetchCancel()
		}
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

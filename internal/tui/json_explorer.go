package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// JSONExplorerModel provides an interactive JSON viewer with fx-inspired navigation.
type JSONExplorerModel struct {
	viewport     viewport.Model
	searchInput  textinput.Model
	content      []byte
	tree         *JSONNode
	flatNodes    []*JSONNode
	cursor       int
	searchMode   bool
	searchQuery  string
	filterActive bool
	width        int
	height       int
	quitting     bool
}

// JSONNode represents a node in the JSON tree structure.
type JSONNode struct {
	Key        string
	Value      interface{}
	Type       string // "object", "array", "string", "number", "bool", "null"
	Children   []*JSONNode
	Parent     *JSONNode
	Expanded   bool
	Depth      int
	Index      int  // Index in flatNodes
	LineNumber int  // Display line number
	Matches    bool // Whether this node matches current search
}

// KeyMap defines keybindings for the JSON explorer.
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	GotoTop      key.Binding
	GotoBottom   key.Binding
	Expand       key.Binding
	Collapse     key.Binding
	ExpandAll    key.Binding
	CollapseAll  key.Binding
	Search       key.Binding
	NextMatch    key.Binding
	PrevMatch    key.Binding
	ClearSearch  key.Binding
	Quit         key.Binding
	Help         key.Binding
}

// DefaultKeyMap returns the default keybindings (vim-style).
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+b"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+f"),
			key.WithHelp("pgdn", "page down"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "half page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "half page down"),
		),
		GotoTop: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g/home", "go to top"),
		),
		GotoBottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G/end", "go to bottom"),
		),
		Expand: key.NewBinding(
			key.WithKeys("right", "l", "enter"),
			key.WithHelp("→/l/enter", "toggle"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "collapse"),
		),
		ExpandAll: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "expand all"),
		),
		CollapseAll: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "collapse all"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		NextMatch: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		PrevMatch: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),
		ClearSearch: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "clear search"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
}

var keyMap = DefaultKeyMap()

// NewJSONExplorerModel creates a new JSON explorer from raw JSON bytes.
func NewJSONExplorerModel(jsonData []byte) (JSONExplorerModel, error) {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return JSONExplorerModel{}, fmt.Errorf("invalid JSON: %w", err)
	}

	tree := buildTree("", data, nil, 0)
	flatNodes := flattenTree(tree)

	// Create search input
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100

	// Start with reasonable defaults; will be updated by WindowSizeMsg
	vp := viewport.New(100, 30)

	model := JSONExplorerModel{
		viewport:    vp,
		searchInput: ti,
		content:     jsonData,
		tree:        tree,
		flatNodes:   flatNodes,
		cursor:      0,
	}

	model.viewport.SetContent(model.renderTree())

	return model, nil
}

// buildTree recursively constructs a tree from JSON data.
func buildTree(key string, value interface{}, parent *JSONNode, depth int) *JSONNode {
	node := &JSONNode{
		Key:      key,
		Value:    value,
		Parent:   parent,
		Depth:    depth,
		Expanded: depth < 2, // Auto-expand first 2 levels
	}

	switch v := value.(type) {
	case map[string]interface{}:
		node.Type = "object"
		for k, val := range v {
			child := buildTree(k, val, node, depth+1)
			node.Children = append(node.Children, child)
		}
	case []interface{}:
		node.Type = "array"
		for i, val := range v {
			child := buildTree(fmt.Sprintf("[%d]", i), val, node, depth+1)
			node.Children = append(node.Children, child)
		}
	case string:
		node.Type = "string"
	case float64, int, int64:
		node.Type = "number"
	case bool:
		node.Type = "bool"
	case nil:
		node.Type = "null"
	default:
		node.Type = "unknown"
	}

	return node
}

// flattenTree converts tree to flat list for cursor navigation.
func flattenTree(root *JSONNode) []*JSONNode {
	var result []*JSONNode
	var traverse func(*JSONNode)

	traverse = func(node *JSONNode) {
		node.Index = len(result)
		node.LineNumber = len(result) + 1
		result = append(result, node)

		if node.Expanded && len(node.Children) > 0 {
			for _, child := range node.Children {
				traverse(child)
			}
		}
	}

	traverse(root)
	return result
}

// Init implements tea.Model.
func (m JSONExplorerModel) Init() tea.Cmd {
	// Set initial content so it displays immediately
	m.viewport.SetContent(m.renderTree())
	return nil
}

// Update implements tea.Model.
func (m JSONExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Header: title line + 2 newlines = 3 lines
		headerHeight := 3
		// Footer: status line + newline = 2 lines (or 3 in search mode)
		footerHeight := 2
		if m.searchMode {
			footerHeight = 3
		}

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight
		m.viewport.SetContent(m.renderTree())
		return m, nil

	case tea.KeyMsg:
		// Search mode handling
		if m.searchMode {
			switch msg.String() {
			case "esc", "ctrl+c":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				m.searchQuery = m.searchInput.Value()
				m.filterActive = m.searchQuery != ""
				m.applySearch()
				m.viewport.SetContent(m.renderTree())
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		}

		// Normal mode handling
		switch {
		case key.Matches(msg, keyMap.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keyMap.Up):
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}

		case key.Matches(msg, keyMap.Down):
			if m.cursor < len(m.flatNodes)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}

		case key.Matches(msg, keyMap.GotoTop):
			m.cursor = 0
			m.viewport.GotoTop()

		case key.Matches(msg, keyMap.GotoBottom):
			m.cursor = len(m.flatNodes) - 1
			m.viewport.GotoBottom()

		case key.Matches(msg, keyMap.PageDown):
			m.cursor = min(m.cursor+m.viewport.Height, len(m.flatNodes)-1)
			m.ensureCursorVisible()

		case key.Matches(msg, keyMap.PageUp):
			m.cursor = max(m.cursor-m.viewport.Height, 0)
			m.ensureCursorVisible()

		case key.Matches(msg, keyMap.HalfPageDown):
			m.cursor = min(m.cursor+m.viewport.Height/2, len(m.flatNodes)-1)
			m.ensureCursorVisible()

		case key.Matches(msg, keyMap.HalfPageUp):
			m.cursor = max(m.cursor-m.viewport.Height/2, 0)
			m.ensureCursorVisible()

		case key.Matches(msg, keyMap.Expand):
			if m.cursor < len(m.flatNodes) {
				node := m.flatNodes[m.cursor]
				if len(node.Children) > 0 {
					// Toggle expand/collapse
					node.Expanded = !node.Expanded
					m.flatNodes = flattenTree(m.tree)
					m.viewport.SetContent(m.renderTree())
				}
			}

		case key.Matches(msg, keyMap.Collapse):
			if m.cursor < len(m.flatNodes) {
				node := m.flatNodes[m.cursor]
				if node.Expanded {
					node.Expanded = false
					m.flatNodes = flattenTree(m.tree)
					m.viewport.SetContent(m.renderTree())
				} else if node.Parent != nil {
					// Collapse parent
					node.Parent.Expanded = false
					m.flatNodes = flattenTree(m.tree)
					m.cursor = node.Parent.Index
					m.viewport.SetContent(m.renderTree())
					m.ensureCursorVisible()
				}
			}

		case key.Matches(msg, keyMap.ExpandAll):
			expandAll(m.tree)
			m.flatNodes = flattenTree(m.tree)
			m.viewport.SetContent(m.renderTree())

		case key.Matches(msg, keyMap.CollapseAll):
			collapseAll(m.tree)
			m.flatNodes = flattenTree(m.tree)
			m.viewport.SetContent(m.renderTree())

		case key.Matches(msg, keyMap.Search):
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, keyMap.NextMatch):
			m.findNextMatch()
			m.ensureCursorVisible()

		case key.Matches(msg, keyMap.PrevMatch):
			m.findPrevMatch()
			m.ensureCursorVisible()

		case key.Matches(msg, keyMap.ClearSearch):
			m.searchQuery = ""
			m.filterActive = false
			m.applySearch()
			m.viewport.SetContent(m.renderTree())
		}

		m.viewport.SetContent(m.renderTree())
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m JSONExplorerModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		Padding(0, 1)

	b.WriteString(titleStyle.Render("JSON Comment Explorer"))
	b.WriteString("\n\n")

	// Viewport content
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Footer
	if m.searchMode {
		b.WriteString("\n")
		b.WriteString(m.searchInput.View())
	} else {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("170"))

		status := fmt.Sprintf("%d/%d", m.cursor+1, len(m.flatNodes))
		if m.filterActive {
			matches := 0
			for _, node := range m.flatNodes {
				if node.Matches {
					matches++
				}
			}
			status += fmt.Sprintf(" | %d matches for '%s'", matches, m.searchQuery)
		}

		b.WriteString(statusStyle.Render(status))
	}

	return b.String()
}

// renderTree generates the visual tree representation.
func (m JSONExplorerModel) renderTree() string {
	var b strings.Builder

	for i, node := range m.flatNodes {
		// Skip nodes that don't match filter
		if m.filterActive && !node.Matches && !hasMatchingChild(node) {
			continue
		}

		// Indentation
		indent := strings.Repeat("  ", node.Depth)
		b.WriteString(indent)

		// Cursor indicator
		if i == m.cursor {
			b.WriteString("▶ ")
		} else {
			b.WriteString("  ")
		}

		// Expand/collapse indicator
		if len(node.Children) > 0 {
			if node.Expanded {
				b.WriteString("▼ ")
			} else {
				b.WriteString("▶ ")
			}
		} else {
			b.WriteString("  ")
		}

		// Key styling
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		if node.Matches {
			keyStyle = keyStyle.Bold(true).Foreground(lipgloss.Color("226"))
		}

		if i == m.cursor {
			keyStyle = keyStyle.Background(lipgloss.Color("237"))
		}

		// Render key
		if node.Key != "" {
			b.WriteString(keyStyle.Render(node.Key))
			b.WriteString(": ")
		}

		// Render value preview
		b.WriteString(m.renderValue(node, i == m.cursor))
		b.WriteString("\n")
	}

	return b.String()
}

// renderValue renders a node's value with appropriate styling.
func (m JSONExplorerModel) renderValue(node *JSONNode, selected bool) string {
	valueStyle := lipgloss.NewStyle()

	if selected {
		valueStyle = valueStyle.Background(lipgloss.Color("237"))
	}

	switch node.Type {
	case "object":
		count := len(node.Children)
		style := valueStyle.Foreground(lipgloss.Color("241"))
		if node.Expanded {
			return style.Render(fmt.Sprintf("{} %d keys", count))
		}
		return style.Render(fmt.Sprintf("{...} %d keys", count))

	case "array":
		count := len(node.Children)
		style := valueStyle.Foreground(lipgloss.Color("241"))
		if node.Expanded {
			return style.Render(fmt.Sprintf("[] %d items", count))
		}
		return style.Render(fmt.Sprintf("[...] %d items", count))

	case "string":
		style := valueStyle.Foreground(lipgloss.Color("142"))
		str := fmt.Sprintf("%v", node.Value)
		if len(str) > 60 {
			str = str[:57] + "..."
		}
		return style.Render(fmt.Sprintf("%q", str))

	case "number":
		style := valueStyle.Foreground(lipgloss.Color("170"))
		return style.Render(fmt.Sprintf("%v", node.Value))

	case "bool":
		style := valueStyle.Foreground(lipgloss.Color("208"))
		return style.Render(fmt.Sprintf("%v", node.Value))

	case "null":
		style := valueStyle.Foreground(lipgloss.Color("241"))
		return style.Render("null")

	default:
		return valueStyle.Render(fmt.Sprintf("%v", node.Value))
	}
}

// applySearch marks nodes that match the search query.
func (m *JSONExplorerModel) applySearch() {
	query := strings.ToLower(m.searchQuery)

	for _, node := range m.flatNodes {
		node.Matches = false
		if query == "" {
			continue
		}

		// Search in key
		if strings.Contains(strings.ToLower(node.Key), query) {
			node.Matches = true
			continue
		}

		// Search in value
		valueStr := fmt.Sprintf("%v", node.Value)
		if strings.Contains(strings.ToLower(valueStr), query) {
			node.Matches = true
		}
	}
}

// findNextMatch moves cursor to next matching node.
func (m *JSONExplorerModel) findNextMatch() {
	for i := m.cursor + 1; i < len(m.flatNodes); i++ {
		if m.flatNodes[i].Matches {
			m.cursor = i
			return
		}
	}
	// Wrap around
	for i := 0; i <= m.cursor; i++ {
		if m.flatNodes[i].Matches {
			m.cursor = i
			return
		}
	}
}

// findPrevMatch moves cursor to previous matching node.
func (m *JSONExplorerModel) findPrevMatch() {
	for i := m.cursor - 1; i >= 0; i-- {
		if m.flatNodes[i].Matches {
			m.cursor = i
			return
		}
	}
	// Wrap around
	for i := len(m.flatNodes) - 1; i >= m.cursor; i-- {
		if m.flatNodes[i].Matches {
			m.cursor = i
			return
		}
	}
}

// hasMatchingChild checks if any descendant matches the search.
func hasMatchingChild(node *JSONNode) bool {
	for _, child := range node.Children {
		if child.Matches || hasMatchingChild(child) {
			return true
		}
	}
	return false
}

// ensureCursorVisible scrolls viewport to keep cursor in view.
func (m *JSONExplorerModel) ensureCursorVisible() {
	lineHeight := 1
	cursorY := m.cursor * lineHeight

	if cursorY < m.viewport.YOffset {
		m.viewport.SetYOffset(cursorY)
	} else if cursorY >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(cursorY - m.viewport.Height + lineHeight)
	}
}

// expandAll recursively expands all nodes.
func expandAll(node *JSONNode) {
	node.Expanded = true
	for _, child := range node.Children {
		expandAll(child)
	}
}

// collapseAll recursively collapses all nodes.
func collapseAll(node *JSONNode) {
	node.Expanded = false
	for _, child := range node.Children {
		collapseAll(child)
	}
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ExploreJSON launches an interactive JSON explorer.
func ExploreJSON(jsonData []byte) error {
	model, err := NewJSONExplorerModel(jsonData)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running JSON explorer: %w", err)
	}

	return nil
}

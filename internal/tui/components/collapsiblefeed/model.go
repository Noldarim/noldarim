// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package collapsiblefeed

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the collapsible activity feed component
type Model struct {
	groups     []ActivityGroup
	focusedIdx int
	viewport   viewport.Model
	width      int
	height     int
	ready      bool

	// Current todos state (extracted from latest TodoWrite)
	currentTodos  []TodoItem
	showTodoPanel bool

	// Styling
	styles Styles
}

// New creates a new collapsible feed model
func New(width, height int) Model {
	vp := viewport.New(width, height)
	vp.SetContent("Waiting for activity...")

	return Model{
		groups:        make([]ActivityGroup, 0),
		viewport:      vp,
		width:         width,
		height:        height,
		ready:         true,
		showTodoPanel: true,
		styles:        DefaultStyles(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.focusedIdx < len(m.groups)-1 {
				m.focusedIdx++
				m.ensureFocusedVisible()
			}
		case "k", "up":
			if m.focusedIdx > 0 {
				m.focusedIdx--
				m.ensureFocusedVisible()
			}
		case "enter", " ":
			// Toggle expand/collapse on focused item
			if m.focusedIdx < len(m.groups) {
				m.groups[m.focusedIdx].Expanded = !m.groups[m.focusedIdx].Expanded
				m.refreshContent()
			}
		case "t":
			// Toggle todo panel
			m.showTodoPanel = !m.showTodoPanel
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// SetGroups replaces all activity groups
func (m Model) SetGroups(groups []ActivityGroup) Model {
	m.groups = groups

	// Extract todos from the latest TodoWrite
	for i := len(groups) - 1; i >= 0; i-- {
		if groups[i].ToolName == "TodoWrite" && len(groups[i].Input.Todos) > 0 {
			m.currentTodos = groups[i].Input.Todos
			break
		}
	}

	m.refreshContent()
	return m
}

// AddGroup adds a new activity group (usually from tool_use)
func (m *Model) AddGroup(group ActivityGroup) {
	// Update todos if it's a TodoWrite
	if group.ToolName == "TodoWrite" && len(group.Input.Todos) > 0 {
		m.currentTodos = group.Input.Todos
	}

	m.groups = append(m.groups, group)
	m.refreshContent()
}

// CompleteGroup marks a group as completed with a result
func (m *Model) CompleteGroup(id string, result ToolResult) {
	for i := range m.groups {
		if m.groups[i].ID == id {
			m.groups[i].Result = &result
			if result.Success {
				m.groups[i].State = StateCompleted
			} else {
				m.groups[i].State = StateFailed
			}
			break
		}
	}
	m.refreshContent()
}

// Groups returns the current activity groups
func (m Model) Groups() []ActivityGroup {
	return m.groups
}

// CurrentTodos returns the current todo list
func (m Model) CurrentTodos() []TodoItem {
	return m.currentTodos
}

// SetSize updates component dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height
	m.refreshContent()
}

// refreshContent renders groups into the viewport
func (m *Model) refreshContent() {
	content := m.renderGroups()
	if content == "" {
		content = "Waiting for activity..."
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

// RenderContent returns just the rendered groups without viewport wrapping.
// Use this when embedding in another component that has its own viewport.
func (m Model) RenderContent() string {
	content := m.renderGroups()
	if content == "" {
		return ""
	}
	return content
}

// ensureFocusedVisible scrolls viewport to show focused item
func (m *Model) ensureFocusedVisible() {
	// Simple approach: just refresh and go to bottom for now
	// Could be enhanced to calculate exact line position
	m.refreshContent()
}

// GetTodoPanelVisible returns whether todo panel should be shown
func (m Model) GetTodoPanelVisible() bool {
	return m.showTodoPanel && len(m.currentTodos) > 0
}

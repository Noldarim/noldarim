// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskdetails

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/components/gitdiffviewer"
	"github.com/noldarim/noldarim/internal/tui/components/hooksactivity"
	"github.com/noldarim/noldarim/internal/tui/components/scrollablecard"
	"github.com/noldarim/noldarim/internal/tui/components/tabbar"
	"github.com/noldarim/noldarim/internal/tui/components/taskinfocard"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// Tab IDs
const (
	TabTaskInfo      = "task-info"
	TabGitDiff       = "git-diff"
	TabHooksActivity = "hooks-activity"
)

// Model is the model for the task details screen.
type Model struct {
	task      *models.Task
	projectID string
	cmdChan   chan<- protocol.Command
	width     int
	height    int

	// Tab navigation
	tabBar tabbar.Model

	// Content for each tab
	cards         []scrollablecard.Model // Task info (0) and Git diff (1)
	hooksActivity hooksactivity.Model    // Hooks activity tab

	focusedCard int
	ready       bool
}

// NewModel creates a new task details model
func NewModel(task *models.Task, projectID string, cmdChan chan<- protocol.Command) Model {
	// Create tab bar
	tabs := []tabbar.Tab{
		{ID: TabTaskInfo, Label: "Task Info"},
		{ID: TabGitDiff, Label: "Git Diff"},
		{ID: TabHooksActivity, Label: "Hooks Activity"},
	}
	tb := tabbar.New(tabs)

	// Create initial cards (will be properly sized in SetSize)
	taskInfoCard := scrollablecard.New(
		"Task Information",
		taskinfocard.Render(task),
		40, // Initial width
		10, // Initial height
	)
	taskInfoCard.SetFocus(true) // Start with task info focused

	gitDiffCard := scrollablecard.New(
		"Git Diff",
		gitdiffviewer.Render(task.GitDiff, 0), // No max height, viewport handles it
		40, // Initial width
		15, // Initial height
	)

	// Create hooks activity component
	taskID := ""
	if task != nil {
		taskID = task.ID
	}
	hooks := hooksactivity.New(taskID, 40, 15)

	return Model{
		task:          task,
		projectID:     projectID,
		cmdChan:       cmdChan,
		width:         50,
		height:        10,
		tabBar:        tb,
		cards:         []scrollablecard.Model{taskInfoCard, gitDiffCard},
		hooksActivity: hooks,
		focusedCard:   0, // Task info focused by default
		ready:         false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// GetLayoutInfo returns layout information for the task details screen
func (m Model) GetLayoutInfo() layout.LayoutInfo {
	helpItems := []layout.HelpItem{
		{Key: "1/2/3", Description: "switch tab"},
		{Key: "↑/k", Description: "scroll up"},
		{Key: "↓/j", Description: "scroll down"},
		{Key: "esc", Description: "back"},
		{Key: "q", Description: "quit"},
	}

	// Build breadcrumbs with task title
	taskTitle := m.task.Title
	if len(taskTitle) > 30 {
		taskTitle = taskTitle[:27] + "..."
	}

	return layout.LayoutInfo{
		Title:       "Task Details",
		Breadcrumbs: []string{"Projects", m.projectID, "Tasks", taskTitle},
		Status:      "",
		HelpItems:   helpItems,
	}
}

// SetSize updates the model's dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Get validated content area from layout
	layoutInfo := m.GetLayoutInfo()
	dims := layout.GetContentArea(layoutInfo, width, height)

	// If dimensions are invalid, mark not ready (will show error)
	if !dims.Valid {
		m.ready = false
		return
	}

	// Update tab bar width
	m.tabBar.SetWidth(dims.Width)

	// Reserve space for tab bar (1 line)
	availableHeight := dims.Height - 2

	// Card overhead (border + padding + title + margin)
	const cardOverhead = 7

	// Content height for the active tab
	contentHeight := availableHeight - cardOverhead
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Card width (account for borders and padding)
	cardWidth := dims.Width - 10
	if cardWidth < 20 {
		cardWidth = dims.Width - 4
	}

	// For tabs 0 and 1 (task info and git diff), we show them as cards
	// For simplicity, size all cards to use full height when selected
	if len(m.cards) >= 2 {
		m.cards[0].SetSize(cardWidth, contentHeight)
		m.cards[1].SetSize(cardWidth, contentHeight)
	}

	// Size hooks activity component
	m.hooksActivity.SetSize(cardWidth, contentHeight)

	m.ready = true
}

// updateFocus updates the focus state based on active tab
func (m *Model) updateFocus() {
	activeTab := m.tabBar.GetActiveTab()

	// Unfocus all cards first
	for i := range m.cards {
		m.cards[i].SetFocus(false)
	}
	m.hooksActivity.SetFocus(false)

	// Focus the appropriate component based on active tab
	switch activeTab {
	case 0: // Task Info
		if len(m.cards) > 0 {
			m.cards[0].SetFocus(true)
		}
	case 1: // Git Diff
		if len(m.cards) > 1 {
			m.cards[1].SetFocus(true)
		}
	case 2: // Hooks Activity
		m.hooksActivity.SetFocus(true)
	}
}

// AddAIActivityRecord adds an AI activity record to the hooks activity component
func (m *Model) AddAIActivityRecord(record *models.AIActivityRecord) {
	if record != nil {
		m.hooksActivity.AddEvent(record)
		// Update badge on hooks tab with event count
		count := m.hooksActivity.GetEventCount()
		if count > 0 {
			m.tabBar.SetBadge(2, "")
		}
	}
}

// StartAIStream marks the start of AI activity streaming
func (m *Model) StartAIStream() {
	m.hooksActivity.StartStream()
}

// EndAIStream marks the end of AI activity streaming
func (m *Model) EndAIStream(finalStatus string) {
	m.hooksActivity.EndStream(finalStatus)
}

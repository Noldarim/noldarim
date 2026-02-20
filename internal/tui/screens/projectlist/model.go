// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectlist

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// ProjectItem represents a project in the list
type ProjectItem struct {
	ID             string
	Name           string
	Desc           string
	RepositoryPath string
}

// FilterValue returns the value to filter against
func (p ProjectItem) FilterValue() string {
	return p.Name
}

// Title returns the project name
func (p ProjectItem) Title() string {
	return p.Name
}

// Description returns the project description
func (p ProjectItem) Description() string {
	return p.Desc
}

// String returns a string representation of the project item
func (p ProjectItem) String() string {
	return fmt.Sprintf("%s: %s", p.Name, p.Desc)
}

// Remove InputMode - no longer needed since form is in separate screen

// Model is the model for the project list screen.
type Model struct {
	list          list.Model
	cmdChan       chan<- protocol.Command
	projects      map[string]*models.Project
	statusMessage string
	width         int
	height        int
}

// NewModel creates a new project list model
func NewModel(cmdChan chan<- protocol.Command) Model {
	// Create list with standard configuration
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 50, 10)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.Title = ""

	return Model{
		list:     l,
		cmdChan:  cmdChan,
		projects: make(map[string]*models.Project),
		width:    50,
		height:   10,
	}
}

func (m Model) Init() tea.Cmd {
	// Send command to load projects
	go func() {
		m.cmdChan <- protocol.LoadProjectsCommand{}
	}()
	return nil
}

// GetLayoutInfo returns layout information for the project list screen
func (m Model) GetLayoutInfo() layout.LayoutInfo {
	status := fmt.Sprintf("Total: %d projects", len(m.projects))
	if m.statusMessage != "" {
		status = m.statusMessage
	}

	helpItems := []layout.HelpItem{
		{Key: "enter", Description: "select"},
		{Key: "n", Description: "new"},
		{Key: "s", Description: "settings"},
		{Key: "q", Description: "quit"},
	}

	return layout.LayoutInfo{
		Title:       "Projects",
		Breadcrumbs: []string{"Projects"},
		Status:      status,
		HelpItems:   helpItems,
	}
}

// clearInputs is no longer needed - removed form functionality

// SetSize updates the model's dimensions and list size
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate content area and update list size
	layoutInfo := m.GetLayoutInfo()
	dims := layout.GetContentArea(layoutInfo, width, height)
	m.list.SetWidth(dims.Width)
	m.list.SetHeight(dims.Height)
}

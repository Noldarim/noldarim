// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package settings

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// Model is the model for the settings screen.
type Model struct {
	selectedIndex int
	options       []string
	width         int
	height        int
}

// NewModel creates a new settings model
func NewModel() Model {
	return Model{
		selectedIndex: 0,
		options: []string{
			"Theme: Default",
			"Auto-refresh: Enabled",
			"Notifications: Enabled",
			"Debug Mode: Disabled",
		},
		width:  50,
		height: 10,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// GetLayoutInfo returns layout information for the settings screen
func (m Model) GetLayoutInfo() layout.LayoutInfo {
	helpItems := []layout.HelpItem{
		{Key: "↑/k", Description: "up"},
		{Key: "↓/j", Description: "down"},
		{Key: "enter", Description: "toggle"},
		{Key: "esc", Description: "back"},
		{Key: "q", Description: "quit"},
	}

	return layout.LayoutInfo{
		Title:       "Settings",
		Breadcrumbs: []string{"Settings"},
		Status:      "",
		HelpItems:   helpItems,
	}
}

// SetSize updates the model's dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

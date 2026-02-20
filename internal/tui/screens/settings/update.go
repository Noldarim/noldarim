// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package settings

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/tui/messages"
)

// Update handles messages and updates the model state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "down", "j":
			if m.selectedIndex < len(m.options)-1 {
				m.selectedIndex++
			}
		case "enter":
			// Toggle the selected option
			switch m.selectedIndex {
			case 1: // Auto-refresh
				if m.options[1] == "Auto-refresh: Enabled" {
					m.options[1] = "Auto-refresh: Disabled"
				} else {
					m.options[1] = "Auto-refresh: Enabled"
				}
			case 2: // Notifications
				if m.options[2] == "Notifications: Enabled" {
					m.options[2] = "Notifications: Disabled"
				} else {
					m.options[2] = "Notifications: Enabled"
				}
			case 3: // Debug Mode
				if m.options[3] == "Debug Mode: Disabled" {
					m.options[3] = "Debug Mode: Enabled"
				} else {
					m.options[3] = "Debug Mode: Disabled"
				}
			}
		case "esc", "backspace":
			// Go back to previous screen
			return m, func() tea.Msg {
				return messages.GoBackMsg{}
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	return m, nil
}

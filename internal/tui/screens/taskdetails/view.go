// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskdetails

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// View renders the task details screen
func (m Model) View() string {
	layoutInfo := m.GetLayoutInfo()

	// If not ready or space invalid, let RenderLayout handle it
	if !m.ready {
		// Check if it's a space issue
		dims := layout.ValidateSpace(m.width, m.height)
		if !dims.Valid {
			// Return layout with error (it will render the error)
			return layout.RenderLayout("", layoutInfo, m.width, m.height)
		}
		// Otherwise just loading
		return layout.RenderLayout("Loading...", layoutInfo, m.width, m.height)
	}

	content := m.renderTaskDetails()
	return layout.RenderLayout(content, layoutInfo, m.width, m.height)
}

// renderTaskDetails renders the task details content with tabs
func (m Model) renderTaskDetails() string {
	// Render tab bar
	tabBar := m.tabBar.View()

	// Render content for active tab
	var tabContent string
	activeTab := m.tabBar.GetActiveTab()

	switch activeTab {
	case 0: // Task Info
		if len(m.cards) > 0 {
			tabContent = m.cards[0].View()
		}
	case 1: // Git Diff
		if len(m.cards) > 1 {
			tabContent = m.cards[1].View()
		}
	case 2: // Hooks Activity
		tabContent = m.hooksActivity.View()
	}

	// Combine tab bar and content vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		"", // Small gap
		tabContent,
	)
}

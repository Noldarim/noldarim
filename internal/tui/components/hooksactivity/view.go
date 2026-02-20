// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package hooksactivity

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/noldarim/noldarim/internal/tui/components/card"
)

// View renders the hooks activity component
func (m Model) View() string {
	// Render summary panel at top
	summaryContent := RenderSummary(m.summary, m.streaming, m.width)

	// Render scrollable event log
	logContent := m.logViewport.View()

	// Combine summary and log
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		summaryContent,
		logContent,
	)

	// Wrap in card with focus styling
	style := card.DefaultStyle()
	if m.focused {
		style.BorderColor = lipgloss.Color("86")  // Cyan when focused
		style.BorderStyle = lipgloss.ThickBorder()
	} else {
		style.BorderColor = lipgloss.Color("240")
		style.BorderStyle = lipgloss.RoundedBorder()
	}

	return card.Render("Hooks Activity", content, style)
}

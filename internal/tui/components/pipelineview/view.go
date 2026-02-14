// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package pipelineview

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("239"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

// View renders the pipeline view with scrollable content and fixed status bar
func (m Model) View() string {
	// If done, return empty - summary will be printed to stdout after TUI exits
	if m.showSummary {
		return ""
	}

	// Build status bar: progress │ timer │ tokens
	var statusParts []string
	statusParts = append(statusParts, m.progress.View())
	statusParts = append(statusParts, m.timer.View())

	if m.tokenData.InputTokens > 0 || m.tokenData.OutputTokens > 0 {
		statusParts = append(statusParts, m.tokens.View())
	}

	statusBar := statusBarStyle.Render(strings.Join(statusParts, " │ "))

	// Separator line
	separator := separatorStyle.Render(strings.Repeat("─", m.width))

	// Combine: viewport + separator + status bar
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		separator,
		statusBar,
	)
}

// ViewStatusBar renders only the status bar (for external use)
func (m Model) ViewStatusBar() string {
	var parts []string
	parts = append(parts, m.progress.View())
	parts = append(parts, m.timer.View())

	if m.tokenData.InputTokens > 0 || m.tokenData.OutputTokens > 0 {
		parts = append(parts, m.tokens.View())
	}

	return strings.Join(parts, " │ ")
}

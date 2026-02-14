// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectcreation

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// View renders the project creation screen
func (m Model) View() string {
	layoutInfo := m.GetLayoutInfo()

	var content string

	switch m.stage {
	case DirSelection:
		content = m.renderDirSelection()
	case FormInput:
		content = m.renderFormInput()
	}

	return layout.RenderLayout(content, layoutInfo, m.width, m.height)
}

// renderDirSelection renders the directory selection stage
func (m Model) renderDirSelection() string {
	style := lipgloss.NewStyle().
		Padding(1, 2)

	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Margin(1, 0).
		Render("Navigate to your project directory and press SPACE to select it")

	currentPath := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Margin(0, 0, 1, 0).
		Render(fmt.Sprintf("Current: %s", m.filePicker.CurrentDirectory))

	filePickerView := m.filePicker.View()

	combined := lipgloss.JoinVertical(
		lipgloss.Left,
		instructions,
		currentPath,
		filePickerView,
	)

	return style.Render(combined)
}

// renderFormInput renders the form input stage
func (m Model) renderFormInput() string {
	style := lipgloss.NewStyle().
		Padding(1, 2)

	selectedPathLabel := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("241")).
		Render("Selected Directory:")

	selectedPath := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Margin(0, 0, 2, 0).
		Render(m.selectedPath)

	formView := m.form.View()

	combined := lipgloss.JoinVertical(
		lipgloss.Left,
		selectedPathLabel,
		selectedPath,
		formView,
	)

	return style.Render(combined)
}

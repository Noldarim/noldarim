// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskinfocard

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// Render creates a card displaying task information
func Render(task *models.Task) string {
	// Task Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))
	title := titleStyle.Render(task.Title)

	// Task Description (limit length for preview)
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
	desc := task.Description
	// If description is too long, show preview
	const maxDescLength = 500
	if len(desc) > maxDescLength {
		desc = desc[:maxDescLength] + "...\n\n(scroll for more)"
	}
	description := descStyle.Render(desc)

	// Status with icon and color
	statusIcon, statusColor, statusText := getStatusDisplay(task.Status)
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColor))
	status := statusStyle.Render(fmt.Sprintf("Status: %s %s", statusIcon, statusText))

	// Timestamps
	timestampStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	createdAt := timestampStyle.Render("Created: " + task.CreatedAt.Format(time.RFC3339))
	updatedAt := timestampStyle.Render("Updated: " + task.LastUpdatedAt.Format(time.RFC3339))

	// Task ID
	idStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	taskID := idStyle.Render("ID: " + task.ID)

	// Combine all elements
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		description,
		"",
		status,
		createdAt,
		updatedAt,
		"",
		taskID,
	)

	return content
}

func getStatusDisplay(status models.TaskStatus) (icon, color, text string) {
	switch status {
	case models.TaskStatusPending:
		return "○", "241", "Pending"
	case models.TaskStatusInProgress:
		return "◐", "226", "In Progress"
	case models.TaskStatusCompleted:
		return "●", "82", "Completed"
	case models.TaskStatusFailed:
		return "✗", "196", "Failed"
	default:
		return "?", "241", "Unknown"
	}
}

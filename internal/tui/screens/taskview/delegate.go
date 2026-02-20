// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskview

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/components/taskstatus"
)

// TaskDelegate is a custom delegate for rendering tasks with status components
type TaskDelegate struct {
	list.DefaultDelegate
	taskStatuses map[string]taskstatus.Model
}

// NewTaskDelegate creates a new task delegate
func NewTaskDelegate() TaskDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = true
	return TaskDelegate{
		DefaultDelegate: d,
		taskStatuses:    make(map[string]taskstatus.Model),
	}
}

// SetTaskStatuses updates the task status components map
func (d *TaskDelegate) SetTaskStatuses(taskStatuses map[string]taskstatus.Model) {
	d.taskStatuses = taskStatuses
}

// Render renders a single task item
func (d TaskDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	taskItem, ok := item.(TaskItem)
	if !ok {
		d.DefaultDelegate.Render(w, m, index, item)
		return
	}

	// Get the base rendering
	str := d.renderTaskWithStatus(m, index, taskItem)

	// Check if item is selected
	if index == m.Index() {
		str = d.Styles.SelectedTitle.Render(str)
	} else {
		str = d.Styles.NormalTitle.Render(str)
	}

	fmt.Fprint(w, str)
}

// renderTaskWithStatus renders a task with its status component on the right
func (d TaskDelegate) renderTaskWithStatus(m list.Model, _ int, task TaskItem) string {
	// Get the task title
	title := task.TaskTitle

	// Get the task status component
	var statusComponent string
	if statusModel, exists := d.taskStatuses[task.ID]; exists {
		// Set width to compact for right-side display
		statusModel = statusModel.SetWidth(15) // Compact width
		statusComponent = statusModel.View()
	} else {
		// Fallback to simple status if component not found
		statusComponent = "○"
	}

	// Calculate available width
	width := m.Width() - d.Styles.NormalTitle.GetHorizontalFrameSize()
	statusWidth := lipgloss.Width(statusComponent)

	// Available width for title
	availableWidth := width - statusWidth - 2 // -2 for spaces

	// Truncate title if needed
	if lipgloss.Width(title) > availableWidth {
		title = truncateString(title, availableWidth-3) + "..."
	}

	// Add padding to align status to the right
	padding := availableWidth - lipgloss.Width(title)
	if padding > 0 {
		title += strings.Repeat(" ", padding)
	}

	// Combine title and status
	return title + " " + statusComponent
}

// truncateString truncates a string to the specified width
func truncateString(s string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= width {
		return s
	}

	return string(runes[:width])
}

// Height returns the height of the rendered item
func (d TaskDelegate) Height() int {
	return 1
}

// Spacing returns the spacing between items
func (d TaskDelegate) Spacing() int {
	return 0
}

// Update handles messages for the delegate
func (d TaskDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

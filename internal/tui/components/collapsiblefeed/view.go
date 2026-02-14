// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package collapsiblefeed

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles holds all the styling for the component
type Styles struct {
	Pending   lipgloss.Style
	Success   lipgloss.Style
	Failed    lipgloss.Style
	ToolName  lipgloss.Style
	FilePath  lipgloss.Style
	Dim       lipgloss.Style
	Expanded  lipgloss.Style
	TodoPanel lipgloss.Style
	Progress  lipgloss.Style
}

// DefaultStyles returns the default color scheme
func DefaultStyles() Styles {
	return Styles{
		Pending:   lipgloss.NewStyle().Foreground(lipgloss.Color("75")),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.Color("35")),
		Failed:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		ToolName:  lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true),
		FilePath:  lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		Dim:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Expanded:  lipgloss.NewStyle().Foreground(lipgloss.Color("252")).PaddingLeft(4),
		TodoPanel: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("241")).Padding(0, 1),
		Progress:  lipgloss.NewStyle().Foreground(lipgloss.Color("75")),
	}
}

// View renders the complete component
func (m Model) View() string {
	activityView := m.viewport.View()

	if !m.showTodoPanel || len(m.currentTodos) == 0 {
		return activityView
	}

	// Side-by-side layout: activity feed | todo panel
	todoPanel := m.renderTodoPanel()

	// Calculate widths
	todoPanelWidth := 35
	activityWidth := m.width - todoPanelWidth - 3 // 3 for spacing

	// Constrain activity view width
	activityStyle := lipgloss.NewStyle().Width(activityWidth)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		activityStyle.Render(activityView),
		"  ",
		todoPanel,
	)
}

// renderGroups renders all activity groups as a string
func (m Model) renderGroups() string {
	if len(m.groups) == 0 {
		return ""
	}

	var lines []string
	for i, group := range m.groups {
		lines = append(lines, m.renderGroup(group, i == m.focusedIdx))
	}

	return strings.Join(lines, "")
}

// renderGroup renders a single activity group
func (m Model) renderGroup(g ActivityGroup, focused bool) string {
	var b strings.Builder

	// Line 1: Tool use with icon
	icon := m.getStateIcon(g.State)
	toolName := m.styles.ToolName.Render(g.ToolName)
	detail := m.getInputSummary(g)

	// Focus indicator
	prefix := "  "
	if focused {
		prefix = m.styles.Pending.Render("› ")
	}

	b.WriteString(fmt.Sprintf("%s%s %s %s\n", prefix, icon, toolName, detail))

	// Line 2: Result (if available)
	if g.Result != nil {
		resultIcon := m.styles.Success.Render("✓")
		if !g.Result.Success {
			resultIcon = m.styles.Failed.Render("✗")
		}
		resultText := m.getResultSummary(g)
		b.WriteString(fmt.Sprintf("  %s %s\n", resultIcon, m.styles.Dim.Render(resultText)))
	}

	// Expanded content (if toggled)
	if g.Expanded {
		b.WriteString(m.renderExpanded(g))
	}

	return b.String()
}

// getStateIcon returns the appropriate icon for a group state
func (m Model) getStateIcon(state GroupState) string {
	switch state {
	case StatePending:
		return m.styles.Pending.Render("▸")
	case StateCompleted:
		return m.styles.Success.Render("▸")
	case StateFailed:
		return m.styles.Failed.Render("▸")
	default:
		return "▸"
	}
}

// getInputSummary returns a concise summary of tool input
func (m Model) getInputSummary(g ActivityGroup) string {
	switch g.ToolName {
	case "Read", "Write", "Edit", "Glob":
		if g.Input.FilePath != "" {
			return m.styles.FilePath.Render(g.Input.FilePath)
		}
	case "Bash":
		if g.Input.Description != "" {
			return m.styles.FilePath.Render(g.Input.Description)
		}
		if g.Input.Command != "" {
			return m.styles.FilePath.Render(truncate(g.Input.Command, 50))
		}
	case "TodoWrite":
		count := len(g.Input.Todos)
		if count > 0 {
			return m.styles.FilePath.Render(fmt.Sprintf("(%d items)", count))
		}
		// Fallback: try to extract count from raw
		if countFromRaw := extractTodoCount(g.Input.Raw); countFromRaw > 0 {
			return m.styles.FilePath.Render(fmt.Sprintf("(%d items)", countFromRaw))
		}
	case "Grep":
		if g.Input.Pattern != "" {
			return m.styles.FilePath.Render(fmt.Sprintf("/%s/", truncate(g.Input.Pattern, 30)))
		}
	case "Task":
		if g.Input.Preview != "" {
			return m.styles.FilePath.Render(truncate(g.Input.Preview, 40))
		}
	}

	// Fallback to preview (not raw JSON)
	if g.Input.Preview != "" {
		return m.styles.FilePath.Render(truncate(g.Input.Preview, 40))
	}

	// Don't show raw JSON - just show empty or a generic indicator
	return ""
}

// getResultSummary returns a concise summary of tool result
func (m Model) getResultSummary(g ActivityGroup) string {
	if g.Result == nil {
		return ""
	}

	if !g.Result.Success {
		return truncate(g.Result.Error, 60)
	}

	switch g.ToolName {
	case "Read":
		if g.Result.LineCount > 0 {
			return fmt.Sprintf("[%s] %d lines", shortenPath(g.Input.FilePath), g.Result.LineCount)
		}
		return fmt.Sprintf("[%s]", shortenPath(g.Input.FilePath))
	case "Write":
		return fmt.Sprintf("Updated %s", shortenPath(g.Input.FilePath))
	case "Edit":
		return fmt.Sprintf("Edited %s", shortenPath(g.Input.FilePath))
	case "Bash":
		if g.Result.Output == "" {
			return "(no output)"
		}
		return truncate(g.Result.Output, 60)
	case "TodoWrite":
		return fmt.Sprintf("Updated todos (%d items)", g.Result.TodoCount)
	case "Glob":
		return g.Result.Output
	case "Grep":
		return g.Result.Output
	case "Task":
		return g.Result.Output
	default:
		if g.Result.Output != "" {
			return truncate(g.Result.Output, 60)
		}
		return "done"
	}
}

// renderExpanded renders expanded content for a group
func (m Model) renderExpanded(g ActivityGroup) string {
	var b strings.Builder
	s := m.styles.Expanded

	// Show raw input if available
	if g.Input.Raw != "" && len(g.Input.Raw) < 500 {
		b.WriteString(s.Render("Input: " + truncate(g.Input.Raw, 200) + "\n"))
	}

	// Show full result output
	if g.Result != nil && g.Result.Output != "" {
		b.WriteString(s.Render("Output: " + g.Result.Output + "\n"))
	}

	// For TodoWrite, show the todo list
	if g.ToolName == "TodoWrite" && len(g.Input.Todos) > 0 {
		b.WriteString(s.Render("Todos:\n"))
		for _, todo := range g.Input.Todos {
			icon := "○"
			switch todo.Status {
			case "in_progress":
				icon = "⏳"
			case "completed":
				icon = "✓"
			}
			b.WriteString(s.Render(fmt.Sprintf("  %s %s\n", icon, todo.Content)))
		}
	}

	return b.String()
}

// renderTodoPanel renders the todo progress sidebar
func (m Model) renderTodoPanel() string {
	if len(m.currentTodos) == 0 {
		return ""
	}

	var lines []string

	// Header
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75")).Render("Todo Progress")
	lines = append(lines, header)

	// Progress bar
	completed := 0
	inProgress := 0
	for _, t := range m.currentTodos {
		switch t.Status {
		case "completed":
			completed++
		case "in_progress":
			inProgress++
		}
	}

	total := len(m.currentTodos)
	barWidth := 20
	filledWidth := (completed * barWidth) / total
	partialWidth := (inProgress * barWidth) / total
	emptyWidth := barWidth - filledWidth - partialWidth

	bar := m.styles.Success.Render(strings.Repeat("▓", filledWidth)) +
		m.styles.Pending.Render(strings.Repeat("▒", partialWidth)) +
		m.styles.Dim.Render(strings.Repeat("░", emptyWidth))

	progressLine := fmt.Sprintf("[%s] %d/%d", bar, completed, total)
	lines = append(lines, progressLine)
	lines = append(lines, "")

	// Todo items (show max 10)
	maxItems := 10
	for i, todo := range m.currentTodos {
		if i >= maxItems {
			remaining := len(m.currentTodos) - maxItems
			lines = append(lines, m.styles.Dim.Render(fmt.Sprintf("  ... and %d more", remaining)))
			break
		}

		icon := "○"
		style := m.styles.Dim

		switch todo.Status {
		case "in_progress":
			icon = "⏳"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			// Show activeForm for in-progress items
			lines = append(lines, style.Render(fmt.Sprintf("%s %s", icon, truncate(todo.ActiveForm, 28))))
			continue
		case "completed":
			icon = "✓"
			style = m.styles.Success
		}

		lines = append(lines, style.Render(fmt.Sprintf("%s %s", icon, truncate(todo.Content, 28))))
	}

	return m.styles.TodoPanel.Render(strings.Join(lines, "\n"))
}

// Helper functions

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

// extractTodoCount tries to count todos from raw JSON string
func extractTodoCount(raw string) int {
	if raw == "" {
		return 0
	}
	// Count occurrences of "status" which appears once per todo
	count := strings.Count(raw, `"status"`)
	return count
}

func shortenPath(path string) string {
	// Keep filename and parent dir only if path is long
	if len(path) <= 40 {
		return path
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

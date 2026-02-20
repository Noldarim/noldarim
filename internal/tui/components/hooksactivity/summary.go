// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package hooksactivity

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Bold(true)

	turnsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")) // Cyan

	tokensStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // Orange

	toolsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")) // Purple

	durationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	statusStreamingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true)

	statusCompleteStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))
)

// RenderSummary renders the summary panel
func RenderSummary(s Summary, streaming bool, width int) string {
	// Build two-column layout
	colWidth := (width - 4) / 2 // Account for separator and padding

	// Row 1: Turns | Tokens
	turns := formatMetric("Turns", fmt.Sprintf("%d", s.TurnCount), turnsStyle)
	tokens := formatMetric("Tokens", formatNumber(s.TotalTokens), tokensStyle)
	row1 := formatRow(turns, tokens, colWidth)

	// Row 2: Tools | Duration
	toolCount := countTotalTools(s.ToolsInvoked)
	tools := formatMetric("Tools", fmt.Sprintf("%d calls", toolCount), toolsStyle)
	duration := formatMetric("Duration", formatDuration(s.SessionDuration), durationStyle)
	row2 := formatRow(tools, duration, colWidth)

	// Row 3: Top Tools | Status
	topTools := formatMetric("Top", getTopTools(s.ToolsInvoked, 3), toolsStyle)
	status := formatStatus(s, streaming)
	row3 := formatRow(topTools, status, colWidth)

	// Combine rows
	divider := dividerStyle.Render(strings.Repeat("─", width))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		row1,
		row2,
		row3,
		divider,
	)
}

func formatMetric(label, value string, style lipgloss.Style) string {
	return labelStyle.Render(label+": ") + style.Render(value)
}

func formatRow(left, right string, colWidth int) string {
	leftPadded := lipgloss.NewStyle().Width(colWidth).Render(left)
	rightPadded := lipgloss.NewStyle().Width(colWidth).Render(right)
	return leftPadded + "  " + rightPadded
}

func formatStatus(s Summary, streaming bool) string {
	var statusText string
	var style lipgloss.Style

	if streaming {
		statusText = "Streaming..."
		style = statusStreamingStyle
	} else if s.IsComplete {
		switch s.FinalReason {
		case "completed", "end_turn", "task_complete":
			statusText = "Completed"
			style = statusCompleteStyle
		case "error", "failed":
			statusText = "Failed"
			style = statusErrorStyle
		case "cancelled", "interrupted":
			statusText = "Cancelled"
			style = statusErrorStyle
		default:
			if s.FinalReason != "" {
				statusText = s.FinalReason
			} else {
				statusText = "Completed"
			}
			style = statusCompleteStyle
		}
	} else if len(s.ToolsInvoked) > 0 {
		statusText = "Active"
		style = statusStreamingStyle
	} else {
		statusText = "Waiting"
		style = labelStyle
	}

	return labelStyle.Render("Status: ") + style.Render(statusText)
}

func formatNumber(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func formatDuration(d interface{}) string {
	switch v := d.(type) {
	case int64:
		secs := v
		if secs < 60 {
			return fmt.Sprintf("%ds", secs)
		}
		mins := secs / 60
		secs = secs % 60
		if mins < 60 {
			return fmt.Sprintf("%dm %ds", mins, secs)
		}
		hours := mins / 60
		mins = mins % 60
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		// Handle time.Duration
		if dur, ok := d.(fmt.Stringer); ok {
			return dur.String()
		}
		return "0s"
	}
}

func countTotalTools(tools map[string]int) int {
	total := 0
	for _, count := range tools {
		total += count
	}
	return total
}

func getTopTools(tools map[string]int, n int) string {
	if len(tools) == 0 {
		return "-"
	}

	// Sort by count descending
	type toolCount struct {
		name  string
		count int
	}
	var sorted []toolCount
	for name, count := range tools {
		sorted = append(sorted, toolCount{name, count})
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		// Secondary sort by name for stability when counts are equal
		return sorted[i].name < sorted[j].name
	})

	// Take top N
	var parts []string
	for i := 0; i < n && i < len(sorted); i++ {
		parts = append(parts, fmt.Sprintf("%s(%d)", sorted[i].name, sorted[i].count))
	}

	return strings.Join(parts, " ")
}

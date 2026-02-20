// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package gitdiffviewer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Render displays a git diff with syntax highlighting
func Render(diff string, maxHeight int) string {
	if diff == "" {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
		return emptyStyle.Render("No git diff available")
	}

	lines := strings.Split(diff, "\n")
	var rendered []string

	for _, line := range lines {
		rendered = append(rendered, renderLine(line))
	}

	content := strings.Join(rendered, "\n")

	// Apply max height if specified
	if maxHeight > 0 && len(lines) > maxHeight {
		style := lipgloss.NewStyle().MaxHeight(maxHeight)
		content = style.Render(content)
		// Add scroll indicator
		scrollHint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Render("\n(scroll for more...)")
		content += scrollHint
	}

	return content
}

func renderLine(line string) string {
	// Empty lines
	if len(line) == 0 {
		return ""
	}

	// Diff headers (diff --git, index, +++, ---)
	if strings.HasPrefix(line, "diff --git") ||
		strings.HasPrefix(line, "index ") ||
		strings.HasPrefix(line, "---") ||
		strings.HasPrefix(line, "+++") {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // Cyan
		return style.Render(line)
	}

	// Hunk headers (@@ -10,7 +10,8 @@)
	if strings.HasPrefix(line, "@@") {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("140")) // Purple
		return style.Render(line)
	}

	// Added lines
	if strings.HasPrefix(line, "+") {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green
		return style.Render(line)
	}

	// Removed lines
	if strings.HasPrefix(line, "-") {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red
		return style.Render(line)
	}

	// Context lines (no color, just regular)
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
	return style.Render(line)
}

// RenderCompact renders a compact view showing only file names and change summary
func RenderCompact(diff string) string {
	if diff == "" {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			Render("No changes")
	}

	lines := strings.Split(diff, "\n")
	var files []string
	additions := 0
	deletions := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// Extract file path
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				file := strings.TrimPrefix(parts[3], "b/")
				files = append(files, file)
			}
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		}
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deletions++
		}
	}

	// Render file list
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	var fileList []string
	for _, file := range files {
		fileList = append(fileList, fileStyle.Render("  • "+file))
	}

	// Render stats
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	stats := addStyle.Render(fmt.Sprintf("+%d", additions)) + " " + delStyle.Render(fmt.Sprintf("-%d", deletions))

	if len(fileList) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render("No files changed")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		strings.Join(fileList, "\n"),
		"",
		stats,
	)
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package layout

import (
	"fmt"
	"strings"
)

// HelpItem represents a single help entry
type HelpItem struct {
	Key         string
	Description string
}

// RenderHeader creates a header with title, breadcrumbs, and optional status
func RenderHeader(title string, breadcrumbs []string, status string, width int) string {
	var header strings.Builder

	// Title and breadcrumbs on the same line
	titleLine := TitleStyle.Render(title)
	if len(breadcrumbs) > 1 {
		breadcrumbText := strings.Join(breadcrumbs, BreadcrumbSeparator.String())
		titleLine += "  " + BreadcrumbStyle.Render(breadcrumbText)
	}

	header.WriteString(titleLine)

	// Add status/stats if provided
	if status != "" {
		header.WriteString("\n")
		header.WriteString(StatsStyle.Render(status))
	}

	// Add divider
	header.WriteString("\n")
	header.WriteString(GetDivider(width))

	return header.String()
}

// RenderFooter creates a footer with help items
func RenderFooter(helpItems []HelpItem, width int) string {
	if len(helpItems) == 0 {
		return ""
	}

	var footer strings.Builder

	// Add divider
	footer.WriteString(GetDivider(width))
	footer.WriteString("\n")

	// Format help items
	var helpTexts []string
	for _, item := range helpItems {
		helpText := fmt.Sprintf("[%s] %s",
			HelpKeyStyle.Render(item.Key),
			HelpTextStyle.Render(item.Description))
		helpTexts = append(helpTexts, helpText)
	}

	// Join help items with bullets - let lipgloss handle wrapping
	helpLine := strings.Join(helpTexts, " • ")
	footer.WriteString(FooterStyle.Width(width).Render(helpLine))

	return footer.String()
}

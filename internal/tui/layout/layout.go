// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package layout

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// MinimumWidth is the minimum terminal width required
	MinimumWidth = 40
	// MinimumHeight is the minimum terminal height required (header + footer + some space)
	MinimumHeight = 10
)

// LayoutInfo contains all the information needed to render a layout
type LayoutInfo struct {
	Title       string
	Breadcrumbs []string
	Status      string
	HelpItems   []HelpItem
}

// Dimensions represents the available space for content
type Dimensions struct {
	Width  int
	Height int
	Valid  bool
	Error  string
}

// ValidateSpace checks if the terminal has enough space to render properly
func ValidateSpace(width, height int) Dimensions {
	if width < MinimumWidth {
		return Dimensions{
			Width:  width,
			Height: height,
			Valid:  false,
			Error:  fmt.Sprintf("Terminal too narrow (%d cols). Minimum: %d cols", width, MinimumWidth),
		}
	}

	if height < MinimumHeight {
		return Dimensions{
			Width:  width,
			Height: height,
			Valid:  false,
			Error:  fmt.Sprintf("Terminal too short (%d lines). Minimum: %d lines", height, MinimumHeight),
		}
	}

	return Dimensions{
		Width:  width,
		Height: height,
		Valid:  true,
	}
}

// RenderLayout combines header, content, and footer into a complete layout
// Returns error view if terminal is too small
func RenderLayout(content string, info LayoutInfo, width, height int) string {
	// Validate space
	dims := ValidateSpace(width, height)
	if !dims.Valid {
		return renderSpaceError(dims.Error, width, height)
	}

	// Render header
	header := RenderHeader(info.Title, info.Breadcrumbs, info.Status, width)

	// Render footer if help items exist
	var footer string
	if len(info.HelpItems) > 0 {
		footer = RenderFooter(info.HelpItems, width)
	}

	// Calculate content height using lipgloss
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	contentHeight := height - headerHeight - footerHeight

	// Content gets whatever space is left (can be small, that's okay)
	// If contentHeight is 0 or negative, something is wrong with MinimumHeight validation
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Style the content area using lipgloss
	// IMPORTANT: Use MaxHeight to enforce the constraint (not Height which is minimum)
	styledContent := lipgloss.NewStyle().
		Width(width).
		MaxHeight(contentHeight). // MaxHeight enforces ceiling
		Height(contentHeight).     // Height sets the box size
		Align(lipgloss.Left, lipgloss.Top).
		Render(content)

	// Combine all components using lipgloss's vertical join
	return lipgloss.JoinVertical(lipgloss.Left, header, styledContent, footer)
}

// GetContentArea calculates the available width and height for content
// Returns dimensions struct with validation
func GetContentArea(info LayoutInfo, totalWidth, totalHeight int) Dimensions {
	// First validate total space
	dims := ValidateSpace(totalWidth, totalHeight)
	if !dims.Valid {
		return dims
	}

	// Calculate header height
	header := RenderHeader(info.Title, info.Breadcrumbs, info.Status, totalWidth)
	headerHeight := lipgloss.Height(header)

	// Calculate footer height
	footerHeight := 0
	if len(info.HelpItems) > 0 {
		footer := RenderFooter(info.HelpItems, totalWidth)
		footerHeight = lipgloss.Height(footer)
	}

	// Calculate available content height
	contentHeight := totalHeight - headerHeight - footerHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	return Dimensions{
		Width:  totalWidth,
		Height: contentHeight,
		Valid:  true,
	}
}

// renderSpaceError renders an error message when terminal is too small
func renderSpaceError(message string, width, height int) string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")). // Red
		Bold(true).
		Align(lipgloss.Center, lipgloss.Center).
		Width(width).
		Height(height)

	// Build multi-line error message
	var lines []string
	lines = append(lines, "⚠ Terminal Too Small ⚠")
	lines = append(lines, "")
	lines = append(lines, message)
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Current: %dx%d", width, height))
	lines = append(lines, fmt.Sprintf("Minimum: %dx%d", MinimumWidth, MinimumHeight))
	lines = append(lines, "")
	lines = append(lines, "Please resize your terminal")

	return errorStyle.Render(strings.Join(lines, "\n"))
}

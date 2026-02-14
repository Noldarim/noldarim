// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package card

import (
	"github.com/charmbracelet/lipgloss"
)

// Style defines the visual appearance of a card
type Style struct {
	BorderColor   lipgloss.Color
	BorderStyle   lipgloss.Border
	Padding       []int // [top, right, bottom, left]
	Margin        []int // [top, right, bottom, left]
	TitleColor    lipgloss.Color
	TitleBold     bool
	Width         int
	Height        int
	MaxHeight     int
	ShowScrollbar bool
}

// DefaultStyle returns a sensible default card style
func DefaultStyle() Style {
	return Style{
		BorderColor: lipgloss.Color("240"),
		BorderStyle: lipgloss.RoundedBorder(),
		Padding:     []int{1, 2, 1, 2},
		Margin:      []int{0, 0, 1, 0},
		TitleColor:  lipgloss.Color("86"),
		TitleBold:   true,
		Width:       0, // Auto-width
		Height:      0, // Auto-height
		MaxHeight:   0, // No max
	}
}

// Render creates a bordered card with optional title
func Render(title, content string, style Style) string {
	// Build title if provided
	var titleRendered string
	if title != "" {
		titleStyle := lipgloss.NewStyle().
			Foreground(style.TitleColor).
			Bold(style.TitleBold)
		titleRendered = titleStyle.Render(title)
	}

	// Build content style
	contentStyle := lipgloss.NewStyle()
	if style.Width > 0 {
		contentStyle = contentStyle.Width(style.Width)
	}
	if style.Height > 0 {
		contentStyle = contentStyle.Height(style.Height)
	}
	if style.MaxHeight > 0 {
		contentStyle = contentStyle.MaxHeight(style.MaxHeight)
	}

	// Combine title and content
	var body string
	if titleRendered != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, titleRendered, "", content)
	} else {
		body = content
	}

	// Apply padding
	paddedStyle := lipgloss.NewStyle()
	if len(style.Padding) == 4 {
		paddedStyle = paddedStyle.Padding(style.Padding[0], style.Padding[1], style.Padding[2], style.Padding[3])
	}
	paddedBody := paddedStyle.Render(body)

	// Apply border
	borderStyle := lipgloss.NewStyle().
		Border(style.BorderStyle).
		BorderForeground(style.BorderColor)

	if style.Width > 0 {
		borderStyle = borderStyle.Width(style.Width)
	}

	bordered := borderStyle.Render(paddedBody)

	// Apply margin
	marginStyle := lipgloss.NewStyle()
	if len(style.Margin) == 4 {
		marginStyle = marginStyle.Margin(style.Margin[0], style.Margin[1], style.Margin[2], style.Margin[3])
	}

	return marginStyle.Render(bordered)
}

// RenderSimple is a convenience function for quick card rendering with defaults
func RenderSimple(title, content string) string {
	return Render(title, content, DefaultStyle())
}

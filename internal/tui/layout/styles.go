// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	PrimaryColor   = lipgloss.Color("#7C3AED")
	SecondaryColor = lipgloss.Color("#A78BFA")
	AccentColor    = lipgloss.Color("#10B981")
	TextColor      = lipgloss.Color("#F3F4F6")
	MutedColor     = lipgloss.Color("#9CA3AF")
	BorderColor    = lipgloss.Color("#4B5563")
	ErrorColor     = lipgloss.Color("#EF4444")
	WarningColor   = lipgloss.Color("#F59E0B")
)

var (
	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(PrimaryColor).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Bold(true).
			Align(lipgloss.Left)

	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	BreadcrumbSeparator = lipgloss.NewStyle().
				Foreground(BorderColor).
				SetString(" > ")

	// Status/Stats styles
	StatusStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	StatsStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	// Content area styles - let lipgloss handle the sizing
	ContentStyle = lipgloss.NewStyle().
			Align(lipgloss.Left, lipgloss.Top)

	// Footer styles
	FooterStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(BorderColor).
			PaddingLeft(1).
			PaddingRight(1)

	HelpTextStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	// Divider styles
	DividerStyle = lipgloss.NewStyle().
			Foreground(BorderColor).
			SetString("─")

	// Error styles
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)
)

// GetDivider returns a horizontal divider of the specified width
func GetDivider(width int) string {
	if width <= 0 {
		return ""
	}
	dividerText := strings.Repeat("─", width)
	return lipgloss.NewStyle().
		Foreground(BorderColor).
		Render(dividerText)
}

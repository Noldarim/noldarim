// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package tokendisplay

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TokenData holds the token counts for display
type TokenData struct {
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheCreateTokens int
}

// Model represents the token display component
type Model struct {
	data  TokenData
	style lipgloss.Style
}

// New creates a new token display model
func New() Model {
	return Model{
		style: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// View renders: In: 45,230 (12k cache) | Out: 8,120 (+5k cache)
func (m Model) View() string {
	bold := m.style.Bold(true).Foreground(lipgloss.Color("252"))
	dim := m.style.Foreground(lipgloss.Color("239"))

	input := fmt.Sprintf("In: %s", bold.Render(formatNumber(m.data.InputTokens)))
	if m.data.CacheReadTokens > 0 {
		input += dim.Render(fmt.Sprintf(" (%s cache)", formatCompact(m.data.CacheReadTokens)))
	}

	output := fmt.Sprintf("Out: %s", bold.Render(formatNumber(m.data.OutputTokens)))
	if m.data.CacheCreateTokens > 0 {
		output += dim.Render(fmt.Sprintf(" (+%s cache)", formatCompact(m.data.CacheCreateTokens)))
	}

	return input + dim.Render(" | ") + output
}

// SetData updates the token data
func (m Model) SetData(data TokenData) Model {
	m.data = data
	return m
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	str := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(str)+(len(str)-1)/3)
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatCompact(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

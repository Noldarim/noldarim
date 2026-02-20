// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package scrollablecard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/components/card"
)

// Model represents a scrollable card with focus management
type Model struct {
	title    string
	viewport viewport.Model
	focused  bool
	style    card.Style
	ready    bool
}

// New creates a new scrollable card
func New(title, content string, width, height int) Model {
	vp := viewport.New(width, height)
	vp.SetContent(content)

	style := card.DefaultStyle()
	style.BorderColor = lipgloss.Color("240") // Start unfocused

	return Model{
		title:    title,
		viewport: vp,
		focused:  false,
		style:    style,
		ready:    true,
	}
}

// Init initializes the scrollable card
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the scrollable card
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	// Only handle input when focused
	if !m.focused {
		return m, nil
	}

	// Forward scroll commands to viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the scrollable card
func (m Model) View() string {
	// Get viewport content
	content := m.viewport.View()

	// Update style based on focus
	if m.focused {
		m.style.BorderColor = lipgloss.Color("86")  // Bright cyan when focused
		m.style.BorderStyle = lipgloss.ThickBorder() // Thicker border when focused
	} else {
		m.style.BorderColor = lipgloss.Color("240")      // Dim when not focused
		m.style.BorderStyle = lipgloss.RoundedBorder()   // Normal border
	}

	// Render card with viewport content
	return card.Render(m.title, content, m.style)
}

// SetFocus sets the focus state of the card
func (m *Model) SetFocus(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the card is focused
func (m Model) IsFocused() bool {
	return m.focused
}

// SetSize updates the card dimensions
func (m *Model) SetSize(width, height int) {
	m.viewport.Width = width
	m.viewport.Height = height
}

// SetContent updates the card content
func (m *Model) SetContent(content string) {
	m.viewport.SetContent(content)
}

// SetTitle updates the card title
func (m *Model) SetTitle(title string) {
	m.title = title
}

// GetTitle returns the card title
func (m Model) GetTitle() string {
	return m.title
}

// ScrollPercent returns the current scroll percentage (0.0 to 1.0)
func (m Model) ScrollPercent() float64 {
	return m.viewport.ScrollPercent()
}

// AtTop returns true if viewport is at the top
func (m Model) AtTop() bool {
	return m.viewport.AtTop()
}

// AtBottom returns true if viewport is at the bottom
func (m Model) AtBottom() bool {
	return m.viewport.AtBottom()
}

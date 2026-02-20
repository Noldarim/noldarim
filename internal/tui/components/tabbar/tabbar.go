// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package tabbar

import (
	"github.com/charmbracelet/lipgloss"
)

// Tab represents a single tab in the tab bar
type Tab struct {
	ID    string
	Label string
	Badge string // Optional badge (e.g., event count)
}

// Model represents the tab bar state
type Model struct {
	tabs      []Tab
	activeTab int
	width     int
}

// New creates a new tab bar with the given tabs
func New(tabs []Tab) Model {
	return Model{
		tabs:      tabs,
		activeTab: 0,
		width:     80,
	}
}

// SetActiveTab sets the active tab by index
func (m *Model) SetActiveTab(index int) {
	if index >= 0 && index < len(m.tabs) {
		m.activeTab = index
	}
}

// GetActiveTab returns the index of the active tab
func (m Model) GetActiveTab() int {
	return m.activeTab
}

// GetActiveTabID returns the ID of the active tab
func (m Model) GetActiveTabID() string {
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		return m.tabs[m.activeTab].ID
	}
	return ""
}

// NextTab switches to the next tab (wrapping around)
func (m *Model) NextTab() {
	m.activeTab = (m.activeTab + 1) % len(m.tabs)
}

// PrevTab switches to the previous tab (wrapping around)
func (m *Model) PrevTab() {
	m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
}

// SetWidth sets the width of the tab bar
func (m *Model) SetWidth(width int) {
	m.width = width
}

// SetBadge sets a badge on a specific tab by index
func (m *Model) SetBadge(index int, badge string) {
	if index >= 0 && index < len(m.tabs) {
		m.tabs[index].Badge = badge
	}
}

// View renders the tab bar
func (m Model) View() string {
	if len(m.tabs) == 0 {
		return ""
	}

	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Background(lipgloss.Color("236")).
		Padding(0, 2)

	badgeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	var tabViews []string
	for i, tab := range m.tabs {
		label := tab.Label
		if tab.Badge != "" {
			label += " " + badgeStyle.Render(tab.Badge)
		}

		if i == m.activeTab {
			tabViews = append(tabViews, activeTabStyle.Render(label))
		} else {
			tabViews = append(tabViews, inactiveTabStyle.Render(label))
		}
	}

	// Join tabs with a small gap
	gap := lipgloss.NewStyle().
		Background(lipgloss.Color("234")).
		Render(" ")

	result := ""
	for i, tv := range tabViews {
		if i > 0 {
			result += gap
		}
		result += tv
	}

	// Add filler to reach width
	tabBarStyle := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("234"))

	return tabBarStyle.Render(result)
}

// Count returns the number of tabs
func (m Model) Count() int {
	return len(m.tabs)
}

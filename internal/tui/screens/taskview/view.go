// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/components/commitgraph"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// View renders the task view screen
func (m Model) View() string {
	layoutInfo := m.GetLayoutInfo()

	// If form is shown, render the form instead of tabs
	if m.showForm {
		content := m.form.View()
		// Pass actual dimensions to layout system
		return layout.RenderLayout(content, layoutInfo, m.width, m.height)
	}

	// Render the complete tab component
	tabComponent := m.renderTabComponent()

	// Pass actual dimensions to layout system
	return layout.RenderLayout(tabComponent, layoutInfo, m.width, m.height)
}

// renderTabComponent creates a self-contained tab component
func (m Model) renderTabComponent() string {
	// Render tab headers
	tabHeaders := m.renderTabs()

	// Render content for the active tab
	var tabContent string
	switch m.activeTab {
	case 0: // Tasks tab
		tabContent = m.renderTasksTabContent()
	case 1: // Commits tab
		tabContent = m.renderCommitsTabContent()
	}

	// Combine tab headers and content
	return lipgloss.JoinVertical(lipgloss.Top, tabHeaders, tabContent)
}

// renderTabs renders the tab headers
func (m Model) renderTabs() string {
	var tabHeaders []string

	activeTabStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 2)

	inactiveTabStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 2)

	for i, tab := range m.tabs {
		if i == m.activeTab {
			tabHeaders = append(tabHeaders, activeTabStyle.Render(tab))
		} else {
			tabHeaders = append(tabHeaders, inactiveTabStyle.Render(tab))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, tabHeaders...)

	// Just return the tab row without extra separator
	// The tab styles already have bottom borders
	return tabRow
}

// renderTasksTabContent renders the tasks list content
func (m Model) renderTasksTabContent() string {
	// Handle empty state
	if len(m.tasks) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Align(lipgloss.Center, lipgloss.Center)
		return emptyStyle.Render("No tasks found. Press 'n' to create a new task.")
	}

	// Get the list content
	return m.list.View()
}

// renderCommitsTabContent renders the commit graph content
func (m Model) renderCommitsTabContent() string {
	if !m.commitsLoaded {
		// Show loading state
		emptyStyle := lipgloss.NewStyle().
			Align(lipgloss.Center, lipgloss.Center)
		return emptyStyle.Render("Loading commits...")
	}

	if len(m.commits) == 0 {
		// Show empty state
		emptyStyle := lipgloss.NewStyle().
			Align(lipgloss.Center, lipgloss.Center)
		return emptyStyle.Render("No commits found in repository")
	}

	// Get selected commit hash pointer
	var selectedHash *string
	if m.selectedCommit >= 0 && m.selectedCommit < len(m.commits) {
		selectedHash = m.commits[m.selectedCommit].HashPtr()
	}

	// Create style function for coloring branches
	branchColors := []string{"1", "2", "3", "4", "5", "6", "9", "10", "11", "12", "13", "14"}
	colorMap := make(map[*string]string)

	// Assign colors based on lane positions
	positionColors := make(map[int16]string)
	nextColorIndex := 0

	for i, commit := range m.commits {
		if lane, exists := m.commitLanes[i]; exists {
			if _, hasColor := positionColors[lane]; !hasColor {
				positionColors[lane] = branchColors[nextColorIndex%len(branchColors)]
				nextColorIndex++
			}
			colorMap[commit.HashPtr()] = positionColors[lane]
		}
	}

	getStyle := func(c *commitgraph.Commit) *lipgloss.Style {
		color := colorMap[c.HashPtr()]
		if color == "" {
			color = "7" // Default gray
		}
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		return &s
	}

	// Render the commit graph
	lines := commitgraph.RenderCommitGraph(m.commits, selectedHash, getStyle)

	var builder strings.Builder
	builder.WriteString("Navigation: ↑/↓ (j/k) = navigate commits\n\n")

	for i, line := range lines {
		if i < len(m.commits) {
			commit := m.commits[i]
			commitStyle := lipgloss.NewStyle()
			if i == m.selectedCommit {
				commitStyle = commitStyle.Bold(true).Foreground(lipgloss.Color("15"))
			}

			// Truncate hash to 7 characters for display
			hashStr := ""
			if commit.Hash != nil && len(*commit.Hash) >= 7 {
				hashStr = (*commit.Hash)[:7]
			}

			formattedLine := fmt.Sprintf("%s %s %s (%s)",
				line,
				commitStyle.Render(hashStr),
				commitStyle.Render(commit.Message),
				commit.Author)
			builder.WriteString(formattedLine)
		} else {
			builder.WriteString(line)
		}
		if i < len(lines)-1 {
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

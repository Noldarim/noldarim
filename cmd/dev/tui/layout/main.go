// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// Component represents a placeholder component in our demo
type Component struct {
	name    string
	content string
	focused bool
	width   int
	height  int
	style   lipgloss.Style
}

// Model represents the state of our layout demo
type Model struct {
	components     []Component
	focusedIndex   int
	terminalWidth  int
	terminalHeight int
	showHelp       bool
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func initialModel() Model {
	return Model{
		components: []Component{
			{
				name:    "Sidebar",
				content: "Navigation\n• Projects\n• Tasks\n• Settings",
				focused: true,
				width:   20,
				height:  8,
				style:   createComponentStyle(true),
			},
			{
				name:    "Main Content",
				content: "Task List\n━━━━━━━━━━\n□ Task 1: Setup\n□ Task 2: Design\n☑ Task 3: Code\n□ Task 4: Test",
				focused: false,
				width:   40,
				height:  8,
				style:   createComponentStyle(false),
			},
			{
				name:    "Details Panel",
				content: "Task Details\n━━━━━━━━━━━━\nID: #123\nStatus: In Progress\nAssignee: User\nDue: Tomorrow",
				focused: false,
				width:   25,
				height:  8,
				style:   createComponentStyle(false),
			},
		},
		focusedIndex:   0,
		terminalWidth:  80,
		terminalHeight: 24,
		showHelp:       true,
	}
}

func createComponentStyle(focused bool) lipgloss.Style {
	baseStyle := lipgloss.NewStyle().
		Padding(1).
		Margin(0, 1)

	if focused {
		return baseStyle.
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Background(lipgloss.Color("#1F1F28"))
	}

	return baseStyle.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#4B5563")).
		Background(lipgloss.Color("#0F0F17"))
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "h", "?":
			m.showHelp = !m.showHelp

		case "tab", "right":
			m.focusedIndex = (m.focusedIndex + 1) % len(m.components)
			// Update focus inline
			for i := range m.components {
				m.components[i].focused = i == m.focusedIndex
				m.components[i].style = createComponentStyle(m.components[i].focused)
			}

		case "shift+tab", "left":
			m.focusedIndex = (m.focusedIndex - 1 + len(m.components)) % len(m.components)
			// Update focus inline
			for i := range m.components {
				m.components[i].focused = i == m.focusedIndex
				m.components[i].style = createComponentStyle(m.components[i].focused)
			}

		case "up":
			if m.components[m.focusedIndex].height > 3 {
				m.components[m.focusedIndex].height--
			}

		case "down":
			if m.components[m.focusedIndex].height < 15 {
				m.components[m.focusedIndex].height++
			}

		case "shift+left":
			if m.components[m.focusedIndex].width > 10 {
				m.components[m.focusedIndex].width--
			}

		case "shift+right":
			if m.components[m.focusedIndex].width < 60 {
				m.components[m.focusedIndex].width++
			}

		case "r":
			// Reset to default sizes
			m.components[0].width, m.components[0].height = 20, 8
			m.components[1].width, m.components[1].height = 40, 8
			m.components[2].width, m.components[2].height = 25, 8
		}
	}

	return m, nil
}

// adjustComponentsForWidth ensures components fit within available width
func (m Model) adjustComponentsForWidth(components []Component, availableWidth int) []Component {
	adjusted := make([]Component, len(components))
	copy(adjusted, components)

	// Calculate total width needed (including borders and margins)
	// Each component adds ~4 chars for borders + 2 for margins = 6 extra chars per component
	totalExtraWidth := len(components) * 6
	totalRequestedWidth := totalExtraWidth

	for _, comp := range adjusted {
		totalRequestedWidth += comp.width
	}

	// If everything fits, return as-is
	if totalRequestedWidth <= availableWidth {
		return adjusted
	}

	// Calculate available width for actual content
	availableContentWidth := availableWidth - totalExtraWidth
	if availableContentWidth < len(components)*5 { // Minimum 5 chars per component
		availableContentWidth = len(components) * 5
	}

	// Scale down components proportionally
	totalOriginalWidth := 0
	for _, comp := range adjusted {
		totalOriginalWidth += comp.width
	}

	for i := range adjusted {
		if totalOriginalWidth > 0 {
			proportion := float64(adjusted[i].width) / float64(totalOriginalWidth)
			newWidth := int(proportion * float64(availableContentWidth))
			if newWidth < 5 {
				newWidth = 5 // Minimum width
			}
			adjusted[i].width = newWidth
		}
	}

	return adjusted
}

func (m Model) View() string {
	// Create layout info for the header and footer
	layoutInfo := layout.LayoutInfo{
		Title:       "Layout Demo",
		Breadcrumbs: []string{"Dev Tools", "TUI", "Layout Showcase"},
		Status:      fmt.Sprintf("Terminal: %dx%d | Focused: %s", m.terminalWidth, m.terminalHeight, m.components[m.focusedIndex].name),
		HelpItems: []layout.HelpItem{
			{Key: "tab/shift+tab", Description: "change focus"},
			{Key: "↑↓", Description: "height"},
			{Key: "shift+←→", Description: "width"},
			{Key: "r", Description: "reset sizes"},
			{Key: "h/?", Description: "toggle help"},
			{Key: "q", Description: "quit"},
		},
	}

	// Create the main content area with component layout
	content := m.renderComponents()

	// Add help section if enabled
	if m.showHelp {
		content += "\n\n" + m.renderHelpText()
	}

	// Use the layout system to wrap everything
	return layout.RenderLayout(content, layoutInfo, m.terminalWidth, m.terminalHeight)
}

func (m Model) renderComponents() string {
	var rows []string

	// Calculate available content area
	layoutInfo := layout.LayoutInfo{
		Title:       "Layout Demo",
		Breadcrumbs: []string{"Dev Tools", "TUI", "Layout Showcase"},
		Status:      fmt.Sprintf("Terminal: %dx%d", m.terminalWidth, m.terminalHeight),
		HelpItems: []layout.HelpItem{
			{Key: "tab", Description: "change focus"},
		},
	}

	dims := layout.GetContentArea(layoutInfo, m.terminalWidth, m.terminalHeight)

	// Title for the component area
	rows = append(rows, lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Bold(true).
		Render("Interactive Component Layout"))

	rows = append(rows, "")

	// Show component information
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	rows = append(rows, infoStyle.Render(fmt.Sprintf("Available content area: %dx%d", dims.Width, dims.Height)))

	// Check if components were auto-adjusted
	totalRequestedWidth := 0
	for _, comp := range m.components {
		totalRequestedWidth += comp.width + 6 // Include borders/margins
	}
	if totalRequestedWidth > dims.Width {
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
		rows = append(rows, warningStyle.Render("⚠ Components auto-sized to fit available width"))
	}

	rows = append(rows, "")

	// Calculate if components fit horizontally and adjust if needed
	adjustedComponents := m.adjustComponentsForWidth(m.components, dims.Width)

	// Render components in a horizontal layout
	var componentBoxes []string
	for i, comp := range adjustedComponents {
		box := m.renderComponent(comp, i == m.focusedIndex)
		componentBoxes = append(componentBoxes, box)
	}

	// Join components horizontally
	componentsRow := lipgloss.JoinHorizontal(lipgloss.Top, componentBoxes...)
	rows = append(rows, componentsRow)

	// Add size information for focused component
	rows = append(rows, "")
	focused := m.components[m.focusedIndex]
	sizeInfo := infoStyle.Render(fmt.Sprintf("Focused: %s (%dx%d)", focused.name, focused.width, focused.height))
	rows = append(rows, sizeInfo)

	return strings.Join(rows, "\n")
}

func (m Model) renderComponent(comp Component, isFocused bool) string {
	// Create header for the component
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F3F4F6")).
		Bold(true).
		Align(lipgloss.Center)

	if isFocused {
		headerStyle = headerStyle.
			Foreground(lipgloss.Color("#7C3AED")).
			Background(lipgloss.Color("#1F1F28"))
	}

	header := headerStyle.Render(fmt.Sprintf("▼ %s ▼", comp.name))

	// Create content area
	contentStyle := comp.style.
		Width(comp.width).
		Height(comp.height).
		Align(lipgloss.Left, lipgloss.Top)

	// Add size indicator to content
	sizeIndicator := fmt.Sprintf("\n%dx%d", comp.width, comp.height)
	enhancedContent := comp.content + sizeIndicator

	contentBox := contentStyle.Render(enhancedContent)

	// Combine header and content
	return lipgloss.JoinVertical(lipgloss.Center, header, contentBox)
}

func (m Model) renderHelpText() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4B5563")).
		Padding(1).
		Margin(1, 0)

	helpContent := `Layout Demo Help
━━━━━━━━━━━━━━━━

This demo showcases the layout system with interactive placeholder components.

Controls:
• Tab/Shift+Tab: Switch focus between components
• ↑/↓: Increase/decrease height of focused component
• Shift+←/→: Increase/decrease width of focused component
• r: Reset all components to default sizes
• h/?: Toggle this help text
• q: Quit the demo

Notice how:
- The layout automatically adapts to terminal size changes
- Header and footer remain consistent while content adjusts
- Component borders change when focused
- Size information is displayed in real-time`

	return helpStyle.Render(helpContent)
}

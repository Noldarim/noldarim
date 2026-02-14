// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// ContentType represents different types of content that affect sizing
type ContentType int

const (
	Short ContentType = iota
	Medium
	Long
	VeryLong
	List
	Table
)

// FlexComponent represents a component that sizes based on content and flex properties
type FlexComponent struct {
	name        string
	contentType ContentType
	flexGrow    int // Similar to CSS flex-grow
	minWidth    int
	focused     bool
}

// Model represents our flexbox-like layout demo
type Model struct {
	components     []FlexComponent
	focusedIndex   int
	terminalWidth  int
	terminalHeight int
	showHelp       bool
	viewport       viewport.Model
	contentCache   string // Cache content to avoid regeneration
	contentDirty   bool   // Track when content needs regeneration
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func initialModel() Model {
	vp := viewport.New(80, 10) // Initial size, will be updated on resize

	return Model{
		components: []FlexComponent{
			{
				name:        "Sidebar",
				contentType: Short,
				flexGrow:    1, // Takes 1 part of available space
				minWidth:    15,
				focused:     true,
			},
			{
				name:        "Main Content",
				contentType: Medium,
				flexGrow:    3, // Takes 3 parts of available space
				minWidth:    25,
				focused:     false,
			},
			{
				name:        "Details",
				contentType: Short,
				flexGrow:    1, // Takes 1 part of available space
				minWidth:    20,
				focused:     false,
			},
		},
		focusedIndex:   0,
		terminalWidth:  80,
		terminalHeight: 24,
		showHelp:       true,
		viewport:       vp,
		contentDirty:   true, // Initial content generation needed
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

		// Update viewport size based on available content area
		layoutInfo := m.getLayoutInfo()
		dims := layout.GetContentArea(layoutInfo, m.terminalWidth, m.terminalHeight)
		m.viewport.Width = m.terminalWidth
		m.viewport.Height = dims.Height
		m.contentDirty = true // Window resize affects layout

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "h", "?":
			m.showHelp = !m.showHelp
			// Help doesn't affect main content, so no need to mark dirty

		case "tab", "right":
			m = m.withFocusedIndex((m.focusedIndex + 1) % len(m.components))
			m.contentDirty = true

		case "shift+tab", "left":
			m = m.withFocusedIndex((m.focusedIndex - 1 + len(m.components)) % len(m.components))
			m.contentDirty = true

		case "1", "2", "3", "4", "5":
			// Change content type to affect sizing
			contentTypes := []ContentType{Short, Medium, Long, VeryLong, List}
			if key := msg.String()[0] - '1'; int(key) < len(contentTypes) {
				m.components[m.focusedIndex].contentType = contentTypes[key]
				m.contentDirty = true
			}

		case "6":
			m.components[m.focusedIndex].contentType = Table
			m.contentDirty = true

		case "+", "=":
			// Increase flex-grow (takes more space)
			if m.components[m.focusedIndex].flexGrow < 5 {
				m.components[m.focusedIndex].flexGrow++
				m.contentDirty = true
			}

		case "-":
			// Decrease flex-grow (takes less space)
			if m.components[m.focusedIndex].flexGrow > 1 {
				m.components[m.focusedIndex].flexGrow--
				m.contentDirty = true
			}

		case "r":
			// Reset to defaults
			m.components[0] = FlexComponent{"Sidebar", Short, 1, 15, false}
			m.components[1] = FlexComponent{"Main Content", Medium, 3, 25, false}
			m.components[2] = FlexComponent{"Details", Short, 1, 20, false}
			m = m.withFocusedIndex(0)
			m.contentDirty = true

		case "up", "k":
			m.viewport.LineUp(1)

		case "down", "j":
			m.viewport.LineDown(1)

		case "pgup":
			m.viewport.HalfViewUp()

		case "pgdown":
			m.viewport.HalfViewDown()

		case "home":
			m.viewport.GotoTop()

		case "end":
			m.viewport.GotoBottom()
		}
	}

	// Regenerate content if dirty
	if m.contentDirty {
		content := m.renderFlexLayout()
		m.viewport.SetContent(content)
		m.contentDirty = false
	}

	// Handle viewport updates
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)

	return m, cmd
}

// withFocusedIndex returns a new model with updated focus (immutable pattern)
func (m Model) withFocusedIndex(index int) Model {
	// Create a copy to maintain immutability
	newComponents := make([]FlexComponent, len(m.components))
	copy(newComponents, m.components)

	for i := range newComponents {
		newComponents[i].focused = i == index
	}

	m.components = newComponents
	m.focusedIndex = index
	return m
}

// getLayoutInfo creates the layout info struct (helper to avoid duplication)
func (m Model) getLayoutInfo() layout.LayoutInfo {
	return layout.LayoutInfo{
		Title:       "Flexbox-Style Layout Demo",
		Breadcrumbs: []string{"Dev Tools", "TUI", "Lipgloss Flexbox"},
		Status:      fmt.Sprintf("Terminal: %dx%d | Focused: %s (flex: %d) | Scroll: %d%%", m.terminalWidth, m.terminalHeight, m.components[m.focusedIndex].name, m.components[m.focusedIndex].flexGrow, int(m.viewport.ScrollPercent()*100)),
		HelpItems: []layout.HelpItem{
			{Key: "tab/shift+tab", Description: "change focus"},
			{Key: "1-6", Description: "content type"},
			{Key: "+/-", Description: "flex grow"},
			{Key: "↑↓/j/k", Description: "scroll"},
			{Key: "pgup/pgdn", Description: "page scroll"},
			{Key: "r", Description: "reset"},
			{Key: "h/?", Description: "toggle help"},
			{Key: "q", Description: "quit"},
		},
	}
}

func (m Model) View() string {
	// Create layout info
	layoutInfo := m.getLayoutInfo()

	// Get viewport content (which is scrollable)
	viewportContent := m.viewport.View()

	// Add help if enabled (outside viewport so it doesn't scroll)
	var finalContent string
	if m.showHelp {
		finalContent = viewportContent + "\n\n" + m.renderHelpText()
	} else {
		finalContent = viewportContent
	}

	// Use layout system to wrap with header/footer (always visible)
	return layout.RenderLayout(finalContent, layoutInfo, m.terminalWidth, m.terminalHeight)
}

func (m Model) renderFlexLayout() string {
	var rows []string

	// Get available content area
	layoutInfo := layout.LayoutInfo{
		Title:       "Flexbox-Style Layout Demo",
		Breadcrumbs: []string{"Dev Tools", "TUI", "Lipgloss Flexbox"},
		Status:      "Status",
		HelpItems:   []layout.HelpItem{{Key: "test", Description: "test"}},
	}

	dims := layout.GetContentArea(layoutInfo, m.terminalWidth, m.terminalHeight)

	// Title
	rows = append(rows, lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Bold(true).
		Render("Content-Driven Flexbox Layout"))

	rows = append(rows, "")

	// Info
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	rows = append(rows, infoStyle.Render(fmt.Sprintf("Available width: %d | Total flex-grow: %d", dims.Width, m.getTotalFlexGrow())))

	// Calculate flex widths
	widths := m.calculateFlexWidths(dims.Width)
	totalCalculatedWidth := 0
	for _, w := range widths {
		totalCalculatedWidth += w
	}

	// Show overflow warning if needed
	if totalCalculatedWidth > dims.Width {
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
		rows = append(rows, warningStyle.Render("⚠ Content overflow detected - components will wrap"))
	}

	rows = append(rows, "")

	// Render components
	componentBoxes := m.renderComponents(widths)

	// Check if we need to wrap (overflow handling)
	if totalCalculatedWidth <= dims.Width {
		// Horizontal layout
		componentsRow := lipgloss.JoinHorizontal(lipgloss.Top, componentBoxes...)
		rows = append(rows, componentsRow)
	} else {
		// Vertical layout when overflow (like CSS flex-wrap)
		rows = append(rows, infoStyle.Render("↳ Wrapped to vertical layout due to overflow:"))
		rows = append(rows, "")
		for _, box := range componentBoxes {
			rows = append(rows, box)
			rows = append(rows, "")
		}
	}

	// Show flex info for focused component
	rows = append(rows, "")
	focused := m.components[m.focusedIndex]
	focusInfo := infoStyle.Render(fmt.Sprintf("Focused: %s | Content: %s | Flex-grow: %d | Calculated width: %d",
		focused.name,
		m.getContentTypeName(focused.contentType),
		focused.flexGrow,
		widths[m.focusedIndex]))
	rows = append(rows, focusInfo)

	return strings.Join(rows, "\n")
}

func (m Model) calculateFlexWidths(availableWidth int) []int {
	totalFlexGrow := m.getTotalFlexGrow()
	totalMinWidth := 0
	borderOverhead := len(m.components) * 4 // Approximate border/margin overhead

	// Calculate minimum required width
	for _, comp := range m.components {
		totalMinWidth += comp.minWidth
	}

	// Available width for flex distribution
	flexibleWidth := availableWidth - totalMinWidth - borderOverhead
	if flexibleWidth < 0 {
		flexibleWidth = 0
	}

	// Calculate widths
	widths := make([]int, len(m.components))
	for i, comp := range m.components {
		baseWidth := comp.minWidth
		if totalFlexGrow > 0 && flexibleWidth > 0 {
			flexWidth := (flexibleWidth * comp.flexGrow) / totalFlexGrow
			baseWidth += flexWidth
		}
		widths[i] = baseWidth
	}

	return widths
}

func (m Model) getTotalFlexGrow() int {
	total := 0
	for _, comp := range m.components {
		total += comp.flexGrow
	}
	return total
}

func (m Model) renderComponents(widths []int) []string {
	var boxes []string

	for i, comp := range m.components {
		content := m.generateContent(comp.contentType)
		box := m.renderComponent(comp, content, widths[i])
		boxes = append(boxes, box)
	}

	return boxes
}

func (m Model) renderComponent(comp FlexComponent, content string, width int) string {
	// Create style based on focus
	var style lipgloss.Style
	if comp.focused {
		style = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Padding(1).
			Width(width).
			Align(lipgloss.Left, lipgloss.Top)
	} else {
		style = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#4B5563")).
			Padding(1).
			Width(width).
			Align(lipgloss.Left, lipgloss.Top)
	}

	// Add component header
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F3F4F6")).
		Bold(true).
		Render(fmt.Sprintf("▼ %s (flex: %d) ▼", comp.name, comp.flexGrow))

	// Add size info to content
	enhancedContent := fmt.Sprintf("%s\n\n%s\nWidth: %d\nMin: %d",
		header, content, width, comp.minWidth)

	return style.Render(enhancedContent)
}

func (m Model) generateContent(contentType ContentType) string {
	switch contentType {
	case Short:
		return "Short content"

	case Medium:
		return "Medium length content\nthat spans multiple\nlines to demonstrate\nhow content affects\ncomponent sizing"

	case Long:
		return strings.Repeat("This is a long line of content that will wrap and affect the component's natural sizing behavior. ", 3)

	case VeryLong:
		return strings.Repeat("Very long content with lots of text that demonstrates how components handle extensive content and automatic wrapping behavior in terminal interfaces. ", 2)

	case List:
		return `List Content:
• Item 1: Short item
• Item 2: Medium length item
• Item 3: Very long item that wraps
• Item 4: Another item
• Item 5: Final item
• Item 6: Extra content
• Item 7: More content
• Item 8: Even more`

	case Table:
		return `Table Content:
Name    | Status  | Progress
--------|---------|----------
Task 1  | Done    | 100%
Task 2  | Active  | 75%
Task 3  | Pending | 0%
Task 4  | Review  | 90%
Task 5  | Blocked | 25%`

	default:
		return "Default content"
	}
}

func (m Model) getContentTypeName(ct ContentType) string {
	names := []string{"Short", "Medium", "Long", "VeryLong", "List", "Table"}
	if int(ct) < len(names) {
		return names[ct]
	}
	return "Unknown"
}

func (m Model) renderHelpText() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4B5563")).
		Padding(1)

	helpContent := `Flexbox-Style Layout Help
━━━━━━━━━━━━━━━━━━━━━━━━━━

This demo shows content-driven, flexbox-like layout behavior:

Content Controls (affects natural sizing):
• 1: Short content
• 2: Medium content
• 3: Long content
• 4: Very long content
• 5: List content
• 6: Table content

Flex Controls (affects space distribution):
• +: Increase flex-grow (takes more available space)
• -: Decrease flex-grow (takes less available space)

Navigation:
• Tab/Shift+Tab: Switch focus between components
• ↑↓ or j/k: Scroll content up/down
• PgUp/PgDn: Page scroll up/down
• Home/End: Jump to top/bottom
• r: Reset all components to defaults
• h/?: Toggle this help text
• q: Quit

Notice how:
- Components size naturally based on their content
- Flex-grow controls how extra space is distributed
- Layout automatically wraps when content overflows
- Everything responds to terminal resizing
- Similar to CSS flexbox behavior`

	return helpStyle.Render(helpContent)
}

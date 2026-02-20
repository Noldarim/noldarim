// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activityfeed

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EventType matches the AI event types
type EventType string

const (
	EventToolUse      EventType = "tool_use"
	EventToolResult   EventType = "tool_result"
	EventThinking     EventType = "thinking"
	EventAIOutput     EventType = "ai_output"
	EventSubagentStart EventType = "subagent_start"
	EventSubagentStop  EventType = "subagent_stop"
	EventError        EventType = "error"
)

// Activity represents a single activity item
type Activity struct {
	EventType      EventType
	ToolName       string
	ContentPreview string
	FilePath       string
	ToolSuccess    *bool
	ToolError      string
}

// Model represents the activity feed component
type Model struct {
	activities []Activity
	maxItems   int
}

// New creates a new activity feed model
func New() Model {
	return Model{
		maxItems: 10,
	}
}

// SetActivities sets the activity list
func (m Model) SetActivities(activities []Activity) Model {
	m.activities = activities
	return m
}

// SetMaxItems sets the maximum number of items to display
func (m Model) SetMaxItems(n int) Model {
	m.maxItems = n
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// View renders the activity feed
func (m Model) View() string {
	if len(m.activities) == 0 {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	tool := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	thinking := lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("35"))
	fail := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	output := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	var lines []string
	start := 0
	if len(m.activities) > m.maxItems {
		start = len(m.activities) - m.maxItems
	}

	for _, a := range m.activities[start:] {
		line := renderActivity(a, dim, tool, thinking, success, fail, output)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func renderActivity(a Activity, dim, tool, thinking, success, fail, output lipgloss.Style) string {
	switch a.EventType {
	case EventToolUse:
		icon := tool.Render("▸")
		name := tool.Render(a.ToolName)
		detail := ""
		if a.FilePath != "" {
			detail = dim.Render(" " + cleanString(a.FilePath))
		} else if a.ContentPreview != "" {
			detail = dim.Render(" " + cleanString(a.ContentPreview))
		}
		return fmt.Sprintf("%s %s%s", icon, name, detail)

	case EventToolResult:
		if a.ToolSuccess != nil && !*a.ToolSuccess {
			icon := fail.Render("✗")
			errMsg := cleanString(a.ToolError)
			return fmt.Sprintf("%s %s", icon, fail.Render(errMsg))
		}
		icon := success.Render("✓")
		detail := ""
		if a.ContentPreview != "" {
			detail = dim.Render(cleanString(a.ContentPreview))
		}
		return fmt.Sprintf("%s %s", icon, detail)

	case EventThinking:
		icon := thinking.Render("◦")
		preview := cleanString(a.ContentPreview)
		return fmt.Sprintf("%s %s", icon, thinking.Render(preview))

	case EventAIOutput:
		preview := cleanString(a.ContentPreview)
		return output.Render(preview)

	case EventSubagentStart:
		icon := tool.Render("↳")
		return fmt.Sprintf("%s %s", icon, tool.Render("subagent started"))

	case EventSubagentStop:
		icon := dim.Render("↲")
		return fmt.Sprintf("%s %s", icon, dim.Render("subagent ended"))

	case EventError:
		icon := fail.Render("!")
		return fmt.Sprintf("%s %s", icon, fail.Render(cleanString(a.ContentPreview)))

	default:
		return dim.Render(cleanString(a.ContentPreview))
	}
}

func cleanString(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

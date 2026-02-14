// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskstatus

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

type TickMsg time.Time

// GlobalTickMsg represents a global tick message from the main TUI
type GlobalTickMsg time.Time

// UIState represents UI-specific states for visual feedback
type UIState string

const (
	UIStateNormal  UIState = ""        // Normal state, show task status
	UIStatePending UIState = "pending" // Task creation pending
	UIStateFailed  UIState = "failed"  // Task creation failed
	UIStateCreated UIState = "created" // Task creation completed
)

type Model struct {
	text    string
	status  models.TaskStatus
	uiState UIState // UI-specific state for visual feedback
	spinner spinner.Model
	width   int
}

func New(text string, status models.TaskStatus) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		text:    text,
		status:  status,
		spinner: s,
		width:   0,
	}
}

func (m Model) Init() tea.Cmd {
	// Start spinner for in-progress tasks or pending UI state
	if m.status == models.TaskStatusInProgress || m.uiState == UIStatePending {
		return m.spinner.Tick
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		// Update spinner for in-progress tasks or pending UI state
		if m.status == models.TaskStatusInProgress || m.uiState == UIStatePending {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(spinner.TickMsg{})
			return m, cmd
		}
	case GlobalTickMsg:
		// Handle global tick messages from main TUI
		if m.status == models.TaskStatusInProgress || m.uiState == UIStatePending {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(spinner.TickMsg{})
			return m, cmd
		}
	case spinner.TickMsg:
		// Update spinner for in-progress tasks or pending UI state
		if m.status == models.TaskStatusInProgress || m.uiState == UIStatePending {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.width > 0 && m.width < 20 {
		return m.renderCompact()
	}
	return m.renderFull()
}

func (m Model) renderFull() string {
	icon := m.getIcon()
	style := m.getStyle()
	text := m.getText()
	return style.Render(icon + " " + text)
}

func (m Model) renderCompact() string {
	icon := m.getIcon()
	style := m.getStyle()
	return style.Render(icon)
}

func (m Model) getIcon() string {
	// UI states override task status for display
	switch m.uiState {
	case UIStatePending:
		return m.spinner.View() // Spinner for pending creation
	case UIStateFailed:
		return "✗" // X mark for failed
	case UIStateCreated:
		return "✓" // Check mark for created
	default:
		// Normal state: show task status
		switch m.status {
		case models.TaskStatusPending:
			return "○"
		case models.TaskStatusInProgress:
			return m.spinner.View()
		case models.TaskStatusCompleted:
			return "✓"
		default:
			return "?"
		}
	}
}

func (m Model) getStyle() lipgloss.Style {
	// UI states override task status for styling
	switch m.uiState {
	case UIStatePending:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow for pending
	case UIStateFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red for failed
	case UIStateCreated:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Green for created
	default:
		// Normal state: show task status colors
		switch m.status {
		case models.TaskStatusPending:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		case models.TaskStatusInProgress:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		case models.TaskStatusCompleted:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
		default:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("red"))
		}
	}
}

func (m Model) SetWidth(width int) Model {
	m.width = width
	return m
}

func (m Model) SetStatus(status models.TaskStatus) Model {
	m.status = status
	return m
}

func (m Model) SetText(text string) Model {
	m.text = text
	return m
}

func (m Model) SetUIState(uiState UIState) Model {
	m.uiState = uiState
	return m
}

// getText returns the appropriate text based on UI state
func (m Model) getText() string {
	switch m.uiState {
	case UIStatePending:
		return "PENDING"
	case UIStateFailed:
		return "FAILED"
	case UIStateCreated:
		return "CREATED"
	default:
		// For normal state, return task status text
		switch m.status {
		case models.TaskStatusPending:
			return "PENDING"
		case models.TaskStatusInProgress:
			return "IN PROGRESS"
		case models.TaskStatusCompleted:
			return "COMPLETED"
		default:
			return "UNKNOWN"
		}
	}
}

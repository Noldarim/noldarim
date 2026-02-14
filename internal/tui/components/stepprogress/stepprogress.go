// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package stepprogress

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StepStatus represents the status of a step
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusSkipped
)

// Step represents a single step in the progress
type Step struct {
	Name   string
	Status StepStatus
}

// Model represents the step progress component
type Model struct {
	steps []Step
	width int
}

// New creates a new step progress model
func New() Model {
	return Model{
		width: 20,
	}
}

// SetSteps sets the list of steps
func (m Model) SetSteps(steps []Step) Model {
	m.steps = steps
	return m
}

// SetWidth sets the progress bar width
func (m Model) SetWidth(w int) Model {
	m.width = w
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// View renders: [▓▓▓▓▓░░░░░] 2/4 Code Review
func (m Model) View() string {
	if len(m.steps) == 0 {
		return ""
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("35"))

	// Count completed and find current
	completed := 0
	currentIdx := -1
	currentName := ""
	for i, s := range m.steps {
		if s.Status == StatusCompleted {
			completed++
		}
		if s.Status == StatusRunning {
			currentIdx = i
			currentName = s.Name
		}
	}

	// Build progress bar
	total := len(m.steps)
	filled := (completed * m.width) / total
	if currentIdx >= 0 {
		filled = ((completed*m.width + m.width/2) / total)
	}

	bar := ""
	for i := 0; i < m.width; i++ {
		if i < filled {
			bar += success.Render("▓")
		} else {
			bar += dim.Render("░")
		}
	}

	// Step counter
	displayStep := completed
	if currentIdx >= 0 {
		displayStep = currentIdx + 1
	}

	// Current step name or "Complete"
	label := ""
	if currentName != "" {
		label = accent.Render(currentName)
	} else if completed == total {
		label = success.Render("Complete ✓")
	}

	return fmt.Sprintf("[%s] %s %s", bar, dim.Render(fmt.Sprintf("%d/%d", displayStep, total)), label)
}

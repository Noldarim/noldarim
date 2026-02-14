// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package elapsedtimer

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TickMsg is sent every second to update the timer
type TickMsg time.Time

// Model represents the elapsed timer component
type Model struct {
	startTime time.Time
	elapsed   time.Duration
	running   bool
	style     lipgloss.Style
}

// New creates a new elapsed timer model
func New() Model {
	return Model{
		style: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}

// Start begins the timer from now
func (m Model) Start() Model {
	m.startTime = time.Now()
	m.running = true
	return m
}

// StartFrom begins the timer from a specific time
func (m Model) StartFrom(t time.Time) Model {
	m.startTime = t
	m.running = true
	m.elapsed = time.Since(t)
	return m
}

// Stop halts the timer
func (m Model) Stop() Model {
	m.elapsed = time.Since(m.startTime)
	m.running = false
	return m
}

// SetElapsed sets a specific elapsed duration (for display without ticking)
func (m Model) SetElapsed(d time.Duration) Model {
	m.elapsed = d
	m.running = false
	return m
}

func (m Model) Init() tea.Cmd {
	if m.running {
		return tick()
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		if m.running {
			m.elapsed = time.Since(m.startTime)
			return m, tick()
		}
	}
	return m, nil
}

// View renders: "⏱ 2m 34s"
func (m Model) View() string {
	dim := m.style.Foreground(lipgloss.Color("239"))
	accent := m.style.Foreground(lipgloss.Color("75"))

	return dim.Render("⏱") + " " + accent.Render(formatDuration(m.Elapsed()))
}

// Elapsed returns the current elapsed duration
func (m Model) Elapsed() time.Duration {
	if m.running {
		return time.Since(m.startTime)
	}
	return m.elapsed
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

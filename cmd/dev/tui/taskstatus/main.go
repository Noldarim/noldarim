// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/tui/components/taskstatus"
)

type statusItem struct {
	title       string
	description string
	status      models.TaskStatus
}

func (i statusItem) FilterValue() string { return i.title }
func (i statusItem) Title() string       { return i.title }
func (i statusItem) Description() string { return i.description }

type statusDelegate struct {
	width      int
	components map[int]taskstatus.Model
}

func (d statusDelegate) Height() int  { return 1 }
func (d statusDelegate) Spacing() int { return 0 }

func (d *statusDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	var cmds []tea.Cmd

	// Update all components
	for i, item := range m.Items() {
		if statusItem, ok := item.(statusItem); ok {
			if d.components == nil {
				d.components = make(map[int]taskstatus.Model)
			}

			// Get or create component
			component, exists := d.components[i]
			if !exists {
				component = taskstatus.New(statusItem.title, statusItem.status)
				component = component.SetWidth(d.width - 4)
			}

			// Update component
			updatedComponent, cmd := component.Update(msg)
			d.components[i] = updatedComponent
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return tea.Batch(cmds...)
}

func (d *statusDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(statusItem)
	if !ok {
		return
	}

	// Get or create component
	var component taskstatus.Model
	if d.components != nil {
		if comp, exists := d.components[index]; exists {
			component = comp
		} else {
			component = taskstatus.New(i.title, i.status)
			component = component.SetWidth(d.width - 4)
		}
	} else {
		component = taskstatus.New(i.title, i.status)
		component = component.SetWidth(d.width - 4)
	}

	selected := index == m.Index()
	style := lipgloss.NewStyle()
	if selected {
		style = style.Background(lipgloss.Color("62"))
	}

	fmt.Fprint(w, style.Render(component.View()))
}

type tickMsg time.Time

type Model struct {
	list     list.Model
	delegate *statusDelegate
	quitting bool
}

func initialModel() Model {
	items := []list.Item{
		statusItem{title: "Deploy to staging", status: models.TaskStatusPending, description: "Pending deployment task"},
		statusItem{title: "Run tests", status: models.TaskStatusInProgress, description: "Currently running tests"},
		statusItem{title: "Code review", status: models.TaskStatusCompleted, description: "Code review completed"},
		statusItem{title: "Build application", status: models.TaskStatusPending, description: "Application build pending"},
		statusItem{title: "Database migration", status: models.TaskStatusInProgress, description: "Running database migration"},
		statusItem{title: "Security scan", status: models.TaskStatusCompleted, description: "Security scan passed"},
		statusItem{title: "Performance test", status: models.TaskStatusPending, description: "Performance testing queued"},
		statusItem{title: "Documentation update", status: models.TaskStatusCompleted, description: "Documentation is up to date"},
	}

	delegate := &statusDelegate{width: 80, components: make(map[int]taskstatus.Model)}

	// Initialize components
	for i, item := range items {
		if statusItem, ok := item.(statusItem); ok {
			component := taskstatus.New(statusItem.title, statusItem.status)
			component = component.SetWidth(76) // 80 - 4 for padding
			delegate.components[i] = component
		}
	}

	l := list.New(items, delegate, 80, 20)
	l.Title = "Task Status Component Demo"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginLeft(2)

	return Model{
		list:     l,
		delegate: delegate,
	}
}

func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, m.tick())

	// Initialize all in-progress components
	for _, component := range m.delegate.components {
		if cmd := component.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
		m.delegate.width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			m.quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		// Send tick to all in-progress components
		return m, tea.Batch(
			m.tick(),
			func() tea.Msg { return taskstatus.TickMsg(time.Time(msg)) },
		)

	case taskstatus.TickMsg:
		// This will be handled by individual components in their Update methods
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1).
		MarginLeft(2)

	help := "↑/↓: Navigate • q: Quit • Demo shows different task statuses with animations"

	return m.list.View() + "\n" + helpStyle.Render(help)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}

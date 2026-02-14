// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/screens/projectcreation"
)

type demoModel struct {
	screen  projectcreation.Model
	width   int
	height  int
	cmdChan chan protocol.Command
	evtChan chan protocol.Event
}

func (m demoModel) Init() tea.Cmd {
	return tea.Batch(
		m.screen.Init(),
		listenForEvents(m.evtChan),
	)
}

func (m demoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.screen.SetSize(m.width, m.height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

	case protocol.Event:
		screenModel, cmd := m.screen.Update(msg)
		if updatedScreen, ok := screenModel.(projectcreation.Model); ok {
			m.screen = updatedScreen
		}
		cmds = append(cmds, cmd)

	case protocol.Command:
		fmt.Printf("Command sent: %T\n", msg)
		// Simulate processing the create command
		if cmd, ok := msg.(protocol.CreateProjectCommand); ok {
			// Use proper Bubble Tea command instead of goroutine
			return m, func() tea.Msg {
				// Simulate some processing time
				time.Sleep(1 * time.Second)

				// Create event
				event := protocol.ProjectCreatedEvent{
					Metadata: protocol.Metadata{},
					Project: &models.Project{
						ID:          "99",
						Name:        cmd.Name,
						Description: cmd.Description,
						CreatedAt:   time.Now(),
					},
				}

				// Send success event with safe channel operation
				select {
				case m.evtChan <- event:
				default:
					// Channel full, skip
				}

				return event
			}
		}
	}

	// Forward other messages to screen
	screenModel, cmd := m.screen.Update(msg)
	if updatedScreen, ok := screenModel.(projectcreation.Model); ok {
		m.screen = updatedScreen
	}
	cmds = append(cmds, cmd)

	// Always restart the event listener
	cmds = append(cmds, listenForEvents(m.evtChan))

	return m, tea.Batch(cmds...)
}

func (m demoModel) View() string {
	return m.screen.View()
}

func listenForEvents(evtChan chan protocol.Event) tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-evtChan:
			return event
		case <-time.After(100 * time.Millisecond):
			// Non-blocking timeout to prevent indefinite blocking
			return nil
		}
	}
}

func main() {
	cmdChan := make(chan protocol.Command, 10)
	evtChan := make(chan protocol.Event, 10)

	screen := projectcreation.NewModel(cmdChan)
	screen.SetSize(80, 24)

	model := demoModel{
		screen:  screen,
		width:   80,
		height:  24,
		cmdChan: cmdChan,
		evtChan: evtChan,
	}

	fmt.Println("Project Creation Screen Demo")
	fmt.Println("Commands:")
	fmt.Println("  Tab/Shift+Tab - Navigate between fields")
	fmt.Println("  Enter - Submit form (when on submit button)")
	fmt.Println("  Esc - Cancel")
	fmt.Println("  Ctrl+C - Quit")
	fmt.Println("")
	fmt.Println("Fill in the form fields:")
	fmt.Println("  - Project Name")
	fmt.Println("  - Description")
	fmt.Println("  - Repository URL")
	fmt.Println("  - Branch (optional)")
	fmt.Println("")
	time.Sleep(3 * time.Second)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/screens/settings"
)

type demoModel struct {
	screen  settings.Model
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
		case "s":
			// Simulate settings saved
			// For demo purposes, just create a generic success message
			event := protocol.ErrorEvent{
				Metadata: protocol.Metadata{},
				Message:  "Settings saved successfully",
				Context:  "Settings",
			}
			return m, func() tea.Msg { return event }
		case "e":
			// Simulate error
			event := protocol.ErrorEvent{
				Metadata: protocol.Metadata{},
				Message:  "Failed to save settings: Permission denied",
				Context:  "Saving settings",
			}
			return m, func() tea.Msg { return event }
		}

	case protocol.Event:
		screenModel, cmd := m.screen.Update(msg)
		if updatedScreen, ok := screenModel.(settings.Model); ok {
			m.screen = updatedScreen
		}
		cmds = append(cmds, cmd)

	case protocol.Command:
		fmt.Printf("Command sent: %T\n", msg)

	default:
		// Forward other messages to screen (avoid double processing)
		screenModel, cmd := m.screen.Update(msg)
		if updatedScreen, ok := screenModel.(settings.Model); ok {
			m.screen = updatedScreen
		}
		cmds = append(cmds, cmd)
	}

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

	screen := settings.NewModel()
	screen.SetSize(80, 24)

	model := demoModel{
		screen:  screen,
		width:   80,
		height:  24,
		cmdChan: cmdChan,
		evtChan: evtChan,
	}

	fmt.Println("Settings Screen Demo")
	fmt.Println("Commands:")
	fmt.Println("  Arrow keys/Tab - Navigate fields")
	fmt.Println("  Enter/Space - Toggle boolean values or edit text")
	fmt.Println("  s - Simulate save success")
	fmt.Println("  e - Simulate save error")
	fmt.Println("  Ctrl+S - Save settings")
	fmt.Println("  Esc - Cancel/Go back")
	fmt.Println("  Ctrl+C - Quit")
	fmt.Println("")
	time.Sleep(3 * time.Second)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

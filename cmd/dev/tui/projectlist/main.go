// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/screens/projectlist"
)

type demoModel struct {
	screen   *projectlist.Model
	width    int
	height   int
	cmdChan  chan protocol.Command
	evtChan  chan protocol.Event
	projects []models.Project
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
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			// Simulate refresh
			projectsMap := make(map[string]*models.Project)
			for i := range m.projects {
				projectsMap[m.projects[i].ID] = &m.projects[i]
			}
			event := protocol.ProjectsLoadedEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "refresh-" + time.Now().Format(time.RFC3339Nano),
				},
				Projects: projectsMap,
			}
			return m, func() tea.Msg { return event }
		case "e":
			// Simulate error
			event := protocol.ErrorEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "error-" + time.Now().Format(time.RFC3339Nano),
				},
				Message: "Simulated error for testing",
				Context: "Loading projects",
			}
			return m, func() tea.Msg { return event }
		}

	case protocol.Event:
		screenModel, cmd := m.screen.Update(msg)
		if updatedScreen, ok := screenModel.(*projectlist.Model); ok {
			m.screen = updatedScreen
		}
		cmds = append(cmds, cmd)

	case protocol.Command:
		// In a real app, this would be sent to orchestrator
		fmt.Printf("Command sent: %T\n", msg)

	default:
		// Forward other messages to screen (avoid double processing)
		screenModel, cmd := m.screen.Update(msg)
		if updatedScreen, ok := screenModel.(*projectlist.Model); ok {
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

func createMockProjects() []models.Project {
	now := time.Now()
	return []models.Project{
		{
			ID:             "proj-1",
			Name:           "Web Application",
			Description:    "Main web application with React frontend",
			RepositoryPath: "/tmp/webapp",
			AgentID:        "agent-1",
			CreatedAt:      now.Add(-72 * time.Hour),
			LastUpdatedAt:  now.Add(-2 * time.Hour),
		},
		{
			ID:             "proj-2",
			Name:           "API Service",
			Description:    "RESTful API backend service",
			RepositoryPath: "/tmp/api",
			AgentID:        "agent-2",
			CreatedAt:      now.Add(-48 * time.Hour),
			LastUpdatedAt:  now.Add(-5 * time.Hour),
		},
		{
			ID:             "proj-3",
			Name:           "Mobile App",
			Description:    "React Native mobile application",
			RepositoryPath: "/tmp/mobile",
			AgentID:        "agent-3",
			CreatedAt:      now.Add(-120 * time.Hour),
			LastUpdatedAt:  now.Add(-24 * time.Hour),
		},
		{
			ID:             "proj-4",
			Name:           "Documentation Site",
			Description:    "Project documentation built with Hugo",
			RepositoryPath: "/tmp/docs",
			AgentID:        "agent-4",
			CreatedAt:      now.Add(-24 * time.Hour),
			LastUpdatedAt:  now.Add(-1 * time.Hour),
		},
	}
}

func main() {
	cmdChan := make(chan protocol.Command, 10)
	evtChan := make(chan protocol.Event, 10)

	screen := projectlist.NewModel(cmdChan)
	screen.SetSize(80, 24)

	projects := createMockProjects()

	model := demoModel{
		screen:   &screen,
		width:    80,
		height:   24,
		cmdChan:  cmdChan,
		evtChan:  evtChan,
		projects: projects,
	}

	// Send initial projects loaded event
	go func() {
		time.Sleep(100 * time.Millisecond)
		projectsMap := make(map[string]*models.Project)
		for i := range projects {
			projectsMap[projects[i].ID] = &projects[i]
		}
		evtChan <- protocol.ProjectsLoadedEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "initial-" + time.Now().Format(time.RFC3339Nano),
			},
			Projects: projectsMap,
		}
	}()

	fmt.Println("Project List Screen Demo")
	fmt.Println("Commands:")
	fmt.Println("  Arrow keys - Navigate")
	fmt.Println("  Enter - Select project")
	fmt.Println("  n - New project")
	fmt.Println("  d - Delete project")
	fmt.Println("  r - Refresh projects")
	fmt.Println("  e - Simulate error")
	fmt.Println("  q - Quit")
	fmt.Println("")
	time.Sleep(2 * time.Second)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/screens/taskview"
)

var debugLog *log.Logger

func init() {
	file, err := os.OpenFile("taskview_debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	debugLog = log.New(file, "", log.Ltime|log.Lmicroseconds)
}

type demoModel struct {
	screen     *taskview.Model
	width      int
	height     int
	cmdChan    chan protocol.Command
	evtChan    chan protocol.Event
	tasks      []models.Task
	commits    []protocol.CommitInfo
	initialCmd tea.Cmd
}

func (m demoModel) Init() tea.Cmd {
	return tea.Batch(
		m.screen.Init(),
		listenForEvents(m.evtChan),
		m.initialCmd,
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
		case "r":
			// Simulate refresh
			tasksMap := make(map[string]*models.Task)
			for i := range m.tasks {
				tasksMap[m.tasks[i].ID] = &m.tasks[i]
			}
			event := protocol.TasksLoadedEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "refresh-" + time.Now().Format(time.RFC3339Nano),
				},
				ProjectID:      "proj-1",
				ProjectName:    "Demo Project",
				RepositoryPath: "/tmp/demo",
				Tasks:          tasksMap,
			}
			return m, func() tea.Msg { return event }
		case "c":
			// Load commits
			event := protocol.CommitsLoadedEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "commits-" + time.Now().Format(time.RFC3339Nano),
				},
				ProjectID:      "proj-1",
				RepositoryPath: "/tmp/demo",
				Commits:        m.commits,
			}
			return m, func() tea.Msg { return event }
		}

	case protocol.Event:
		debugLog.Printf("Received protocol event: %T", msg)
		screenModel, cmd := m.screen.Update(msg)
		if updatedScreen, ok := screenModel.(*taskview.Model); ok {
			m.screen = updatedScreen
		}
		cmds = append(cmds, cmd)

	case protocol.Command:
		fmt.Printf("Command sent: %T\n", msg)
	}

	// Forward other messages to screen
	screenModel, cmd := m.screen.Update(msg)
	if updatedScreen, ok := screenModel.(*taskview.Model); ok {
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

func createMockTasks() []models.Task {
	now := time.Now()
	return []models.Task{
		{
			ID:          "task-1",
			ProjectID:   "proj-1",
			Title:       "Implement user authentication",
			Description: "Add JWT-based authentication to the API",
			Status:      models.TaskStatusInProgress,
			CreatedAt:   now.Add(-48 * time.Hour),
		},
		{
			ID:          "task-2",
			ProjectID:   "proj-1",
			Title:       "Add database migrations",
			Description: "Set up migration system for database schema changes",
			Status:      models.TaskStatusCompleted,
			CreatedAt:   now.Add(-72 * time.Hour),
		},
		{
			ID:          "task-3",
			ProjectID:   "proj-1",
			Title:       "Write API documentation",
			Description: "Document all API endpoints with OpenAPI spec",
			Status:      models.TaskStatusPending,
			CreatedAt:   now.Add(-24 * time.Hour),
		},
		{
			ID:          "task-4",
			ProjectID:   "proj-1",
			Title:       "Fix memory leak in worker process",
			Description: "Investigate and fix memory leak reported in production",
			Status:      models.TaskStatusInProgress,
			CreatedAt:   now.Add(-12 * time.Hour),
		},
		{
			ID:          "task-5",
			ProjectID:   "proj-1",
			Title:       "Upgrade dependencies",
			Description: "Update all npm packages to latest stable versions",
			Status:      models.TaskStatusPending,
			CreatedAt:   now.Add(-36 * time.Hour),
		},
	}
}

func createMockCommits() []protocol.CommitInfo {
	return []protocol.CommitInfo{
		{
			Hash:    "abc123def",
			Message: "feat: Add user authentication endpoints",
			Author:  "Alice <alice@example.com>",
			Parents: []string{"def456ghi"},
		},
		{
			Hash:    "def456ghi",
			Message: "fix: Resolve database connection timeout",
			Author:  "Bob <bob@example.com>",
			Parents: []string{"ghi789jkl"},
		},
		{
			Hash:    "ghi789jkl",
			Message: "chore: Update dependencies",
			Author:  "Charlie <charlie@example.com>",
			Parents: []string{"jkl012mno"},
		},
		{
			Hash:    "jkl012mno",
			Message: "docs: Add API documentation",
			Author:  "Alice <alice@example.com>",
			Parents: []string{"mno345pqr"},
		},
		{
			Hash:    "mno345pqr",
			Message: "refactor: Improve error handling",
			Author:  "Bob <bob@example.com>",
			Parents: []string{},
		},
	}
}

func main() {
	cmdChan := make(chan protocol.Command, 10)
	evtChan := make(chan protocol.Event, 10)

	project := models.Project{
		ID:             "proj-1",
		Name:           "Demo Project",
		Description:    "A demo project for testing the task view",
		RepositoryPath: "/tmp/demo",
		AgentID:        "agent-demo",
		CreatedAt:      time.Now(),
		LastUpdatedAt:  time.Now(),
	}

	screen := taskview.NewModel(project.ID, cmdChan)
	screen.SetSize(100, 30)

	tasks := createMockTasks()
	commits := createMockCommits()

	// Send initial data via proper commands
	sendInitialData := func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		tasksMap := make(map[string]*models.Task)
		for i := range tasks {
			tasksMap[tasks[i].ID] = &tasks[i]
		}

		debugLog.Printf("Sending TasksLoadedEvent with %d tasks", len(tasksMap))
		// Return tasks event directly
		return protocol.TasksLoadedEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "initial-tasks-" + time.Now().Format(time.RFC3339Nano),
			},
			ProjectID:      "proj-1",
			ProjectName:    "Demo Project",
			RepositoryPath: "/tmp/demo",
			Tasks:          tasksMap,
		}
	}

	// Send commits after tasks are loaded
	sendCommitsData := func() tea.Msg {
		return protocol.CommitsLoadedEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "initial-commits-" + time.Now().Format(time.RFC3339Nano),
			},
			ProjectID:      "proj-1",
			RepositoryPath: "/tmp/demo",
			Commits:        commits,
		}
	}

	model := demoModel{
		screen:     &screen,
		width:      100,
		height:     30,
		cmdChan:    cmdChan,
		evtChan:    evtChan,
		tasks:      tasks,
		commits:    commits,
		initialCmd: tea.Sequence(sendInitialData, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg { return sendCommitsData() })),
	}

	fmt.Println("Task View Screen Demo")
	fmt.Println("Commands:")
	fmt.Println("  Tab/1/2 - Switch between Tasks and Commits tabs")
	fmt.Println("  Arrow keys/j/k - Navigate items")
	fmt.Println("  Enter - View task details")
	fmt.Println("  n - New task")
	fmt.Println("  d - Delete task")
	fmt.Println("  r - Refresh tasks")
	fmt.Println("  c - Load commits")
	fmt.Println("  Esc - Go back")
	fmt.Println("  Ctrl+C - Quit")
	fmt.Println("")
	time.Sleep(3 * time.Second)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

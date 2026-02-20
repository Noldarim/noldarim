// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/components/taskstatus"
	"github.com/noldarim/noldarim/internal/tui/messages"
	"github.com/noldarim/noldarim/internal/tui/screens/projectcreation"
	"github.com/noldarim/noldarim/internal/tui/screens/projectlist"
	"github.com/noldarim/noldarim/internal/tui/screens/settings"
	"github.com/noldarim/noldarim/internal/tui/screens/taskdetails"
	"github.com/noldarim/noldarim/internal/tui/screens/taskview"
)

// ScreenType represents the current active screen
type ScreenType int

const (
	ProjectListScreen ScreenType = iota
	TaskViewScreen
	TaskDetailsScreen
	SettingsScreen
	ProjectCreationScreen
)

type MainModel struct {
	// Current screen state
	currentScreen ScreenType
	// Screen history for back navigation
	screenHistory []ScreenType

	// Individual screen models
	projectList     projectlist.Model
	taskView        taskview.Model
	taskDetails     taskdetails.Model
	settings        settings.Model
	projectCreation projectcreation.Model

	// Global state
	width, height int
	cmdChan       chan<- protocol.Command
	eventChan     <-chan protocol.Event
}

// NewMainModel creates a new MainModel with the project list as the initial screen
func NewMainModel(cmdChan chan<- protocol.Command, eventChan <-chan protocol.Event) MainModel {
	return MainModel{
		currentScreen: ProjectListScreen,
		screenHistory: []ScreenType{},
		projectList:   projectlist.NewModel(cmdChan),
		taskView:      taskview.Model{}, // Will be initialized when needed
		settings:      settings.NewModel(),
		cmdChan:       cmdChan,
		eventChan:     eventChan,
	}
}

func (m MainModel) Init() tea.Cmd {
	return tea.Batch(
		m.projectList.Init(),
		tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

// tickMsg is used for global animation timing
type tickMsg time.Time

// setSize updates the size for the current screen
func (m *MainModel) setSize(width, height int) {
	m.width = width
	m.height = height
	switch m.currentScreen {
	case ProjectListScreen:
		m.projectList.SetSize(width, height)
	case TaskViewScreen:
		m.taskView.SetSize(width, height)
	case TaskDetailsScreen:
		m.taskDetails.SetSize(width, height)
	case SettingsScreen:
		m.settings.SetSize(width, height)
	case ProjectCreationScreen:
		m.projectCreation.SetSize(width, height)
	}
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle global tick messages for animations
	if tickMsgValue, ok := msg.(tickMsg); ok {
		// Schedule next tick
		cmds = append(cmds, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))
		// Convert to GlobalTickMsg for taskstatus components
		msg = taskstatus.GlobalTickMsg(tickMsgValue)
	}

	// Handle window size messages at the top level
	if windowSize, ok := msg.(tea.WindowSizeMsg); ok {
		m.setSize(windowSize.Width, windowSize.Height)
	}

	// Handle Navigation Messages First (these return early to avoid screen delegation)
	switch msg := msg.(type) {
	case messages.GoToTasksScreenMsg:
		// Push current screen to history
		m.screenHistory = append(m.screenHistory, m.currentScreen)
		// Initialize task view with project ID
		m.taskView = taskview.NewModel(msg.ProjectID, m.cmdChan)
		m.taskView.SetSize(m.width, m.height)
		m.currentScreen = TaskViewScreen
		navCmd := m.taskView.Init()
		if len(cmds) > 0 {
			return m, tea.Batch(append(cmds, navCmd)...)
		}
		return m, navCmd

	case messages.GoToTaskDetailsMsg:
		// Push current screen to history
		m.screenHistory = append(m.screenHistory, m.currentScreen)
		// Initialize task details with task data
		m.taskDetails = taskdetails.NewModel(msg.Task, msg.ProjectID, m.cmdChan)
		m.taskDetails.SetSize(m.width, m.height)
		m.currentScreen = TaskDetailsScreen
		navCmd := m.taskDetails.Init()

		// Request historical AI activity events for this task
		taskID := msg.Task.ID
		projectID := msg.ProjectID
		go func() {
			m.cmdChan <- protocol.LoadAIActivityCommand{
				ProjectID: projectID,
				TaskID:    taskID,
			}
		}()

		if len(cmds) > 0 {
			return m, tea.Batch(append(cmds, navCmd)...)
		}
		return m, navCmd

	case messages.GoToSettingsMsg:
		// Push current screen to history
		m.screenHistory = append(m.screenHistory, m.currentScreen)
		m.currentScreen = SettingsScreen
		m.settings.SetSize(m.width, m.height)
		navCmd := m.settings.Init()
		if len(cmds) > 0 {
			return m, tea.Batch(append(cmds, navCmd)...)
		}
		return m, navCmd

	case messages.GoBackMsg:
		// Pop from history if available
		if len(m.screenHistory) > 0 {
			m.currentScreen = m.screenHistory[len(m.screenHistory)-1]
			m.screenHistory = m.screenHistory[:len(m.screenHistory)-1]
			m.setSize(m.width, m.height) // Refresh size for the screen we're going back to
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil

	case messages.GoToProjectCreationMsg:
		// Push current screen to history
		m.screenHistory = append(m.screenHistory, m.currentScreen)
		// Initialize project creation screen
		m.projectCreation = projectcreation.NewModel(m.cmdChan)
		m.projectCreation.SetSize(m.width, m.height)
		m.currentScreen = ProjectCreationScreen
		navCmd := m.projectCreation.Init()
		if len(cmds) > 0 {
			return m, tea.Batch(append(cmds, navCmd)...)
		}
		return m, navCmd

	case messages.GoToProjectListMsg:
		// Clear history and go back to project list
		m.currentScreen = ProjectListScreen
		m.screenHistory = []ScreenType{}
		m.projectList.SetSize(m.width, m.height)
		navCmd := m.projectList.Init()
		if len(cmds) > 0 {
			return m, tea.Batch(append(cmds, navCmd)...)
		}
		return m, navCmd
	}

	// Log protocol events for debugging
	if batchEvent, ok := msg.(protocol.AIActivityBatchEvent); ok {
		log := logger.GetTUILogger().With().Str("component", "main_model").Logger()
		log.Info().
			Str("currentScreen", screenName(m.currentScreen)).
			Str("taskID", batchEvent.TaskID).
			Int("activityCount", len(batchEvent.Activities)).
			Msg("AIActivityBatchEvent received in main_model")
	}

	// Delegate to the current screen
	var screenCmd tea.Cmd
	switch m.currentScreen {
	case ProjectListScreen:
		var model tea.Model
		model, screenCmd = m.projectList.Update(msg)
		m.projectList = model.(projectlist.Model)
	case TaskViewScreen:
		var model tea.Model
		model, screenCmd = m.taskView.Update(msg)
		m.taskView = model.(taskview.Model)
	case TaskDetailsScreen:
		var model tea.Model
		model, screenCmd = m.taskDetails.Update(msg)
		m.taskDetails = model.(taskdetails.Model)
	case SettingsScreen:
		var model tea.Model
		model, screenCmd = m.settings.Update(msg)
		m.settings = model.(settings.Model)
	case ProjectCreationScreen:
		var model tea.Model
		model, screenCmd = m.projectCreation.Update(msg)
		m.projectCreation = model.(projectcreation.Model)
	}

	// Add screen command to batch if it exists
	if screenCmd != nil {
		cmds = append(cmds, screenCmd)
	}

	// Return with batched commands
	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m MainModel) View() string {
	switch m.currentScreen {
	case ProjectListScreen:
		return m.projectList.View()
	case TaskViewScreen:
		return m.taskView.View()
	case TaskDetailsScreen:
		return m.taskDetails.View()
	case SettingsScreen:
		return m.settings.View()
	case ProjectCreationScreen:
		return m.projectCreation.View()
	default:
		return "Unknown screen"
	}
}

// screenName returns a string representation of the screen type for logging
func screenName(s ScreenType) string {
	switch s {
	case ProjectListScreen:
		return "ProjectList"
	case TaskViewScreen:
		return "TaskView"
	case TaskDetailsScreen:
		return "TaskDetails"
	case SettingsScreen:
		return "Settings"
	case ProjectCreationScreen:
		return "ProjectCreation"
	default:
		return "Unknown"
	}
}

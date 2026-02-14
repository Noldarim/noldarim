// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskview

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/components/commitgraph"
	"github.com/noldarim/noldarim/internal/tui/components/taskstatus"
	"github.com/noldarim/noldarim/internal/tui/messages"
)

// getTUILog returns the logger for this component (lazy initialization)
func getTUILog() *zerolog.Logger {
	log := logger.GetTUILogger().With().Str("component", "taskview").Logger()
	return &log
}

// Update handles messages and updates the model state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If form is shown, handle form-specific messages
	if m.showForm {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				// Close form without saving
				m.showForm = false
				m.formTitle = ""
				m.formDesc = ""
				m.initForm()
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			}
		}

		// Update the form and check if it's complete
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			// Form is completed, extract values directly from form
			title := m.form.GetString("title")
			description := m.form.GetString("description")

			// Fallback to model fields if form values are empty (useful for tests)
			if title == "" {
				title = m.formTitle
			}
			if description == "" {
				description = m.formDesc
			}

			// Use a temporary display ID for optimistic UI
			// Real task ID will be computed by orchestrator from content hash (including current commit)
			tempDisplayID := fmt.Sprintf("pending-%d", time.Now().UnixNano())

			// Create a pending task immediately for optimistic UI
			pendingTask := &models.Task{
				ID:          tempDisplayID,
				Title:       title,
				Description: description,
				Status:      models.TaskStatusPending,
				ProjectID:   m.projectID,
			}

			// Add to tasks and mark as pending
			m.tasks[tempDisplayID] = pendingTask
			m.pendingTasks[tempDisplayID] = true
			m.refreshTaskList()

			go func() {
				cmd := protocol.CreateTaskCommand{
					Metadata: protocol.Metadata{
						// TaskID intentionally empty - orchestrator computes content-based ID
						Version: protocol.CurrentProtocolVersion,
					},
					ProjectID:   m.projectID,
					Title:       title,
					Description: description,
					// BaseCommitSHA is empty - orchestrator will get current HEAD from git service
				}
				m.cmdChan <- cmd
			}()
			getTUILog().Info().Str("title", title).Str("description", description).Msg("Task creation requested")
			// Reset form state
			m.showForm = false
			m.formTitle = ""
			m.formDesc = ""
			m.initForm()
		}

		return m, cmd
	}

	// Normal list handling when form is not shown
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Tab switching
		switch msg.String() {
		case "tab":
			// Cycle through tabs
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			// Load commits when switching to commits tab for the first time
			if m.activeTab == 1 && !m.commitsLoaded && m.repositoryPath != "" {
				go func() {
					m.cmdChan <- protocol.LoadCommitsCommand{
						ProjectID: m.projectID,
						Limit:     100,
					}
				}()
			}
			return m, nil
		case "1":
			// Switch to Tasks tab
			m.activeTab = 0
			return m, nil
		case "2":
			// Switch to Commits tab
			m.activeTab = 1
			// Load commits when switching to commits tab for the first time
			if !m.commitsLoaded && m.repositoryPath != "" {
				go func() {
					m.cmdChan <- protocol.LoadCommitsCommand{
						ProjectID: m.projectID,
						Limit:     100,
					}
				}()
			}
			return m, nil
		}

		// Tab-specific navigation
		if m.activeTab == 0 {
			// Tasks tab navigation
			switch msg.String() {
			case "enter":
				// Handle task selection - go to task details
				if selectedItem := m.list.SelectedItem(); selectedItem != nil {
					if taskItem, ok := selectedItem.(TaskItem); ok {
						// Get the full task data
						if task, exists := m.tasks[taskItem.ID]; exists {
							return m, func() tea.Msg {
								return messages.GoToTaskDetailsMsg{
									Task:      task,
									ProjectID: m.projectID,
								}
							}
						}
					}
				}
			case "n":
				// Show form to create new task
				m.showForm = true
				return m, m.form.Init()
			case "r":
				// Retry selected failed task by re-sending CreateTaskCommand with same taskID
				// CreateTaskWorkflow is idempotent and will resume from where it failed
				if selectedItem := m.list.SelectedItem(); selectedItem != nil {
					if taskItem, ok := selectedItem.(TaskItem); ok {
						// Only allow retry for failed tasks (check actual status, not in-memory map)
						if taskItem.Status == models.TaskStatusFailed {
							// Optimistic UI update: mark task as pending immediately
							if task, exists := m.tasks[taskItem.ID]; exists {
								task.Status = models.TaskStatusPending
							}
							delete(m.failedTasks, taskItem.ID)
							m.pendingTasks[taskItem.ID] = true
							if statusModel, exists := m.taskStatuses[taskItem.ID]; exists {
								statusModel = statusModel.SetStatus(models.TaskStatusPending)
								statusModel = statusModel.SetUIState(taskstatus.UIStateNormal)
								m.taskStatuses[taskItem.ID] = statusModel
							}
							m.refreshTaskList()

							go func() {
								m.cmdChan <- protocol.CreateTaskCommand{
									Metadata: protocol.Metadata{
										TaskID:  taskItem.ID, // Reuse same taskID for idempotent retry
										Version: protocol.CurrentProtocolVersion,
									},
									ProjectID:   m.projectID,
									Title:       taskItem.TaskTitle,
									Description: taskItem.Desc,
									// BaseCommitSHA is empty - orchestrator will get current HEAD from git service
								}
							}()
						}
					}
				}
			case "d":
				// Delete selected task
				if selectedItem := m.list.SelectedItem(); selectedItem != nil {
					if taskItem, ok := selectedItem.(TaskItem); ok {
						go func() {
							m.cmdChan <- protocol.DeleteTaskCommand{
								ProjectID: m.projectID,
								TaskID:    taskItem.ID,
							}
						}()
					}
				}
			case "esc", "backspace":
				// Go back to project list
				return m, func() tea.Msg {
					return messages.GoBackMsg{}
				}
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		} else if m.activeTab == 1 {
			// Commits tab navigation
			switch msg.String() {
			case "up", "k":
				// Move to previous commit
				if m.selectedCommit > 0 {
					m.selectedCommit--
				}
			case "down", "j":
				// Move to next commit
				if m.selectedCommit < len(m.commits)-1 {
					m.selectedCommit++
				}
			case "esc", "backspace":
				// Go back to project list
				return m, func() tea.Msg {
					return messages.GoBackMsg{}
				}
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		}

	case protocol.TasksLoadedEvent:
		if msg.ProjectID == m.projectID {
			// Update tasks and project details, then refresh list
			m.tasks = msg.Tasks
			m.projectName = msg.ProjectName
			m.repositoryPath = msg.RepositoryPath
			m.refreshTaskList()
		}

	case protocol.CommitsLoadedEvent:
		if msg.ProjectID == m.projectID {
			// Convert protocol commits to commitgraph commits
			m.commits = make([]*commitgraph.Commit, len(msg.Commits))
			for i, c := range msg.Commits {
				m.commits[i] = commitgraph.NewCommit(
					m.hashPool,
					c.Hash,
					c.Message,
					c.Author,
					c.Parents,
				)
			}
			m.commitsLoaded = true
			m.selectedCommit = 0

			// Build commit lane mapping for navigation
			if len(m.commits) > 0 {
				m.buildCommitLaneMapping()
				m.currentLane = m.commitLanes[0]
			}
		}

	case protocol.TaskLifecycleEvent:
		if msg.ProjectID == m.projectID {
			switch msg.Type {
			case protocol.TaskStatusUpdated:
				// Update the specific task status
				if task, exists := m.tasks[msg.TaskID]; exists {
					task.Status = msg.NewStatus
					if statusModel, exists := m.taskStatuses[msg.TaskID]; exists {
						statusModel = statusModel.SetStatus(msg.NewStatus)
						m.taskStatuses[msg.TaskID] = statusModel
					}
					m.refreshTaskList()
				}

			case protocol.TaskCreated:
				// Task creation completed - update with full task data
				if msg.Task != nil {
					// Remove any pending task with matching title (optimistic UI cleanup)
					// The pending task has a temporary ID, but the real task has a content-based ID
					for id, task := range m.tasks {
						if strings.HasPrefix(id, "pending-") && task.Title == msg.Task.Title {
							delete(m.tasks, id)
							delete(m.pendingTasks, id)
							delete(m.taskStatuses, id)
							break
						}
					}

					// Add/update the real task with content-based ID
					if task, exists := m.tasks[msg.Task.ID]; exists {
						*task = *msg.Task
					} else {
						m.tasks[msg.Task.ID] = msg.Task
					}

					delete(m.pendingTasks, msg.Task.ID)

					if statusModel, exists := m.taskStatuses[msg.Task.ID]; exists {
						statusModel = statusModel.SetStatus(msg.Task.Status)
						statusModel = statusModel.SetUIState(taskstatus.UIStateCreated)
						m.taskStatuses[msg.Task.ID] = statusModel
					}

					m.refreshTaskList()

					return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
						return clearCreatedStateMsg{taskID: msg.Task.ID}
					})
				}

			case protocol.TaskRequested:
				// Mark task as pending
				if task, exists := m.tasks[msg.TaskID]; exists {
					task.Status = models.TaskStatusPending
					if statusModel, exists := m.taskStatuses[msg.TaskID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusPending)
						m.taskStatuses[msg.TaskID] = statusModel
					}
					m.refreshTaskList()
				}

			case protocol.TaskInProgress:
				// Mark task as in progress
				if task, exists := m.tasks[msg.TaskID]; exists {
					task.Status = models.TaskStatusInProgress
					// Clear pending state - task is now actively processing
					delete(m.pendingTasks, msg.TaskID)
					if statusModel, exists := m.taskStatuses[msg.TaskID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusInProgress)
						m.taskStatuses[msg.TaskID] = statusModel
					}
					m.refreshTaskList()
				}

			case protocol.TaskFinished:
				// Mark task as completed
				if task, exists := m.tasks[msg.TaskID]; exists {
					task.Status = models.TaskStatusCompleted
					// Clear pending and failed states - task completed successfully
					delete(m.pendingTasks, msg.TaskID)
					delete(m.failedTasks, msg.TaskID)
					if statusModel, exists := m.taskStatuses[msg.TaskID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusCompleted)
						m.taskStatuses[msg.TaskID] = statusModel
					}
					m.refreshTaskList()

					// Reload tasks from database to get updated fields (GitDiff, etc.)
					go func() {
						m.cmdChan <- protocol.LoadTasksCommand{ProjectID: m.projectID}
					}()
				}

			case protocol.TaskDeleted:
				// Remove the task from our map and refresh list
				delete(m.tasks, msg.TaskID)
				delete(m.taskStatuses, msg.TaskID)
				delete(m.pendingTasks, msg.TaskID)
				delete(m.failedTasks, msg.TaskID)
				m.refreshTaskList()
			}
		}

	case protocol.ErrorEvent:
		// Check if this error is related to a specific task
		if msg.TaskID != "" {
			// Clear pending state - task has failed
			delete(m.pendingTasks, msg.TaskID)

			// Update existing task status if it exists in tasks map (during processing)
			if task, exists := m.tasks[msg.TaskID]; exists {
				task.Status = models.TaskStatusFailed
			}

			// Also check pipeline runs
			if run, exists := m.pipelineRuns[msg.TaskID]; exists {
				run.Status = models.PipelineRunStatusFailed
			}

			// Mark as failed for UI tracking
			m.failedTasks[msg.TaskID] = time.Now()

			// Update the task status component to show "failed" state
			if statusModel, exists := m.taskStatuses[msg.TaskID]; exists {
				statusModel = statusModel.SetStatus(models.TaskStatusFailed)
				statusModel = statusModel.SetUIState(taskstatus.UIStateFailed)
				m.taskStatuses[msg.TaskID] = statusModel
			}

			m.refreshTaskList()
		}

	case protocol.PipelineRunsLoadedEvent:
		if msg.ProjectID == m.projectID {
			// Update pipeline runs and project details, then refresh list
			m.pipelineRuns = msg.Runs
			m.projectName = msg.ProjectName
			m.repositoryPath = msg.RepositoryPath
			m.refreshTaskList()
		}

	case protocol.PipelineRunStartedEvent:
		if msg.ProjectID == m.projectID {
			// A new pipeline run was started (could be from task creation)
			// If it already exists (idempotent), just update; otherwise create optimistic entry
			if msg.AlreadyExists {
				// Run already exists - update status if we have it
				if run, exists := m.pipelineRuns[msg.RunID]; exists {
					switch msg.Status {
					case protocol.PipelineStatusRunning:
						run.Status = models.PipelineRunStatusRunning
					case protocol.PipelineStatusCompleted:
						run.Status = models.PipelineRunStatusCompleted
					}
				}
			} else {
				// New run started - add optimistic entry
				m.pipelineRuns[msg.RunID] = &models.PipelineRun{
					ID:        msg.RunID,
					ProjectID: msg.ProjectID,
					Name:      msg.Name,
					Status:    models.PipelineRunStatusPending,
				}
				m.pendingTasks[msg.RunID] = true
			}
			m.refreshTaskList()
		}

	case protocol.PipelineLifecycleEvent:
		if msg.ProjectID == m.projectID {
			switch msg.Type {
			case protocol.PipelineCreated:
				// Pipeline run created - update or add to our map
				if msg.Run != nil {
					// Update the run with real data (replaces optimistic entry if exists)
					// RunID is content-based, so PipelineRunStartedEvent and PipelineCreated
					// use the same ID - no prefix matching needed
					m.pipelineRuns[msg.Run.ID] = msg.Run
					delete(m.pendingTasks, msg.Run.ID)

					if statusModel, exists := m.taskStatuses[msg.Run.ID]; exists {
						statusModel = statusModel.SetStatus(pipelineRunStatusToTaskStatus(msg.Run.Status))
						statusModel = statusModel.SetUIState(taskstatus.UIStateCreated)
						m.taskStatuses[msg.Run.ID] = statusModel
					}

					m.refreshTaskList()

					return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
						return clearCreatedStateMsg{taskID: msg.Run.ID}
					})
				}

			case protocol.PipelineStepStarted:
				// Mark run as in progress
				if run, exists := m.pipelineRuns[msg.RunID]; exists {
					run.Status = models.PipelineRunStatusRunning
					delete(m.pendingTasks, msg.RunID)
					if statusModel, exists := m.taskStatuses[msg.RunID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusInProgress)
						m.taskStatuses[msg.RunID] = statusModel
					}
					m.refreshTaskList()
				}

			case protocol.PipelineStepCompleted:
				// Step completed - run might still be in progress if multi-step
				// Just update the run data if provided
				if run, exists := m.pipelineRuns[msg.RunID]; exists {
					if statusModel, exists := m.taskStatuses[msg.RunID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusInProgress)
						m.taskStatuses[msg.RunID] = statusModel
					}
					// Keep as running until pipeline finishes
					run.Status = models.PipelineRunStatusRunning
					m.refreshTaskList()
				}

			case protocol.PipelineStepFailed:
				// Step failed - mark run as failed
				if run, exists := m.pipelineRuns[msg.RunID]; exists {
					run.Status = models.PipelineRunStatusFailed
					delete(m.pendingTasks, msg.RunID)
					m.failedTasks[msg.RunID] = time.Now()
					if statusModel, exists := m.taskStatuses[msg.RunID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusFailed)
						statusModel = statusModel.SetUIState(taskstatus.UIStateFailed)
						m.taskStatuses[msg.RunID] = statusModel
					}
					m.refreshTaskList()
				}

			case protocol.PipelineFinished:
				// Pipeline completed successfully
				if run, exists := m.pipelineRuns[msg.RunID]; exists {
					run.Status = models.PipelineRunStatusCompleted
					delete(m.pendingTasks, msg.RunID)
					delete(m.failedTasks, msg.RunID)
					if statusModel, exists := m.taskStatuses[msg.RunID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusCompleted)
						m.taskStatuses[msg.RunID] = statusModel
					}
					m.refreshTaskList()

					// Reload pipeline runs from database to get updated fields
					go func() {
						m.cmdChan <- protocol.LoadPipelineRunsCommand{ProjectID: m.projectID}
					}()
				}

			case protocol.PipelineFailed:
				// Pipeline failed
				if run, exists := m.pipelineRuns[msg.RunID]; exists {
					run.Status = models.PipelineRunStatusFailed
					delete(m.pendingTasks, msg.RunID)
					m.failedTasks[msg.RunID] = time.Now()
					if statusModel, exists := m.taskStatuses[msg.RunID]; exists {
						statusModel = statusModel.SetStatus(models.TaskStatusFailed)
						statusModel = statusModel.SetUIState(taskstatus.UIStateFailed)
						m.taskStatuses[msg.RunID] = statusModel
					}
					m.refreshTaskList()
				}
			}
		}

	case clearCreatedStateMsg:
		// Clear the created state for a task
		if statusModel, exists := m.taskStatuses[msg.taskID]; exists {
			statusModel = statusModel.SetUIState(taskstatus.UIStateNormal)
			m.taskStatuses[msg.taskID] = statusModel
		}
		m.refreshTaskList()

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	// Update task status components for spinner animations
	var statusCmds []tea.Cmd
	for taskID, statusModel := range m.taskStatuses {
		updatedModel, cmd := statusModel.Update(msg)
		m.taskStatuses[taskID] = updatedModel
		if cmd != nil {
			statusCmds = append(statusCmds, cmd)
		}
	}

	// Update the list component only if form is not shown and we're on the tasks tab
	if !m.showForm && m.activeTab == 0 {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if cmd != nil {
			statusCmds = append(statusCmds, cmd)
		}
	}

	// Combine all commands
	if len(statusCmds) > 0 {
		return m, tea.Batch(statusCmds...)
	}

	return m, nil
}

// Custom message types for internal state management
type clearCreatedStateMsg struct {
	taskID string
}

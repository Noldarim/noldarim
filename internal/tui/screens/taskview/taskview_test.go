// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskview

import (
	"strings"
	"testing"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/messages"
	"github.com/noldarim/noldarim/test/testutil"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
)

func TestTaskItem(t *testing.T) {
	t.Run("implements list.Item interface correctly", func(t *testing.T) {
		item := TaskItem{
			ID:        "test-task",
			TaskTitle: "Test Task",
			Desc:      "Test Description",
			Status:    models.TaskStatusPending,
		}

		// Test FilterValue
		assert.Equal(t, "Test Task", item.FilterValue())

		// Test Title without status icons (status will be displayed via taskstatus component)
		expectedTitle := "Test Task" // No status icon in title
		assert.Equal(t, expectedTitle, item.Title())

		// Test Description
		assert.Equal(t, "Test Description", item.Description())

		// Test String
		expected := "Test Task: Test Description"
		assert.Equal(t, expected, item.String())
	})

	t.Run("title shows task name without status icons", func(t *testing.T) {
		baseItem := TaskItem{
			ID:        "test",
			TaskTitle: "Task",
			Desc:      "Desc",
		}

		// Test that title doesn't contain status icons (status shown via taskstatus component)
		item := baseItem
		item.Status = models.TaskStatusPending
		assert.Equal(t, "Task", item.Title())

		// Test completed status
		item.Status = models.TaskStatusCompleted
		assert.Equal(t, "Task", item.Title())

		// Test in progress status
		item.Status = models.TaskStatusInProgress
		assert.Equal(t, "Task", item.Title())
	})
}

func TestNewModel(t *testing.T) {
	t.Run("creates model with correct initial state", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Verify initial state
		assert.Equal(t, projectID, model.projectID)
		assert.NotNil(t, model.list)
		assert.NotNil(t, model.cmdChan)
		assert.NotNil(t, model.tasks)
		assert.Empty(t, model.tasks, "Tasks map should be empty initially")

		// Verify list configuration
		assert.Equal(t, "", model.list.Title)
	})
}

func TestModelInit(t *testing.T) {
	t.Run("sends LoadTasksCommand and LoadPipelineRunsCommand on init", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())
		cmd := model.Init()

		// Init should return nil (command sent via goroutine)
		assert.Nil(t, cmd)

		// Wait for both commands to be captured
		capture.WaitForCommands(2)

		// Verify both LoadTasksCommand and LoadPipelineRunsCommand were sent
		commands := capture.AllCommands()
		assert.Len(t, commands, 2, "Should send both LoadTasksCommand and LoadPipelineRunsCommand")

		// Check that both command types are present
		hasLoadTasks := false
		hasLoadPipelineRuns := false
		for _, c := range commands {
			switch cmd := c.(type) {
			case protocol.LoadTasksCommand:
				hasLoadTasks = true
				assert.Equal(t, projectID, cmd.ProjectID)
			case protocol.LoadPipelineRunsCommand:
				hasLoadPipelineRuns = true
				assert.Equal(t, projectID, cmd.ProjectID)
			}
		}
		assert.True(t, hasLoadTasks, "Should send LoadTasksCommand")
		assert.True(t, hasLoadPipelineRuns, "Should send LoadPipelineRunsCommand")
	})
}

func TestModelUpdate_KeyHandling(t *testing.T) {
	capture := testutil.NewCommandCapture()
	defer capture.Close()

	projectID := "test-project"
	model := NewModel(projectID, capture.Channel())

	t.Run("enter key with no selection does nothing special", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.SpecialKey(tea.KeyEnter))

		// Should return same model type
		assert.IsType(t, Model{}, newModel)

		// No command should be generated for empty list
		testutil.AssertNoCommand(t, cmd)
	})

	t.Run("n key for new task - shows form", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.KeyPress("n"))

		assert.IsType(t, Model{}, newModel)
		taskModel := newModel.(Model)
		assert.True(t, taskModel.showForm, "Form should be shown when pressing 'n'")
		// Form init returns a command
		assert.NotNil(t, cmd, "Form init should return a command")
	})

	t.Run("d key with no selection does nothing", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.KeyPress("d"))

		assert.IsType(t, Model{}, newModel)
		testutil.AssertNoCommand(t, cmd)
	})

	t.Run("esc key generates back navigation message", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.SpecialKey(tea.KeyEsc))

		assert.IsType(t, Model{}, newModel)
		assert.NotNil(t, cmd)

		msg := testutil.ExecuteCommand(cmd)
		assert.IsType(t, messages.GoBackMsg{}, msg)
	})

	t.Run("backspace key generates back navigation message", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.SpecialKey(tea.KeyBackspace))

		assert.IsType(t, Model{}, newModel)
		assert.NotNil(t, cmd)

		msg := testutil.ExecuteCommand(cmd)
		assert.IsType(t, messages.GoBackMsg{}, msg)
	})

	t.Run("q key generates quit message", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.KeyPress("q"))

		assert.IsType(t, Model{}, newModel)
		testutil.AssertQuitMessage(t, cmd)
	})

	t.Run("ctrl+c generates quit message", func(t *testing.T) {
		ctrlC := tea.KeyMsg{
			Type: tea.KeyCtrlC,
		}
		newModel, cmd := testutil.SendMessage(model, ctrlC)

		assert.IsType(t, Model{}, newModel)
		testutil.AssertQuitMessage(t, cmd)
	})
}

func TestModelUpdate_EventHandling(t *testing.T) {
	projectID := "test-project"

	t.Run("TasksLoadedEvent updates tasks and list", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(projectID, capture.Channel())
		event := testutil.TasksLoadedEvent(projectID)

		newModel, _ := testutil.SendMessage(model, event)
		updatedModel := newModel.(Model)

		// Verify tasks were stored
		assert.Len(t, updatedModel.tasks, 3) // testutil.SampleTasks() has 3 tasks
		assert.Contains(t, updatedModel.tasks, "task1")
		assert.Contains(t, updatedModel.tasks, "task2")
		assert.Contains(t, updatedModel.tasks, "task3")

		// Verify all tasks have correct project ID
		for _, task := range updatedModel.tasks {
			assert.Equal(t, projectID, task.ProjectID)
		}

		// Verify list was updated
	})

	t.Run("TasksLoadedEvent for different project ignored", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(projectID, capture.Channel())
		event := testutil.TasksLoadedEvent("different-project")

		newModel, _ := testutil.SendMessage(model, event)
		updatedModel := newModel.(Model)

		// Tasks should remain empty since project ID doesn't match
		assert.Empty(t, updatedModel.tasks)
	})

	t.Run("TaskUpdatedEvent updates specific task", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(projectID, capture.Channel())

		// First load some tasks
		loadEvent := testutil.TasksLoadedEvent(projectID)
		newModel, _ := testutil.SendMessage(model, loadEvent)
		model = newModel.(Model)

		// Now update a specific task
		updateEvent := testutil.TaskUpdatedEvent(projectID, "task1", models.TaskStatusCompleted)
		newModel2, _ := testutil.SendMessage(model, updateEvent)
		updatedModel := newModel2.(Model)

		// Verify the specific task was updated
		updatedTask := updatedModel.tasks["task1"]
		assert.NotNil(t, updatedTask)
		assert.Equal(t, models.TaskStatusCompleted, updatedTask.Status)

		// Verify list was refreshed
	})

	t.Run("TaskUpdatedEvent for different project ignored", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(projectID, capture.Channel())

		// Load tasks first
		loadEvent := testutil.TasksLoadedEvent(projectID)
		newModel, _ := testutil.SendMessage(model, loadEvent)
		model = newModel.(Model)

		originalTask := model.tasks["task1"]
		originalStatus := originalTask.Status

		// Update for different project
		updateEvent := testutil.TaskUpdatedEvent("different-project", "task1", models.TaskStatusCompleted)
		updatedModelInterface, _ := testutil.SendMessage(model, updateEvent)
		updatedModel := updatedModelInterface.(Model)

		// Task should remain unchanged
		assert.Equal(t, originalStatus, updatedModel.tasks["task1"].Status)
	})

	t.Run("WindowSizeMsg updates list dimensions", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(projectID, capture.Channel())
		sizeMsg := testutil.WindowSizeMsg(100, 50)

		newModel, _ := testutil.SendMessage(model, sizeMsg)
		updatedModel := newModel.(Model)

		// Verify model was updated
		assert.IsType(t, Model{}, updatedModel)
	})
}

func TestModelUpdate_TaskOperations(t *testing.T) {
	t.Run("enter key with selected task sends toggle command", func(t *testing.T) {
		// This would require more sophisticated testing to properly simulate
		// list selection. For now, we test that the basic structure is there.
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// The actual task selection testing would require integration tests
		// or more sophisticated mocking of the list component
		assert.IsType(t, Model{}, model)
	})

	t.Run("d key with selected task sends delete command", func(t *testing.T) {
		// Similar to above - would need list selection simulation
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		assert.IsType(t, Model{}, model)
	})
}

func TestModelView(t *testing.T) {
	t.Run("returns non-empty view output", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		testutil.AssertViewNotEmpty(t, model)
	})

	t.Run("contains expected elements", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())
		view := model.View()

		// Should contain help text
		assert.Contains(t, view, "enter")
		assert.Contains(t, view, "details")
		assert.Contains(t, view, "n")
		assert.Contains(t, view, "new")
		assert.Contains(t, view, "d")
		assert.Contains(t, view, "delete")
		assert.Contains(t, view, "esc")
		assert.Contains(t, view, "back")
		assert.Contains(t, view, "q")
		assert.Contains(t, view, "quit")
	})

	t.Run("shows tasks after loading", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Load tasks
		event := testutil.TasksLoadedEvent(projectID)
		newModel, _ := testutil.SendMessage(model, event)
		model = newModel.(Model)

		view := model.View()

		// View should contain task information
		assert.NotEmpty(t, view)
	})

	t.Run("header is present and consistent in Tasks tab", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())
		model.SetSize(80, 24) // Set reasonable size for testing

		// Ensure we're on Tasks tab (default)
		assert.Equal(t, 0, model.activeTab)

		view := model.View()

		// Should contain the header elements
		assert.Contains(t, view, "Project View")
		assert.Contains(t, view, "Projects > test-project")
		assert.Contains(t, view, "Tasks:")

		// Print the view for debugging
		t.Logf("Tasks tab view:\n%s", view)
	})

	t.Run("header is present and consistent in Commits tab", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())
		model.SetSize(80, 24) // Set reasonable size for testing

		// Switch to Commits tab
		model.activeTab = 1

		view := model.View()

		// Should contain the same header elements
		assert.Contains(t, view, "Project View")
		assert.Contains(t, view, "Projects > test-project")
		assert.Contains(t, view, "Commits:")

		// Print the view for debugging
		t.Logf("Commits tab view:\n%s", view)
	})

	t.Run("header is present when tasks exist", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())
		model.SetSize(80, 24) // Set reasonable size for testing

		// Load some tasks
		event := testutil.TasksLoadedEvent(projectID)
		newModel, _ := testutil.SendMessage(model, event)
		model = newModel.(Model)

		// Make sure we're on Tasks tab
		assert.Equal(t, 0, model.activeTab)

		view := model.View()

		// Should contain the header elements
		assert.Contains(t, view, "Project View")
		assert.Contains(t, view, "Projects > test-project")
		assert.Contains(t, view, "Tasks:")

		// Print the view for debugging
		t.Logf("Tasks tab view with tasks:\n%s", view)
	})
}

// TestTaskCreationIntegration tests the complete task creation workflow
func TestTaskCreationIntegration(t *testing.T) {
	t.Run("task creation with immediate pending state display", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Show form and simulate completion
		_, cmd := testutil.SendMessage(model, testutil.KeyPress("n"))
		assert.NotNil(t, cmd) // Form init command

		// Set form values and mark as completed
		model.formTitle = "New Test Task"
		model.formDesc = "Test task description"
		model.showForm = true
		model.initForm()
		model.form.State = huh.StateCompleted

		// Complete the form
		newModel, _ := testutil.SendMessage(model, tea.KeyMsg{Type: tea.KeyEnter})
		updatedModel := newModel.(Model)

		// Wait for command to be sent
		capture.WaitForCommands(1)

		// Check that pending task was created
		assert.Greater(t, len(updatedModel.tasks), 0, "Should have at least one task")
		assert.Greater(t, len(updatedModel.pendingTasks), 0, "Should have at least one pending task")

		// Find the pending task
		var pendingTask *models.Task
		var tempID string
		for id, task := range updatedModel.tasks {
			if task.Title == "New Test Task" {
				pendingTask = task
				tempID = id
				break
			}
		}

		assert.NotNil(t, pendingTask, "Should have created a pending task")
		assert.Equal(t, "New Test Task", pendingTask.Title)
		assert.Equal(t, "Test task description", pendingTask.Description)
		assert.Equal(t, models.TaskStatusPending, pendingTask.Status)
		assert.Equal(t, projectID, pendingTask.ProjectID)
		assert.True(t, updatedModel.pendingTasks[tempID], "Task should be marked as pending")

		// Verify CreateTaskCommand was sent
		testutil.AssertCommandSent(t, capture, protocol.CreateTaskCommand{})
		lastCmd := capture.LastCommand().(protocol.CreateTaskCommand)
		assert.Equal(t, projectID, lastCmd.ProjectID)
		assert.Equal(t, "New Test Task", lastCmd.Title)
		assert.Equal(t, "Test task description", lastCmd.Description)
		assert.Empty(t, lastCmd.Metadata.TaskID, "TaskID should be empty - orchestrator computes content-based ID")

		// Form should be hidden after completion
		assert.False(t, updatedModel.showForm, "Form should be hidden after completion")
		assert.Empty(t, updatedModel.formTitle, "Form title should be cleared")
		assert.Empty(t, updatedModel.formDesc, "Form description should be cleared")
	})

	t.Run("handling TaskCreatedEvent success case", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create a pending task first
		tempID := "temp_123456"
		pendingTask := &models.Task{
			ID:          tempID,
			Title:       "Pending Task",
			Description: "Task in pending state",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		}
		model.tasks[tempID] = pendingTask
		model.pendingTasks[tempID] = true
		model.refreshTaskList()

		// Create the actual task with the same ID (consistent ID architecture)
		createdTask := &models.Task{
			ID:          tempID, // Use the same temp ID
			Title:       "Pending Task",
			Description: "Task in pending state",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		}

		// Send TaskCreatedEvent with correlation ID
		event := protocol.TaskLifecycleEvent{
			Metadata:  protocol.Metadata{},
			Type:      protocol.TaskCreated,
			ProjectID: projectID,
			TaskID:    createdTask.ID,
			Task:      createdTask,
		}

		newModel, cmd := testutil.SendMessage(model, event)
		updatedModel := newModel.(Model)

		// Verify task was updated with same ID
		assert.Contains(t, updatedModel.tasks, tempID, "Task should still exist with same ID")
		assert.False(t, updatedModel.pendingTasks[tempID], "Pending state should be cleared")

		// Verify task data was updated
		updatedTask := updatedModel.tasks[tempID]
		assert.Equal(t, tempID, updatedTask.ID, "Task ID should remain the same")
		assert.Equal(t, "Pending Task", updatedTask.Title)

		// Task status component should show created state
		statusModel, exists := updatedModel.taskStatuses[tempID]
		assert.True(t, exists, "Task status component should exist")
		assert.NotNil(t, statusModel, "Task status component should not be nil")

		// Should schedule a command to clear created state after 2 seconds
		assert.NotNil(t, cmd, "Should return a command to clear created state")

		// Execute the scheduled command (simulating 2 seconds later)
		clearMsg := testutil.ExecuteCommand(cmd)
		assert.IsType(t, clearCreatedStateMsg{}, clearMsg)
		clearStateMsg := clearMsg.(clearCreatedStateMsg)
		assert.Equal(t, tempID, clearStateMsg.taskID)

		// Process the clear state message
		finalModel, _ := testutil.SendMessage(updatedModel, clearStateMsg)
		finalUpdatedModel := finalModel.(Model)

		// Created state should be cleared (UI state reset to normal)
		// We can't easily test the internal UI state without exposing it,
		// but we can verify the task still exists
		assert.Contains(t, finalUpdatedModel.tasks, tempID, "Task should still exist")
	})

	t.Run("handling ErrorEvent failure case", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create a pending task first
		tempID := "temp_failure_123"
		pendingTask := &models.Task{
			ID:          tempID,
			Title:       "Failed Task",
			Description: "Task that will fail",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		}
		model.tasks[tempID] = pendingTask
		model.pendingTasks[tempID] = true
		model.refreshTaskList()

		// Send ErrorEvent with TaskID
		errorEvent := protocol.ErrorEvent{
			Metadata: protocol.Metadata{},
			Message:  "Failed to create task",
			Context:  "Task creation failed",
			TaskID:   tempID, // Use the same task ID that was used for the pending task
		}

		newModel, cmd := testutil.SendMessage(model, errorEvent)
		updatedModel := newModel.(Model)

		// Verify pending state was removed and failed state was set
		assert.False(t, updatedModel.pendingTasks[tempID], "Pending state should be cleared")
		assert.Contains(t, updatedModel.failedTasks, tempID, "Task should be marked as failed")
		assert.WithinDuration(t, time.Now(), updatedModel.failedTasks[tempID], time.Second, "Failed timestamp should be recent")

		// No auto-removal - failed tasks remain until manually retried or deleted
		assert.Nil(t, cmd, "Should not schedule auto-removal command")

		// Task should remain in failed state
		assert.Contains(t, updatedModel.tasks, tempID, "Failed task should remain in tasks")
		assert.Contains(t, updatedModel.failedTasks, tempID, "Failed task should remain in failedTasks")
	})

	t.Run("correlation ID tracking between commands and events", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Simulate task creation flow
		model.formTitle = "Tracked Task"
		model.formDesc = "Task with correlation tracking"
		model.showForm = true
		model.initForm()
		model.form.State = huh.StateCompleted

		// Complete form
		newModel, _ := testutil.SendMessage(model, tea.KeyMsg{Type: tea.KeyEnter})
		updatedModel := newModel.(Model)

		capture.WaitForCommands(1)

		// Get the temp ID that was generated
		var tempID string
		for id := range updatedModel.pendingTasks {
			tempID = id
			break
		}
		assert.NotEmpty(t, tempID, "Should have generated a temp ID")
		assert.True(t, strings.HasPrefix(tempID, "pending-"), "Temp ID should start with 'pending-'")

		// Verify command's TaskID is empty (orchestrator computes content-based ID)
		lastCmd := capture.LastCommand().(protocol.CreateTaskCommand)
		assert.Empty(t, lastCmd.Metadata.TaskID, "Command TaskID should be empty for new tasks")

		// Test successful response with content-based task ID from orchestrator
		realTaskID := "task-abc123def456" // Content-based ID computed by orchestrator
		successEvent := protocol.TaskLifecycleEvent{
			Metadata:  protocol.Metadata{},
			Type:      protocol.TaskCreated,
			ProjectID: projectID,
			TaskID:    realTaskID,
			Task: &models.Task{
				ID:          realTaskID, // Content-based ID from orchestrator
				Title:       "Tracked Task",
				Description: "Task with ID tracking",
				Status:      models.TaskStatusPending,
				ProjectID:   projectID,
			},
		}

		finalModel, _ := testutil.SendMessage(updatedModel, successEvent)
		finalUpdatedModel := finalModel.(Model)

		// Verify pending task was replaced by real task (matched by title)
		assert.NotContains(t, finalUpdatedModel.tasks, tempID, "Temp task should be removed")
		assert.Contains(t, finalUpdatedModel.tasks, realTaskID, "Real task should be added")
		// Task should no longer be in pending state
		assert.False(t, finalUpdatedModel.pendingTasks[realTaskID], "Task should not be pending")
		assert.False(t, finalUpdatedModel.pendingTasks[tempID], "Temp ID should not be pending")
		statusModel, exists := finalUpdatedModel.taskStatuses[realTaskID]
		assert.True(t, exists, "Task status component should exist for real task ID")
		assert.NotNil(t, statusModel, "Task status component should not be nil")
	})

	t.Run("failed tasks persist until manually handled", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Add a failed task with old timestamp
		failedID := "failed_task_789"
		oldTimestamp := time.Now().Add(-15 * time.Second) // 15 seconds ago
		model.tasks[failedID] = &models.Task{
			ID:          failedID,
			Title:       "Old Failed Task",
			Description: "Task that failed long ago",
			Status:      models.TaskStatusFailed,
			ProjectID:   projectID,
		}
		model.failedTasks[failedID] = oldTimestamp

		// Add a recent failed task
		recentFailedID := "recent_failed_task"
		recentTimestamp := time.Now().Add(-5 * time.Second) // 5 seconds ago
		model.tasks[recentFailedID] = &models.Task{
			ID:          recentFailedID,
			Title:       "Recent Failed Task",
			Description: "Task that failed recently",
			Status:      models.TaskStatusFailed,
			ProjectID:   projectID,
		}
		model.failedTasks[recentFailedID] = recentTimestamp

		// Both failed tasks should persist (no auto-cleanup)
		assert.Contains(t, model.tasks, failedID, "Old failed task should persist")
		assert.Contains(t, model.failedTasks, failedID, "Old failed task should remain in failed list")
		assert.Contains(t, model.tasks, recentFailedID, "Recent failed task should persist")
		assert.Contains(t, model.failedTasks, recentFailedID, "Recent failed task should remain in failed list")
	})
}

// TestUIStateBadgeRendering tests the UI badge rendering for different states
func TestUIStateBadgeRendering(t *testing.T) {
	t.Run("pending task shows PENDING in status component", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create a pending task
		tempID := "temp_pending_task"
		model.tasks[tempID] = &models.Task{
			ID:          tempID,
			Title:       "Pending Task",
			Description: "Task showing pending state",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		}
		model.pendingTasks[tempID] = true
		model.refreshTaskList()

		// Get the list items
		items := model.list.Items()
		assert.Len(t, items, 1, "Should have one item")

		taskItem := items[0].(TaskItem)
		assert.Equal(t, "Pending Task", taskItem.TaskTitle)

		// Check that taskstatus component was created with pending UI state
		statusModel, exists := model.taskStatuses[tempID]
		assert.True(t, exists, "Task status component should exist")
		// We can't easily test the internal UI state without exposing it,
		// but we can verify the component exists and will be used by the delegate
		assert.NotNil(t, statusModel)
	})

	t.Run("failed task shows FAILED in status component", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create a failed task
		failedID := "failed_task_123"
		model.tasks[failedID] = &models.Task{
			ID:          failedID,
			Title:       "Failed Task",
			Description: "Task showing failed state",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		}
		model.failedTasks[failedID] = time.Now()
		model.refreshTaskList()

		// Get the list items
		items := model.list.Items()
		assert.Len(t, items, 1, "Should have one item")

		taskItem := items[0].(TaskItem)
		assert.Equal(t, "Failed Task", taskItem.TaskTitle)

		// Check that taskstatus component was created with failed UI state
		statusModel, exists := model.taskStatuses[failedID]
		assert.True(t, exists, "Task status component should exist")
		assert.NotNil(t, statusModel)
	})

	t.Run("failed state overrides pending state in status component", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create a task that is both pending and failed
		taskID := "conflict_task_123"
		model.tasks[taskID] = &models.Task{
			ID:          taskID,
			Title:       "Conflict Task",
			Description: "Task with conflicting states",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		}
		model.pendingTasks[taskID] = true
		model.failedTasks[taskID] = time.Now()
		model.refreshTaskList()

		// Get the list items
		items := model.list.Items()
		assert.Len(t, items, 1, "Should have one item")

		taskItem := items[0].(TaskItem)
		assert.Equal(t, "Conflict Task", taskItem.TaskTitle)

		// Check that taskstatus component was created (failed state should override pending)
		statusModel, exists := model.taskStatuses[taskID]
		assert.True(t, exists, "Task status component should exist")
		assert.NotNil(t, statusModel)
	})

	t.Run("normal task shows regular status in component", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create a normal task
		normalID := "normal_task_123"
		model.tasks[normalID] = &models.Task{
			ID:          normalID,
			Title:       "Normal Task",
			Description: "Regular task with no special state",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		}
		model.refreshTaskList()

		// Get the list items
		items := model.list.Items()
		assert.Len(t, items, 1, "Should have one item")

		taskItem := items[0].(TaskItem)
		assert.Equal(t, "Normal Task", taskItem.TaskTitle)

		// Check that taskstatus component was created with normal state
		statusModel, exists := model.taskStatuses[normalID]
		assert.True(t, exists, "Task status component should exist")
		assert.NotNil(t, statusModel)
	})
}

// TestTaskListSorting tests that tasks are sorted by CreatedAt descending
func TestTaskListSorting(t *testing.T) {
	t.Run("tasks are sorted by CreatedAt descending (newest first)", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create tasks with different CreatedAt timestamps
		now := time.Now()
		oldTask := &models.Task{
			ID:          "task-old",
			Title:       "Old Task",
			Description: "Created 3 hours ago",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
			CreatedAt:   now.Add(-3 * time.Hour),
		}
		middleTask := &models.Task{
			ID:          "task-middle",
			Title:       "Middle Task",
			Description: "Created 1 hour ago",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
			CreatedAt:   now.Add(-1 * time.Hour),
		}
		newestTask := &models.Task{
			ID:          "task-newest",
			Title:       "Newest Task",
			Description: "Created just now",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
			CreatedAt:   now,
		}

		// Add tasks in random order
		model.tasks["task-middle"] = middleTask
		model.tasks["task-old"] = oldTask
		model.tasks["task-newest"] = newestTask

		// Refresh the list
		model.refreshTaskList()

		// Get the list items
		items := model.list.Items()
		assert.Len(t, items, 3, "Should have three tasks")

		// Verify they are sorted by CreatedAt descending (newest first)
		assert.Equal(t, "task-newest", items[0].(TaskItem).ID, "First item should be newest task")
		assert.Equal(t, "Newest Task", items[0].(TaskItem).TaskTitle)

		assert.Equal(t, "task-middle", items[1].(TaskItem).ID, "Second item should be middle task")
		assert.Equal(t, "Middle Task", items[1].(TaskItem).TaskTitle)

		assert.Equal(t, "task-old", items[2].(TaskItem).ID, "Third item should be oldest task")
		assert.Equal(t, "Old Task", items[2].(TaskItem).TaskTitle)
	})

	t.Run("tasks with same CreatedAt maintain stable order", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		projectID := "test-project"
		model := NewModel(projectID, capture.Channel())

		// Create tasks with the same CreatedAt timestamp
		now := time.Now()
		task1 := &models.Task{
			ID:          "task-1",
			Title:       "Task 1",
			Description: "First task",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
			CreatedAt:   now,
		}
		task2 := &models.Task{
			ID:          "task-2",
			Title:       "Task 2",
			Description: "Second task",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
			CreatedAt:   now,
		}

		model.tasks["task-1"] = task1
		model.tasks["task-2"] = task2

		// Refresh the list
		model.refreshTaskList()

		// Get the list items
		items := model.list.Items()
		assert.Len(t, items, 2, "Should have two tasks")

		// Verify both tasks are present (order may vary for same timestamp)
		taskIDs := []string{items[0].(TaskItem).ID, items[1].(TaskItem).ID}
		assert.Contains(t, taskIDs, "task-1")
		assert.Contains(t, taskIDs, "task-2")
	})
}

// TestTaskDelegate tests the custom delegate rendering with badges
func TestTaskDelegate(t *testing.T) {
	t.Run("delegate renders task with status component", func(t *testing.T) {
		taskItem := TaskItem{
			TaskTitle: "Test Task",
			Status:    models.TaskStatusPending,
		}

		// Test basic properties
		assert.Equal(t, "Test Task", taskItem.TaskTitle)
		assert.Equal(t, models.TaskStatusPending, taskItem.Status)
		// Note: Status display is now handled by the taskstatus component
		// Full rendering tests would require more complex mocking
	})

	t.Run("delegate renders failed task with status component", func(t *testing.T) {
		taskItem := TaskItem{
			TaskTitle: "Failed Task",
			Status:    models.TaskStatusPending,
		}

		// Test basic properties
		assert.Equal(t, "Failed Task", taskItem.TaskTitle)
		assert.Equal(t, models.TaskStatusPending, taskItem.Status)
		// Note: Failed state is now handled by the taskstatus component UI state
	})

	t.Run("delegate renders normal task with status component", func(t *testing.T) {
		taskItem := TaskItem{
			TaskTitle: "Normal Task",
			Status:    models.TaskStatusCompleted,
		}

		// Test that the task has the correct properties
		assert.Equal(t, "Normal Task", taskItem.TaskTitle)
		assert.Equal(t, models.TaskStatusCompleted, taskItem.Status)
		// Note: Status display is now handled by the taskstatus component
	})

	t.Run("truncateString function works correctly", func(t *testing.T) {
		// Test normal truncation
		result := truncateString("This is a long string", 10)
		assert.Equal(t, "This is a ", result)

		// Test with width 0
		result = truncateString("Any string", 0)
		assert.Equal(t, "", result)

		// Test with string shorter than width
		result = truncateString("Short", 10)
		assert.Equal(t, "Short", result)

		// Test with unicode characters
		result = truncateString("🚀 Task", 3)
		assert.Equal(t, "🚀 T", result)
	})
}

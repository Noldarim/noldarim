// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package testutil

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
)

// Sample data creators for consistent testing

// SampleProjects returns a map of sample projects for testing
func SampleProjects() map[string]*models.Project {
	return map[string]*models.Project{
		"proj1": {
			ID:          "proj1",
			Name:        "Test Project 1",
			Description: "First test project",
		},
		"proj2": {
			ID:          "proj2",
			Name:        "Test Project 2",
			Description: "Second test project",
		},
		"proj3": {
			ID:          "proj3",
			Name:        "Empty Project",
			Description: "Project with no tasks",
		},
	}
}

// SampleTasks returns a map of sample tasks for a given project
func SampleTasks(projectID string) map[string]*models.Task {
	return map[string]*models.Task{
		"task1": {
			ID:          "task1",
			Title:       "First Task",
			Description: "Description of first task",
			Status:      models.TaskStatusPending,
			ProjectID:   projectID,
		},
		"task2": {
			ID:          "task2",
			Title:       "Second Task",
			Description: "Description of second task",
			Status:      models.TaskStatusCompleted,
			ProjectID:   projectID,
		},
		"task3": {
			ID:          "task3",
			Title:       "In Progress Task",
			Description: "Task currently being worked on",
			Status:      models.TaskStatusInProgress,
			ProjectID:   projectID,
		},
	}
}

// ProjectsLoadedEvent creates a sample ProjectsLoadedEvent
func ProjectsLoadedEvent() protocol.ProjectsLoadedEvent {
	return protocol.ProjectsLoadedEvent{
		Projects: SampleProjects(),
	}
}

// TasksLoadedEvent creates a sample TasksLoadedEvent for a given project
func TasksLoadedEvent(projectID string) protocol.TasksLoadedEvent {
	return protocol.TasksLoadedEvent{
		ProjectID: projectID,
		Tasks:     SampleTasks(projectID),
	}
}

// TaskUpdatedEvent creates a sample TaskLifecycleEvent with TaskStatusUpdated type
func TaskUpdatedEvent(projectID, taskID string, newStatus models.TaskStatus) protocol.TaskLifecycleEvent {
	return protocol.NewTaskStatusUpdatedEvent(projectID, taskID, newStatus)
}

// SingleProject returns a single project for simpler tests
func SingleProject() *models.Project {
	return &models.Project{
		ID:          "single",
		Name:        "Single Project",
		Description: "A single project for testing",
	}
}

// SingleTask returns a single task for simpler tests
func SingleTask(projectID string) *models.Task {
	return &models.Task{
		ID:          "single-task",
		Title:       "Single Task",
		Description: "A single task for testing",
		Status:      models.TaskStatusPending,
		ProjectID:   projectID,
	}
}

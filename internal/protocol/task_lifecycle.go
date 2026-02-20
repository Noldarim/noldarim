// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// TaskLifecycleType defines the type of task lifecycle event
type TaskLifecycleType string

const (
	// TaskRequested - task has been requested for processing
	TaskRequested TaskLifecycleType = "requested"
	// TaskInProgress - task processing has started
	TaskInProgress TaskLifecycleType = "in_progress"
	// TaskFinished - task processing completed successfully
	TaskFinished TaskLifecycleType = "finished"
	// TaskDeleted - task has been deleted
	TaskDeleted TaskLifecycleType = "deleted"
	// TaskCreated - task has been created (includes full task data)
	TaskCreated TaskLifecycleType = "created"
	// TaskStatusUpdated - task status changed (generic status update)
	TaskStatusUpdated TaskLifecycleType = "status_updated"
)

// TaskLifecycleEvent represents any task lifecycle state change.
// This unified event replaces: TaskRequestedEvent, TaskInProgressEvent,
// TaskFinishedEvent, TaskDeletedEvent, TaskCreationCompletedEvent, TaskStatusUpdatedEvent
type TaskLifecycleEvent struct {
	Metadata
	Type      TaskLifecycleType
	ProjectID string
	TaskID    string
	// Task is populated for TaskCreated and TaskStatusUpdated events
	Task *models.Task
	// NewStatus is populated for TaskStatusUpdated events
	NewStatus models.TaskStatus
}

func (e TaskLifecycleEvent) GetMetadata() Metadata {
	return e.Metadata
}

// Helper constructors for common lifecycle events

// NewTaskRequestedEvent creates a TaskRequested lifecycle event
func NewTaskRequestedEvent(projectID, taskID string) TaskLifecycleEvent {
	return TaskLifecycleEvent{
		Type:      TaskRequested,
		ProjectID: projectID,
		TaskID:    taskID,
	}
}

// NewTaskInProgressEvent creates a TaskInProgress lifecycle event
func NewTaskInProgressEvent(projectID, taskID string) TaskLifecycleEvent {
	return TaskLifecycleEvent{
		Type:      TaskInProgress,
		ProjectID: projectID,
		TaskID:    taskID,
	}
}

// NewTaskFinishedEvent creates a TaskFinished lifecycle event
func NewTaskFinishedEvent(projectID, taskID string) TaskLifecycleEvent {
	return TaskLifecycleEvent{
		Type:      TaskFinished,
		ProjectID: projectID,
		TaskID:    taskID,
	}
}

// NewTaskDeletedEvent creates a TaskDeleted lifecycle event
func NewTaskDeletedEvent(projectID, taskID string) TaskLifecycleEvent {
	return TaskLifecycleEvent{
		Type:      TaskDeleted,
		ProjectID: projectID,
		TaskID:    taskID,
	}
}

// NewTaskCreatedEvent creates a TaskCreated lifecycle event with full task data
func NewTaskCreatedEvent(projectID string, task *models.Task) TaskLifecycleEvent {
	return TaskLifecycleEvent{
		Type:      TaskCreated,
		ProjectID: projectID,
		TaskID:    task.ID,
		Task:      task,
	}
}

// NewTaskStatusUpdatedEvent creates a TaskStatusUpdated lifecycle event
func NewTaskStatusUpdatedEvent(projectID, taskID string, newStatus models.TaskStatus) TaskLifecycleEvent {
	return TaskLifecycleEvent{
		Type:      TaskStatusUpdated,
		ProjectID: projectID,
		TaskID:    taskID,
		NewStatus: newStatus,
	}
}

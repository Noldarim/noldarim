// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskLifecycleEvent_GetMetadata(t *testing.T) {
	event := TaskLifecycleEvent{
		Metadata: Metadata{
			IdempotencyKey: "test-key",
			Version:        "1.0.0",
		},
		Type:      TaskRequested,
		ProjectID: "proj-123",
		TaskID:    "task-456",
	}

	metadata := event.GetMetadata()
	assert.Equal(t, "test-key", metadata.IdempotencyKey)
	assert.Equal(t, "1.0.0", metadata.Version)
}

func TestTaskLifecycleEvent_AllTypes(t *testing.T) {
	types := []TaskLifecycleType{
		TaskRequested,
		TaskInProgress,
		TaskFinished,
		TaskDeleted,
		TaskCreated,
		TaskStatusUpdated,
	}

	for _, eventType := range types {
		t.Run(string(eventType), func(t *testing.T) {
			event := TaskLifecycleEvent{
				Metadata: Metadata{
					IdempotencyKey: "test-key-" + string(eventType),
					Version:        CurrentProtocolVersion,
				},
				Type:      eventType,
				ProjectID: "proj-123",
				TaskID:    "task-456",
			}

			metadata := event.GetMetadata()
			assert.Equal(t, "test-key-"+string(eventType), metadata.IdempotencyKey)
			assert.Equal(t, CurrentProtocolVersion, metadata.Version)
			assert.Equal(t, eventType, event.Type)
		})
	}
}

func TestGetIdempotencyKey_WithTaskLifecycleEvent(t *testing.T) {
	event := TaskLifecycleEvent{
		Metadata: Metadata{
			IdempotencyKey: "task-lifecycle-key",
			Version:        "1.0.0",
		},
		Type:      TaskRequested,
		ProjectID: "proj-123",
		TaskID:    "task-456",
	}

	key := GetIdempotencyKey(event)
	assert.Equal(t, "task-lifecycle-key", key)
}

func TestTaskLifecycleEvent_FieldsPopulation(t *testing.T) {
	event := TaskLifecycleEvent{
		Metadata: Metadata{
			IdempotencyKey: "test-key",
			Version:        CurrentProtocolVersion,
		},
		Type:      TaskInProgress,
		ProjectID: "project-abc",
		TaskID:    "task-xyz",
	}

	assert.Equal(t, "project-abc", event.ProjectID)
	assert.Equal(t, "task-xyz", event.TaskID)
	assert.Equal(t, TaskInProgress, event.Type)
	assert.Equal(t, CurrentProtocolVersion, event.Metadata.Version)
	assert.Equal(t, "test-key", event.Metadata.IdempotencyKey)
}

func TestNewTaskLifecycleEvent_Helpers(t *testing.T) {
	t.Run("NewTaskRequestedEvent", func(t *testing.T) {
		event := NewTaskRequestedEvent("proj-1", "task-1")
		assert.Equal(t, TaskRequested, event.Type)
		assert.Equal(t, "proj-1", event.ProjectID)
		assert.Equal(t, "task-1", event.TaskID)
	})

	t.Run("NewTaskInProgressEvent", func(t *testing.T) {
		event := NewTaskInProgressEvent("proj-2", "task-2")
		assert.Equal(t, TaskInProgress, event.Type)
		assert.Equal(t, "proj-2", event.ProjectID)
		assert.Equal(t, "task-2", event.TaskID)
	})

	t.Run("NewTaskFinishedEvent", func(t *testing.T) {
		event := NewTaskFinishedEvent("proj-3", "task-3")
		assert.Equal(t, TaskFinished, event.Type)
		assert.Equal(t, "proj-3", event.ProjectID)
		assert.Equal(t, "task-3", event.TaskID)
	})

	t.Run("NewTaskDeletedEvent", func(t *testing.T) {
		event := NewTaskDeletedEvent("proj-4", "task-4")
		assert.Equal(t, TaskDeleted, event.Type)
		assert.Equal(t, "proj-4", event.ProjectID)
		assert.Equal(t, "task-4", event.TaskID)
	})
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/pkg/containers/models"
)

func TestEventType_String(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{ContainerCreated, "container.created"},
		{ContainerStarted, "container.started"},
		{ContainerStopped, "container.stopped"},
		{ContainerDeleted, "container.deleted"},
		{ContainerFailed, "container.failed"},
		{ContainerStatusChanged, "container.status_changed"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.eventType))
		})
	}
}

func TestEvent_JSONSerialization(t *testing.T) {
	now := time.Now()
	event := Event{
		ID:        "evt_123456789",
		Type:      ContainerCreated,
		Timestamp: now,
		Data: map[string]interface{}{
			"payload": ContainerCreatedEvent{
				ContainerID: "container-123",
				Name:        "test-container",
				Image:       "python:3.9",
				Timestamp:   now,
			},
		},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled Event
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ID, unmarshaled.ID)
	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.NotNil(t, unmarshaled.Data["payload"])
}

func TestContainerCreatedEvent_JSONSerialization(t *testing.T) {
	now := time.Now()
	config := models.ContainerConfig{
		Name:  "test-container",
		Image: "python:3.9",
		Environment: map[string]string{
			"PYTHONPATH": "/app",
		},
	}

	event := ContainerCreatedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		Image:       "python:3.9",
		Config:      config,
		Timestamp:   now,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled ContainerCreatedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ContainerID, unmarshaled.ContainerID)
	assert.Equal(t, event.Name, unmarshaled.Name)
	assert.Equal(t, event.Image, unmarshaled.Image)
	assert.Equal(t, event.Config.Name, unmarshaled.Config.Name)
	assert.Equal(t, event.Config.Image, unmarshaled.Config.Image)
	assert.Equal(t, event.Config.Environment, unmarshaled.Config.Environment)
}

func TestContainerStartedEvent_JSONSerialization(t *testing.T) {
	now := time.Now()
	event := ContainerStartedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		Timestamp:   now,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled ContainerStartedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ContainerID, unmarshaled.ContainerID)
	assert.Equal(t, event.Name, unmarshaled.Name)
}

func TestContainerStoppedEvent_JSONSerialization(t *testing.T) {
	now := time.Now()
	event := ContainerStoppedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		ExitCode:    0,
		Timestamp:   now,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled ContainerStoppedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ContainerID, unmarshaled.ContainerID)
	assert.Equal(t, event.Name, unmarshaled.Name)
	assert.Equal(t, event.ExitCode, unmarshaled.ExitCode)
}

func TestContainerDeletedEvent_JSONSerialization(t *testing.T) {
	now := time.Now()
	event := ContainerDeletedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		Timestamp:   now,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled ContainerDeletedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ContainerID, unmarshaled.ContainerID)
	assert.Equal(t, event.Name, unmarshaled.Name)
}

func TestContainerFailedEvent_JSONSerialization(t *testing.T) {
	now := time.Now()
	event := ContainerFailedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		Operation:   "start",
		Error:       "failed to start container",
		Timestamp:   now,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled ContainerFailedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ContainerID, unmarshaled.ContainerID)
	assert.Equal(t, event.Name, unmarshaled.Name)
	assert.Equal(t, event.Operation, unmarshaled.Operation)
	assert.Equal(t, event.Error, unmarshaled.Error)
}

func TestContainerStatusChangedEvent_JSONSerialization(t *testing.T) {
	now := time.Now()
	event := ContainerStatusChangedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		OldStatus:   models.StatusCreated,
		NewStatus:   models.StatusRunning,
		Timestamp:   now,
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled ContainerStatusChangedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.ContainerID, unmarshaled.ContainerID)
	assert.Equal(t, event.Name, unmarshaled.Name)
	assert.Equal(t, event.OldStatus, unmarshaled.OldStatus)
	assert.Equal(t, event.NewStatus, unmarshaled.NewStatus)
}

func TestEvent_Validation(t *testing.T) {
	now := time.Now()
	event := Event{
		ID:        "evt_123456789",
		Type:      ContainerCreated,
		Timestamp: now,
		Data: map[string]interface{}{
			"test": "data",
		},
	}

	assert.NotEmpty(t, event.ID)
	assert.NotEmpty(t, string(event.Type))
	assert.False(t, event.Timestamp.IsZero())
	assert.NotNil(t, event.Data)
}

func TestContainerCreatedEvent_Validation(t *testing.T) {
	now := time.Now()
	event := ContainerCreatedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		Image:       "python:3.9",
		Config: models.ContainerConfig{
			Name:  "test-container",
			Image: "python:3.9",
		},
		Timestamp: now,
	}

	assert.NotEmpty(t, event.ContainerID)
	assert.NotEmpty(t, event.Name)
	assert.NotEmpty(t, event.Image)
	assert.NotEmpty(t, event.Config.Name)
	assert.NotEmpty(t, event.Config.Image)
	assert.False(t, event.Timestamp.IsZero())
}

func TestContainerFailedEvent_Validation(t *testing.T) {
	now := time.Now()
	event := ContainerFailedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		Operation:   "start",
		Error:       "connection refused",
		Timestamp:   now,
	}

	assert.NotEmpty(t, event.ContainerID)
	assert.NotEmpty(t, event.Name)
	assert.NotEmpty(t, event.Operation)
	assert.NotEmpty(t, event.Error)
	assert.False(t, event.Timestamp.IsZero())
}

func TestContainerStatusChangedEvent_Validation(t *testing.T) {
	now := time.Now()
	event := ContainerStatusChangedEvent{
		ContainerID: "container-123",
		Name:        "test-container",
		OldStatus:   models.StatusCreated,
		NewStatus:   models.StatusRunning,
		Timestamp:   now,
	}

	assert.NotEmpty(t, event.ContainerID)
	assert.NotEmpty(t, event.Name)
	assert.NotEmpty(t, string(event.OldStatus))
	assert.NotEmpty(t, string(event.NewStatus))
	assert.NotEqual(t, event.OldStatus, event.NewStatus)
	assert.False(t, event.Timestamp.IsZero())
}

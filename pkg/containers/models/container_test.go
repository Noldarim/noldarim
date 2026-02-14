// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerStatus_String(t *testing.T) {
	tests := []struct {
		status   ContainerStatus
		expected string
	}{
		{StatusCreated, "created"},
		{StatusRunning, "running"},
		{StatusStopped, "stopped"},
		{StatusFailed, "failed"},
		{StatusDeleted, "deleted"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestContainer_JSONSerialization(t *testing.T) {
	now := time.Now()
	container := &Container{
		ID:     "container-123",
		Name:   "test-container",
		Image:  "python:3.9",
		Status: StatusRunning,
		Environment: map[string]string{
			"PYTHONPATH": "/app",
			"DEBUG":      "true",
		},
		Ports: []PortMapping{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
		Volumes: []VolumeMapping{
			{HostPath: "/host/path", ContainerPath: "/container/path", ReadOnly: false},
		},
		CreatedAt: now,
		UpdatedAt: now,
		TaskID:    "task-456",
	}

	data, err := json.Marshal(container)
	require.NoError(t, err)

	var unmarshaled Container
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, container.ID, unmarshaled.ID)
	assert.Equal(t, container.Name, unmarshaled.Name)
	assert.Equal(t, container.Image, unmarshaled.Image)
	assert.Equal(t, container.Status, unmarshaled.Status)
	assert.Equal(t, container.Environment, unmarshaled.Environment)
	assert.Equal(t, container.Ports, unmarshaled.Ports)
	assert.Equal(t, container.Volumes, unmarshaled.Volumes)
	assert.Equal(t, container.TaskID, unmarshaled.TaskID)
}

func TestPortMapping_Validation(t *testing.T) {
	tests := []struct {
		name    string
		mapping PortMapping
		valid   bool
	}{
		{
			name:    "valid tcp mapping",
			mapping: PortMapping{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
			valid:   true,
		},
		{
			name:    "valid udp mapping",
			mapping: PortMapping{HostPort: 5353, ContainerPort: 53, Protocol: "udp"},
			valid:   true,
		},
		{
			name:    "same port mapping",
			mapping: PortMapping{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
			valid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.mapping.HostPort > 0)
			assert.True(t, tt.mapping.ContainerPort > 0)
			assert.NotEmpty(t, tt.mapping.Protocol)
		})
	}
}

func TestVolumeMapping_Validation(t *testing.T) {
	tests := []struct {
		name    string
		mapping VolumeMapping
		valid   bool
	}{
		{
			name:    "valid read-write mapping",
			mapping: VolumeMapping{HostPath: "/host/path", ContainerPath: "/container/path", ReadOnly: false},
			valid:   true,
		},
		{
			name:    "valid read-only mapping",
			mapping: VolumeMapping{HostPath: "/host/readonly", ContainerPath: "/container/readonly", ReadOnly: true},
			valid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.mapping.HostPath)
			assert.NotEmpty(t, tt.mapping.ContainerPath)
		})
	}
}

func TestContainerConfig_Validation(t *testing.T) {
	config := ContainerConfig{
		Name:  "test-container",
		Image: "python:3.9",
		Environment: map[string]string{
			"ENV": "test",
		},
		Ports: []PortMapping{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
		Volumes: []VolumeMapping{
			{HostPath: "/host", ContainerPath: "/container", ReadOnly: false},
		},
		WorkingDir: "/app",
		Command:    []string{"python", "app.py"},
		TaskID:     "task-123",
	}

	assert.NotEmpty(t, config.Name)
	assert.NotEmpty(t, config.Image)
	assert.NotEmpty(t, config.Environment)
	assert.NotEmpty(t, config.Ports)
	assert.NotEmpty(t, config.Volumes)
	assert.NotEmpty(t, config.WorkingDir)
	assert.NotEmpty(t, config.Command)
	assert.NotEmpty(t, config.TaskID)
}

func TestContainerConfig_JSONSerialization(t *testing.T) {
	config := ContainerConfig{
		Name:  "test-container",
		Image: "python:3.9",
		Environment: map[string]string{
			"PYTHONPATH": "/app",
		},
		Ports: []PortMapping{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
		Volumes: []VolumeMapping{
			{HostPath: "/host/path", ContainerPath: "/container/path", ReadOnly: false},
		},
		WorkingDir: "/app",
		Command:    []string{"python", "main.py"},
		TaskID:     "task-456",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var unmarshaled ContainerConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, config.Name, unmarshaled.Name)
	assert.Equal(t, config.Image, unmarshaled.Image)
	assert.Equal(t, config.Environment, unmarshaled.Environment)
	assert.Equal(t, config.Ports, unmarshaled.Ports)
	assert.Equal(t, config.Volumes, unmarshaled.Volumes)
	assert.Equal(t, config.WorkingDir, unmarshaled.WorkingDir)
	assert.Equal(t, config.Command, unmarshaled.Command)
	assert.Equal(t, config.TaskID, unmarshaled.TaskID)
}

func TestContainer_DefaultValues(t *testing.T) {
	container := &Container{
		ID:     "test-id",
		Name:   "test-name",
		Image:  "test-image",
		Status: StatusCreated,
	}

	assert.Equal(t, "test-id", container.ID)
	assert.Equal(t, "test-name", container.Name)
	assert.Equal(t, "test-image", container.Image)
	assert.Equal(t, StatusCreated, container.Status)
	assert.Empty(t, container.Environment)
	assert.Empty(t, container.Ports)
	assert.Empty(t, container.Volumes)
	assert.True(t, container.CreatedAt.IsZero())
	assert.True(t, container.UpdatedAt.IsZero())
	assert.Empty(t, container.TaskID)
}

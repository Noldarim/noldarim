// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package events

import (
	"time"

	"github.com/noldarim/noldarim/pkg/containers/models"
)

// EventType defines the type of container event
type EventType string

const (
	ContainerCreated       EventType = "container.created"
	ContainerStarted       EventType = "container.started"
	ContainerStopped       EventType = "container.stopped"
	ContainerDeleted       EventType = "container.deleted"
	ContainerFailed        EventType = "container.failed"
	ContainerStatusChanged EventType = "container.status_changed"
)

// Event represents a container lifecycle event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// ContainerCreatedEvent is published when a container is created
type ContainerCreatedEvent struct {
	ContainerID string                 `json:"container_id"`
	Name        string                 `json:"name"`
	Image       string                 `json:"image"`
	Config      models.ContainerConfig `json:"config"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ContainerStartedEvent is published when a container starts
type ContainerStartedEvent struct {
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	Timestamp   time.Time `json:"timestamp"`
}

// ContainerStoppedEvent is published when a container stops
type ContainerStoppedEvent struct {
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	ExitCode    int       `json:"exit_code"`
	Timestamp   time.Time `json:"timestamp"`
}

// ContainerDeletedEvent is published when a container is deleted
type ContainerDeletedEvent struct {
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	Timestamp   time.Time `json:"timestamp"`
}

// ContainerFailedEvent is published when a container operation fails
type ContainerFailedEvent struct {
	ContainerID string    `json:"container_id"`
	Name        string    `json:"name"`
	Operation   string    `json:"operation"`
	Error       string    `json:"error"`
	Timestamp   time.Time `json:"timestamp"`
}

// ContainerStatusChangedEvent is published when container status changes
type ContainerStatusChangedEvent struct {
	ContainerID string                 `json:"container_id"`
	Name        string                 `json:"name"`
	OldStatus   models.ContainerStatus `json:"old_status"`
	NewStatus   models.ContainerStatus `json:"new_status"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Publisher defines the interface for publishing container events
type Publisher interface {
	Publish(event Event) error
}

// Subscriber defines the interface for subscribing to container events
type Subscriber interface {
	Subscribe(eventType EventType, handler func(Event)) error
	Unsubscribe(eventType EventType) error
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/noldarim/noldarim/pkg/containers/docker"
	"github.com/noldarim/noldarim/pkg/containers/events"
	"github.com/noldarim/noldarim/pkg/containers/models"
)

// Service manages container lifecycle and publishes events
type Service struct {
	client     docker.ClientInterface
	publisher  events.Publisher
	containers map[string]*models.Container
	mutex      sync.RWMutex
}

// NewService creates a new container service using default Docker settings
func NewService(publisher events.Publisher) (*Service, error) {
	return NewServiceWithDockerHost(publisher, "")
}

// NewServiceWithDockerHost creates a new container service with specific Docker host
func NewServiceWithDockerHost(publisher events.Publisher, dockerHost string) (*Service, error) {
	client, err := docker.NewClientWithHost(dockerHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Service{
		client:     client,
		publisher:  publisher,
		containers: make(map[string]*models.Container),
	}, nil
}

// NewServiceWithClient creates a new container service with provided client
func NewServiceWithClient(client docker.ClientInterface, publisher events.Publisher) *Service {
	return &Service{
		client:     client,
		publisher:  publisher,
		containers: make(map[string]*models.Container),
	}
}

// CreateContainer creates a new container and publishes creation event
func (s *Service) CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error) {
	container, err := s.client.CreateContainer(ctx, config)
	if err != nil {
		s.publishFailedEvent(config.Name, "create", err)
		return nil, err
	}

	s.mutex.Lock()
	s.containers[container.ID] = container
	s.mutex.Unlock()

	s.publishEvent(events.ContainerCreated, events.ContainerCreatedEvent{
		ContainerID: container.ID,
		Name:        container.Name,
		Image:       container.Image,
		Config:      config,
		Timestamp:   time.Now(),
	})

	return container, nil
}

// StartContainer starts an existing container and publishes start event
func (s *Service) StartContainer(ctx context.Context, containerID string) error {
	container := s.getContainer(containerID)
	if container == nil {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if err := s.client.StartContainer(ctx, containerID); err != nil {
		s.publishFailedEvent(container.Name, "start", err)
		return err
	}

	// Update container status
	s.mutex.Lock()
	oldStatus := container.Status
	container.Status = models.StatusRunning
	container.UpdatedAt = time.Now()
	s.mutex.Unlock()

	s.publishEvent(events.ContainerStarted, events.ContainerStartedEvent{
		ContainerID: containerID,
		Name:        container.Name,
		Timestamp:   time.Now(),
	})

	s.publishEvent(events.ContainerStatusChanged, events.ContainerStatusChangedEvent{
		ContainerID: containerID,
		Name:        container.Name,
		OldStatus:   oldStatus,
		NewStatus:   models.StatusRunning,
		Timestamp:   time.Now(),
	})

	return nil
}

// StopContainer stops a running container and publishes stop event
func (s *Service) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error {
	container := s.getContainer(containerID)
	if container == nil {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if err := s.client.StopContainer(ctx, containerID, timeout); err != nil {
		s.publishFailedEvent(container.Name, "stop", err)
		return err
	}

	// Update container status
	s.mutex.Lock()
	oldStatus := container.Status
	container.Status = models.StatusStopped
	container.UpdatedAt = time.Now()
	s.mutex.Unlock()

	s.publishEvent(events.ContainerStopped, events.ContainerStoppedEvent{
		ContainerID: containerID,
		Name:        container.Name,
		ExitCode:    0, // TODO: Get actual exit code from Docker
		Timestamp:   time.Now(),
	})

	s.publishEvent(events.ContainerStatusChanged, events.ContainerStatusChangedEvent{
		ContainerID: containerID,
		Name:        container.Name,
		OldStatus:   oldStatus,
		NewStatus:   models.StatusStopped,
		Timestamp:   time.Now(),
	})

	return nil
}

// DeleteContainer removes a container and publishes deletion event
func (s *Service) DeleteContainer(ctx context.Context, containerID string, force bool) error {
	container := s.getContainer(containerID)
	if container == nil {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if err := s.client.RemoveContainer(ctx, containerID, force); err != nil {
		s.publishFailedEvent(container.Name, "delete", err)
		return err
	}

	// Remove from internal tracking
	s.mutex.Lock()
	delete(s.containers, containerID)
	s.mutex.Unlock()

	s.publishEvent(events.ContainerDeleted, events.ContainerDeletedEvent{
		ContainerID: containerID,
		Name:        container.Name,
		Timestamp:   time.Now(),
	})

	return nil
}

// GetContainer retrieves container information
func (s *Service) GetContainer(ctx context.Context, containerID string) (*models.Container, error) {
	// Try to get from internal cache first
	if container := s.getContainer(containerID); container != nil {
		return container, nil
	}

	// If not in cache, inspect from Docker
	container, err := s.client.InspectContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}

	// Update cache
	s.mutex.Lock()
	s.containers[containerID] = container
	s.mutex.Unlock()

	return container, nil
}

// ListContainers lists all containers
func (s *Service) ListContainers(ctx context.Context) ([]*models.Container, error) {
	containers, err := s.client.ListContainers(ctx)
	if err != nil {
		return nil, err
	}

	// Update internal cache
	s.mutex.Lock()
	for _, container := range containers {
		s.containers[container.ID] = container
	}
	s.mutex.Unlock()

	return containers, nil
}

// RefreshContainer updates container information from Docker
func (s *Service) RefreshContainer(ctx context.Context, containerID string) (*models.Container, error) {
	container, err := s.client.InspectContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}

	s.mutex.Lock()
	oldContainer := s.containers[containerID]
	s.containers[containerID] = container
	s.mutex.Unlock()

	// Publish status change event if status changed
	if oldContainer != nil && oldContainer.Status != container.Status {
		s.publishEvent(events.ContainerStatusChanged, events.ContainerStatusChangedEvent{
			ContainerID: containerID,
			Name:        container.Name,
			OldStatus:   oldContainer.Status,
			NewStatus:   container.Status,
			Timestamp:   time.Now(),
		})
	}

	return container, nil
}

// ListContainersByLabels lists containers filtered by labels
func (s *Service) ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error) {
	// Call client method
	containers, err := s.client.ListContainersByLabels(ctx, labels)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers by labels: %w", err)
	}

	// Update internal cache
	s.mutex.Lock()
	for _, container := range containers {
		s.containers[container.ID] = container
	}
	s.mutex.Unlock()

	return containers, nil
}

// KillContainer kills a container forcefully
func (s *Service) KillContainer(ctx context.Context, containerID string) error {
	// Get container from cache
	container := s.getContainer(containerID)

	// Call client method
	err := s.client.KillContainer(ctx, containerID)

	// Handle success (including "not found" which returns nil)
	if err == nil {
		if container != nil {
			// Update status and publish event
			s.mutex.Lock()
			oldStatus := container.Status
			container.Status = models.StatusStopped
			container.UpdatedAt = time.Now()
			s.mutex.Unlock()

			s.publishEvent(events.ContainerStopped, events.ContainerStoppedEvent{
				ContainerID: containerID,
				Name:        container.Name,
				ExitCode:    137, // SIGKILL exit code
				Timestamp:   time.Now(),
			})

			// Only publish status changed event if status actually changed
			if oldStatus != models.StatusStopped {
				s.publishEvent(events.ContainerStatusChanged, events.ContainerStatusChangedEvent{
					ContainerID: containerID,
					Name:        container.Name,
					OldStatus:   oldStatus,
					NewStatus:   models.StatusStopped,
					Timestamp:   time.Now(),
				})
			}
		}
		return nil
	}

	// Handle actual errors
	if container != nil {
		s.publishFailedEvent(container.Name, "kill", err)
	}
	return err
}

// CopyFileToContainer copies a file from the host to a container and publishes event
func (s *Service) CopyFileToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	// Try to get from internal cache first
	container := s.getContainer(containerID)
	if container == nil {
		// Fallback to Docker inspect
		var err error
		container, err = s.client.InspectContainer(ctx, containerID)
		if err != nil {
			return fmt.Errorf("container not found: %s", containerID)
		}

		// Update cache
		s.mutex.Lock()
		s.containers[containerID] = container
		s.mutex.Unlock()
	}

	if err := s.client.CopyToContainer(ctx, containerID, srcPath, dstPath); err != nil {
		s.publishFailedEvent(container.Name, "copy-file", err)
		return err
	}

	// Note: We don't publish a specific event for file copy success as it's typically
	// part of a larger workflow operation. The calling activity can handle success reporting.

	return nil
}

// CopyFileFromContainer copies a file from a container to the host
func (s *Service) CopyFileFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	container := s.getContainer(containerID)
	if container == nil {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if err := s.client.CopyFromContainer(ctx, containerID, srcPath, dstPath); err != nil {
		s.publishFailedEvent(container.Name, "copy-from-container", err)
		return err
	}

	return nil
}

// WriteToContainer writes string content directly to a container file and publishes event on failure
func (s *Service) WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error {
	// Try to get from internal cache first
	container := s.getContainer(containerID)
	if container == nil {
		// Fallback to Docker inspect
		var err error
		container, err = s.client.InspectContainer(ctx, containerID)
		if err != nil {
			return fmt.Errorf("container not found: %s", containerID)
		}

		// Update cache
		s.mutex.Lock()
		s.containers[containerID] = container
		s.mutex.Unlock()
	}

	if err := s.client.WriteToContainer(ctx, containerID, content, dstPath); err != nil {
		s.publishFailedEvent(container.Name, "write-to-container", err)
		return err
	}

	// Note: We don't publish a specific event for write success as it's typically
	// part of a larger workflow operation. The calling activity can handle success reporting.

	return nil
}

// ExecContainer executes a command in a running container and publishes event on failure
func (s *Service) ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error) {
	// Try to get from internal cache first
	container := s.getContainer(containerID)
	if container == nil {
		// Fallback to Docker inspect
		var err error
		container, err = s.client.InspectContainer(ctx, containerID)
		if err != nil {
			return nil, fmt.Errorf("container not found: %s", containerID)
		}

		// Update cache
		s.mutex.Lock()
		s.containers[containerID] = container
		s.mutex.Unlock()
	}

	result, err := s.client.ExecContainer(ctx, containerID, cmd, workDir)
	if err != nil {
		s.publishFailedEvent(container.Name, "exec-container", err)
		return nil, err
	}

	// Note: We don't publish a specific event for exec success as it's typically
	// part of a larger workflow operation. The calling activity can handle success reporting.

	return result, nil
}

// Close closes the service and releases resources
func (s *Service) Close() error {
	return s.client.Close()
}

// Helper methods

func (s *Service) getContainer(containerID string) *models.Container {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.containers[containerID]
}

func (s *Service) publishEvent(eventType events.EventType, data interface{}) {
	if s.publisher == nil {
		return
	}

	event := events.Event{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"payload": data},
	}

	s.publisher.Publish(event)
}

func (s *Service) publishFailedEvent(containerName, operation string, err error) {
	s.publishEvent(events.ContainerFailed, events.ContainerFailedEvent{
		Name:      containerName,
		Operation: operation,
		Error:     err.Error(),
		Timestamp: time.Now(),
	})
}

// generateEventID creates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

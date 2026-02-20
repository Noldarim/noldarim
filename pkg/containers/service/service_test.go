// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/pkg/containers/docker"
	"github.com/noldarim/noldarim/pkg/containers/events"
	"github.com/noldarim/noldarim/pkg/containers/models"
)

func TestService_CreateContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	config := models.ContainerConfig{
		Name:  "test-container",
		Image: "python:3.9",
		Environment: map[string]string{
			"PYTHONPATH": "/app",
		},
	}

	expectedContainer := &models.Container{
		ID:          "container-123",
		Name:        "test-container",
		Image:       "python:3.9",
		Status:      models.StatusCreated,
		Environment: config.Environment,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockClient.On("CreateContainer", mock.Anything, config).Return(expectedContainer, nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerCreated
	})).Return(nil)

	result, err := service.CreateContainer(context.Background(), config)

	require.NoError(t, err)
	assert.Equal(t, expectedContainer.ID, result.ID)
	assert.Equal(t, expectedContainer.Name, result.Name)
	assert.Equal(t, expectedContainer.Image, result.Image)
	assert.Equal(t, expectedContainer.Status, result.Status)

	assert.Equal(t, expectedContainer, service.containers[expectedContainer.ID])

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_CreateContainer_ClientError(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	config := models.ContainerConfig{
		Name:  "test-container",
		Image: "python:3.9",
	}

	expectedError := fmt.Errorf("docker error")
	mockClient.On("CreateContainer", mock.Anything, config).Return((*models.Container)(nil), expectedError)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerFailed
	})).Return(nil)

	result, err := service.CreateContainer(context.Background(), config)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	assert.Empty(t, service.containers)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_StartContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusCreated,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	mockClient.On("StartContainer", mock.Anything, "container-123").Return(nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStarted
	})).Return(nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStatusChanged
	})).Return(nil)

	err := service.StartContainer(context.Background(), "container-123")

	assert.NoError(t, err)
	assert.Equal(t, models.StatusRunning, service.containers["container-123"].Status)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_StartContainer_NotFound(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	err := service.StartContainer(context.Background(), "container-123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")

	mockClient.AssertNotCalled(t, "StartContainer")
	mockPublisher.AssertNotCalled(t, "Publish")
}

func TestService_StartContainer_ClientError(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusCreated,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	expectedError := fmt.Errorf("start error")
	mockClient.On("StartContainer", mock.Anything, "container-123").Return(expectedError)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerFailed
	})).Return(nil)

	err := service.StartContainer(context.Background(), "container-123")

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Equal(t, models.StatusCreated, service.containers["container-123"].Status)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_StopContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	timeout := 30 * time.Second
	mockClient.On("StopContainer", mock.Anything, "container-123", &timeout).Return(nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStopped
	})).Return(nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStatusChanged
	})).Return(nil)

	err := service.StopContainer(context.Background(), "container-123", &timeout)

	assert.NoError(t, err)
	assert.Equal(t, models.StatusStopped, service.containers["container-123"].Status)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_DeleteContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusStopped,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	mockClient.On("RemoveContainer", mock.Anything, "container-123", true).Return(nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerDeleted
	})).Return(nil)

	err := service.DeleteContainer(context.Background(), "container-123", true)

	assert.NoError(t, err)
	assert.NotContains(t, service.containers, "container-123")

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_GetContainer_FromCache(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	result, err := service.GetContainer(context.Background(), "container-123")

	assert.NoError(t, err)
	assert.Equal(t, existingContainer, result)

	mockClient.AssertNotCalled(t, "InspectContainer")
}

func TestService_GetContainer_FromDocker(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	dockerContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	mockClient.On("InspectContainer", mock.Anything, "container-123").Return(dockerContainer, nil)

	result, err := service.GetContainer(context.Background(), "container-123")

	assert.NoError(t, err)
	assert.Equal(t, dockerContainer, result)
	assert.Equal(t, dockerContainer, service.containers["container-123"])

	mockClient.AssertExpectations(t)
}

func TestService_ListContainers_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	dockerContainers := []*models.Container{
		{ID: "container-1", Name: "test-1", Status: models.StatusRunning},
		{ID: "container-2", Name: "test-2", Status: models.StatusStopped},
	}

	mockClient.On("ListContainers", mock.Anything).Return(dockerContainers, nil)

	result, err := service.ListContainers(context.Background())

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, dockerContainers[0], result[0])
	assert.Equal(t, dockerContainers[1], result[1])

	assert.Equal(t, dockerContainers[0], service.containers["container-1"])
	assert.Equal(t, dockerContainers[1], service.containers["container-2"])

	mockClient.AssertExpectations(t)
}

func TestService_RefreshContainer_StatusChanged(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	oldContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := &Service{
		client:    mockClient,
		publisher: mockPublisher,
		containers: map[string]*models.Container{
			"container-123": oldContainer,
		},
	}

	refreshedContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusStopped,
	}

	mockClient.On("InspectContainer", mock.Anything, "container-123").Return(refreshedContainer, nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStatusChanged
	})).Return(nil)

	result, err := service.RefreshContainer(context.Background(), "container-123")

	assert.NoError(t, err)
	assert.Equal(t, refreshedContainer, result)
	assert.Equal(t, refreshedContainer, service.containers["container-123"])

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_Close(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	mockClient.On("Close").Return(nil)

	err := service.Close()

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestService_publishEvent_NilPublisher(t *testing.T) {
	service := NewServiceWithClient(nil, nil)

	service.publishEvent(events.ContainerCreated, events.ContainerCreatedEvent{
		ContainerID: "test",
		Name:        "test",
		Timestamp:   time.Now(),
	})
}

func TestService_publishFailedEvent(t *testing.T) {
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(nil, mockPublisher)

	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerFailed &&
			event.Data["payload"].(events.ContainerFailedEvent).Name == "test-container" &&
			event.Data["payload"].(events.ContainerFailedEvent).Operation == "create" &&
			event.Data["payload"].(events.ContainerFailedEvent).Error == "test error"
	})).Return(nil)

	service.publishFailedEvent("test-container", "create", fmt.Errorf("test error"))

	mockPublisher.AssertExpectations(t)
}

// Tests for new helper methods

func TestService_ListContainersByLabels_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	labels := map[string]string{
		"app":         "noldarim",
		"environment": "test",
		"task_id":     "task-123",
	}

	expectedContainers := []*models.Container{
		{
			ID:     "container-1",
			Name:   "test-container-1",
			Image:  "python:3.9",
			Status: models.StatusRunning,
			TaskID: "task-123",
		},
		{
			ID:     "container-2",
			Name:   "test-container-2",
			Image:  "node:16",
			Status: models.StatusRunning,
			TaskID: "task-123",
		},
	}

	mockClient.On("ListContainersByLabels", mock.Anything, labels).Return(expectedContainers, nil)

	result, err := service.ListContainersByLabels(context.Background(), labels)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, expectedContainers[0], result[0])
	assert.Equal(t, expectedContainers[1], result[1])

	// Verify containers were added to cache
	assert.Equal(t, expectedContainers[0], service.containers["container-1"])
	assert.Equal(t, expectedContainers[1], service.containers["container-2"])

	mockClient.AssertExpectations(t)
}

func TestService_ListContainersByLabels_ClientError(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	labels := map[string]string{
		"app": "noldarim",
	}

	expectedError := fmt.Errorf("failed to connect to docker")
	mockClient.On("ListContainersByLabels", mock.Anything, labels).Return(([]*models.Container)(nil), expectedError)

	result, err := service.ListContainersByLabels(context.Background(), labels)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list containers by labels")
	assert.Contains(t, err.Error(), expectedError.Error())

	mockClient.AssertExpectations(t)
}

func TestService_ListContainersByLabels_EmptyLabels(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	// Empty labels should still work - returns all containers
	labels := map[string]string{}

	expectedContainers := []*models.Container{
		{
			ID:     "container-1",
			Name:   "all-containers-1",
			Status: models.StatusRunning,
		},
	}

	mockClient.On("ListContainersByLabels", mock.Anything, labels).Return(expectedContainers, nil)

	result, err := service.ListContainersByLabels(context.Background(), labels)

	require.NoError(t, err)
	assert.Len(t, result, 1)

	mockClient.AssertExpectations(t)
}

func TestService_ListContainersByLabels_NilLabels(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	// Nil labels should be handled gracefully
	var labels map[string]string = nil

	expectedContainers := []*models.Container{}

	mockClient.On("ListContainersByLabels", mock.Anything, labels).Return(expectedContainers, nil)

	result, err := service.ListContainersByLabels(context.Background(), labels)

	require.NoError(t, err)
	assert.Empty(t, result)

	mockClient.AssertExpectations(t)
}

func TestService_KillContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	mockClient.On("KillContainer", mock.Anything, "container-123").Return(nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStopped &&
			event.Data["payload"].(events.ContainerStoppedEvent).ExitCode == 137 // SIGKILL
	})).Return(nil)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStatusChanged
	})).Return(nil)

	err := service.KillContainer(context.Background(), "container-123")

	assert.NoError(t, err)
	assert.Equal(t, models.StatusStopped, service.containers["container-123"].Status)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_KillContainer_ContainerNotInCache(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)
	// Container not in cache

	mockClient.On("KillContainer", mock.Anything, "container-123").Return(nil)
	// No events published when container not in cache

	err := service.KillContainer(context.Background(), "container-123")

	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "Publish")
}

func TestService_KillContainer_ClientError(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	expectedError := fmt.Errorf("failed to kill container")
	mockClient.On("KillContainer", mock.Anything, "container-123").Return(expectedError)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerFailed
	})).Return(nil)

	err := service.KillContainer(context.Background(), "container-123")

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	// Status should not have changed
	assert.Equal(t, models.StatusRunning, service.containers["container-123"].Status)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_KillContainer_AlreadyStopped(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusStopped,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	// Docker client returns nil for already stopped containers
	mockClient.On("KillContainer", mock.Anything, "container-123").Return(nil)
	// No status change event since already stopped
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStopped
	})).Return(nil)

	err := service.KillContainer(context.Background(), "container-123")

	assert.NoError(t, err)
	// Status remains stopped
	assert.Equal(t, models.StatusStopped, service.containers["container-123"].Status)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
	// Should not publish status changed event since status didn't change
	mockPublisher.AssertNotCalled(t, "Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStatusChanged
	}))
}

func TestService_KillContainer_IdempotencyIntegration(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	// First kill - succeeds and publishes events
	mockClient.On("KillContainer", mock.Anything, "container-123").Return(nil).Once()
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStopped
	})).Return(nil).Once()
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStatusChanged
	})).Return(nil).Once()

	err := service.KillContainer(context.Background(), "container-123")
	assert.NoError(t, err)
	assert.Equal(t, models.StatusStopped, service.containers["container-123"].Status)

	// Second kill - still succeeds but only publishes stopped event
	mockClient.On("KillContainer", mock.Anything, "container-123").Return(nil).Once()
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerStopped
	})).Return(nil).Once()

	err = service.KillContainer(context.Background(), "container-123")
	assert.NoError(t, err)
	assert.Equal(t, models.StatusStopped, service.containers["container-123"].Status)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_ListContainersByLabels_UpdatesCacheCorrectly(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	// Start with some containers already in cache
	existingContainer := &models.Container{
		ID:     "container-old",
		Name:   "old-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-old"] = existingContainer

	labels := map[string]string{"app": "noldarim"}

	newContainers := []*models.Container{
		{
			ID:     "container-1",
			Name:   "new-container-1",
			Status: models.StatusRunning,
		},
		{
			ID:     "container-2",
			Name:   "new-container-2",
			Status: models.StatusStopped,
		},
	}

	mockClient.On("ListContainersByLabels", mock.Anything, labels).Return(newContainers, nil)

	result, err := service.ListContainersByLabels(context.Background(), labels)

	require.NoError(t, err)
	assert.Len(t, result, 2)

	// Verify cache is updated with new containers
	assert.Equal(t, newContainers[0], service.containers["container-1"])
	assert.Equal(t, newContainers[1], service.containers["container-2"])
	// Old container still in cache
	assert.Equal(t, existingContainer, service.containers["container-old"])

	mockClient.AssertExpectations(t)
}

func TestService_CopyFileToContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	srcPath := "/host/path/test.txt"
	dstPath := "/container/path/test.txt"

	mockClient.On("CopyToContainer", mock.Anything, "container-123", srcPath, dstPath).Return(nil)

	err := service.CopyFileToContainer(context.Background(), "container-123", srcPath, dstPath)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "Publish") // No events for successful copy
}

func TestService_CopyFileToContainer_ContainerNotFound(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	srcPath := "/host/path/test.txt"
	dstPath := "/container/path/test.txt"

	// Mock InspectContainer to return error (container not found)
	mockClient.On("InspectContainer", mock.Anything, "container-123").Return(nil, fmt.Errorf("container not found"))

	err := service.CopyFileToContainer(context.Background(), "container-123", srcPath, dstPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")
	mockClient.AssertNotCalled(t, "CopyToContainer")
	mockPublisher.AssertNotCalled(t, "Publish")
}

func TestService_CopyFileToContainer_ClientError(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	srcPath := "/host/path/test.txt"
	dstPath := "/container/path/test.txt"
	expectedError := fmt.Errorf("copy failed")

	mockClient.On("CopyToContainer", mock.Anything, "container-123", srcPath, dstPath).Return(expectedError)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerFailed
	})).Return(nil)

	err := service.CopyFileToContainer(context.Background(), "container-123", srcPath, dstPath)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_CopyFileFromContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	srcPath := "/container/path/test.txt"
	dstPath := "/host/path/test.txt"

	mockClient.On("CopyFromContainer", mock.Anything, "container-123", srcPath, dstPath).Return(nil)

	err := service.CopyFileFromContainer(context.Background(), "container-123", srcPath, dstPath)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "Publish") // No events for successful copy
}

func TestService_CopyFileFromContainer_ContainerNotFound(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	srcPath := "/container/path/test.txt"
	dstPath := "/host/path/test.txt"

	err := service.CopyFileFromContainer(context.Background(), "container-123", srcPath, dstPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")
	mockClient.AssertNotCalled(t, "CopyFromContainer")
	mockPublisher.AssertNotCalled(t, "Publish")
}

func TestService_CopyFileFromContainer_ClientError(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	srcPath := "/container/path/test.txt"
	dstPath := "/host/path/test.txt"
	expectedError := fmt.Errorf("copy from container failed")

	mockClient.On("CopyFromContainer", mock.Anything, "container-123", srcPath, dstPath).Return(expectedError)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerFailed
	})).Return(nil)

	err := service.CopyFileFromContainer(context.Background(), "container-123", srcPath, dstPath)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_ExecContainer_Success(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	cmd := []string{"echo", "hello world"}
	workDir := "/workspace"

	expectedResult := &models.ExecResult{
		ExitCode: 0,
		Stdout:   "hello world\n",
		Stderr:   "",
	}

	mockClient.On("ExecContainer", mock.Anything, "container-123", cmd, workDir).Return(expectedResult, nil)

	result, err := service.ExecContainer(context.Background(), "container-123", cmd, workDir)

	require.NoError(t, err)
	assert.Equal(t, expectedResult.ExitCode, result.ExitCode)
	assert.Equal(t, expectedResult.Stdout, result.Stdout)
	assert.Equal(t, expectedResult.Stderr, result.Stderr)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "Publish") // No events for successful exec
}

func TestService_ExecContainer_ContainerNotInCache(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	cmd := []string{"echo", "test"}
	workDir := "/workspace"

	// Container not in cache, but exists in Docker
	dockerContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	expectedResult := &models.ExecResult{
		ExitCode: 0,
		Stdout:   "test\n",
		Stderr:   "",
	}

	mockClient.On("InspectContainer", mock.Anything, "container-123").Return(dockerContainer, nil)
	mockClient.On("ExecContainer", mock.Anything, "container-123", cmd, workDir).Return(expectedResult, nil)

	result, err := service.ExecContainer(context.Background(), "container-123", cmd, workDir)

	require.NoError(t, err)
	assert.Equal(t, expectedResult.ExitCode, result.ExitCode)
	assert.Equal(t, expectedResult.Stdout, result.Stdout)
	assert.Equal(t, expectedResult.Stderr, result.Stderr)

	// Verify container was added to cache
	assert.Equal(t, dockerContainer, service.containers["container-123"])

	mockClient.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "Publish")
}

func TestService_ExecContainer_ContainerNotFound(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	service := NewServiceWithClient(mockClient, mockPublisher)

	cmd := []string{"echo", "test"}
	workDir := "/workspace"

	mockClient.On("InspectContainer", mock.Anything, "container-123").Return(nil, fmt.Errorf("container not found"))

	result, err := service.ExecContainer(context.Background(), "container-123", cmd, workDir)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not found")

	mockClient.AssertNotCalled(t, "ExecContainer")
	mockPublisher.AssertNotCalled(t, "Publish")
	mockClient.AssertExpectations(t)
}

func TestService_ExecContainer_ClientError(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	cmd := []string{"invalid-command"}
	workDir := "/workspace"
	expectedError := fmt.Errorf("failed to execute command")

	mockClient.On("ExecContainer", mock.Anything, "container-123", cmd, workDir).Return((*models.ExecResult)(nil), expectedError)
	mockPublisher.On("Publish", mock.MatchedBy(func(event events.Event) bool {
		return event.Type == events.ContainerFailed
	})).Return(nil)

	result, err := service.ExecContainer(context.Background(), "container-123", cmd, workDir)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockClient.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestService_ExecContainer_NonZeroExitCode(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	cmd := []string{"ls", "/nonexistent"}
	workDir := "/workspace"

	expectedResult := &models.ExecResult{
		ExitCode: 2,
		Stdout:   "",
		Stderr:   "ls: cannot access '/nonexistent': No such file or directory\n",
	}

	mockClient.On("ExecContainer", mock.Anything, "container-123", cmd, workDir).Return(expectedResult, nil)

	result, err := service.ExecContainer(context.Background(), "container-123", cmd, workDir)

	require.NoError(t, err) // Service call succeeds even with non-zero exit code
	assert.Equal(t, 2, result.ExitCode)
	assert.Equal(t, "", result.Stdout)
	assert.Contains(t, result.Stderr, "No such file or directory")

	mockClient.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "Publish") // No events for command failure (non-zero exit code)
}

func TestService_ExecContainer_ComplexCommand(t *testing.T) {
	mockClient := &docker.MockClient{}
	mockPublisher := &events.MockPublisher{}

	existingContainer := &models.Container{
		ID:     "container-123",
		Name:   "test-container",
		Status: models.StatusRunning,
	}

	service := NewServiceWithClient(mockClient, mockPublisher)
	service.containers["container-123"] = existingContainer

	cmd := []string{"sh", "-c", "echo 'Processing task: task-123' && echo 'Task title: Test Task' && pwd"}
	workDir := "/workspace"

	expectedResult := &models.ExecResult{
		ExitCode: 0,
		Stdout:   "Processing task: task-123\nTask title: Test Task\n/workspace\n",
		Stderr:   "",
	}

	mockClient.On("ExecContainer", mock.Anything, "container-123", cmd, workDir).Return(expectedResult, nil)

	result, err := service.ExecContainer(context.Background(), "container-123", cmd, workDir)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "Processing task: task-123")
	assert.Contains(t, result.Stdout, "Task title: Test Task")
	assert.Contains(t, result.Stdout, "/workspace")

	mockClient.AssertExpectations(t)
	mockPublisher.AssertNotCalled(t, "Publish")
}

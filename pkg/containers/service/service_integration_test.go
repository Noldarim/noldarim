// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/noldarim/noldarim/pkg/containers/events"
	"github.com/noldarim/noldarim/pkg/containers/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Global cleanup tracker
var (
	testCleanup    = &cleanupFixture{}
	cleanupService *Service
)

// cleanupFixture tracks containers created during tests for cleanup
type cleanupFixture struct {
	containerIDs []string
	mutex        sync.Mutex
}

func (c *cleanupFixture) AddContainer(containerID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.containerIDs = append(c.containerIDs, containerID)
}

func (c *cleanupFixture) Cleanup(service *Service) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	ctx := context.Background()
	for _, containerID := range c.containerIDs {
		// Force delete any remaining containers
		if err := service.DeleteContainer(ctx, containerID, true); err != nil {
			fmt.Printf("Warning: Failed to cleanup container %s: %v\n", containerID, err)
		}
	}
	c.containerIDs = nil
}

// TestMain checks if Docker is available before running integration tests
func TestMain(m *testing.M) {
	// Skip integration tests if SKIP_INTEGRATION is set
	if os.Getenv("SKIP_INTEGRATION") != "" {
		fmt.Println("Skipping integration tests (SKIP_INTEGRATION is set)")
		os.Exit(0)
	}

	// Check if Docker is available
	service, err := NewService(&mockPublisher{})
	if err != nil {
		fmt.Printf("Docker not available, skipping integration tests: %v\n", err)
		os.Exit(0)
	}
	defer service.Close()

	// Store service for cleanup
	cleanupService = service

	// Print message about required images
	fmt.Println("Integration tests require 'alpine:latest' and 'nginx:alpine' images.")
	fmt.Println("If tests fail due to missing images, please run:")
	fmt.Println("  docker pull alpine:latest")
	fmt.Println("  docker pull nginx:alpine")

	// Run the tests
	code := m.Run()

	// Cleanup any remaining test containers
	fmt.Println("Cleaning up test containers...")
	testCleanup.Cleanup(service)

	os.Exit(code)
}

// mockPublisher is a simple event publisher for testing
type mockPublisher struct {
	events []events.Event
}

func (p *mockPublisher) Publish(event events.Event) error {
	p.events = append(p.events, event)
	return nil
}

func (p *mockPublisher) GetEvents() []events.Event {
	return p.events
}

func (p *mockPublisher) Clear() {
	p.events = nil
}

func TestServiceIntegration_CreateInspectStopDelete(t *testing.T) {
	publisher := &mockPublisher{}
	service, err := NewService(publisher)
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()

	// Test 1: Create Container
	t.Run("CreateContainer", func(t *testing.T) {
		containerName := fmt.Sprintf("test-create-%d", time.Now().UnixNano())
		config := models.ContainerConfig{
			Name:  containerName,
			Image: "alpine:latest",
			Environment: map[string]string{
				"TEST_VAR": "test_value",
			},
			Command: []string{"sleep", "30"},
		}
		publisher.Clear()

		container, err := service.CreateContainer(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, container)

		// Register for cleanup
		testCleanup.AddContainer(container.ID)
		defer service.DeleteContainer(ctx, container.ID, true)

		assert.NotEmpty(t, container.ID)
		assert.Equal(t, containerName, container.Name)
		assert.Equal(t, "alpine:latest", container.Image)
		assert.Equal(t, models.StatusCreated, container.Status)
		assert.Equal(t, "test_value", container.Environment["TEST_VAR"])

		// Verify event was published
		publishedEvents := publisher.GetEvents()
		require.Len(t, publishedEvents, 1)
		assert.Equal(t, events.ContainerCreated, publishedEvents[0].Type)
	})

	// Test 2: Inspect Container
	t.Run("InspectContainer", func(t *testing.T) {
		containerName := fmt.Sprintf("test-inspect-%d", time.Now().UnixNano())
		config := models.ContainerConfig{
			Name:  containerName,
			Image: "alpine:latest",
			Environment: map[string]string{
				"TEST_VAR": "test_value",
			},
			Command: []string{"sleep", "30"},
		}

		// First create a container to inspect
		container, err := service.CreateContainer(ctx, config)
		require.NoError(t, err)

		// Register for cleanup
		testCleanup.AddContainer(container.ID)
		defer service.DeleteContainer(ctx, container.ID, true)

		// Now inspect it
		inspected, err := service.GetContainer(ctx, container.ID)
		require.NoError(t, err)
		require.NotNil(t, inspected)

		assert.Equal(t, container.ID, inspected.ID)
		// Docker adds leading "/" to container names
		expectedName := containerName
		if inspected.Name[0] == '/' {
			expectedName = "/" + containerName
		}
		assert.Equal(t, expectedName, inspected.Name)
		assert.Equal(t, "alpine:latest", inspected.Image)
		assert.Contains(t, []models.ContainerStatus{models.StatusCreated, models.StatusRunning}, inspected.Status)
	})

	// Test 3: Start, Stop, and Delete Container
	t.Run("StartStopDeleteContainer", func(t *testing.T) {
		containerName := fmt.Sprintf("test-lifecycle-%d", time.Now().UnixNano())
		config := models.ContainerConfig{
			Name:  containerName,
			Image: "alpine:latest",
			Environment: map[string]string{
				"TEST_VAR": "test_value",
			},
			Command: []string{"sleep", "30"},
		}

		publisher.Clear()

		// Create container
		container, err := service.CreateContainer(ctx, config)
		require.NoError(t, err)

		// Register for cleanup
		testCleanup.AddContainer(container.ID)

		// Start container
		err = service.StartContainer(ctx, container.ID)
		require.NoError(t, err)

		// Verify container is running
		inspected, err := service.GetContainer(ctx, container.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusRunning, inspected.Status)

		// Stop container
		timeout := 5 * time.Second
		err = service.StopContainer(ctx, container.ID, &timeout)
		require.NoError(t, err)

		// Verify container is stopped
		time.Sleep(1 * time.Second) // Give Docker time to update status
		inspected, err = service.GetContainer(ctx, container.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusStopped, inspected.Status)

		// Delete container
		err = service.DeleteContainer(ctx, container.ID, false)
		require.NoError(t, err)

		// Verify container is deleted (should not be found)
		_, err = service.GetContainer(ctx, container.ID)
		assert.Error(t, err)

		// Verify events were published
		publishedEvents := publisher.GetEvents()
		assert.GreaterOrEqual(t, len(publishedEvents), 4) // Created, Started, Stopped, Deleted + status changes

		eventTypes := make(map[events.EventType]bool)
		for _, event := range publishedEvents {
			eventTypes[event.Type] = true
		}
		assert.True(t, eventTypes[events.ContainerCreated])
		assert.True(t, eventTypes[events.ContainerStarted])
		assert.True(t, eventTypes[events.ContainerStopped])
		assert.True(t, eventTypes[events.ContainerDeleted])
	})
}

func TestServiceIntegration_CreateWithPortsAndVolumes(t *testing.T) {
	publisher := &mockPublisher{}
	service, err := NewService(publisher)
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()
	containerName := fmt.Sprintf("test-container-advanced-%d", time.Now().Unix())

	// Create a temporary directory for volume mapping
	tempDir := t.TempDir()

	config := models.ContainerConfig{
		Name:  containerName,
		Image: "nginx:alpine",
		Ports: []models.PortMapping{
			{
				HostPort:      8080,
				ContainerPort: 80,
				Protocol:      "tcp",
			},
		},
		Volumes: []models.VolumeMapping{
			{
				HostPath:      tempDir,
				ContainerPath: "/usr/share/nginx/html",
				ReadOnly:      false,
			},
		},
		Environment: map[string]string{
			"NGINX_HOST": "localhost",
		},
	}

	t.Run("CreateContainerWithPortsAndVolumes", func(t *testing.T) {
		container, err := service.CreateContainer(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, container)

		// Register for cleanup
		testCleanup.AddContainer(container.ID)
		defer service.DeleteContainer(ctx, container.ID, true)

		assert.NotEmpty(t, container.ID)
		assert.Equal(t, containerName, container.Name)
		assert.Equal(t, "nginx:alpine", container.Image)
		assert.Len(t, container.Ports, 1)
		assert.Equal(t, 8080, container.Ports[0].HostPort)
		assert.Equal(t, 80, container.Ports[0].ContainerPort)
		assert.Len(t, container.Volumes, 1)
		assert.Equal(t, tempDir, container.Volumes[0].HostPath)
		assert.Equal(t, "/usr/share/nginx/html", container.Volumes[0].ContainerPath)
	})
}

func TestServiceIntegration_ListContainers(t *testing.T) {
	publisher := &mockPublisher{}
	service, err := NewService(publisher)
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()

	// Get initial container count
	initialContainers, err := service.ListContainers(ctx)
	require.NoError(t, err)
	initialCount := len(initialContainers)

	// Create a test container
	containerName := fmt.Sprintf("test-list-container-%d", time.Now().Unix())
	config := models.ContainerConfig{
		Name:    containerName,
		Image:   "alpine:latest",
		Command: []string{"sleep", "30"},
	}

	container, err := service.CreateContainer(ctx, config)
	require.NoError(t, err)

	// Register for cleanup
	testCleanup.AddContainer(container.ID)
	defer service.DeleteContainer(ctx, container.ID, true)

	// List containers and verify our container is included
	containers, err := service.ListContainers(ctx)
	require.NoError(t, err)
	assert.Equal(t, initialCount+1, len(containers))

	// Find our container in the list
	found := false
	for _, c := range containers {
		if c.ID == container.ID {
			found = true
			// Docker adds leading "/" to container names
			expectedName := containerName
			if c.Name[0] == '/' {
				expectedName = "/" + containerName
			}
			assert.Equal(t, expectedName, c.Name)
			assert.Equal(t, "alpine:latest", c.Image)
			break
		}
	}
	assert.True(t, found, "Created container should be found in the list")
}

func TestServiceIntegration_ForceDelete(t *testing.T) {
	publisher := &mockPublisher{}
	service, err := NewService(publisher)
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()
	containerName := fmt.Sprintf("test-force-delete-%d", time.Now().Unix())

	config := models.ContainerConfig{
		Name:    containerName,
		Image:   "alpine:latest",
		Command: []string{"sleep", "60"},
	}

	t.Run("ForceDeleteRunningContainer", func(t *testing.T) {
		// Create and start container
		container, err := service.CreateContainer(ctx, config)
		require.NoError(t, err)

		// Register for cleanup
		testCleanup.AddContainer(container.ID)

		err = service.StartContainer(ctx, container.ID)
		require.NoError(t, err)

		// Verify it's running
		inspected, err := service.GetContainer(ctx, container.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusRunning, inspected.Status)

		// Force delete without stopping
		err = service.DeleteContainer(ctx, container.ID, true)
		require.NoError(t, err)

		// Verify container is deleted
		_, err = service.GetContainer(ctx, container.ID)
		assert.Error(t, err)
	})
}

func TestServiceIntegration_CopyFileToContainer(t *testing.T) {
	publisher := &mockPublisher{}
	service, err := NewService(publisher)
	require.NoError(t, err)
	defer service.Close()

	ctx := context.Background()
	containerName := fmt.Sprintf("test-copy-file-%d", time.Now().Unix())

	// Create a temporary directory and test file
	tempDir := t.TempDir()
	testFileName := "test-file.txt"
	testFilePath := filepath.Join(tempDir, testFileName)
	testContent := fmt.Sprintf("Hello from test at %d", time.Now().UnixNano())

	// Write test content to file
	err = os.WriteFile(testFilePath, []byte(testContent), 0644)
	require.NoError(t, err)

	config := models.ContainerConfig{
		Name:    containerName,
		Image:   "alpine:latest",
		Command: []string{"sleep", "60"}, // Keep container running
	}

	t.Run("CopyFileToRunningContainer", func(t *testing.T) {
		// Create and start container
		container, err := service.CreateContainer(ctx, config)
		require.NoError(t, err)

		// Register for cleanup
		testCleanup.AddContainer(container.ID)
		defer service.DeleteContainer(ctx, container.ID, true)

		err = service.StartContainer(ctx, container.ID)
		require.NoError(t, err)

		// Verify container is running
		inspected, err := service.GetContainer(ctx, container.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusRunning, inspected.Status)

		// Copy file to container
		containerFilePath := "/tmp/" + testFileName
		err = service.CopyFileToContainer(ctx, container.ID, testFilePath, containerFilePath)
		require.NoError(t, err, "Failed to copy file to container")

		// Verify the file was copied by reading it back from the container
		copiedFilePath := filepath.Join(tempDir, "copied-"+testFileName)
		err = service.CopyFileFromContainer(ctx, container.ID, containerFilePath, copiedFilePath)
		require.NoError(t, err, "Failed to copy file from container")

		// Read the copied file content and verify it matches
		copiedContent, err := os.ReadFile(copiedFilePath)
		require.NoError(t, err, "Failed to read copied file")
		assert.Equal(t, testContent, string(copiedContent), "File content should match original")

		t.Log("File copy operation completed successfully and verified")
	})
}

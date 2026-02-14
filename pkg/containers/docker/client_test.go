// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package docker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/pkg/containers/models"
)

func TestClient_CreateContainer_Success(t *testing.T) {
	mockClient := &MockClient{}

	config := models.ContainerConfig{
		Name:  "test-container",
		Image: "python:3.9",
		Environment: map[string]string{
			"PYTHONPATH": "/app",
		},
		Ports: []models.PortMapping{
			{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
		Volumes: []models.VolumeMapping{
			{HostPath: "/host/path", ContainerPath: "/container/path", ReadOnly: false},
		},
		WorkingDir: "/app",
		Command:    []string{"python", "main.py"},
	}

	expectedContainer := &models.Container{
		ID:          "container-123",
		Name:        "test-container",
		Image:       "python:3.9",
		Status:      models.StatusCreated,
		Environment: config.Environment,
		Ports:       config.Ports,
		Volumes:     config.Volumes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	mockClient.On("CreateContainer", mock.Anything, config).Return(expectedContainer, nil)

	result, err := mockClient.CreateContainer(context.Background(), config)

	require.NoError(t, err)
	assert.Equal(t, expectedContainer.ID, result.ID)
	assert.Equal(t, expectedContainer.Name, result.Name)
	assert.Equal(t, expectedContainer.Image, result.Image)
	assert.Equal(t, expectedContainer.Status, result.Status)

	mockClient.AssertExpectations(t)
}

func TestClient_CreateContainer_Error(t *testing.T) {
	mockClient := &MockClient{}

	config := models.ContainerConfig{
		Name:  "test-container",
		Image: "python:3.9",
	}

	expectedError := fmt.Errorf("docker error")
	mockClient.On("CreateContainer", mock.Anything, config).Return((*models.Container)(nil), expectedError)

	result, err := mockClient.CreateContainer(context.Background(), config)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockClient.AssertExpectations(t)
}

func TestClient_StartContainer_Success(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"

	mockClient.On("StartContainer", mock.Anything, containerID).Return(nil)

	err := mockClient.StartContainer(context.Background(), containerID)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClient_StartContainer_Error(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	expectedError := fmt.Errorf("start error")

	mockClient.On("StartContainer", mock.Anything, containerID).Return(expectedError)

	err := mockClient.StartContainer(context.Background(), containerID)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	mockClient.AssertExpectations(t)
}

func TestClient_StopContainer_Success(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	timeout := 30 * time.Second

	mockClient.On("StopContainer", mock.Anything, containerID, &timeout).Return(nil)

	err := mockClient.StopContainer(context.Background(), containerID, &timeout)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClient_StopContainer_NoTimeout(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"

	mockClient.On("StopContainer", mock.Anything, containerID, (*time.Duration)(nil)).Return(nil)

	err := mockClient.StopContainer(context.Background(), containerID, nil)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClient_RemoveContainer_Success(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"

	mockClient.On("RemoveContainer", mock.Anything, containerID, true).Return(nil)

	err := mockClient.RemoveContainer(context.Background(), containerID, true)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClient_InspectContainer_Success(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"

	expectedContainer := &models.Container{
		ID:     containerID,
		Name:   "test-container",
		Image:  "python:3.9",
		Status: models.StatusRunning,
		Environment: map[string]string{
			"PYTHONPATH": "/app",
			"DEBUG":      "true",
		},
	}

	mockClient.On("InspectContainer", mock.Anything, containerID).Return(expectedContainer, nil)

	result, err := mockClient.InspectContainer(context.Background(), containerID)

	require.NoError(t, err)
	assert.Equal(t, expectedContainer.ID, result.ID)
	assert.Equal(t, expectedContainer.Name, result.Name)
	assert.Equal(t, expectedContainer.Image, result.Image)
	assert.Equal(t, expectedContainer.Status, result.Status)

	mockClient.AssertExpectations(t)
}

func TestClient_InspectContainer_Error(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	expectedError := fmt.Errorf("inspect error")

	mockClient.On("InspectContainer", mock.Anything, containerID).Return((*models.Container)(nil), expectedError)

	result, err := mockClient.InspectContainer(context.Background(), containerID)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockClient.AssertExpectations(t)
}

func TestClient_ListContainers_Success(t *testing.T) {
	mockClient := &MockClient{}

	expectedContainers := []*models.Container{
		{ID: "container-1", Name: "test-1", Status: models.StatusRunning},
		{ID: "container-2", Name: "test-2", Status: models.StatusStopped},
	}

	mockClient.On("ListContainers", mock.Anything).Return(expectedContainers, nil)

	result, err := mockClient.ListContainers(context.Background())

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, expectedContainers[0], result[0])
	assert.Equal(t, expectedContainers[1], result[1])

	mockClient.AssertExpectations(t)
}

func TestClient_Close_Success(t *testing.T) {
	mockClient := &MockClient{}

	mockClient.On("Close").Return(nil)

	err := mockClient.Close()

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestEnvMapToSlice(t *testing.T) {
	envMap := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	result := envMapToSlice(envMap)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "KEY1=value1")
	assert.Contains(t, result, "KEY2=value2")
}

func TestEnvSliceToMap(t *testing.T) {
	envSlice := []string{
		"KEY1=value1",
		"KEY2=value2",
		"KEY3=value=with=equals",
		"EMPTY_KEY=",
		"INVALID",
	}

	result := envSliceToMap(envSlice)

	assert.Equal(t, "value1", result["KEY1"])
	assert.Equal(t, "value2", result["KEY2"])
	assert.Equal(t, "value=with=equals", result["KEY3"])
	assert.Equal(t, "", result["EMPTY_KEY"])
	assert.NotContains(t, result, "INVALID")
}

func TestClient_CopyToContainer_Success(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	srcPath := "/host/path/test.txt"
	dstPath := "/container/path/test.txt"

	mockClient.On("CopyToContainer", mock.Anything, containerID, srcPath, dstPath).Return(nil)

	err := mockClient.CopyToContainer(context.Background(), containerID, srcPath, dstPath)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClient_CopyToContainer_Error(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	srcPath := "/host/path/test.txt"
	dstPath := "/container/path/test.txt"
	expectedError := fmt.Errorf("copy failed")

	mockClient.On("CopyToContainer", mock.Anything, containerID, srcPath, dstPath).Return(expectedError)

	err := mockClient.CopyToContainer(context.Background(), containerID, srcPath, dstPath)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	mockClient.AssertExpectations(t)
}

func TestClient_CopyFromContainer_Success(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	srcPath := "/container/path/test.txt"
	dstPath := "/host/path/test.txt"

	mockClient.On("CopyFromContainer", mock.Anything, containerID, srcPath, dstPath).Return(nil)

	err := mockClient.CopyFromContainer(context.Background(), containerID, srcPath, dstPath)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestClient_CopyFromContainer_Error(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	srcPath := "/container/path/test.txt"
	dstPath := "/host/path/test.txt"
	expectedError := fmt.Errorf("copy from container failed")

	mockClient.On("CopyFromContainer", mock.Anything, containerID, srcPath, dstPath).Return(expectedError)

	err := mockClient.CopyFromContainer(context.Background(), containerID, srcPath, dstPath)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	mockClient.AssertExpectations(t)
}

func TestClient_ExecContainer_Success(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	cmd := []string{"echo", "hello world"}
	workDir := "/workspace"

	expectedResult := &models.ExecResult{
		ExitCode: 0,
		Stdout:   "hello world\n",
		Stderr:   "",
	}

	mockClient.On("ExecContainer", mock.Anything, containerID, cmd, workDir).Return(expectedResult, nil)

	result, err := mockClient.ExecContainer(context.Background(), containerID, cmd, workDir)

	require.NoError(t, err)
	assert.Equal(t, expectedResult.ExitCode, result.ExitCode)
	assert.Equal(t, expectedResult.Stdout, result.Stdout)
	assert.Equal(t, expectedResult.Stderr, result.Stderr)

	mockClient.AssertExpectations(t)
}

func TestClient_ExecContainer_NonZeroExitCode(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	cmd := []string{"ls", "/nonexistent"}
	workDir := "/workspace"

	expectedResult := &models.ExecResult{
		ExitCode: 2,
		Stdout:   "",
		Stderr:   "ls: cannot access '/nonexistent': No such file or directory\n",
	}

	mockClient.On("ExecContainer", mock.Anything, containerID, cmd, workDir).Return(expectedResult, nil)

	result, err := mockClient.ExecContainer(context.Background(), containerID, cmd, workDir)

	require.NoError(t, err)
	assert.Equal(t, 2, result.ExitCode)
	assert.Equal(t, "", result.Stdout)
	assert.Contains(t, result.Stderr, "No such file or directory")

	mockClient.AssertExpectations(t)
}

func TestClient_ExecContainer_Error(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "nonexistent-container"
	cmd := []string{"echo", "test"}
	workDir := "/workspace"
	expectedError := fmt.Errorf("container not found")

	mockClient.On("ExecContainer", mock.Anything, containerID, cmd, workDir).Return((*models.ExecResult)(nil), expectedError)

	result, err := mockClient.ExecContainer(context.Background(), containerID, cmd, workDir)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockClient.AssertExpectations(t)
}

func TestClient_ExecContainer_EmptyWorkDir(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	cmd := []string{"pwd"}
	workDir := ""

	expectedResult := &models.ExecResult{
		ExitCode: 0,
		Stdout:   "/\n",
		Stderr:   "",
	}

	mockClient.On("ExecContainer", mock.Anything, containerID, cmd, workDir).Return(expectedResult, nil)

	result, err := mockClient.ExecContainer(context.Background(), containerID, cmd, workDir)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Equal(t, "/\n", result.Stdout)

	mockClient.AssertExpectations(t)
}

func TestClient_ExecContainer_ComplexCommand(t *testing.T) {
	mockClient := &MockClient{}
	containerID := "container-123"
	cmd := []string{"sh", "-c", "echo 'Task title: Test Task' && echo 'Task ID: task-123'"}
	workDir := "/workspace"

	expectedResult := &models.ExecResult{
		ExitCode: 0,
		Stdout:   "Task title: Test Task\nTask ID: task-123\n",
		Stderr:   "",
	}

	mockClient.On("ExecContainer", mock.Anything, containerID, cmd, workDir).Return(expectedResult, nil)

	result, err := mockClient.ExecContainer(context.Background(), containerID, cmd, workDir)

	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, result.Stdout, "Task title: Test Task")
	assert.Contains(t, result.Stdout, "Task ID: task-123")

	mockClient.AssertExpectations(t)
}

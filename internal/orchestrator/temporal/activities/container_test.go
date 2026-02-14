// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/pkg/containers/models"
)

// MockContainerServiceForTest implements ContainerServiceInterface for testing
type MockContainerServiceForTest struct {
	mock.Mock
}

// Core container lifecycle methods
func (m *MockContainerServiceForTest) CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error) {
	args := m.Called(ctx, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockContainerServiceForTest) StartContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockContainerServiceForTest) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error {
	args := m.Called(ctx, containerID, timeout)
	return args.Error(0)
}

func (m *MockContainerServiceForTest) DeleteContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func (m *MockContainerServiceForTest) GetContainer(ctx context.Context, containerID string) (*models.Container, error) {
	args := m.Called(ctx, containerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockContainerServiceForTest) ListContainers(ctx context.Context) ([]*models.Container, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Container), args.Error(1)
}

func (m *MockContainerServiceForTest) RefreshContainer(ctx context.Context, containerID string) (*models.Container, error) {
	args := m.Called(ctx, containerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockContainerServiceForTest) ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error) {
	args := m.Called(ctx, labels)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Container), args.Error(1)
}

func (m *MockContainerServiceForTest) KillContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

// File operations
func (m *MockContainerServiceForTest) CopyFileToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	args := m.Called(ctx, containerID, srcPath, dstPath)
	return args.Error(0)
}

func (m *MockContainerServiceForTest) CopyFileFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	args := m.Called(ctx, containerID, srcPath, dstPath)
	return args.Error(0)
}

func (m *MockContainerServiceForTest) WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error {
	args := m.Called(ctx, containerID, content, dstPath)
	return args.Error(0)
}

// Command execution
func (m *MockContainerServiceForTest) ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error) {
	args := m.Called(ctx, containerID, cmd, workDir)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ExecResult), args.Error(1)
}

// Cleanup
func (m *MockContainerServiceForTest) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Ensure MockContainerServiceForTest implements ContainerServiceInterface
var _ ContainerServiceInterface = (*MockContainerServiceForTest)(nil)

// ExecuteCommandActivity tests removed - this activity is no longer used by the main orchestrator.
// Command execution is now handled by LocalExecuteActivity in the agent.

func TestContainerActivities_NewContainerActivities(t *testing.T) {
	// Test the constructor
	mockService := &MockContainerServiceForTest{}
	cfg := &config.AppConfig{}

	activities := NewContainerActivities(mockService, cfg)

	assert.NotNil(t, activities)
	assert.Equal(t, cfg, activities.config)
	assert.NotNil(t, activities.containerService)
}

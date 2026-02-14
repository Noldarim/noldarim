// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package docker

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/noldarim/noldarim/pkg/containers/models"
)

// MockClient is a mock implementation of ClientInterface
type MockClient struct {
	mock.Mock
}

func (m *MockClient) CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error) {
	args := m.Called(ctx, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockClient) StartContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockClient) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error {
	args := m.Called(ctx, containerID, timeout)
	return args.Error(0)
}

func (m *MockClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func (m *MockClient) InspectContainer(ctx context.Context, containerID string) (*models.Container, error) {
	args := m.Called(ctx, containerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Container), args.Error(1)
}

func (m *MockClient) ListContainers(ctx context.Context) ([]*models.Container, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Container), args.Error(1)
}

func (m *MockClient) ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error) {
	args := m.Called(ctx, labels)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Container), args.Error(1)
}

func (m *MockClient) KillContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockClient) CopyToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	args := m.Called(ctx, containerID, srcPath, dstPath)
	return args.Error(0)
}

func (m *MockClient) CopyFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	args := m.Called(ctx, containerID, srcPath, dstPath)
	return args.Error(0)
}

func (m *MockClient) WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error {
	args := m.Called(ctx, containerID, content, dstPath)
	return args.Error(0)
}

func (m *MockClient) ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error) {
	args := m.Called(ctx, containerID, cmd, workDir)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ExecResult), args.Error(1)
}

func (m *MockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

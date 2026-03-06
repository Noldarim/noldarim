// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package containers

import (
	"context"
	"time"

	"github.com/noldarim/noldarim/pkg/containers/models"
)

// Backend defines the container operations interface.
// Implementations: docker.Client (local Docker daemon).
type Backend interface {
	CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	InspectContainer(ctx context.Context, containerID string) (*models.Container, error)
	ListContainers(ctx context.Context) ([]*models.Container, error)
	ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error)
	KillContainer(ctx context.Context, containerID string) error
	CopyToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error
	CopyFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error
	WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error
	ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error)
	GetContainerLogs(ctx context.Context, containerID string, tail string) (string, string, error)
	Close() error
}

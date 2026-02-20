// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/pkg/containers/models"
	"github.com/noldarim/noldarim/pkg/containers/validation"

	"go.temporal.io/sdk/activity"
)

// ContainerServiceInterface defines the methods needed by container activities
// This comprehensive interface includes all methods needed by both container and agent setup activities
type ContainerServiceInterface interface {
	// Core container lifecycle methods
	CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error
	DeleteContainer(ctx context.Context, containerID string, force bool) error
	GetContainer(ctx context.Context, containerID string) (*models.Container, error)
	ListContainers(ctx context.Context) ([]*models.Container, error)
	RefreshContainer(ctx context.Context, containerID string) (*models.Container, error)
	ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error)
	KillContainer(ctx context.Context, containerID string) error

	// File operations
	CopyFileToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error
	CopyFileFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error
	WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error

	// Command execution
	ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error)

	// Cleanup
	Close() error
}

// ContainerActivities provides container-related activities
type ContainerActivities struct {
	containerService ContainerServiceInterface
	config           *config.AppConfig
}

// NewContainerActivities creates a new instance of ContainerActivities
func NewContainerActivities(containerService ContainerServiceInterface, config *config.AppConfig) *ContainerActivities {
	return &ContainerActivities{
		containerService: containerService,
		config:           config,
	}
}

// CreateContainerActivity creates a new container for a task with idempotency
func (a *ContainerActivities) CreateContainerActivity(ctx context.Context, input types.CreateContainerActivityInput) (*types.CreateContainerActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating container", "taskID", input.TaskID, "worktreePath", input.WorktreePath)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Checking for existing container")

	// Container name with timestamp for uniqueness while maintaining readability
	timestamp := time.Now().Unix()
	containerName := fmt.Sprintf("task-%s-%d", input.TaskID, timestamp)

	// Check if container already exists
	existingContainers, err := a.containerService.ListContainersByLabels(ctx, map[string]string{
		"noldarim.task.id": input.TaskID,
	})
	if err != nil {
		logger.Warn("Failed to check for existing containers", "error", err)
		// Continue anyway - we'll handle conflicts below
	} else if len(existingContainers) > 0 {
		// Found existing container
		existingContainer := existingContainers[0]
		if existingContainer.Status == models.StatusRunning {
			logger.Info("Container already exists and is running", "containerID", existingContainer.ID)
			return &types.CreateContainerActivityOutput{
				ContainerID: existingContainer.ID,
				Status:      string(existingContainer.Status),
			}, nil
		}
		// Container exists but not running - remove it
		logger.Info("Removing stopped container", "containerID", existingContainer.ID)
		if err := a.containerService.DeleteContainer(ctx, existingContainer.ID, true); err != nil {
			logger.Warn("Failed to remove stopped container", "error", err)
		}
	}

	// Get config from context or use defaults
	config := a.getConfig(ctx)

	// Record heartbeat before potentially long operation
	activity.RecordHeartbeat(ctx, "Creating container")

	// Fix Temporal host for container connectivity
	temporalHost := config.Temporal.HostPort
	// On Docker Desktop (macOS/Windows), containers need to use host.docker.internal
	// to reach services running on the host machine
	if strings.HasPrefix(temporalHost, "localhost:") || strings.HasPrefix(temporalHost, "127.0.0.1:") {
		port := strings.Split(temporalHost, ":")[1]
		temporalHost = "host.docker.internal:" + port
		logger.Info("Rewriting Temporal host for container", "original", config.Temporal.HostPort, "container", temporalHost)
	}

	// Create container configuration
	// Workflow ID follows the pattern used in CreateTaskWorkflow
	workflowID := fmt.Sprintf("process-task-%s", input.TaskID)

	containerConfig := models.ContainerConfig{
		Name:        containerName,
		Image:       config.Container.DefaultImage,
		Command:     []string{"/app/agent"}, // Run temporal agent
		WorkingDir:  config.Container.WorkspaceDir,
		NetworkMode: config.Container.NetworkMode,
		Environment: map[string]string{
			"TASK_ID":             input.TaskID,
			"PROJECT_ID":          input.ProjectID,
			"TEMPORAL_HOST_PORT":  temporalHost,
			"TEMPORAL_NAMESPACE":  config.Temporal.Namespace,
			"TEMPORAL_TASK_QUEUE": input.TaskQueue,
			"WORKFLOW_ID":         workflowID, // For signaling AI activity events
		},
		Volumes: []models.VolumeMapping{
			{
				HostPath:      input.WorktreePath,
				ContainerPath: config.Container.WorkspaceDir,
				ReadOnly:      false,
			},
		},
		Labels: map[string]string{
			"noldarim.task.id":    input.TaskID,
			"noldarim.project.id": input.ProjectID,
			"noldarim.managed":    "true",
		},
		MemoryMB:  config.Container.ResourceLimits.MemoryMB,
		CPUShares: config.Container.ResourceLimits.CPUShares,
		TaskID:    input.TaskID,
	}

	// Add any additional configured environment variables
	maps.Copy(containerConfig.Environment, config.Container.Environment)

	// Validate container labels and environment variables
	if err := validation.ValidateContainerLabels(containerConfig.Labels); err != nil {
		return nil, fmt.Errorf("invalid container labels: %w", err)
	}

	if err := validation.ValidateEnvironmentVariables(containerConfig.Environment); err != nil {
		return nil, fmt.Errorf("invalid environment variables: %w", err)
	}

	// Create the container using the container service
	container, err := a.containerService.CreateContainer(ctx, containerConfig)
	if err != nil {
		// Check if it's a name conflict error
		if strings.Contains(err.Error(), "Conflict") || strings.Contains(err.Error(), "already in use") {
			// Try to get the existing container
			logger.Info("Container name conflict, checking existing container")
			existingContainers, err := a.containerService.ListContainersByLabels(ctx, map[string]string{
				"noldarim.task.id": input.TaskID,
			})
			if err == nil && len(existingContainers) > 0 {
				return &types.CreateContainerActivityOutput{
					ContainerID: existingContainers[0].ID,
					Status:      string(existingContainers[0].Status),
				}, nil
			}
		}
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Record heartbeat before starting
	activity.RecordHeartbeat(ctx, "Starting container")

	// Start the container
	if err := a.containerService.StartContainer(ctx, container.ID); err != nil {
		// Cleanup the created container
		if cleanupErr := a.containerService.DeleteContainer(ctx, container.ID, true); cleanupErr != nil {
			logger.Warn("Failed to cleanup container after start failure", "containerID", container.ID, "cleanupError", cleanupErr)
		}
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	logger.Info("Successfully created and started container", "containerID", container.ID)
	return &types.CreateContainerActivityOutput{
		ContainerID: container.ID,
		Status:      string(models.StatusRunning),
	}, nil
}

// StopContainerActivity stops a running container (compensation activity)
func (a *ContainerActivities) StopContainerActivity(ctx context.Context, containerID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Stopping container", "containerID", containerID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Checking container status")

	// Check if container exists
	container, err := a.containerService.GetContainer(ctx, containerID)
	if err != nil {
		// Container doesn't exist - idempotent success
		if strings.Contains(err.Error(), "not found") {
			logger.Info("Container not found, already removed", "containerID", containerID)
			return nil
		}
		return fmt.Errorf("failed to get container: %w", err)
	}

	// Check if already stopped
	if container.Status != models.StatusRunning {
		logger.Info("Container not running", "containerID", containerID, "status", container.Status)
		// Remove the container if it's stopped
		if err := a.containerService.DeleteContainer(ctx, containerID, true); err != nil {
			logger.Warn("Failed to remove stopped container", "error", err)
		}
		return nil
	}

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Stopping container")

	// Stop the container using the container service
	config := a.getConfig(ctx)
	timeout := config.Container.Timeouts.StopTimeout
	if err := a.containerService.StopContainer(ctx, containerID, &timeout); err != nil {
		// If stop fails, try to kill
		logger.Warn("Failed to stop container gracefully, trying to kill", "error", err)
		if killErr := a.containerService.KillContainer(ctx, containerID); killErr != nil {
			return fmt.Errorf("failed to stop/kill container: stop error: %w, kill error: %v", err, killErr)
		}
	}

	// Remove the container after stopping
	if err := a.containerService.DeleteContainer(ctx, containerID, true); err != nil {
		logger.Warn("Failed to remove container after stopping", "error", err)
		// Don't fail - container is stopped which is the main goal
	}

	logger.Info("Successfully stopped and removed container", "containerID", containerID)
	return nil
}

// RemoveContainerActivity removes a container
func (a *ContainerActivities) RemoveContainerActivity(ctx context.Context, containerID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Removing container", "containerID", containerID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Removing container")

	// Remove the container using the container service
	if err := a.containerService.DeleteContainer(ctx, containerID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	logger.Info("Successfully removed container", "containerID", containerID)
	return nil
}

// GetContainerStatusActivity gets the status of a container
func (a *ContainerActivities) GetContainerStatusActivity(ctx context.Context, containerID string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting container status", "containerID", containerID)

	// Get container status using the container service
	container, err := a.containerService.GetContainer(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to get container status: %w", err)
	}

	return string(container.Status), nil
}

// ExecuteCommandActivity executes a command in a running container
func (a *ContainerActivities) ExecuteCommandActivity(ctx context.Context, input types.ExecuteCommandActivityInput) (*types.ExecuteCommandActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing command in container", "containerID", input.ContainerID, "command", input.Command, "workDir", input.WorkDir)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Executing command in container")

	// Execute the command using the container service
	result, err := a.containerService.ExecContainer(ctx, input.ContainerID, input.Command, input.WorkDir)
	if err != nil {
		logger.Error("Failed to execute command in container", "error", err)
		return &types.ExecuteCommandActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("Failed to execute command: %v", err),
		}, err
	}

	logger.Info("Successfully executed command in container",
		"containerID", input.ContainerID,
		"exitCode", result.ExitCode,
		"stdoutLength", len(result.Stdout),
		"stderrLength", len(result.Stderr))

	return &types.ExecuteCommandActivityOutput{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		Success:  result.ExitCode == 0,
		Error:    "",
	}, nil
}

// getConfig returns the configuration, either from context or the default
func (a *ContainerActivities) getConfig(ctx context.Context) *config.AppConfig {
	// Try to get config from context first
	if cfg, ok := ctx.Value("config").(*config.AppConfig); ok && cfg != nil {
		return cfg
	}
	// Fall back to the config provided during initialization
	return a.config
}

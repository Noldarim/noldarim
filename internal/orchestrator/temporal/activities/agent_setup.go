// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/activity"
)

// AgentSetupActivities provides agent setup activities for containers
type AgentSetupActivities struct {
	containerService ContainerServiceInterface
	config           *config.AppConfig
}

// NewAgentSetupActivities creates a new instance of AgentSetupActivities
func NewAgentSetupActivities(containerService ContainerServiceInterface, config *config.AppConfig) *AgentSetupActivities {
	return &AgentSetupActivities{
		containerService: containerService,
		config:           config,
	}
}

// CopyClaudeConfigActivity copies ~/.claude.json from host to container with idempotency
func (a *AgentSetupActivities) CopyClaudeConfigActivity(ctx context.Context, input types.CopyClaudeConfigActivityInput) (*types.CopyClaudeConfigActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Debug("Starting CopyClaudeConfigActivity", "containerID", input.ContainerID, "hostConfigPath", input.HostConfigPath)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Checking container status")

	// Verify container exists and is running
	container, err := a.containerService.GetContainer(ctx, input.ContainerID)
	if err != nil {
		return &types.CopyClaudeConfigActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to get container: %v", err),
		}, fmt.Errorf("failed to get container: %w", err)
	}

	if container.Status != "running" {
		return &types.CopyClaudeConfigActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("container is not running: %s", container.Status),
		}, fmt.Errorf("container is not running: %s", container.Status)
	}

	// Record heartbeat before copy operation
	activity.RecordHeartbeat(ctx, "Copying Claude config file")

	logger.Info("DEBUG: Copying fresh Claude config file to container", "containerID", input.ContainerID)

	// Copy the file using the container service
	if err := a.containerService.CopyFileToContainer(ctx, input.ContainerID, input.HostConfigPath, "/home/noldarim/.claude.json"); err != nil {
		logger.Error("Failed to copy Claude config", "error", err)
		return &types.CopyClaudeConfigActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to copy config file: %v", err),
		}, fmt.Errorf("failed to copy config file: %w", err)
	}

	// Verify the file was copied successfully
	verifyCmd := exec.CommandContext(ctx, "docker", "exec", input.ContainerID, "test", "-f", "/home/noldarim/.claude.json")
	if err := verifyCmd.Run(); err != nil {
		logger.Error("Claude config verification failed", "error", err)
		return &types.CopyClaudeConfigActivityOutput{
			Success: false,
			Error:   "config file copy verification failed",
		}, fmt.Errorf("config file copy verification failed: %w", err)
	}

	logger.Info("Claude config copied successfully", "containerID", input.ContainerID)
	return &types.CopyClaudeConfigActivityOutput{
		Success: true,
	}, nil
}

// getClaudeCredentialsFromKeychainFunc is a function variable that can be overridden in tests
var getClaudeCredentialsFromKeychainFunc = getClaudeCredentialsFromKeychainImpl

// getClaudeCredentialsFromKeychainImpl is the actual implementation
func getClaudeCredentialsFromKeychainImpl(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get credentials from keychain: %w", err)
	}
	return string(output), nil
}

// getClaudeCredentialsFromKeychain gets Claude credentials from macOS keychain
// This function is separated for easy mocking in tests
func getClaudeCredentialsFromKeychain(ctx context.Context) (string, error) {
	return getClaudeCredentialsFromKeychainFunc(ctx)
}

// GetClaudeCredentialsFromKeychainFunc returns the current function for getting Claude credentials
func GetClaudeCredentialsFromKeychainFunc() func(context.Context) (string, error) {
	return getClaudeCredentialsFromKeychainFunc
}

// SetClaudeCredentialsFromKeychainFunc sets the function for getting Claude credentials (for testing)
func SetClaudeCredentialsFromKeychainFunc(f func(context.Context) (string, error)) {
	getClaudeCredentialsFromKeychainFunc = f
}

// CopyClaudeCredentialsActivity obtains Claude credentials and writes them to container
func (a *AgentSetupActivities) CopyClaudeCredentialsActivity(ctx context.Context, input types.CopyClaudeCredentialsActivityInput) (*types.CopyClaudeCredentialsActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Copying Claude credentials to container", "containerID", input.ContainerID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Checking host architecture")

	// Check host architecture
	if runtime.GOOS != "darwin" {
		return &types.CopyClaudeCredentialsActivityOutput{
			Success: false,
			Error:   "Sorry, right now only MacOS credentials obtaining for claude is supported",
		}, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	// Record heartbeat before getting credentials
	activity.RecordHeartbeat(ctx, "Getting Claude credentials from keychain")

	// Get Claude credentials from keychain
	claudeJSONCredentials, err := getClaudeCredentialsFromKeychain(ctx)
	if err != nil {
		logger.Error("Failed to get Claude credentials from keychain", "error", err)
		return &types.CopyClaudeCredentialsActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to get credentials from keychain: %v", err),
		}, fmt.Errorf("failed to get credentials from keychain: %w", err)
	}

	// Record heartbeat before validating JSON
	activity.RecordHeartbeat(ctx, "Validating credentials JSON")

	// Verify that claude_json_credentials is valid JSON
	if !json.Valid([]byte(claudeJSONCredentials)) {
		logger.Error("Invalid JSON credentials received from keychain")
		return &types.CopyClaudeCredentialsActivityOutput{
			Success: false,
			Error:   "invalid JSON credentials received from keychain",
		}, fmt.Errorf("invalid JSON credentials received from keychain")
	}

	// Record heartbeat before writing credentials
	activity.RecordHeartbeat(ctx, "Writing Claude credentials to container")

	// Write the credentials to the container using WriteToContainer method
	if err := a.containerService.WriteToContainer(ctx, input.ContainerID, claudeJSONCredentials, "/home/noldarim/.claude/.credentials.json"); err != nil {
		logger.Error("Failed to write Claude credentials", "error", err)
		return &types.CopyClaudeCredentialsActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to write credentials file: %v", err),
		}, fmt.Errorf("failed to write credentials file: %w", err)
	}

	logger.Info("Claude credentials copied successfully", "containerID", input.ContainerID)
	return &types.CopyClaudeCredentialsActivityOutput{
		Success: true,
	}, nil
}

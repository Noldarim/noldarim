// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	SetupWorkflowName    = "SetupWorkflow"
	SetupWorkflowVersion = "v2.1.0" // Bumped for saga pattern refactoring
)

// SetupWorkflow handles ALL setup for pipeline execution:
// 1. Resolves fork logic (determine start commit from parent run)
// 2. Creates PipelineRun record in DB
// 3. Creates git worktree at resolved commit
// 4. Creates container with mounted worktree
// 5. Copies Claude configuration and credentials
// 6. Updates PipelineRun with infrastructure info
//
// Uses the saga pattern for cleanup: compensations are accumulated as resources
// are created and executed in reverse order (LIFO) on failure.
//
// This is the single source of truth for all setup operations.
// Designed to be called as a child workflow from PipelineWorkflow.
func SetupWorkflow(ctx workflow.Context, input types.PipelineSetupInput) (*types.PipelineSetupOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting SetupWorkflow",
		"runID", input.RunID,
		"projectID", input.ProjectID,
		"branchName", input.BranchName)

	output := &types.PipelineSetupOutput{
		Success: false,
	}

	// Saga compensation tracker - will be executed in reverse order on failure
	var compensations []compensation

	// Configure activity options with retries
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Extended timeout for container creation (may need to pull images)
	containerCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	})

	// Orchestrator context for DB activities (cross-worker)
	orchestratorCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           input.OrchestratorTaskQueue,
		StartToCloseTimeout: 2 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	})

	// =========================================================================
	// Phase 1: Resolve fork logic and determine start commit
	// =========================================================================
	startCommit := input.StartCommitSHA
	if input.ForkFromRunID != "" && startCommit == "" {
		// Load parent run and get commit from fork point
		logger.Info("Resolving fork point",
			"parentRunID", input.ForkFromRunID,
			"forkAfterStep", input.ForkAfterStepID)

		var parentRun types.GetPipelineRunActivityOutput
		err := workflow.ExecuteActivity(orchestratorCtx, "GetPipelineRunActivity",
			types.GetPipelineRunActivityInput{RunID: input.ForkFromRunID}).Get(ctx, &parentRun)
		if err != nil {
			logger.Error("Failed to load parent run for fork", "error", err)
			output.Error = fmt.Sprintf("Failed to load parent run: %v", err)
			return output, err
		}
		if parentRun.Run != nil {
			startCommit = parentRun.Run.GetCommitAfterStep(input.ForkAfterStepID)
			logger.Info("Resolved fork start commit",
				"parentRunID", input.ForkFromRunID,
				"forkAfterStep", input.ForkAfterStepID,
				"startCommit", startCommit)
		}
	}
	if startCommit == "" {
		startCommit = input.BaseCommitSHA // Use base if no fork
	}
	output.StartCommitSHA = startCommit

	// Generate branch name if not provided
	branchName := input.BranchName
	if branchName == "" {
		branchName = fmt.Sprintf("pipeline/%s", input.RunID[:8])
	}
	output.BranchName = branchName

	// =========================================================================
	// Phase 2: Create PipelineRun record in DB
	// =========================================================================
	logger.Info("Creating PipelineRun record in DB")

	now := workflow.Now(ctx)
	pipelineRun := &models.PipelineRun{
		ID:                 input.RunID,
		PipelineID:         input.PipelineID,
		ProjectID:          input.ProjectID,
		Name:               input.Name,
		Status:             models.PipelineRunStatusRunning,
		ParentRunID:        input.ForkFromRunID,
		ForkAfterStepID:    input.ForkAfterStepID,
		StartCommitSHA:     startCommit,
		BaseCommitSHA:      input.BaseCommitSHA,
		BranchName:         branchName,
		TemporalWorkflowID: input.ParentWorkflowID,
		StartedAt:          &now,
	}

	err := workflow.ExecuteActivity(orchestratorCtx, "SavePipelineRunActivity",
		types.SavePipelineRunActivityInput{Run: pipelineRun}).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to save pipeline run to DB", "error", err)
		output.Error = fmt.Sprintf("Failed to save pipeline run: %v", err)
		return output, err
	}
	logger.Info("PipelineRun record created", "runID", input.RunID)

	// =========================================================================
	// Phase 3: Create infrastructure (with saga compensations)
	// =========================================================================

	// Step 3a: Create git worktree
	logger.Info("Creating git worktree", "branchName", branchName, "startCommit", startCommit)

	var worktreeResult types.CreateWorktreeActivityOutput
	err = workflow.ExecuteActivity(ctx, "CreateWorktreeActivity", types.CreateWorktreeActivityInput{
		TaskID:         input.RunID, // Use RunID for worktree naming
		BranchName:     branchName,
		RepositoryPath: input.RepositoryPath,
		BaseCommitSHA:  startCommit,
	}).Get(ctx, &worktreeResult)

	if err != nil {
		logger.Error("Failed to create git worktree", "error", err)
		output.Error = fmt.Sprintf("Failed to create worktree: %v", err)
		runCompensations(ctx, compensations)
		markSetupFailed(orchestratorCtx, input.RunID, output.Error)
		return output, err
	}

	// Add worktree compensation (will be cleaned up on subsequent failures)
	compensations = append(compensations, worktreeCompensation(worktreeResult.WorktreePath, input.RepositoryPath))
	output.WorktreePath = worktreeResult.WorktreePath
	logger.Info("Worktree created", "path", worktreeResult.WorktreePath)

	// Step 3b: Create container with mounted worktree
	logger.Info("Creating container")

	var containerResult types.CreateContainerActivityOutput
	err = workflow.ExecuteActivity(containerCtx, "CreateContainerActivity", types.CreateContainerActivityInput{
		TaskID:       input.RunID,
		WorktreePath: worktreeResult.WorktreePath,
		ProjectID:    input.ProjectID,
		TaskQueue:    input.TaskQueue,
	}).Get(containerCtx, &containerResult)

	if err != nil {
		logger.Error("Failed to create container", "error", err)
		output.Error = fmt.Sprintf("Failed to create container: %v", err)
		runCompensations(ctx, compensations)
		markSetupFailed(orchestratorCtx, input.RunID, output.Error)
		return output, err
	}

	// Add container compensation (will be cleaned up on subsequent failures)
	compensations = append(compensations, containerCompensation(containerResult.ContainerID))
	output.ContainerID = containerResult.ContainerID
	logger.Info("Container created", "containerID", containerResult.ContainerID)

	// Step 3c: Copy Claude configuration
	logger.Info("Copying Claude configuration")

	var configResult types.CopyClaudeConfigActivityOutput
	err = workflow.ExecuteActivity(ctx, "CopyClaudeConfigActivity", types.CopyClaudeConfigActivityInput{
		ContainerID:    containerResult.ContainerID,
		HostConfigPath: input.ClaudeConfigPath,
	}).Get(ctx, &configResult)

	if err != nil {
		logger.Error("Failed to copy Claude config", "error", err)
		output.Error = fmt.Sprintf("Failed to copy Claude config: %v", err)
		runCompensations(ctx, compensations)
		markSetupFailed(orchestratorCtx, input.RunID, output.Error)
		return output, err
	}

	logger.Info("Claude config copied", "success", configResult.Success)

	// Step 3d: Copy Claude credentials
	logger.Info("Copying Claude credentials")

	var credentialsResult types.CopyClaudeCredentialsActivityOutput
	err = workflow.ExecuteActivity(ctx, "CopyClaudeCredentialsActivity", types.CopyClaudeCredentialsActivityInput{
		ContainerID: containerResult.ContainerID,
	}).Get(ctx, &credentialsResult)

	if err != nil {
		logger.Error("Failed to copy Claude credentials", "error", err)
		output.Error = fmt.Sprintf("Failed to copy Claude credentials: %v", err)
		runCompensations(ctx, compensations)
		markSetupFailed(orchestratorCtx, input.RunID, output.Error)
		return output, err
	}

	logger.Info("Claude credentials copied", "success", credentialsResult.Success)

	// =========================================================================
	// Phase 4: Update PipelineRun with infrastructure info
	// =========================================================================
	logger.Info("Updating PipelineRun with infrastructure info")

	pipelineRun.WorktreePath = worktreeResult.WorktreePath
	pipelineRun.ContainerID = containerResult.ContainerID

	err = workflow.ExecuteActivity(orchestratorCtx, "SavePipelineRunActivity",
		types.SavePipelineRunActivityInput{Run: pipelineRun}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to update pipeline run with infrastructure info", "error", err)
		// Non-fatal: infrastructure is created, proceed
	}

	// =========================================================================
	// Phase 5: Emit PipelineCreated event to TUI
	// =========================================================================
	logger.Info("Emitting PipelineCreated event")

	err = workflow.ExecuteActivity(orchestratorCtx, "PublishPipelineCreatedEventActivity",
		types.PublishPipelineEventInput{
			ProjectID: input.ProjectID,
			RunID:     input.RunID,
			Name:      input.Name,
			Run:       pipelineRun,
		}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish PipelineCreated event", "error", err)
		// Non-fatal: event publish failure shouldn't fail the setup
	}

	// Success - no compensations needed
	output.Success = true
	logger.Info("SetupWorkflow completed successfully",
		"runID", input.RunID,
		"worktreePath", output.WorktreePath,
		"containerID", output.ContainerID,
		"branchName", output.BranchName,
		"startCommit", output.StartCommitSHA)

	return output, nil
}

// markSetupFailed updates the pipeline run status to failed with error message
func markSetupFailed(ctx workflow.Context, runID, errorMsg string) {
	logger := workflow.GetLogger(ctx)
	err := workflow.ExecuteActivity(ctx, "UpdatePipelineRunStatusActivity",
		types.UpdatePipelineRunStatusActivityInput{
			RunID:        runID,
			Status:       models.PipelineRunStatusFailed,
			ErrorMessage: errorMsg,
		}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to mark pipeline run as failed", "error", err)
	}
}

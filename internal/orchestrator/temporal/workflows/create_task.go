// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/utils"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	CreateTaskWorkflowName    = "CreateTaskWorkflow"
	CreateTaskWorkflowVersion = "v1.0.0" // Bump when workflow logic changes (affects task ID hash)
)

// generateTaskQueueName wraps utils.GenerateTaskQueueName for use within workflows.
// The actual implementation is in the utils package to enable sharing with dev tools.
func generateTaskQueueName(taskID string) string {
	return utils.GenerateTaskQueueName(taskID)
}

// NOTE: compensation struct and runCompensations are now in compensation.go

// handleWorkflowError marks task as failed, publishes error event, and runs compensations
func handleWorkflowError(
	ctx workflow.Context,
	compensations []compensation,
	input types.CreateTaskWorkflowInput,
	taskID string,
	err error,
	message string,
	errorContext string,
) {
	logger := workflow.GetLogger(ctx)

	// Run compensations in reverse order first
	runCompensations(ctx, compensations)

	// Mark task as failed (if task was created in DB)
	markFailedErr := workflow.ExecuteActivity(ctx, "UpdateTaskStatusActivity", types.UpdateTaskStatusActivityInput{
		ProjectID: input.ProjectID,
		TaskID:    taskID,
		Status:    3, // TaskStatusFailed
	}).Get(ctx, nil)
	if markFailedErr != nil {
		logger.Warn("Failed to mark task as failed", "error", markFailedErr)
	}

	// Publish error event
	publishErr := workflow.ExecuteActivity(ctx, "PublishErrorEventActivity", types.PublishErrorEventInput{
		TaskID:       taskID,
		Message:      message,
		ErrorContext: errorContext,
	}).Get(ctx, nil)
	if publishErr != nil {
		logger.Warn("Failed to publish error event", "error", publishErr)
	}
}

// CreateTaskWorkflow orchestrates the creation of a new task using the saga pattern
func CreateTaskWorkflow(ctx workflow.Context, input types.CreateTaskWorkflowInput) (*types.CreateTaskWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting CreateTask workflow", "projectID", input.ProjectID, "title", input.Title)

	output := &types.CreateTaskWorkflowOutput{
		Success: false,
	}

	// Saga compensations tracker - will be executed in reverse order on failure
	var compensations []compensation

	// Configure activity options with retry policy
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Use the task ID provided by orchestrator
	taskID := input.TaskID

	// Validate required input fields early
	if taskID == "" {
		err := fmt.Errorf("TaskID is required but was empty")
		logger.Error("Invalid workflow input", "error", err)
		output.Error = err.Error()
		return output, err
	}

	// Step 1: Write task details to file first
	var writeFileResult types.WriteTaskFileActivityOutput
	err := workflow.ExecuteActivity(ctx, "WriteTaskFileActivity", types.WriteTaskFileActivityInput{
		TaskID:         taskID,
		Title:          input.Title,
		Description:    input.Description,
		RepositoryPath: input.RepositoryPath,
	}).Get(ctx, &writeFileResult)
	if err != nil {
		logger.Error("Failed to write task file", "error", err)
		// No compensations yet, just publish error
		publishErr := workflow.ExecuteActivity(ctx, "PublishErrorEventActivity", types.PublishErrorEventInput{
			TaskID:       taskID,
			Message:      "Failed to create task",
			ErrorContext: fmt.Sprintf("Task file write error: %v", err),
		}).Get(ctx, nil)
		if publishErr != nil {
			logger.Warn("Failed to publish error event", "error", publishErr)
		}
		output.Error = fmt.Sprintf("Failed to write task file: %v", err)
		return output, err
	}
	// Note: Task file is idempotent and reusable, no compensation needed

	// Update agent config with actual file path
	if input.AgentConfig != nil && input.AgentConfig.Variables != nil {
		input.AgentConfig.Variables["task_file"] = writeFileResult.FilePath
	}

	// Step 2: Create task in database with file path
	var createTaskResult types.CreateTaskActivityOutput
	err = workflow.ExecuteActivity(ctx, "CreateTaskActivity", types.CreateTaskActivityInput{
		ProjectID:    input.ProjectID,
		TaskID:       taskID,
		Title:        input.Title,
		Description:  input.Description,
		TaskFilePath: writeFileResult.FilePath,
	}).Get(ctx, &createTaskResult)
	if err != nil {
		logger.Error("Failed to create task in database", "error", err)
		// No compensations yet (task file is fine to keep), just publish error
		publishErr := workflow.ExecuteActivity(ctx, "PublishErrorEventActivity", types.PublishErrorEventInput{
			TaskID:       taskID,
			Message:      "Failed to create task",
			ErrorContext: fmt.Sprintf("Database error: %v", err),
		}).Get(ctx, nil)
		if publishErr != nil {
			logger.Warn("Failed to publish error event", "error", publishErr)
		}
		output.Error = fmt.Sprintf("Failed to create task: %v", err)
		return output, err
	}
	output.Task = createTaskResult.Task
	// Note: Task in DB will be marked as failed on error, not deleted (for retry support)

	// Step 2.5: Commit the newly created task file
	var commitResult types.GitCommitActivityOutput
	err = workflow.ExecuteActivity(ctx, "GitCommitActivity", types.GitCommitActivityInput{
		RepositoryPath: input.RepositoryPath,
		FileNames:      []string{writeFileResult.FilePath},
		CommitMessage:  fmt.Sprintf("Add task file: %s", writeFileResult.FileName),
	}).Get(ctx, &commitResult)
	if err != nil {
		logger.Error("Failed to commit task file", "error", err)
		handleWorkflowError(ctx, compensations, input, taskID, err, "Failed to create task", fmt.Sprintf("Task file commit error: %v", err))
		output.Error = fmt.Sprintf("Failed to commit task file: %v", err)
		return output, err
	}
	// Note: Git commits are part of project history, no compensation needed

	// Generate dynamic task queue name for this task
	taskQueueName := generateTaskQueueName(taskID)
	logger.Info("Generated task queue name", "taskQueue", taskQueueName)
	logger.Info("Task file written and committed successfully", "filePath", writeFileResult.FilePath, "commitSuccess", commitResult.Success)

	// Step 3: Create git worktree
	var worktreeResult types.CreateWorktreeActivityOutput
	err = workflow.ExecuteActivity(ctx, "CreateWorktreeActivity", types.CreateWorktreeActivityInput{
		TaskID:         taskID,
		BranchName:     fmt.Sprintf("task/%s", taskID),
		RepositoryPath: input.RepositoryPath,
		BaseCommitSHA:  input.BaseCommitSHA,
	}).Get(ctx, &worktreeResult)
	if err != nil {
		logger.Error("Failed to create git worktree", "error", err)
		handleWorkflowError(ctx, compensations, input, taskID, err, "Failed to create task", fmt.Sprintf("Git worktree error: %v", err))
		output.Error = fmt.Sprintf("Failed to create worktree: %v", err)
		return output, err
	}
	// Add worktree compensation
	compensations = append(compensations, compensation{
		name: "RemoveWorktree",
		action: func(ctx workflow.Context) error {
			return workflow.ExecuteActivity(ctx, "RemoveWorktreeActivity", types.RemoveWorktreeActivityInput{
				WorktreePath:   worktreeResult.WorktreePath,
				RepositoryPath: input.RepositoryPath,
			}).Get(ctx, nil)
		},
	})

	// Step 4: Create container (with extended timeout for image pulls)
	containerCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute, // Extended timeout for image pulls
		HeartbeatTimeout:    10 * time.Second,
		RetryPolicy:         activityOptions.RetryPolicy,
	})

	var containerResult types.CreateContainerActivityOutput
	err = workflow.ExecuteActivity(containerCtx, "CreateContainerActivity", types.CreateContainerActivityInput{
		TaskID:       taskID,
		WorktreePath: worktreeResult.WorktreePath,
		ProjectID:    input.ProjectID,
		TaskQueue:    taskQueueName,
	}).Get(ctx, &containerResult)
	if err != nil {
		logger.Error("Failed to create container", "error", err)
		handleWorkflowError(ctx, compensations, input, taskID, err, "Failed to create task", fmt.Sprintf("Container error: %v", err))
		output.Error = fmt.Sprintf("Failed to create container: %v", err)
		return output, err
	}
	// Add container compensation
	compensations = append(compensations, compensation{
		name: "StopContainer",
		action: func(ctx workflow.Context) error {
			return workflow.ExecuteActivity(ctx, "StopContainerActivity", containerResult.ContainerID).Get(ctx, nil)
		},
	})

	// Step 5: Copy Claude configuration (with standard timeout)
	var configResult types.CopyClaudeConfigActivityOutput
	err = workflow.ExecuteActivity(ctx, "CopyClaudeConfigActivity", types.CopyClaudeConfigActivityInput{
		ContainerID:    containerResult.ContainerID,
		HostConfigPath: input.ClaudeConfigPath,
	}).Get(ctx, &configResult)
	if err != nil {
		logger.Error("Failed to copy Claude config", "error", err)
		handleWorkflowError(ctx, compensations, input, taskID, err, "Failed to create task", fmt.Sprintf("Claude config copy error: %v", err))
		output.Error = fmt.Sprintf("Failed to copy Claude config: %v", err)
		return output, err
	}
	// Note: Config copy is idempotent, no compensation needed

	// Step 6: Copy Claude credentials (with standard timeout)
	var credentialsResult types.CopyClaudeCredentialsActivityOutput
	err = workflow.ExecuteActivity(ctx, "CopyClaudeCredentialsActivity", types.CopyClaudeCredentialsActivityInput{
		ContainerID: containerResult.ContainerID,
	}).Get(ctx, &credentialsResult)
	if err != nil {
		logger.Error("Failed to copy Claude credentials", "error", err)
		handleWorkflowError(ctx, compensations, input, taskID, err, "Failed to create task", fmt.Sprintf("Claude credentials copy error: %v", err))
		output.Error = fmt.Sprintf("Failed to copy Claude credentials: %v", err)
		return output, err
	}
	// Note: Credentials copy is idempotent, no compensation needed

	// Step 7: Publish TaskCreated event (non-critical with shorter timeout)
	eventCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    2, // Only retry once for events
		},
	})

	err = workflow.ExecuteActivity(eventCtx, "PublishTaskCreatedEventActivity", types.PublishEventInput{
		ProjectID: input.ProjectID,
		TaskID:    createTaskResult.Task.ID,
		Task:      createTaskResult.Task,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish TaskCreatedEvent (non-critical)", "error", err)
		// Non-critical failure, continue
	}

	// Step 8: Start AI processing as child workflow
	childWorkflowOptions := workflow.ChildWorkflowOptions{
		WorkflowID:               fmt.Sprintf("process-task-%s", taskID), // Set explicit workflow ID
		WorkflowExecutionTimeout: 30 * time.Minute,                       // Extended timeout for AI processing
		WorkflowTaskTimeout:      time.Minute,
		ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_ABANDON, // Let processing continue even if parent finishes
		TaskQueue:                taskQueueName,                     // Use dynamic task queue for remote execution
	}
	childCtx := workflow.WithChildOptions(ctx, childWorkflowOptions)

	processWorkflowFuture := workflow.ExecuteChildWorkflow(childCtx, ProcessTaskWorkflow, types.ProcessTaskWorkflowInput{
		TaskID:                taskID,
		TaskFilePath:          writeFileResult.FilePath,
		ProjectID:             input.ProjectID,
		WorkspaceDir:          input.WorkspaceDir,
		AgentConfig:           input.AgentConfig,
		WorktreePath:          worktreeResult.WorktreePath,
		OrchestratorTaskQueue: input.OrchestratorTaskQueue,
	})

	// Wait for child workflow to start (but not complete)
	var processWorkflowExecution workflow.Execution
	if err := processWorkflowFuture.GetChildWorkflowExecution().Get(ctx, &processWorkflowExecution); err != nil {
		logger.Error("Failed to start ProcessTask child workflow", "error", err)
		// This is non-critical - the task creation itself succeeded
		logger.Warn("Task created successfully but AI processing failed to start", "error", err)
	} else {
		logger.Info("ProcessTask child workflow started successfully",
			"workflowID", processWorkflowExecution.ID,
			"runID", processWorkflowExecution.RunID)
	}

	// Note: AIObservabilityWorkflow is now started by ProcessTaskWorkflow as a child
	// with PARENT_CLOSE_POLICY_TERMINATE for automatic cleanup

	// Step 9: Publish TaskRequested event to indicate task is ready for processing
	err = workflow.ExecuteActivity(eventCtx, "PublishTaskRequestedEventActivity", types.PublishEventInput{
		ProjectID: input.ProjectID,
		TaskID:    taskID,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to publish TaskRequestedEvent (non-critical)", "error", err)
		// Non-critical failure, continue
	}

	output.Success = true
	logger.Info("CreateTask workflow completed successfully",
		"taskID", taskID,
		"containerID", containerResult.ContainerID,
		"worktreePath", worktreeResult.WorktreePath,
		"claudeConfigCopied", configResult.Success,
		"claudeCredentialsCopied", credentialsResult.Success,
		"processWorkflowStarted", processWorkflowExecution.ID != "",
		"duration", workflow.Now(ctx).Sub(workflow.GetInfo(ctx).WorkflowStartTime))
	return output, nil
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ProcessTaskWorkflowName = "ProcessTaskWorkflow"
)

// handleProcessTaskError handles error scenarios by updating status and publishing events
func handleProcessTaskError(
	orchestratorCtx workflow.Context,
	input types.ProcessTaskWorkflowInput,
	err error,
	message string,
	errorContext string,
) {
	logger := workflow.GetLogger(orchestratorCtx)

	// Update task status to failed in database
	updateErr := workflow.ExecuteActivity(orchestratorCtx, "UpdateTaskStatusActivity", types.UpdateTaskStatusActivityInput{
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
		Status:    3, // TaskStatusFailed
	}).Get(orchestratorCtx, nil)
	if updateErr != nil {
		logger.Warn("Failed to update task status to failed", "error", updateErr)
	}

	// Publish error event
	publishErr := workflow.ExecuteActivity(orchestratorCtx, "PublishErrorEventActivity", types.PublishErrorEventInput{
		TaskID:       input.TaskID,
		Message:      message,
		ErrorContext: errorContext,
	}).Get(orchestratorCtx, nil)
	if publishErr != nil {
		logger.Warn("Failed to publish error event", "error", publishErr)
	}
}

// eventInput creates a PublishEventInput for task lifecycle events
func eventInput(projectID, taskID string) types.PublishEventInput {
	return types.PublishEventInput{
		ProjectID: projectID,
		TaskID:    taskID,
	}
}

// ProcessTaskWorkflow orchestrates AI processing of a created task
func ProcessTaskWorkflow(ctx workflow.Context, input types.ProcessTaskWorkflowInput) (*types.ProcessTaskWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ProcessTask workflow",
		"taskID", input.TaskID,
		"taskFilePath", input.TaskFilePath)

	output := &types.ProcessTaskWorkflowOutput{
		Success: false,
	}

	// Initialize metadata for query handler
	metadata := &types.ProcessingMetadata{
		Timestamp: workflow.Now(ctx),
	}

	// Register query handler for processing metadata
	err := workflow.SetQueryHandler(ctx, "GetProcessingMetadata", func() (*types.ProcessingMetadata, error) {
		return metadata, nil
	})
	if err != nil {
		logger.Warn("Failed to register query handler", "error", err)
	}

	// Configure activity options with extended timeouts for AI processing
	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Extended timeout for AI processing
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	// Configure orchestrator activity options for error events and git operations
	orchestratorActivityOptions := workflow.ActivityOptions{
		TaskQueue:           input.OrchestratorTaskQueue, // Route to orchestrator's queue
		StartToCloseTimeout: 2 * time.Minute,
		HeartbeatTimeout:    30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	}
	orchestratorCtx := workflow.WithActivityOptions(ctx, orchestratorActivityOptions)

	// Start AIObservabilityWorkflow as a child workflow
	// Uses TERMINATE policy so it automatically stops when ProcessTask completes
	obsWorkflowOptions := workflow.ChildWorkflowOptions{
		WorkflowID:               fmt.Sprintf("%s-observability", input.TaskID),
		WorkflowExecutionTimeout: 30 * time.Minute,
		WorkflowTaskTimeout:      time.Minute,
		ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_TERMINATE, // Auto-terminate when parent completes
		TaskQueue:                workflow.GetInfo(ctx).TaskQueueName, // Same queue as ProcessTask (agent container)
	}
	obsCtx := workflow.WithChildOptions(ctx, obsWorkflowOptions)

	// Claude writes transcripts to the configured transcript dir
	transcriptDir := input.TranscriptDir
	if transcriptDir == "" {
		transcriptDir = "/home/noldarim/.claude/projects/-workspace" // Default
	}

	obsWorkflowFuture := workflow.ExecuteChildWorkflow(obsCtx, AIObservabilityWorkflow, types.AIObservabilityWorkflowInput{
		TaskID:                input.TaskID,
		ProjectID:             input.ProjectID,
		TranscriptDir:         transcriptDir,
		ProcessTaskWorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
		OrchestratorTaskQueue: input.OrchestratorTaskQueue,
	})

	// Wait for observability workflow to start (but not complete)
	// CRITICAL: Observability must start successfully for task processing to proceed
	var obsWorkflowExecution workflow.Execution
	if err := obsWorkflowFuture.GetChildWorkflowExecution().Get(ctx, &obsWorkflowExecution); err != nil {
		logger.Error("Failed to start AIObservability child workflow", "error", err)
		handleProcessTaskError(orchestratorCtx, input, err, "Failed to start observability workflow", fmt.Sprintf("Child workflow error: %v", err))
		output.Error = fmt.Sprintf("Failed to start observability workflow: %v", err)
		return output, fmt.Errorf("critical: observability workflow failed to start: %w", err)
	}
	logger.Info("AIObservability child workflow started",
		"workflowID", obsWorkflowExecution.ID)

	// Step 1: Update task status to in_progress in database
	err = workflow.ExecuteActivity(orchestratorCtx, "UpdateTaskStatusActivity", types.UpdateTaskStatusActivityInput{
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
		Status:    0x1, // TaskStatusInProgress
	}).Get(orchestratorCtx, nil)
	if err != nil {
		logger.Error("Failed to update task status to in_progress", "error", err)
		output.Error = fmt.Sprintf("Failed to update task status: %v", err)
		return output, fmt.Errorf("failed to update task status: %w", err)
	}

	// Step 2: Publish TaskInProgress event to indicate processing has started
	err = workflow.ExecuteActivity(orchestratorCtx, "PublishTaskInProgressEventActivity", eventInput(input.ProjectID, input.TaskID)).Get(orchestratorCtx, nil)
	if err != nil {
		logger.Error("Failed to publish TaskInProgressEvent", "error", err)
		output.Error = fmt.Sprintf("Failed to publish TaskInProgressEvent: %v", err)
		return output, fmt.Errorf("failed to publish TaskInProgressEvent: %w", err)
	}

	// Step 3: Prepare command from AgentConfig
	if input.AgentConfig == nil {
		err := fmt.Errorf("AgentConfig is required but was not provided")
		logger.Error("Missing agent configuration", "error", err)
		output.Error = err.Error()
		return output, err
	}

	var commandToExecute []string
	err = workflow.ExecuteActivity(ctx, "PrepareAgentCommandActivity", input.AgentConfig).Get(ctx, &commandToExecute)
	if err != nil {
		logger.Error("Failed to prepare agent command", "error", err)
		handleProcessTaskError(orchestratorCtx, input, err, "Failed to prepare agent command", fmt.Sprintf("Preparation error: %v", err))
		output.Error = fmt.Sprintf("Failed to prepare agent command: %v", err)
		return output, err
	}
	logger.Info("Using agent config", "tool", input.AgentConfig.ToolName)

	// Step 4: Execute dynamic processing command locally
	var commandResult types.LocalExecuteActivityOutput
	err = workflow.ExecuteActivity(ctx, "LocalExecuteActivity", types.LocalExecuteActivityInput{
		Command: commandToExecute,
		WorkDir: input.WorkspaceDir,
	}).Get(ctx, &commandResult)

	if err != nil {
		logger.Error("Failed to execute AI processing command", "error", err)
		handleProcessTaskError(orchestratorCtx, input, err, "Failed to process task", fmt.Sprintf("Command execution error: %v", err))
		output.Error = fmt.Sprintf("Failed to execute processing command: %v", err)
		return output, err
	}

	// Check if command executed successfully
	if !commandResult.Success {
		err := fmt.Errorf("processing command failed: exit code %d", commandResult.ExitCode)
		logger.Error("AI processing command failed",
			"exitCode", commandResult.ExitCode,
			"errorOutput", commandResult.ErrorOutput)
		handleProcessTaskError(orchestratorCtx, input, err, "Task processing failed", fmt.Sprintf("Exit code: %d, stderr: %s", commandResult.ExitCode, commandResult.ErrorOutput))
		output.Error = fmt.Sprintf("Processing command failed with exit code %d: %s", commandResult.ExitCode, commandResult.ErrorOutput)
		return output, err
	}

	// Store processing results
	output.Success = true
	output.ProcessedData = commandResult.Output
	metadata.CommandOutput = commandResult.Output
	metadata.ProcessingTime = commandResult.Duration

	// Step 5: Capture git diff before committing (for metadata)
	if input.WorktreePath != "" {
		logger.Info("Capturing git diff", "worktreePath", input.WorktreePath)

		var diffResult types.CaptureGitDiffActivityOutput
		err = workflow.ExecuteActivity(orchestratorCtx, "CaptureGitDiffActivity", types.CaptureGitDiffActivityInput{
			RepositoryPath: input.WorktreePath,
		}).Get(orchestratorCtx, &diffResult)

		if err != nil {
			logger.Error("Failed to capture git diff", "error", err)
			handleProcessTaskError(orchestratorCtx, input, err, "Failed to capture git diff", fmt.Sprintf("Git diff error: %v", err))
			output.Success = false
			output.Error = fmt.Sprintf("Failed to capture git diff: %v", err)
			return output, fmt.Errorf("failed to capture git diff: %w", err)
		}

		metadata.GitDiff = &diffResult
		logger.Info("Successfully captured git diff",
			"filesChanged", len(diffResult.FilesChanged),
			"insertions", diffResult.Insertions,
			"deletions", diffResult.Deletions)

		// Save git diff to database for TUI visibility (non-blocking)
		if diffResult.HasChanges {
			err = workflow.ExecuteActivity(orchestratorCtx, "UpdateTaskGitDiffActivity", types.UpdateTaskGitDiffActivityInput{
				TaskID:  input.TaskID,
				GitDiff: diffResult.Diff,
			}).Get(orchestratorCtx, nil)
			if err != nil {
				logger.Warn("Failed to save git diff to database", "error", err)
				// Non-fatal: continue workflow even if save fails
			} else {
				logger.Info("Successfully saved git diff to database")
			}
		}
	}

	// Step 6: Commit any changes made by the agent (CRITICAL for idempotency)
	// This executes on the orchestrator worker since GitCommitActivity is not registered on agent worker
	if input.WorktreePath != "" {
		logger.Info("Attempting to commit agent changes", "worktreePath", input.WorktreePath)

		var commitResult types.GitCommitActivityOutput
		err = workflow.ExecuteActivity(orchestratorCtx, "GitCommitActivity", types.GitCommitActivityInput{
			RepositoryPath: input.WorktreePath, // Pass the worktree path
			FileNames:      []string{"."},      // Commit all changes in worktree
			CommitMessage:  fmt.Sprintf("Agent automated changes for task %s", input.TaskID),
		}).Get(orchestratorCtx, &commitResult)

		if err != nil {
			logger.Error("Failed to commit agent changes",
				"error", err,
				"worktreePath", input.WorktreePath)
			handleProcessTaskError(orchestratorCtx, input, err, "Failed to commit agent changes", fmt.Sprintf("Git commit error: %v", err))
			output.Success = false
			output.Error = fmt.Sprintf("Failed to commit agent changes: %v", err)
			return output, fmt.Errorf("git commit failed: %w", err)
		}

		if !commitResult.Success {
			err := fmt.Errorf("git commit unsuccessful: %s", commitResult.Error)
			logger.Error("Git commit reported failure",
				"error", commitResult.Error,
				"worktreePath", input.WorktreePath)
			handleProcessTaskError(orchestratorCtx, input, err, "Git commit unsuccessful", commitResult.Error)
			output.Success = false
			output.Error = fmt.Sprintf("Git commit unsuccessful: %s", commitResult.Error)
			return output, err
		}

		logger.Info("Successfully committed agent changes",
			"worktreePath", input.WorktreePath)
	}

	// Step 7: Wait 5 seconds for observability to finish reading final data
	// The child workflow will be terminated when we complete (PARENT_CLOSE_POLICY_TERMINATE)
	// but we give it time to read/forward the last transcript lines
	logger.Info("Waiting 5s for observability to read final data")
	_ = workflow.NewTimer(ctx, 5*time.Second).Get(ctx, nil)

	// Step 8: Update task status to completed in database
	err = workflow.ExecuteActivity(orchestratorCtx, "UpdateTaskStatusActivity", types.UpdateTaskStatusActivityInput{
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
		Status:    0x2, // TaskStatusCompleted
	}).Get(orchestratorCtx, nil)
	if err != nil {
		logger.Error("Failed to update task status to completed", "error", err)
		output.Success = false
		output.Error = fmt.Sprintf("Failed to update task status: %v", err)
		return output, fmt.Errorf("failed to update task status: %w", err)
	}

	// Step 9: Publish TaskFinished event to indicate successful completion
	err = workflow.ExecuteActivity(orchestratorCtx, "PublishTaskFinishedEventActivity", eventInput(input.ProjectID, input.TaskID)).Get(orchestratorCtx, nil)
	if err != nil {
		logger.Error("Failed to publish TaskFinishedEvent", "error", err)
		output.Success = false
		output.Error = fmt.Sprintf("Failed to publish TaskFinishedEvent: %v", err)
		return output, fmt.Errorf("failed to publish TaskFinishedEvent: %w", err)
	}

	logger.Info("ProcessTask workflow completed successfully",
		"taskID", input.TaskID,
		"outputLength", len(output.ProcessedData),
		"duration", workflow.Now(ctx).Sub(workflow.GetInfo(ctx).WorkflowStartTime))

	return output, nil
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	ProcessingStepWorkflowName    = "ProcessingStepWorkflow"
	ProcessingStepWorkflowVersion = "v2.5.0" // Added cancellation handling helper
)

// propagateCancellation checks if err is a cancellation error and returns a standardized response.
// Returns (true, cancellationError) if cancelled, (false, nil) otherwise.
func propagateCancellation(err error, ctx workflow.Context, output *types.ProcessingStepOutput, operation string) (bool, error) {
	if temporal.IsCanceledError(err) {
		workflow.GetLogger(ctx).Info("Operation cancelled", "operation", operation)
		if output != nil {
			output.Error = "Cancelled by user"
		}
		return true, temporal.NewCanceledError("user requested cancellation")
	}
	return false, nil
}

// handleProcessingStepError handles error scenarios by publishing error events.
// ProcessingStepWorkflow doesn't create resources that need cleanup (container and worktree
// are managed by SetupWorkflow), so this just publishes error events for visibility.
func handleProcessingStepError(
	orchestratorCtx workflow.Context,
	input types.ProcessingStepInput,
	err error,
	message string,
	errorContext string,
) {
	logger := workflow.GetLogger(orchestratorCtx)

	// Publish error event for TUI visibility
	publishErr := workflow.ExecuteActivity(orchestratorCtx, "PublishErrorEventActivity", types.PublishErrorEventInput{
		TaskID:       fmt.Sprintf("%s-%s", input.RunID, input.StepID),
		Message:      message,
		ErrorContext: errorContext,
	}).Get(orchestratorCtx, nil)
	if publishErr != nil {
		logger.Warn("Failed to publish error event", "error", publishErr)
	}
}

// ProcessingStepWorkflow executes a single processing step within a pipeline:
// 1. Prepares agent command from config
// 2. Executes agent (AI processing)
// 3. Captures git diff
// 4. Commits changes with step-specific message
// 5. Retrieves token totals from AI activity records
//
// Note: AIObservabilityWorkflow is started at the pipeline level (not per-step)
// to avoid duplicate transcript reading across steps.
//
// This is the single source of truth for all execution logic.
// Designed to run inside the container worker on a run-specific task queue.
func ProcessingStepWorkflow(ctx workflow.Context, input types.ProcessingStepInput) (*types.ProcessingStepOutput, error) {
	logger := workflow.GetLogger(ctx)
	startTime := workflow.Now(ctx)

	logger.Info("Starting ProcessingStepWorkflow",
		"runID", input.RunID,
		"stepID", input.StepID,
		"stepName", input.StepName,
		"stepIndex", input.StepIndex)

	output := &types.ProcessingStepOutput{
		Success: false,
		StepID:  input.StepID,
	}

	// Validate required input
	if input.AgentConfig == nil {
		err := fmt.Errorf("AgentConfig is required but was not provided")
		logger.Error("Missing agent configuration", "error", err)
		output.Error = err.Error()
		return output, err
	}

	// Configure activity options for local (container) activities
	localActivityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 15 * time.Minute, // AI processing can take time
		HeartbeatTimeout:    60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    2 * time.Minute,
			MaximumAttempts:    2, // Limited retries for AI - it's expensive
		},
	}
	localCtx := workflow.WithActivityOptions(ctx, localActivityOptions)

	// Configure orchestrator context for git operations (cross-worker)
	orchestratorActivityOptions := workflow.ActivityOptions{
		TaskQueue:           input.OrchestratorTaskQueue,
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

	// =========================================================================
	// Phase 1: Execute agent
	// =========================================================================
	// Note: AIObservabilityWorkflow runs at pipeline level (started by PipelineWorkflow)

	// Step 1a: Prepare agent command
	// Note: Prompt composition (prefix/suffix) happens in PipelineWorkflow
	logger.Info("Preparing agent command", "tool", input.AgentConfig.ToolName)

	var commandToExecute []string
	err := workflow.ExecuteActivity(localCtx, "PrepareAgentCommandActivity", input.AgentConfig).Get(localCtx, &commandToExecute)
	if cancelled, cancelErr := propagateCancellation(err, ctx, output, "PrepareAgentCommandActivity"); cancelled {
		return output, cancelErr
	}
	if err != nil {
		logger.Error("Failed to prepare agent command", "error", err)
		output.Error = fmt.Sprintf("Failed to prepare agent command: %v", err)
		handleProcessingStepError(orchestratorCtx, input, err, "Failed to prepare agent command", fmt.Sprintf("Preparation error: %v", err))
		return output, err
	}

	logger.Info("Agent command prepared", "command", commandToExecute)

	// Step 1b: Execute agent
	logger.Info("Executing agent")

	var commandResult types.LocalExecuteActivityOutput
	err = workflow.ExecuteActivity(localCtx, "LocalExecuteActivity", types.LocalExecuteActivityInput{
		Command: commandToExecute,
		WorkDir: input.WorkspaceDir,
	}).Get(localCtx, &commandResult)

	if cancelled, cancelErr := propagateCancellation(err, ctx, output, "LocalExecuteActivity"); cancelled {
		return output, cancelErr
	}
	if err != nil {
		logger.Error("Failed to execute agent", "error", err)
		output.Error = fmt.Sprintf("Failed to execute agent: %v", err)
		handleProcessingStepError(orchestratorCtx, input, err, "Failed to execute agent", fmt.Sprintf("Execution error: %v", err))
		return output, err
	}

	if !commandResult.Success {
		err := fmt.Errorf("agent execution failed: exit code %d", commandResult.ExitCode)
		logger.Error("Agent execution failed",
			"exitCode", commandResult.ExitCode,
			"errorOutput", commandResult.ErrorOutput)
		output.Error = fmt.Sprintf("Agent failed with exit code %d: %s", commandResult.ExitCode, commandResult.ErrorOutput)
		handleProcessingStepError(orchestratorCtx, input, err, "Agent execution failed", fmt.Sprintf("Exit code: %d, stderr: %s", commandResult.ExitCode, commandResult.ErrorOutput))
		return output, err
	}

	output.AgentOutput = commandResult.Output
	logger.Info("Agent execution completed", "outputLength", len(commandResult.Output))

	// =========================================================================
	// Phase 2: Capture git diff and commit
	// =========================================================================

	// Step 2a: Capture git diff (on orchestrator worker where git is available)
	logger.Info("Capturing git diff", "worktreePath", input.WorktreePath)

	var diffResult types.CaptureGitDiffActivityOutput
	err = workflow.ExecuteActivity(orchestratorCtx, "CaptureGitDiffActivity", types.CaptureGitDiffActivityInput{
		RepositoryPath: input.WorktreePath,
	}).Get(orchestratorCtx, &diffResult)

	if cancelled, cancelErr := propagateCancellation(err, ctx, output, "CaptureGitDiffActivity"); cancelled {
		return output, cancelErr
	}
	if err != nil {
		logger.Error("Failed to capture git diff", "error", err)
		output.Error = fmt.Sprintf("Failed to capture git diff: %v", err)
		handleProcessingStepError(orchestratorCtx, input, err, "Failed to capture git diff", fmt.Sprintf("Git diff error: %v", err))
		return output, err
	}

	output.GitDiff = diffResult.Diff
	output.FilesChanged = len(diffResult.FilesChanged)
	output.Insertions = diffResult.Insertions
	output.Deletions = diffResult.Deletions

	logger.Info("Git diff captured",
		"hasChanges", diffResult.HasChanges,
		"filesChanged", output.FilesChanged,
		"insertions", output.Insertions,
		"deletions", output.Deletions)

	// Step 2b: Generate step documentation (written to worktree before commit)
	logger.Info("Generating step documentation")

	var docResult types.GenerateStepDocumentationActivityOutput
	err = workflow.ExecuteActivity(orchestratorCtx, "GenerateStepDocumentationActivity",
		types.GenerateStepDocumentationActivityInput{
			RunID:        input.RunID,
			StepID:       input.StepID,
			StepName:     input.StepName,
			StepIndex:    input.StepIndex,
			WorktreePath: input.WorktreePath,
			PromptUsed:   input.AgentConfig.PromptTemplate,
			AgentOutput:  output.AgentOutput,
			GitDiff:      output.GitDiff,
			DiffStat:     diffResult.DiffStat,
			FilesChanged: diffResult.FilesChanged,
			Insertions:   output.Insertions,
			Deletions:    output.Deletions,
		}).Get(orchestratorCtx, &docResult)

	if cancelled, cancelErr := propagateCancellation(err, ctx, output, "GenerateStepDocumentationActivity"); cancelled {
		return output, cancelErr
	}
	if err != nil {
		logger.Error("Failed to execute documentation activity",
			"error", err,
			"runID", input.RunID,
			"stepID", input.StepID,
			"worktreePath", input.WorktreePath)
		output.Error = fmt.Sprintf("Failed to generate step documentation: %v", err)
		handleProcessingStepError(orchestratorCtx, input, err, "Failed to generate step documentation", fmt.Sprintf("Documentation error: %v", err))
		return output, err
	}

	if !docResult.Success {
		err := fmt.Errorf("documentation generation failed: %s", docResult.Error)
		logger.Error("Documentation generation returned failure",
			"error", docResult.Error,
			"runID", input.RunID,
			"stepID", input.StepID,
			"worktreePath", input.WorktreePath)
		output.Error = fmt.Sprintf("Documentation generation failed: %s", docResult.Error)
		handleProcessingStepError(orchestratorCtx, input, err, "Documentation generation failed", docResult.Error)
		return output, err
	}

	logger.Info("Step documentation generated successfully",
		"path", docResult.DocumentPath,
		"hasSummary", docResult.Summary != nil)

	// Step 2c: Commit changes (on orchestrator worker)
	commitMessage := fmt.Sprintf("Step %s: %s", input.StepID, input.StepName)
	logger.Info("Committing changes", "message", commitMessage)

	var commitResult types.GitCommitActivityOutput
	err = workflow.ExecuteActivity(orchestratorCtx, "GitCommitActivity", types.GitCommitActivityInput{
		RepositoryPath: input.WorktreePath,
		FileNames:      []string{"."},
		CommitMessage:  commitMessage,
	}).Get(orchestratorCtx, &commitResult)

	if cancelled, cancelErr := propagateCancellation(err, ctx, output, "GitCommitActivity"); cancelled {
		return output, cancelErr
	}
	if err != nil {
		logger.Error("Failed to commit changes", "error", err)
		output.Error = fmt.Sprintf("Failed to commit changes: %v", err)
		handleProcessingStepError(orchestratorCtx, input, err, "Failed to commit changes", fmt.Sprintf("Git commit error: %v", err))
		return output, err
	}

	if !commitResult.Success {
		// No changes to commit is not an error for pipeline steps
		if commitResult.Error == "nothing to commit" || commitResult.Error == "no changes to commit" {
			logger.Info("No changes to commit (agent made no modifications)")
			// Use previous commit SHA if no new commit was created
			output.CommitSHA = input.PreviousCommitSHA
			output.CommitMessage = "No changes"
		} else {
			err := fmt.Errorf("git commit failed: %s", commitResult.Error)
			logger.Error("Git commit reported failure", "error", commitResult.Error)
			output.Error = fmt.Sprintf("Git commit failed: %s", commitResult.Error)
			handleProcessingStepError(orchestratorCtx, input, err, "Git commit failed", commitResult.Error)
			return output, err
		}
	} else {
		output.CommitSHA = commitResult.CommitSHA
		output.CommitMessage = commitMessage
		logger.Info("Changes committed", "commitSHA", commitResult.CommitSHA)
	}

	// =========================================================================
	// Phase 3: Get token totals from AI activity records
	// =========================================================================
	// Note: We don't wait for observability here - the pipeline-level observability
	// workflow continues running and will be waited on at the end of PipelineWorkflow.
	taskID := fmt.Sprintf("%s-%s", input.RunID, input.StepID)
	var tokenTotals types.GetTokenTotalsActivityOutput
	err = workflow.ExecuteActivity(orchestratorCtx, "GetTokenTotalsActivity",
		types.GetTokenTotalsActivityInput{TaskID: taskID}).Get(orchestratorCtx, &tokenTotals)
	if cancelled, cancelErr := propagateCancellation(err, ctx, output, "GetTokenTotalsActivity"); cancelled {
		return output, cancelErr
	}
	if err != nil {
		logger.Warn("Failed to get token totals", "error", err)
		// Non-fatal - continue without tokens
	} else {
		output.InputTokens = tokenTotals.InputTokens
		output.OutputTokens = tokenTotals.OutputTokens
		output.CacheReadTokens = tokenTotals.CacheReadTokens
		output.CacheCreateTokens = tokenTotals.CacheCreateTokens
		logger.Info("Token totals retrieved",
			"inputTokens", output.InputTokens,
			"outputTokens", output.OutputTokens)
	}

	// Calculate duration
	output.Duration = workflow.Now(ctx).Sub(startTime)
	output.Success = true

	logger.Info("ProcessingStepWorkflow completed successfully",
		"stepID", input.StepID,
		"commitSHA", output.CommitSHA,
		"filesChanged", output.FilesChanged,
		"duration", output.Duration)

	return output, nil
}

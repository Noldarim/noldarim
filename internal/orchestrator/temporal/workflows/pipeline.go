// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"fmt"
	"strings"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	PipelineWorkflowName    = "PipelineWorkflow"
	PipelineWorkflowVersion = "v2.3.0" // Explicit child workflow cancellation for faster Ctrl+C response
)

// handlePipelineCancellation handles cleanup when pipeline is cancelled
func handlePipelineCancellation(ctx workflow.Context, runID string, orchestratorActivityOptions workflow.ActivityOptions, output *types.PipelineWorkflowOutput, operation string) error {
	workflow.GetLogger(ctx).Info("Pipeline cancelled by user", "operation", operation)
	output.Error = "Pipeline cancelled by user"

	// Use disconnected context for cleanup - ensures activities run even after cancellation
	cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
	cleanupOrchestratorCtx := workflow.WithActivityOptions(cleanupCtx, orchestratorActivityOptions)
	markPipelineRunFailed(cleanupOrchestratorCtx, runID, "Cancelled by user")

	return temporal.NewCanceledError("user requested cancellation")
}

// propagatePipelineCancellation checks if err is a cancellation error and handles cleanup
// Returns (true, cancellationError) if cancelled, (false, nil) otherwise
func propagatePipelineCancellation(err error, ctx workflow.Context, runID string, orchestratorActivityOptions workflow.ActivityOptions, output *types.PipelineWorkflowOutput, operation string) (bool, error) {
	if temporal.IsCanceledError(err) {
		cancelErr := handlePipelineCancellation(ctx, runID, orchestratorActivityOptions, output, operation)
		return true, cancelErr
	}
	return false, nil
}

// PipelineWorkflow is a thin orchestrator that sequences child workflows:
// 1. Setup (via SetupWorkflow child - handles DB, fork resolution, infrastructure)
// 1b. AIObservabilityWorkflow (runs for entire pipeline, watches transcripts)
// 2. N processing steps (via ProcessingStepWorkflow children - each produces a commit)
// 2b. Wait for observability to process final events
// 3. Final status update
//
// This workflow contains minimal logic - it delegates to child workflows.
func PipelineWorkflow(ctx workflow.Context, input types.PipelineWorkflowInput) (*types.PipelineWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	startTime := workflow.Now(ctx)

	logger.Info("Starting PipelineWorkflow",
		"runID", input.RunID,
		"projectID", input.ProjectID,
		"steps", len(input.Steps),
		"hasPromptPrefix", input.PromptPrefix != "",
		"hasPromptSuffix", input.PromptSuffix != "")

	output := &types.PipelineWorkflowOutput{
		Success:     false,
		RunID:       input.RunID,
		StepResults: make([]types.ProcessingStepOutput, 0),
	}

	// Configure activity options for orchestrator activities (status updates)
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
	// Fork Validation (if forking from another run)
	// =========================================================================
	if input.ForkFromRunID != "" {
		logger.Info("Validating fork compatibility", "parentRunID", input.ForkFromRunID)

		var parentRunOutput types.GetPipelineRunActivityOutput
		err := workflow.ExecuteActivity(orchestratorCtx, "GetPipelineRunActivity",
			types.GetPipelineRunActivityInput{RunID: input.ForkFromRunID}).Get(ctx, &parentRunOutput)

		if err != nil {
			errMsg := fmt.Sprintf("Failed to fetch parent run for fork validation: %v", err)
			logger.Error(errMsg)
			output.Error = errMsg
			return output, fmt.Errorf("%s", errMsg)
		}

		if parentRunOutput.Run == nil {
			errMsg := fmt.Sprintf("Parent run not found: %s", input.ForkFromRunID)
			logger.Error(errMsg)
			output.Error = errMsg
			return output, fmt.Errorf("%s", errMsg)
		}

		// Strict validation: prompt configuration must match exactly
		if parentRunOutput.Run.PromptPrefix != input.PromptPrefix ||
			parentRunOutput.Run.PromptSuffix != input.PromptSuffix {
			errMsg := fmt.Sprintf("Cannot fork: prompt configuration differs from parent run "+
				"(parent prefix=%q, new prefix=%q, parent suffix=%q, new suffix=%q)",
				parentRunOutput.Run.PromptPrefix, input.PromptPrefix,
				parentRunOutput.Run.PromptSuffix, input.PromptSuffix)
			logger.Error(errMsg)
			output.Error = errMsg
			return output, fmt.Errorf("%s", errMsg)
		}

		logger.Info("Fork validation passed")
	}

	// Generate branch name if not provided
	branchName := input.BranchName
	if branchName == "" {
		branchName = fmt.Sprintf("pipeline/%s", input.RunID[:8])
	}
	output.BranchName = branchName

	// Generate task queue name for this run's container worker
	runTaskQueue := generateTaskQueueName(input.Name, input.RunID)

	// =========================================================================
	// Phase 1: Setup via Child Workflow
	// =========================================================================
	logger.Info("Phase 1: Setup (child workflow)")

	setupChildOpts := workflow.ChildWorkflowOptions{
		WorkflowID:               fmt.Sprintf("%s-setup", input.RunID),
		WorkflowExecutionTimeout: 10 * time.Minute,
		WorkflowTaskTimeout:      time.Minute,
		TaskQueue:                input.OrchestratorTaskQueue, // Setup runs on orchestrator
		ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_TERMINATE,
	}
	setupCtx := workflow.WithChildOptions(ctx, setupChildOpts)

	setupInput := types.PipelineSetupInput{
		RunID:                 input.RunID,
		PipelineID:            input.PipelineID,
		ProjectID:             input.ProjectID,
		Name:                  input.Name,
		RepositoryPath:        input.RepositoryPath,
		BranchName:            branchName,
		BaseCommitSHA:         input.BaseCommitSHA,
		StartCommitSHA:        input.StartCommitSHA,
		ForkFromRunID:         input.ForkFromRunID,
		ForkAfterStepID:       input.ForkAfterStepID,
		ClaudeConfigPath:      input.ClaudeConfigPath,
		WorkspaceDir:          input.WorkspaceDir,
		TaskQueue:             runTaskQueue,
		OrchestratorTaskQueue: input.OrchestratorTaskQueue,
		ParentWorkflowID:      workflow.GetInfo(ctx).WorkflowExecution.ID,
	}

	var setupOutput types.PipelineSetupOutput
	err := workflow.ExecuteChildWorkflow(setupCtx, SetupWorkflow, setupInput).Get(ctx, &setupOutput)

	// Check for cancellation during setup
	if cancelled, cancelErr := propagatePipelineCancellation(err, ctx, input.RunID, orchestratorActivityOptions, output, "setup"); cancelled {
		return output, cancelErr
	}

	if err != nil || !setupOutput.Success {
		errMsg := "Setup failed"
		if err != nil {
			errMsg = fmt.Sprintf("Setup failed: %v", err)
		} else if setupOutput.Error != "" {
			errMsg = fmt.Sprintf("Setup failed: %s", setupOutput.Error)
		}
		logger.Error(errMsg)
		output.Error = errMsg
		// SetupWorkflow already marked run as failed in DB
		return output, fmt.Errorf("%s", errMsg)
	}

	logger.Info("Setup completed",
		"worktreePath", setupOutput.WorktreePath,
		"containerID", setupOutput.ContainerID,
		"startCommit", setupOutput.StartCommitSHA,
		"runTaskQueue", runTaskQueue)

	// Store prompt configuration and identity hash for idempotency/fork validation
	identityHash := models.ComputePipelineIdentityHash(
		input.PipelineID,
		input.Steps,
		input.PromptPrefix,
		input.PromptSuffix,
		input.BaseCommitSHA,
	)
	logger.Info("Computed identity hash", "hash", identityHash)

	// Update run with prompt config (non-fatal)
	_ = workflow.ExecuteActivity(orchestratorCtx, "SavePipelineRunActivity",
		types.SavePipelineRunActivityInput{
			Run: &models.PipelineRun{
				ID:           input.RunID,
				PromptPrefix: input.PromptPrefix,
				PromptSuffix: input.PromptSuffix,
				IdentityHash: identityHash,
			},
		}).Get(ctx, nil)

	// =========================================================================
	// Phase 1b: Start AIObservabilityWorkflow (runs for entire pipeline)
	// =========================================================================
	// This single observability workflow watches transcripts for ALL steps.
	// Uses TERMINATE policy so it automatically stops when pipeline completes.
	logger.Info("Starting pipeline-level AIObservabilityWorkflow")

	// Default transcript directory (where Claude writes session files)
	transcriptDir := "/home/noldarim/.claude/projects/-workspace"

	obsWorkflowOptions := workflow.ChildWorkflowOptions{
		WorkflowID:               fmt.Sprintf("%s-observability", input.RunID),
		WorkflowExecutionTimeout: 60 * time.Minute, // Long enough for entire pipeline
		WorkflowTaskTimeout:      time.Minute,
		ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_TERMINATE, // Auto-terminate when pipeline completes
		TaskQueue:                runTaskQueue,                        // Runs in container worker (where transcripts are)
	}
	obsCtx := workflow.WithChildOptions(ctx, obsWorkflowOptions)

	obsWorkflowFuture := workflow.ExecuteChildWorkflow(obsCtx, AIObservabilityWorkflow, types.AIObservabilityWorkflowInput{
		TaskID:                input.RunID, // Use RunID as TaskID for pipeline-level aggregation
		RunID:                 input.RunID,
		ProjectID:             input.ProjectID,
		TranscriptDir:         transcriptDir,
		ProcessTaskWorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
		OrchestratorTaskQueue: input.OrchestratorTaskQueue,
	})

	// Wait for observability workflow to start (but not complete)
	var obsWorkflowExecution workflow.Execution
	if err := obsWorkflowFuture.GetChildWorkflowExecution().Get(ctx, &obsWorkflowExecution); err != nil {
		// Check for cancellation
		if cancelled, cancelErr := propagatePipelineCancellation(err, ctx, input.RunID, orchestratorActivityOptions, output, "observability startup"); cancelled {
			return output, cancelErr
		}
		logger.Error("Failed to start AIObservability child workflow", "error", err)
		output.Error = fmt.Sprintf("Failed to start observability workflow: %v", err)
		markPipelineRunFailed(orchestratorCtx, input.RunID, output.Error)
		return output, fmt.Errorf("critical: observability workflow failed to start: %w", err)
	}
	logger.Info("AIObservability child workflow started", "workflowID", obsWorkflowExecution.ID)

	// =========================================================================
	// Phase 2: Execute Steps via Child Workflows
	// =========================================================================
	logger.Info("Phase 2: Execute Steps", "count", len(input.Steps))

	currentCommit := setupOutput.StartCommitSHA
	stepSkipMode := input.ForkAfterStepID != "" // Skip steps until we pass fork point
	previousStepID := ""                        // Track previous step for runtime vars

	for i, stepDef := range input.Steps {
		// Skip steps before fork point
		if stepSkipMode {
			if stepDef.StepID == input.ForkAfterStepID {
				stepSkipMode = false // Found fork point, stop skipping after this
				logger.Info("Reached fork point, will execute subsequent steps",
					"forkAfterStep", stepDef.StepID)
			}
			// Record skipped step
			skippedResult := types.ProcessingStepOutput{
				Success: true,
				StepID:  stepDef.StepID,
			}
			output.StepResults = append(output.StepResults, skippedResult)
			previousStepID = stepDef.StepID // Track for next step's runtime vars
			continue
		}

		logger.Info("Executing step",
			"stepID", stepDef.StepID,
			"stepName", stepDef.Name,
			"index", i)

		// Emit PipelineStepStarted event (non-fatal: TUI visibility only)
		_ = workflow.ExecuteActivity(orchestratorCtx, "PublishPipelineStepStartedEventActivity",
			types.PublishPipelineEventInput{
				ProjectID: input.ProjectID,
				RunID:     input.RunID,
				Name:      input.Name,
				StepID:    stepDef.StepID,
				StepIndex: i,
				StepName:  stepDef.Name,
			}).Get(ctx, nil)

		// Signal observability workflow with current step ID so events are tagged correctly
		signalErr := workflow.SignalExternalWorkflow(ctx, obsWorkflowExecution.ID, "", StepChangeSignal, stepDef.StepID).Get(ctx, nil)
		if signalErr != nil {
			logger.Warn("Failed to signal step change to observability workflow",
				"stepID", stepDef.StepID, "error", signalErr)
		}

		// Create step result record in DB
		stepResultID := fmt.Sprintf("%s-step-%s", input.RunID, stepDef.StepID)
		stepResult := &models.StepResult{
			ID:             stepResultID,
			PipelineRunID:  input.RunID,
			StepID:         stepDef.StepID,
			StepIndex:      i,
			Status:         models.StepStatusRunning,
			DefinitionHash: models.ComputeStepDefinitionHash(stepDef), // For fork comparison
		}
		stepStartTime := workflow.Now(ctx)
		stepResult.StartedAt = &stepStartTime

		// Non-fatal: best-effort save of initial step status
		_ = workflow.ExecuteActivity(orchestratorCtx, "SaveStepResultActivity",
			types.SaveStepResultActivityInput{Result: stepResult}).Get(ctx, nil)

		// Convert step agent config to protocol type with prompt composition
		var agentConfig *protocol.AgentConfigInput
		if stepDef.AgentConfig != nil {
			// Compose prompt: prefix + step prompt + suffix
			composedPrompt := input.PromptPrefix + stepDef.AgentConfig.PromptTemplate + input.PromptSuffix

			// Inject runtime variables ({{.RunID}}, {{.PreviousStepID}}, etc.)
			runtimeVars := RuntimeVars{
				RunID:          input.RunID,
				StepIndex:      i,
				StepID:         stepDef.StepID,
				PreviousStepID: previousStepID,
			}
			finalPrompt, err := injectRuntimeVars(composedPrompt, runtimeVars)
			if err != nil {
				logger.Warn("Failed to inject runtime vars, using original prompt", "error", err)
				finalPrompt = composedPrompt
			}

			agentConfig = &protocol.AgentConfigInput{
				ToolName:       stepDef.AgentConfig.ToolName,
				ToolVersion:    stepDef.AgentConfig.ToolVersion,
				PromptTemplate: finalPrompt,
				Variables:      stepDef.AgentConfig.Variables,
				ToolOptions:    stepDef.AgentConfig.ToolOptions,
				FlagFormat:     stepDef.AgentConfig.FlagFormat,
			}
		}

		// Execute ProcessingStepWorkflow as child
		stepChildOpts := workflow.ChildWorkflowOptions{
			WorkflowID:               fmt.Sprintf("%s-step-%s", input.RunID, stepDef.StepID),
			WorkflowExecutionTimeout: 30 * time.Minute, // AI processing can take time
			WorkflowTaskTimeout:      time.Minute,
			TaskQueue:                runTaskQueue, // Steps run in container worker
			ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_TERMINATE,
		}
		stepCtx := workflow.WithChildOptions(ctx, stepChildOpts)

		stepInput := types.ProcessingStepInput{
			RunID:                 input.RunID,
			ProjectID:             input.ProjectID,
			StepID:                stepDef.StepID,
			StepIndex:             i,
			StepName:              stepDef.Name,
			AgentConfig:           agentConfig,
			WorktreePath:          setupOutput.WorktreePath,
			WorkspaceDir:          input.WorkspaceDir,
			OrchestratorTaskQueue: input.OrchestratorTaskQueue,
			PreviousCommitSHA:     currentCommit,
		}

		// Execute child workflow and track it for potential cancellation
		childWorkflowID := fmt.Sprintf("%s-step-%s", input.RunID, stepDef.StepID)
		childFuture := workflow.ExecuteChildWorkflow(stepCtx, ProcessingStepWorkflow, stepInput)

		var stepOutput types.ProcessingStepOutput
		err = childFuture.Get(ctx, &stepOutput)

		stepEndTime := workflow.Now(ctx)

		// Check for user cancellation FIRST (before other errors)
		if temporal.IsCanceledError(err) {
			logger.Info("Pipeline cancelled by user", "stepID", stepDef.StepID)

			// Use disconnected context for cleanup - ensures activities run even after cancellation
			cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
			cleanupOrchestratorCtx := workflow.WithActivityOptions(cleanupCtx, orchestratorActivityOptions)

			// CRITICAL: Explicitly cancel the child workflow to stop the running agent
			// Without this, the child workflow continues running until PARENT_CLOSE_POLICY_TERMINATE
			// kicks in when this workflow closes, which can take several seconds
			logger.Info("Requesting cancellation of child workflow", "childWorkflowID", childWorkflowID)
			cancelFuture := workflow.RequestCancelExternalWorkflow(cleanupCtx, childWorkflowID, "")
			if cancelErr := cancelFuture.Get(cleanupCtx, nil); cancelErr != nil {
				logger.Warn("Failed to request child workflow cancellation", "childWorkflowID", childWorkflowID, "error", cancelErr)
			} else {
				logger.Info("Child workflow cancellation requested successfully", "childWorkflowID", childWorkflowID)
			}

			// Give the child workflow time to propagate cancellation to its activities
			// This allows LocalExecuteActivity to kill the subprocess via exec.CommandContext
			_ = workflow.NewTimer(cleanupCtx, 2*time.Second).Get(cleanupCtx, nil)

			// Mark step as failed with cancellation message
			stepResult.Status = models.StepStatusFailed
			stepResult.ErrorMessage = "Cancelled by user"
			stepResult.CompletedAt = &stepEndTime
			if err := workflow.ExecuteActivity(cleanupOrchestratorCtx, "SaveStepResultActivity",
				types.SaveStepResultActivityInput{Result: stepResult}).Get(cleanupCtx, nil); err != nil {
				logger.Warn("Failed to save step result during cancellation cleanup", "error", err)
			}

			// Mark pipeline as failed
			output.Error = "Pipeline cancelled by user"
			markPipelineRunFailed(cleanupOrchestratorCtx, input.RunID, "Cancelled by user")

			// Emit PipelineFailed event for TUI visibility
			if err := workflow.ExecuteActivity(cleanupOrchestratorCtx, "PublishPipelineFailedEventActivity",
				types.PublishPipelineEventInput{
					ProjectID: input.ProjectID,
					RunID:     input.RunID,
					Name:      input.Name,
				}).Get(cleanupCtx, nil); err != nil {
				logger.Warn("Failed to publish pipeline failed event during cancellation cleanup", "error", err)
			}

			return output, temporal.NewCanceledError("user requested cancellation")
		}

		if err != nil || !stepOutput.Success {
			errMsg := fmt.Sprintf("Step %s failed", stepDef.StepID)
			if err != nil {
				errMsg = fmt.Sprintf("Step %s failed: %v", stepDef.StepID, err)
			} else if stepOutput.Error != "" {
				errMsg = fmt.Sprintf("Step %s failed: %s", stepDef.StepID, stepOutput.Error)
			}
			logger.Error(errMsg)

			// Emit PipelineStepFailed event (non-fatal: TUI visibility only)
			_ = workflow.ExecuteActivity(orchestratorCtx, "PublishPipelineStepFailedEventActivity",
				types.PublishPipelineEventInput{
					ProjectID: input.ProjectID,
					RunID:     input.RunID,
					Name:      input.Name,
					StepID:    stepDef.StepID,
					StepIndex: i,
					StepName:  stepDef.Name,
				}).Get(ctx, nil)

			// Update step result as failed
			stepResult.Status = models.StepStatusFailed
			stepResult.ErrorMessage = errMsg
			stepResult.CompletedAt = &stepEndTime
			// Non-fatal: best-effort save of failed step status
			_ = workflow.ExecuteActivity(orchestratorCtx, "SaveStepResultActivity",
				types.SaveStepResultActivityInput{Result: stepResult}).Get(ctx, nil)

			output.StepResults = append(output.StepResults, stepOutput)
			output.Error = errMsg
			markPipelineRunFailed(orchestratorCtx, input.RunID, errMsg)

			// Emit PipelineFailed event (non-fatal: TUI visibility only)
			_ = workflow.ExecuteActivity(orchestratorCtx, "PublishPipelineFailedEventActivity",
				types.PublishPipelineEventInput{
					ProjectID: input.ProjectID,
					RunID:     input.RunID,
					Name:      input.Name,
				}).Get(ctx, nil)

			return output, fmt.Errorf("%s", errMsg)
		}

		// Update step result as completed
		stepResult.Status = models.StepStatusCompleted
		stepResult.CommitSHA = stepOutput.CommitSHA
		stepResult.CommitMessage = stepOutput.CommitMessage
		stepResult.GitDiff = stepOutput.GitDiff
		stepResult.FilesChanged = stepOutput.FilesChanged
		stepResult.Insertions = stepOutput.Insertions
		stepResult.Deletions = stepOutput.Deletions
		stepResult.InputTokens = stepOutput.InputTokens
		stepResult.OutputTokens = stepOutput.OutputTokens
		stepResult.CacheReadTokens = stepOutput.CacheReadTokens
		stepResult.CacheCreateTokens = stepOutput.CacheCreateTokens
		stepResult.AgentOutput = stepOutput.AgentOutput
		stepResult.Duration = stepOutput.Duration
		stepResult.CompletedAt = &stepEndTime

		// Non-fatal: best-effort save of completed step status
		_ = workflow.ExecuteActivity(orchestratorCtx, "SaveStepResultActivity",
			types.SaveStepResultActivityInput{Result: stepResult}).Get(ctx, nil)

		// Emit PipelineStepCompleted event (non-fatal: TUI visibility only)
		_ = workflow.ExecuteActivity(orchestratorCtx, "PublishPipelineStepCompletedEventActivity",
			types.PublishPipelineEventInput{
				ProjectID:  input.ProjectID,
				RunID:      input.RunID,
				Name:       input.Name,
				StepID:     stepDef.StepID,
				StepIndex:  i,
				StepResult: stepResult,
			}).Get(ctx, nil)

		output.StepResults = append(output.StepResults, stepOutput)
		currentCommit = stepOutput.CommitSHA
		previousStepID = stepDef.StepID // Track for next step's runtime vars

		logger.Info("Step completed",
			"stepID", stepDef.StepID,
			"commitSHA", stepOutput.CommitSHA,
			"filesChanged", stepOutput.FilesChanged)
	}

	output.HeadCommitSHA = currentCommit

	// Clear step context on observability workflow now that all steps are done
	signalErr := workflow.SignalExternalWorkflow(ctx, obsWorkflowExecution.ID, "", StepChangeSignal, "").Get(ctx, nil)
	if signalErr != nil {
		logger.Warn("Failed to clear step context on observability workflow", "error", signalErr)
	}

	// =========================================================================
	// Phase 2b: Wait for observability to finish reading final data
	// =========================================================================
	// Give observability workflow time to read/forward the last transcript lines
	// before we complete (which will terminate the observability child workflow)
	logger.Info("Waiting 5s for observability to read final data")
	_ = workflow.NewTimer(ctx, 5*time.Second).Get(ctx, nil) // Timer errors are non-fatal

	// =========================================================================
	// Phase 3: Finalize
	// =========================================================================
	logger.Info("Phase 3: Finalize")

	// Finalize pipeline run - set status, head commit, and completion time
	completedTime := workflow.Now(ctx)
	finalRun := &models.PipelineRun{
		ID:            input.RunID,
		Status:        models.PipelineRunStatusCompleted,
		HeadCommitSHA: currentCommit,
		CompletedAt:   &completedTime,
	}
	// Non-fatal: best-effort save of final run status
	_ = workflow.ExecuteActivity(orchestratorCtx, "SavePipelineRunActivity",
		types.SavePipelineRunActivityInput{Run: finalRun}).Get(ctx, nil)

	// Emit PipelineFinished event (non-fatal: TUI visibility only)
	_ = workflow.ExecuteActivity(orchestratorCtx, "PublishPipelineFinishedEventActivity",
		types.PublishPipelineEventInput{
			ProjectID: input.ProjectID,
			RunID:     input.RunID,
			Name:      input.Name,
			Run:       finalRun,
		}).Get(ctx, nil)

	output.Success = true
	output.Duration = workflow.Now(ctx).Sub(startTime)

	logger.Info("PipelineWorkflow completed successfully",
		"runID", input.RunID,
		"stepsCompleted", len(output.StepResults),
		"headCommit", currentCommit,
		"duration", output.Duration)

	return output, nil
}

// markPipelineRunFailed updates the pipeline run status to failed
func markPipelineRunFailed(ctx workflow.Context, runID, errorMsg string) {
	logger := workflow.GetLogger(ctx)
	err := workflow.ExecuteActivity(ctx, "UpdatePipelineRunStatusActivity",
		types.UpdatePipelineRunStatusActivityInput{
			RunID:  runID,
			Status: models.PipelineRunStatusFailed,
		}).Get(ctx, nil)
	if err != nil {
		logger.Warn("Failed to mark pipeline run as failed", "error", err)
	}
}

// RuntimeVars contains variables available at step execution time
type RuntimeVars struct {
	RunID          string // Current pipeline run ID
	StepIndex      int    // Current step index (0-based)
	StepID         string // Current step ID
	PreviousStepID string // Previous step ID (empty for first step)
}

// injectRuntimeVars performs second-pass variable substitution with runtime variables.
// Uses simple string replacement instead of text/template to avoid template injection
// from user-controlled prompt content.
func injectRuntimeVars(prompt string, vars RuntimeVars) (string, error) {
	r := strings.NewReplacer(
		"{{.RunID}}", vars.RunID,
		"{{ .RunID }}", vars.RunID,
		"{{.StepIndex}}", fmt.Sprintf("%d", vars.StepIndex),
		"{{ .StepIndex }}", fmt.Sprintf("%d", vars.StepIndex),
		"{{.StepID}}", vars.StepID,
		"{{ .StepID }}", vars.StepID,
		"{{.PreviousStepID}}", vars.PreviousStepID,
		"{{ .PreviousStepID }}", vars.PreviousStepID,
	)
	return r.Replace(prompt), nil
}

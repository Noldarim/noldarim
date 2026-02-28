// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	PromoteWorkflowName    = "PromoteWorkflow"
	PromoteWorkflowVersion = "v1.0.0"
)

// truncateID safely truncates an ID string to at most n characters.
func truncateID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n]
}

// PromoteWorkflow merges a completed pipeline run's task branch into the main branch.
// It creates a PipelineRun record with run_type "promote" for graph observability.
//
// Merge strategy:
//  1. Check if fast-forward is possible (main hasn't diverged)
//  2. If FF: advance main ref directly — no merge commit needed
//  3. If not FF: create worktree from main, merge task branch in
//     a. If clean merge: advance main to merge commit
//     b. If conflicts: spawn AI agent to resolve, then advance main
func PromoteWorkflow(ctx workflow.Context, input types.PromoteWorkflowInput) (*types.PromoteWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting PromoteWorkflow",
		"promoteRunID", input.PromoteRunID,
		"sourceRunID", input.SourceRunID,
		"mainBranch", input.MainBranch,
		"sourceBranch", input.SourceBranchName)

	output := &types.PromoteWorkflowOutput{
		Success: false,
		RunID:   input.PromoteRunID,
	}

	// Saga compensations for cleanup on failure
	var compensations []compensation

	// Orchestrator activity options (DB + git on orchestrator queue)
	orchestratorOpts := workflow.ActivityOptions{
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
	orchCtx := workflow.WithActivityOptions(ctx, orchestratorOpts)

	// =========================================================================
	// Phase 1: Create promote PipelineRun record
	// =========================================================================
	logger.Info("Phase 1: Creating promote PipelineRun record")

	now := workflow.Now(ctx)
	promoteRun := &models.PipelineRun{
		ID:             input.PromoteRunID,
		ProjectID:      input.ProjectID,
		Name:           fmt.Sprintf("Promote %s", truncateID(input.SourceRunID, 8)),
		Status:         models.PipelineRunStatusRunning,
		RunType:        models.PipelineRunTypePromote,
		SourceRunID:    input.SourceRunID,
		BaseCommitSHA:  input.SourceHeadCommitSHA,
		StartCommitSHA: input.SourceHeadCommitSHA,
		BranchName:     input.SourceBranchName,
		StartedAt:      &now,
	}

	err := workflow.ExecuteActivity(orchCtx, "SavePipelineRunActivity",
		types.SavePipelineRunActivityInput{Run: promoteRun}).Get(ctx, nil)
	if err != nil {
		output.Error = fmt.Sprintf("Failed to save promote run: %v", err)
		return output, fmt.Errorf("%s", output.Error)
	}

	// Emit PipelineCreated event
	_ = workflow.ExecuteActivity(orchCtx, "PublishPipelineCreatedEventActivity",
		types.PublishPipelineEventInput{
			ProjectID: input.ProjectID,
			RunID:     input.PromoteRunID,
			Name:      promoteRun.Name,
			Run:       promoteRun,
		}).Get(ctx, nil)

	// failPromote and checkCancelled are closures (not top-level functions like the
	// pipeline equivalents) because they capture PromoteWorkflowOutput and the promote-
	// specific compensation list, which differ from PipelineWorkflowOutput.
	failPromote := func(errMsg string) (*types.PromoteWorkflowOutput, error) {
		logger.Error("Promote failed: " + errMsg)
		output.Error = errMsg

		// Run compensations in disconnected context
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		runCompensations(cleanupCtx, compensations)

		// Mark run as failed
		cleanupOrchCtx := workflow.WithActivityOptions(cleanupCtx, orchestratorOpts)
		markPipelineRunFailed(cleanupOrchCtx, input.PromoteRunID, errMsg)

		// Emit failed event
		_ = workflow.ExecuteActivity(cleanupOrchCtx, "PublishPipelineFailedEventActivity",
			types.PublishPipelineEventInput{
				ProjectID: input.ProjectID,
				RunID:     input.PromoteRunID,
				Name:      promoteRun.Name,
			}).Get(cleanupCtx, nil)

		return output, fmt.Errorf("%s", errMsg)
	}

	// Helper to detect cancellation and handle cleanup
	checkCancelled := func(err error, operation string) (*types.PromoteWorkflowOutput, error, bool) {
		if !temporal.IsCanceledError(err) {
			return nil, nil, false
		}
		logger.Info("PromoteWorkflow cancelled", "operation", operation)
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		runCompensations(cleanupCtx, compensations)
		cleanupOrchCtx := workflow.WithActivityOptions(cleanupCtx, orchestratorOpts)
		markPipelineRunFailed(cleanupOrchCtx, input.PromoteRunID, "Cancelled by user")
		_ = workflow.ExecuteActivity(cleanupOrchCtx, "PublishPipelineFailedEventActivity",
			types.PublishPipelineEventInput{
				ProjectID: input.ProjectID,
				RunID:     input.PromoteRunID,
				Name:      promoteRun.Name,
			}).Get(cleanupCtx, nil)
		output.Error = "Cancelled by user"
		return output, temporal.NewCanceledError("user requested cancellation"), true
	}

	// =========================================================================
	// Phase 2: Check FF feasibility
	// =========================================================================
	logger.Info("Phase 2: Checking fast-forward feasibility")

	var ffResult types.CheckFastForwardOutput
	err = workflow.ExecuteActivity(orchCtx, "CheckFastForwardActivity",
		types.CheckFastForwardInput{
			RepoPath:   input.RepositoryPath,
			MainBranch: input.MainBranch,
			TaskBranch: input.SourceBranchName,
		}).Get(ctx, &ffResult)
	if out, cancelErr, cancelled := checkCancelled(err, "CheckFastForwardActivity"); cancelled {
		return out, cancelErr
	}
	if err != nil {
		return failPromote(fmt.Sprintf("Failed to check fast-forward: %v", err))
	}

	logger.Info("FF check result", "isFF", ffResult.IsFF, "mainHead", ffResult.MainHeadSHA)

	// =========================================================================
	// Phase 3: Merge
	// =========================================================================
	if ffResult.IsFF {
		// --- Phase 3a: Fast-forward path ---
		logger.Info("Phase 3a: Fast-forward merge")

		err = workflow.ExecuteActivity(orchCtx, "FastForwardBranchActivity",
			types.FastForwardBranchInput{
				RepoPath:       input.RepositoryPath,
				Branch:         input.MainBranch,
				TargetSHA:      input.SourceHeadCommitSHA,
				ExpectedOldSHA: ffResult.MainHeadSHA,
			}).Get(ctx, nil)
		if out, cancelErr, cancelled := checkCancelled(err, "FastForwardBranchActivity"); cancelled {
			return out, cancelErr
		}
		if err != nil {
			return failPromote(fmt.Sprintf("Failed to fast-forward: %v", err))
		}

		// Save merge step result
		mergeStepResult := &models.StepResult{
			ID:            fmt.Sprintf("%s-step-merge", input.PromoteRunID),
			PipelineRunID: input.PromoteRunID,
			StepID:        "merge",
			StepName:      "Fast-forward merge",
			StepIndex:     0,
			Status:        models.StepStatusCompleted,
			CommitSHA:     input.SourceHeadCommitSHA,
			CommitMessage: fmt.Sprintf("Fast-forward %s to %s", input.MainBranch, truncateID(input.SourceHeadCommitSHA, 8)),
		}
		completedAt := workflow.Now(ctx)
		mergeStepResult.CompletedAt = &completedAt
		_ = workflow.ExecuteActivity(orchCtx, "SaveStepResultActivity",
			types.SaveStepResultActivityInput{Result: mergeStepResult}).Get(ctx, nil)

		output.MergeMethod = "fast-forward"
		output.FinalCommitSHA = input.SourceHeadCommitSHA

	} else {
		// --- Phase 3b: Non-fast-forward path ---
		logger.Info("Phase 3b: Non-fast-forward merge (main has diverged)")

		// Step 1: Create worktree from main HEAD for the merge
		promoteTaskID := fmt.Sprintf("promote-%s", truncateID(input.PromoteRunID, 8))
		var worktreeResult types.CreateWorktreeActivityOutput
		err = workflow.ExecuteActivity(orchCtx, "CreateWorktreeActivity",
			types.CreateWorktreeActivityInput{
				TaskID:         promoteTaskID,
				BranchName:     fmt.Sprintf("promote-%s", truncateID(input.PromoteRunID, 8)),
				RepositoryPath: input.RepositoryPath,
				BaseCommitSHA:  ffResult.MainHeadSHA,
			}).Get(ctx, &worktreeResult)
		if out, cancelErr, cancelled := checkCancelled(err, "CreateWorktreeActivity"); cancelled {
			return out, cancelErr
		}
		if err != nil {
			return failPromote(fmt.Sprintf("Failed to create promote worktree: %v", err))
		}
		compensations = append(compensations, worktreeCompensation(worktreeResult.WorktreePath, input.RepositoryPath))

		// Step 2: Merge source branch into promote worktree
		var mergeResult types.MergeInWorktreeOutput
		err = workflow.ExecuteActivity(orchCtx, "MergeInWorktreeActivity",
			types.MergeInWorktreeInput{
				WorktreePath:  worktreeResult.WorktreePath,
				BranchToMerge: input.SourceBranchName,
			}).Get(ctx, &mergeResult)
		if out, cancelErr, cancelled := checkCancelled(err, "MergeInWorktreeActivity"); cancelled {
			return out, cancelErr
		}
		if err != nil {
			return failPromote(fmt.Sprintf("Merge failed: %v", err))
		}

		if !mergeResult.HasConflicts {
			// Clean merge — advance main to the merge commit
			logger.Info("Clean merge, advancing main", "mergeCommitSHA", mergeResult.CommitSHA)

			// Re-check main HEAD hasn't moved during merge
			var currentMainHead types.GetBranchHeadOutput
			err = workflow.ExecuteActivity(orchCtx, "GetBranchHeadActivity",
				types.GetBranchHeadInput{RepoPath: input.RepositoryPath, Branch: input.MainBranch},
			).Get(ctx, &currentMainHead)
			if err != nil {
				return failPromote(fmt.Sprintf("Failed to re-check main HEAD: %v", err))
			}
			if currentMainHead.SHA != ffResult.MainHeadSHA {
				return failPromote(fmt.Sprintf(
					"main branch modified externally during promote (expected %s, found %s); please retry",
					truncateID(ffResult.MainHeadSHA, 8), truncateID(currentMainHead.SHA, 8)))
			}

			err = workflow.ExecuteActivity(orchCtx, "FastForwardBranchActivity",
				types.FastForwardBranchInput{
					RepoPath:       input.RepositoryPath,
					Branch:         input.MainBranch,
					TargetSHA:      mergeResult.CommitSHA,
					ExpectedOldSHA: ffResult.MainHeadSHA,
				}).Get(ctx, nil)
			if out, cancelErr, cancelled := checkCancelled(err, "FastForwardBranchActivity"); cancelled {
				return out, cancelErr
			}
			if err != nil {
				return failPromote(fmt.Sprintf("Failed to advance main after merge: %v", err))
			}

			// Save merge step result
			mergeStepResult := &models.StepResult{
				ID:            fmt.Sprintf("%s-step-merge", input.PromoteRunID),
				PipelineRunID: input.PromoteRunID,
				StepID:        "merge",
				StepName:      "Merge",
				StepIndex:     0,
				Status:        models.StepStatusCompleted,
				CommitSHA:     mergeResult.CommitSHA,
				CommitMessage: fmt.Sprintf("Merge %s into %s", input.SourceBranchName, input.MainBranch),
			}
			completedAt := workflow.Now(ctx)
			mergeStepResult.CompletedAt = &completedAt
			_ = workflow.ExecuteActivity(orchCtx, "SaveStepResultActivity",
				types.SaveStepResultActivityInput{Result: mergeStepResult}).Get(ctx, nil)

			output.MergeMethod = "clean-merge"
			output.FinalCommitSHA = mergeResult.CommitSHA

		} else {
			// Conflicts — need AI resolution
			logger.Info("Merge has conflicts, attempting AI resolution")

			// Save informational merge step as failed (shows conflict in UI)
			mergeStepResult := &models.StepResult{
				ID:            fmt.Sprintf("%s-step-merge", input.PromoteRunID),
				PipelineRunID: input.PromoteRunID,
				StepID:        "merge",
				StepName:      "Merge (conflicts)",
				StepIndex:     0,
				Status:        models.StepStatusFailed,
				ErrorMessage:  "Merge conflicts detected, AI resolution needed",
			}
			failedAt := workflow.Now(ctx)
			mergeStepResult.CompletedAt = &failedAt
			_ = workflow.ExecuteActivity(orchCtx, "SaveStepResultActivity",
				types.SaveStepResultActivityInput{Result: mergeStepResult}).Get(ctx, nil)

			// Create container for AI conflict resolution
			runTaskQueue := generateTaskQueueName(input.PromoteRunID)
			var containerResult types.CreateContainerActivityOutput
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
			err = workflow.ExecuteActivity(containerCtx, "CreateContainerActivity",
				types.CreateContainerActivityInput{
					TaskID:       input.PromoteRunID,
					WorktreePath: worktreeResult.WorktreePath,
					ProjectID:    input.ProjectID,
					TaskQueue:    runTaskQueue,
				}).Get(ctx, &containerResult)
			if err != nil {
				return failPromote(fmt.Sprintf("Failed to create container for conflict resolution: %v", err))
			}
			compensations = append(compensations, containerCompensation(containerResult.ContainerID))

			// Copy Claude config
			var configResult types.CopyClaudeConfigActivityOutput
			err = workflow.ExecuteActivity(orchCtx, "CopyClaudeConfigActivity",
				types.CopyClaudeConfigActivityInput{
					ContainerID:    containerResult.ContainerID,
					HostConfigPath: input.ClaudeConfigPath,
				}).Get(ctx, &configResult)
			if err != nil {
				return failPromote(fmt.Sprintf("Failed to copy Claude config: %v", err))
			}

			// Copy credentials
			var credentialsResult types.CopyClaudeCredentialsActivityOutput
			err = workflow.ExecuteActivity(orchCtx, "CopyClaudeCredentialsActivity",
				types.CopyClaudeCredentialsActivityInput{
					ContainerID: containerResult.ContainerID,
				}).Get(ctx, &credentialsResult)
			if err != nil {
				return failPromote(fmt.Sprintf("Failed to copy Claude credentials: %v", err))
			}

			// Spawn ProcessingStepWorkflow for conflict resolution
			resolvePrompt := `Resolve all git merge conflicts in the working directory. Files with conflicts contain <<<<<<< markers. Understand both sides of each conflict, resolve to preserve the intent of both changes, then stage all resolved files with 'git add'. Finally commit the resolved merge.`

			resolveStepInput := types.ProcessingStepInput{
				RunID:     input.PromoteRunID,
				ProjectID: input.ProjectID,
				StepID:    "resolve",
				StepIndex: 1,
				StepName:  "AI conflict resolution",
				AgentConfig: &protocol.AgentConfigInput{
					ToolName:       "claude",
					PromptTemplate: resolvePrompt,
				},
				WorktreePath:          worktreeResult.WorktreePath,
				WorkspaceDir:          input.WorkspaceDir,
				OrchestratorTaskQueue: input.OrchestratorTaskQueue,
			}

			resolveChildOpts := workflow.ChildWorkflowOptions{
				WorkflowID:               fmt.Sprintf("%s-step-resolve", input.PromoteRunID),
				WorkflowExecutionTimeout: 30 * time.Minute,
				WorkflowTaskTimeout:      time.Minute,
				TaskQueue:                runTaskQueue,
				ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_TERMINATE,
			}
			resolveCtx := workflow.WithChildOptions(ctx, resolveChildOpts)

			var resolveOutput types.ProcessingStepOutput
			err = workflow.ExecuteChildWorkflow(resolveCtx, ProcessingStepWorkflow, resolveStepInput).Get(ctx, &resolveOutput)
			if out, cancelErr, cancelled := checkCancelled(err, "ProcessingStepWorkflow"); cancelled {
				return out, cancelErr
			}
			if err != nil || !resolveOutput.Success {
				errMsg := "AI conflict resolution failed"
				if err != nil {
					errMsg = fmt.Sprintf("AI conflict resolution failed: %v", err)
				} else if resolveOutput.Error != "" {
					errMsg = fmt.Sprintf("AI conflict resolution failed: %s", resolveOutput.Error)
				}
				return failPromote(errMsg)
			}

			// Get the resolved commit SHA and advance main
			resolvedSHA := resolveOutput.CommitSHA
			if resolvedSHA == "" {
				// Fallback: get HEAD of the worktree
				var headResult types.GetBranchHeadOutput
				err = workflow.ExecuteActivity(orchCtx, "GetBranchHeadActivity",
					types.GetBranchHeadInput{
						RepoPath: worktreeResult.WorktreePath,
						Branch:   fmt.Sprintf("promote-%s", truncateID(input.PromoteRunID, 8)),
					}).Get(ctx, &headResult)
				if err != nil {
					return failPromote(fmt.Sprintf("Failed to get resolved commit SHA: %v", err))
				}
				resolvedSHA = headResult.SHA
			}

			// Re-check main HEAD hasn't moved during AI resolution
			var currentMainHeadAI types.GetBranchHeadOutput
			err = workflow.ExecuteActivity(orchCtx, "GetBranchHeadActivity",
				types.GetBranchHeadInput{RepoPath: input.RepositoryPath, Branch: input.MainBranch},
			).Get(ctx, &currentMainHeadAI)
			if err != nil {
				return failPromote(fmt.Sprintf("Failed to re-check main HEAD: %v", err))
			}
			if currentMainHeadAI.SHA != ffResult.MainHeadSHA {
				return failPromote(fmt.Sprintf(
					"main branch modified externally during promote (expected %s, found %s); please retry",
					ffResult.MainHeadSHA[:8], currentMainHeadAI.SHA[:8]))
			}

			err = workflow.ExecuteActivity(orchCtx, "FastForwardBranchActivity",
				types.FastForwardBranchInput{
					RepoPath:       input.RepositoryPath,
					Branch:         input.MainBranch,
					TargetSHA:      resolvedSHA,
					ExpectedOldSHA: ffResult.MainHeadSHA,
				}).Get(ctx, nil)
			if out, cancelErr, cancelled := checkCancelled(err, "FastForwardBranchActivity"); cancelled {
				return out, cancelErr
			}
			if err != nil {
				return failPromote(fmt.Sprintf("Failed to advance main after conflict resolution: %v", err))
			}

			// Save resolve step result
			resolveStepResult := &models.StepResult{
				ID:            fmt.Sprintf("%s-step-resolve", input.PromoteRunID),
				PipelineRunID: input.PromoteRunID,
				StepID:        "resolve",
				StepName:      "AI conflict resolution",
				StepIndex:     1,
				Status:        models.StepStatusCompleted,
				CommitSHA:     resolvedSHA,
				CommitMessage: fmt.Sprintf("AI-resolved merge of %s into %s", input.SourceBranchName, input.MainBranch),
				AgentOutput:   resolveOutput.AgentOutput,
				Duration:      resolveOutput.Duration,
			}
			resolveCompleted := workflow.Now(ctx)
			resolveStepResult.CompletedAt = &resolveCompleted
			_ = workflow.ExecuteActivity(orchCtx, "SaveStepResultActivity",
				types.SaveStepResultActivityInput{Result: resolveStepResult}).Get(ctx, nil)

			output.MergeMethod = "ai-resolved"
			output.FinalCommitSHA = resolvedSHA
		}

		// Cleanup: remove promote worktree (best-effort)
		cleanupCtx, _ := workflow.NewDisconnectedContext(ctx)
		runCompensations(cleanupCtx, compensations)
		compensations = nil // Clear so failPromote doesn't double-run
	}

	// =========================================================================
	// Phase 4: Finalize — mark promote run as completed
	// =========================================================================
	logger.Info("Phase 4: Finalizing promote", "method", output.MergeMethod, "finalCommit", output.FinalCommitSHA)

	completedTime := workflow.Now(ctx)
	finalRun := &models.PipelineRun{
		ID:            input.PromoteRunID,
		Status:        models.PipelineRunStatusCompleted,
		RunType:       models.PipelineRunTypePromote,
		HeadCommitSHA: output.FinalCommitSHA,
		CompletedAt:   &completedTime,
	}
	if err := workflow.ExecuteActivity(orchCtx, "SavePipelineRunActivity",
		types.SavePipelineRunActivityInput{Run: finalRun}).Get(ctx, nil); err != nil {
		logger.Error("Failed to save final promote run status", "error", err)
		// The merge itself succeeded; log but don't fail the workflow
	}

	// Emit finished event
	_ = workflow.ExecuteActivity(orchCtx, "PublishPipelineFinishedEventActivity",
		types.PublishPipelineEventInput{
			ProjectID: input.ProjectID,
			RunID:     input.PromoteRunID,
			Name:      promoteRun.Name,
			Run:       finalRun,
		}).Get(ctx, nil)

	output.Success = true
	logger.Info("PromoteWorkflow completed successfully",
		"method", output.MergeMethod,
		"finalCommit", output.FinalCommitSHA)

	return output, nil
}

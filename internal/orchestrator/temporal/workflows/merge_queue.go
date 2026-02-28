// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

const (
	MergeQueueWorkflowName = "MergeQueueWorkflow"

	// Signal names for the merge queue
	PromoteSignal       = "promote"
	CancelPromoteSignal = "cancel-promote"

	// Query name
	MergeQueueStateQuery = "merge-queue-state"

	// ContinueAsNew after this many processed items to bound history size
	ContinueAsNewThreshold = 50

	// IdleTimeout controls how long the merge queue waits with no signals before terminating.
	// SignalWithStartWorkflow handles restart-on-demand.
	IdleTimeout = 30 * time.Minute

	// idleTickInterval is the polling interval; idle termination occurs after
	// IdleTimeout / idleTickInterval consecutive idle ticks.
	idleTickInterval = 30 * time.Second
)

// MergeQueueWorkflow is a long-running per-project workflow that serializes
// promote operations. Items are added via PromoteSignal and processed FIFO.
//
// Workflow ID: merge-queue-<projectID>
func MergeQueueWorkflow(ctx workflow.Context, input types.MergeQueueWorkflowInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MergeQueueWorkflow",
		"projectID", input.ProjectID,
		"pendingItems", len(input.PendingItems),
		"processedCount", input.ProcessedCount)

	queue := make([]types.MergeQueueItem, len(input.PendingItems))
	copy(queue, input.PendingItems)
	currentlyProcessing := ""
	processedCount := input.ProcessedCount
	maxIdleTicks := int(IdleTimeout / idleTickInterval)
	consecutiveIdleTicks := 0

	// Register signal handlers
	promoteCh := workflow.GetSignalChannel(ctx, PromoteSignal)
	cancelCh := workflow.GetSignalChannel(ctx, CancelPromoteSignal)

	// Register query handler
	err := workflow.SetQueryHandler(ctx, MergeQueueStateQuery, func() (*types.MergeQueueState, error) {
		return &types.MergeQueueState{
			Items:               queue,
			CurrentlyProcessing: currentlyProcessing,
		}, nil
	})
	if err != nil {
		logger.Error("Failed to set query handler", "error", err)
	}

	// cancelCurrentChild is set when a child PromoteWorkflow is running and
	// allows drainSignals to cancel it if a cancel signal matches the in-flight RunID.
	var cancelCurrentChild func()

	// drainSignals reads all pending signals without blocking.
	// Returns true if the currently processing child should be cancelled.
	drainSignals := func() bool {
		cancelInFlight := false
		for {
			var item types.MergeQueueItem
			ok := promoteCh.ReceiveAsync(&item)
			if !ok {
				break
			}
			logger.Info("Received promote signal", "runID", item.RunID)
			queue = append(queue, item)
		}
		for {
			var cancelRunID string
			ok := cancelCh.ReceiveAsync(&cancelRunID)
			if !ok {
				break
			}
			logger.Info("Received cancel-promote signal", "runID", cancelRunID)

			// If the cancel targets the currently processing item, cancel its child workflow
			if cancelRunID == currentlyProcessing && cancelCurrentChild != nil {
				logger.Info("Cancelling in-progress promote", "runID", cancelRunID)
				cancelCurrentChild()
				cancelInFlight = true
			}

			newQueue := make([]types.MergeQueueItem, 0, len(queue))
			for _, item := range queue {
				if item.RunID != cancelRunID {
					newQueue = append(newQueue, item)
				}
			}
			queue = newQueue
		}

		// Deduplicate by RunID (auto-promote + manual click can queue the same run twice).
		// Also skip any item whose RunID is currently being processed to prevent spawning
		// a second PromoteWorkflow for the same run.
		seen := make(map[string]struct{}, len(queue))
		if currentlyProcessing != "" {
			seen[currentlyProcessing] = struct{}{}
		}
		deduped := make([]types.MergeQueueItem, 0, len(queue))
		for _, item := range queue {
			if _, exists := seen[item.RunID]; !exists {
				seen[item.RunID] = struct{}{}
				deduped = append(deduped, item)
			}
		}
		queue = deduped
		return cancelInFlight
	}

	for {
		// Drain any pending signals
		drainSignals()

		if len(queue) == 0 {
			// Wait for a signal (with timeout to periodically check)
			timerCtx, cancelTimer := workflow.WithCancel(ctx)
			timerFuture := workflow.NewTimer(timerCtx, idleTickInterval)

			// Create a selector to wait for either a signal or the timer
			selector := workflow.NewSelector(ctx)
			gotSignal := false

			selector.AddReceive(promoteCh, func(c workflow.ReceiveChannel, more bool) {
				var item types.MergeQueueItem
				c.Receive(ctx, &item)
				logger.Info("Received promote signal (waiting)", "runID", item.RunID)
				queue = append(queue, item)
				gotSignal = true
			})
			selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, more bool) {
				var cancelRunID string
				c.Receive(ctx, &cancelRunID)
				logger.Info("Received cancel signal (waiting)", "runID", cancelRunID)
				gotSignal = true
			})
			selector.AddFuture(timerFuture, func(f workflow.Future) {
				// Timer fired — loop again
			})

			selector.Select(ctx)
			cancelTimer()

			if !gotSignal {
				consecutiveIdleTicks++

				// Terminate cleanly if idle for too long
				if consecutiveIdleTicks >= maxIdleTicks {
					logger.Info("Merge queue idle timeout reached, terminating",
						"idleTicks", consecutiveIdleTicks,
						"idleTimeout", IdleTimeout)
					return nil
				}

				// Check for ContinueAsNew
				if processedCount >= ContinueAsNewThreshold {
					return workflow.NewContinueAsNewError(ctx, MergeQueueWorkflow, types.MergeQueueWorkflowInput{
						ProjectID:             input.ProjectID,
						RepositoryPath:        input.RepositoryPath,
						MainBranch:            input.MainBranch,
						ClaudeConfigPath:      input.ClaudeConfigPath,
						WorkspaceDir:          input.WorkspaceDir,
						OrchestratorTaskQueue: input.OrchestratorTaskQueue,
						PendingItems:          queue,
						ProcessedCount:        0,
					})
				}
				continue
			}
			// Got a signal — reset idle counter
			consecutiveIdleTicks = 0
			// Drain any additional signals that arrived
			drainSignals()
		}

		if len(queue) == 0 {
			continue
		}

		// Dequeue first item (FIFO)
		consecutiveIdleTicks = 0
		item := queue[0]
		queue = queue[1:]
		currentlyProcessing = item.RunID

		logger.Info("Processing promote", "runID", item.RunID, "queueRemaining", len(queue))

		// Generate a deterministic promote run ID
		promoteRunID := computePromoteRunID(item.RunID, input.MainBranch, item.QueuedAt)

		// Spawn PromoteWorkflow as child with a cancellable context so
		// drainSignals can cancel it if a CancelPromoteSignal arrives mid-flight.
		childCtx, childCancel := workflow.WithCancel(ctx)
		cancelCurrentChild = childCancel

		promoteChildOpts := workflow.ChildWorkflowOptions{
			WorkflowID:               fmt.Sprintf("promote-%s", item.RunID),
			WorkflowExecutionTimeout: 60 * time.Minute,
			WorkflowTaskTimeout:      time.Minute,
			TaskQueue:                input.OrchestratorTaskQueue,
			ParentClosePolicy:        enums.PARENT_CLOSE_POLICY_TERMINATE,
		}
		promoteCtx := workflow.WithChildOptions(childCtx, promoteChildOpts)

		promoteInput := types.PromoteWorkflowInput{
			PromoteRunID:          promoteRunID,
			SourceRunID:           item.RunID,
			ProjectID:             input.ProjectID,
			RepositoryPath:        input.RepositoryPath,
			MainBranch:            input.MainBranch,
			SourceBranchName:      item.SourceBranchName,
			SourceHeadCommitSHA:   item.SourceHeadCommitSHA,
			ClaudeConfigPath:      input.ClaudeConfigPath,
			WorkspaceDir:          input.WorkspaceDir,
			OrchestratorTaskQueue: input.OrchestratorTaskQueue,
		}

		var promoteOutput types.PromoteWorkflowOutput
		err := workflow.ExecuteChildWorkflow(promoteCtx, PromoteWorkflow, promoteInput).Get(childCtx, &promoteOutput)
		cancelCurrentChild = nil
		if err != nil {
			logger.Error("Promote workflow failed", "runID", item.RunID, "error", err)
		} else if !promoteOutput.Success {
			logger.Error("Promote workflow returned failure", "runID", item.RunID, "error", promoteOutput.Error)
		} else {
			logger.Info("Promote completed", "runID", item.RunID, "method", promoteOutput.MergeMethod, "finalCommit", promoteOutput.FinalCommitSHA)
		}

		currentlyProcessing = ""
		processedCount++

		// Check if we should ContinueAsNew
		if processedCount >= ContinueAsNewThreshold {
			return workflow.NewContinueAsNewError(ctx, MergeQueueWorkflow, types.MergeQueueWorkflowInput{
				ProjectID:             input.ProjectID,
				RepositoryPath:        input.RepositoryPath,
				MainBranch:            input.MainBranch,
				ClaudeConfigPath:      input.ClaudeConfigPath,
				WorkspaceDir:          input.WorkspaceDir,
				OrchestratorTaskQueue: input.OrchestratorTaskQueue,
				PendingItems:          queue,
				ProcessedCount:        0,
			})
		}
	}
}

// computePromoteRunID generates a deterministic promote run ID from inputs.
func computePromoteRunID(sourceRunID, mainBranch string, queuedAt time.Time) string {
	h := sha256.New()
	h.Write([]byte(sourceRunID))
	h.Write([]byte(mainBranch))
	h.Write([]byte(PromoteWorkflowVersion))
	h.Write([]byte(queuedAt.Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

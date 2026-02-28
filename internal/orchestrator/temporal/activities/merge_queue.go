// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workflows"
)

// MergeQueueSignaler abstracts the SignalWithStartWorkflow call so both the
// pipeline service and this activity use the same code path (the temporal.Client wrapper).
type MergeQueueSignaler interface {
	SignalWithStartWorkflow(ctx context.Context, workflowID, signalName string, signalArg interface{}, workflowFunc interface{}, workflowArgs ...interface{}) (client.WorkflowRun, error)
}

// MergeQueueActivities holds dependencies for merge-queue related activities.
type MergeQueueActivities struct {
	signaler MergeQueueSignaler
}

// NewMergeQueueActivities creates a MergeQueueActivities instance.
func NewMergeQueueActivities(signaler MergeQueueSignaler) *MergeQueueActivities {
	return &MergeQueueActivities{
		signaler: signaler,
	}
}

// EnsureMergeQueueAndSignalActivity atomically signals the merge queue workflow
// and starts it if it doesn't exist. This avoids the race where the merge queue
// has timed out between a status check and a subsequent signal.
func (a *MergeQueueActivities) EnsureMergeQueueAndSignalActivity(ctx context.Context, input types.EnsureMergeQueueAndSignalInput) error {
	mergeQueueID := types.MergeQueueWorkflowID(input.ProjectID)

	workflowInput := types.MergeQueueWorkflowInput{
		ProjectID:             input.ProjectID,
		RepositoryPath:        input.RepositoryPath,
		MainBranch:            input.MainBranch,
		ClaudeConfigPath:      input.ClaudeConfigPath,
		WorkspaceDir:          input.WorkspaceDir,
		OrchestratorTaskQueue: input.OrchestratorTaskQueue,
	}

	_, err := a.signaler.SignalWithStartWorkflow(ctx, mergeQueueID, input.SignalName, input.Item, workflows.MergeQueueWorkflowName, workflowInput)
	if err != nil {
		return fmt.Errorf("failed to signal-with-start merge queue: %w", err)
	}

	return nil
}

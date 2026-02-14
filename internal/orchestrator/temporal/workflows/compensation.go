// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

// compensation represents a compensating action to undo a step in the saga pattern.
// Compensations are accumulated as a workflow progresses and executed in reverse order (LIFO)
// on failure to ensure proper cleanup of resources.
type compensation struct {
	name   string
	action func(workflow.Context) error
}

// runCompensations executes all compensations in reverse order (LIFO).
// This ensures that resources are cleaned up in the correct order:
// the last resource created is the first one destroyed.
//
// Compensation failures are logged but do not stop subsequent compensations,
// ensuring best-effort cleanup of all resources.
func runCompensations(ctx workflow.Context, compensations []compensation) {
	logger := workflow.GetLogger(ctx)
	for i := len(compensations) - 1; i >= 0; i-- {
		c := compensations[i]
		if err := c.action(ctx); err != nil {
			logger.Error("Compensation failed", "compensation", c.name, "error", err)
		} else {
			logger.Info("Compensation succeeded", "compensation", c.name)
		}
	}
}

// compensationActivityOptions returns activity options suitable for compensation activities.
// These use minimal retries since compensations are best-effort cleanup.
func compensationActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1, // Don't retry cleanup - best effort only
		},
	}
}

// worktreeCompensation creates a compensation that removes a git worktree.
// The compensation is idempotent - it succeeds even if the worktree is already removed.
func worktreeCompensation(worktreePath, repositoryPath string) compensation {
	return compensation{
		name: "RemoveWorktree",
		action: func(ctx workflow.Context) error {
			cleanupCtx := workflow.WithActivityOptions(ctx, compensationActivityOptions())
			return workflow.ExecuteActivity(cleanupCtx, "RemoveWorktreeActivity", types.RemoveWorktreeActivityInput{
				WorktreePath:   worktreePath,
				RepositoryPath: repositoryPath,
			}).Get(cleanupCtx, nil)
		},
	}
}

// containerCompensation creates a compensation that stops and removes a container.
// The compensation is idempotent - it succeeds even if the container is already stopped/removed.
func containerCompensation(containerID string) compensation {
	return compensation{
		name: "StopContainer",
		action: func(ctx workflow.Context) error {
			cleanupCtx := workflow.WithActivityOptions(ctx, compensationActivityOptions())
			return workflow.ExecuteActivity(cleanupCtx, "StopContainerActivity", containerID).Get(cleanupCtx, nil)
		},
	}
}

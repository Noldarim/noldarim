// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"testing"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func baseMergeQueueInput() types.MergeQueueWorkflowInput {
	return types.MergeQueueWorkflowInput{
		ProjectID:             "project-1",
		RepositoryPath:        "/tmp/repo",
		MainBranch:            "main",
		ClaudeConfigPath:      "/tmp/claude.json",
		WorkspaceDir:          "/workspace",
		OrchestratorTaskQueue: "orchestrator-queue",
	}
}

func TestMergeQueueWorkflow_ProcessesSignaledItem(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(PromoteWorkflow)

	input := baseMergeQueueInput()

	item := types.MergeQueueItem{
		RunID:               "run-abcd",
		SourceBranchName:    "task-run-abcd",
		SourceHeadCommitSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		QueuedAt:            time.Now(),
	}

	// Signal the promote item before execution starts
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(PromoteSignal, item)
	}, 0)

	// Mock the PromoteWorkflow child to succeed
	env.OnWorkflow(PromoteWorkflow, mock.Anything, mock.Anything).Return(&types.PromoteWorkflowOutput{
		Success:        true,
		MergeMethod:    "fast-forward",
		FinalCommitSHA: item.SourceHeadCommitSHA,
	}, nil)

	env.ExecuteWorkflow(MergeQueueWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	// Workflow completes via idle termination (nil) after processing one item
	env.AssertExpectations(t)
}

func TestMergeQueueWorkflow_DeduplicatesSignals(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(PromoteWorkflow)

	input := baseMergeQueueInput()

	item := types.MergeQueueItem{
		RunID:               "run-dup",
		SourceBranchName:    "task-run-dup",
		SourceHeadCommitSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		QueuedAt:            time.Now(),
	}

	// Signal the same RunID twice before the workflow processes anything
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(PromoteSignal, item)
		env.SignalWorkflow(PromoteSignal, item)
	}, 0)

	promoteCallCount := 0
	env.OnWorkflow(PromoteWorkflow, mock.Anything, mock.Anything).Return(
		func(ctx workflow.Context, input types.PromoteWorkflowInput) (*types.PromoteWorkflowOutput, error) {
			promoteCallCount++
			return &types.PromoteWorkflowOutput{
				Success:        true,
				MergeMethod:    "fast-forward",
				FinalCommitSHA: item.SourceHeadCommitSHA,
			}, nil
		})

	env.ExecuteWorkflow(MergeQueueWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	// PromoteWorkflow should have been called only once despite two signals with the same RunID
	assert.Equal(t, 1, promoteCallCount, "duplicate RunID should be deduplicated to a single promote")
	env.AssertExpectations(t)
}

func TestMergeQueueWorkflow_IdleTermination(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := baseMergeQueueInput()

	// No signals — the workflow should idle-timeout and return nil
	env.ExecuteWorkflow(MergeQueueWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func setupSavePipelineRunActivity(context.Context, types.SavePipelineRunActivityInput) error {
	return nil
}

func setupSaveRunStepSnapshotsActivity(context.Context, types.SaveRunStepSnapshotsActivityInput) error {
	return nil
}

func setupUpdatePipelineRunStatusActivity(context.Context, types.UpdatePipelineRunStatusActivityInput) error {
	return nil
}

func TestSetupWorkflow_MarksRunFailedWhenSavingStepSnapshotsFails(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(setupSavePipelineRunActivity, activity.RegisterOptions{Name: "SavePipelineRunActivity"})
	env.RegisterActivityWithOptions(setupSaveRunStepSnapshotsActivity, activity.RegisterOptions{Name: "SaveRunStepSnapshotsActivity"})
	env.RegisterActivityWithOptions(setupUpdatePipelineRunStatusActivity, activity.RegisterOptions{Name: "UpdatePipelineRunStatusActivity"})

	input := types.PipelineSetupInput{
		RunID:                 "run-1",
		PipelineID:            "pipe-1",
		ProjectID:             "proj-1",
		Name:                  "Pipeline",
		Steps:                 []models.StepDefinition{{StepID: "s1", Name: "Step 1"}},
		RepositoryPath:        "/tmp/repo",
		BranchName:            "pipeline/run-1",
		BaseCommitSHA:         "abc123",
		StartCommitSHA:        "abc123",
		ClaudeConfigPath:      "/tmp/claude.json",
		WorkspaceDir:          "/workspace",
		TaskQueue:             "worker-task-queue",
		OrchestratorTaskQueue: "orchestrator-task-queue",
		ParentWorkflowID:      "wf-run-1",
	}

	saveSnapshotsErr := errors.New("write failed")

	env.OnActivity("SavePipelineRunActivity", mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity("SaveRunStepSnapshotsActivity", mock.Anything, mock.Anything).Return(saveSnapshotsErr).Times(3)
	env.OnActivity("UpdatePipelineRunStatusActivity", mock.Anything, mock.MatchedBy(func(in types.UpdatePipelineRunStatusActivityInput) bool {
		return in.RunID == input.RunID &&
			in.Status == models.PipelineRunStatusFailed &&
			in.ErrorMessage != "" &&
			strings.Contains(strings.ToLower(in.ErrorMessage), "save run step snapshots")
	})).Return(nil).Once()

	env.ExecuteWorkflow(SetupWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	workflowErr := env.GetWorkflowError()
	assert.Error(t, workflowErr)
	assert.Contains(t, workflowErr.Error(), "write failed")
	env.AssertExpectations(t)
}

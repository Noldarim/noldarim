// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"context"
	"errors"
	"testing"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

// Stub activities used by PromoteWorkflow tests
func promoteSavePipelineRunActivity(context.Context, types.SavePipelineRunActivityInput) error {
	return nil
}
func promotePublishPipelineCreatedEventActivity(context.Context, types.PublishPipelineEventInput) error {
	return nil
}
func promotePublishPipelineFailedEventActivity(context.Context, types.PublishPipelineEventInput) error {
	return nil
}
func promotePublishPipelineFinishedEventActivity(context.Context, types.PublishPipelineEventInput) error {
	return nil
}
func promoteUpdatePipelineRunStatusActivity(context.Context, types.UpdatePipelineRunStatusActivityInput) error {
	return nil
}
func promoteSaveStepResultActivity(context.Context, types.SaveStepResultActivityInput) error {
	return nil
}
func promoteCheckFastForwardActivity(context.Context, types.CheckFastForwardInput) (*types.CheckFastForwardOutput, error) {
	return nil, nil
}
func promoteFastForwardBranchActivity(context.Context, types.FastForwardBranchInput) error {
	return nil
}
func promoteGetBranchHeadActivity(context.Context, types.GetBranchHeadInput) (*types.GetBranchHeadOutput, error) {
	return nil, nil
}
func promoteCreateContainerActivity(context.Context, types.CreateContainerActivityInput) (*types.CreateContainerActivityOutput, error) {
	return nil, nil
}
func promoteCopyClaudeConfigActivity(context.Context, types.CopyClaudeConfigActivityInput) (*types.CopyClaudeConfigActivityOutput, error) {
	return nil, nil
}
func promoteCopyClaudeCredentialsActivity(context.Context, types.CopyClaudeCredentialsActivityInput) (*types.CopyClaudeCredentialsActivityOutput, error) {
	return nil, nil
}
func promoteStopContainerActivity(context.Context, string) error {
	return nil
}

func basePromoteInput() types.PromoteWorkflowInput {
	return types.PromoteWorkflowInput{
		PromoteRunID:          "promote-run-1234",
		SourceRunID:           "source-run-abcd",
		ProjectID:             "project-1",
		RepositoryPath:        "/tmp/repo",
		MainBranch:            "main",
		SourceBranchName:      "task-source-run",
		SourceHeadCommitSHA:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ClaudeConfigPath:      "/tmp/claude.json",
		WorkspaceDir:          "/workspace",
		OrchestratorTaskQueue: "orchestrator-queue",
	}
}

func registerPromoteActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(promoteSavePipelineRunActivity, activity.RegisterOptions{Name: "SavePipelineRunActivity"})
	env.RegisterActivityWithOptions(promotePublishPipelineCreatedEventActivity, activity.RegisterOptions{Name: "PublishPipelineCreatedEventActivity"})
	env.RegisterActivityWithOptions(promotePublishPipelineFailedEventActivity, activity.RegisterOptions{Name: "PublishPipelineFailedEventActivity"})
	env.RegisterActivityWithOptions(promotePublishPipelineFinishedEventActivity, activity.RegisterOptions{Name: "PublishPipelineFinishedEventActivity"})
	env.RegisterActivityWithOptions(promoteUpdatePipelineRunStatusActivity, activity.RegisterOptions{Name: "UpdatePipelineRunStatusActivity"})
	env.RegisterActivityWithOptions(promoteSaveStepResultActivity, activity.RegisterOptions{Name: "SaveStepResultActivity"})
	env.RegisterActivityWithOptions(promoteCheckFastForwardActivity, activity.RegisterOptions{Name: "CheckFastForwardActivity"})
	env.RegisterActivityWithOptions(promoteFastForwardBranchActivity, activity.RegisterOptions{Name: "FastForwardBranchActivity"})
	env.RegisterActivityWithOptions(promoteGetBranchHeadActivity, activity.RegisterOptions{Name: "GetBranchHeadActivity"})
}

func registerPromoteAIActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(promoteCreateContainerActivity, activity.RegisterOptions{Name: "CreateContainerActivity"})
	env.RegisterActivityWithOptions(promoteCopyClaudeConfigActivity, activity.RegisterOptions{Name: "CopyClaudeConfigActivity"})
	env.RegisterActivityWithOptions(promoteCopyClaudeCredentialsActivity, activity.RegisterOptions{Name: "CopyClaudeCredentialsActivity"})
	env.RegisterActivityWithOptions(promoteStopContainerActivity, activity.RegisterOptions{Name: "StopContainerActivity"})
}

func TestPromoteWorkflow_FastForwardPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerPromoteActivities(env)

	input := basePromoteInput()
	mainHeadSHA := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	// Phase 1: save run + publish created
	env.OnActivity("SavePipelineRunActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineCreatedEventActivity", mock.Anything, mock.Anything).Return(nil)

	// Phase 2: check FF → yes
	env.OnActivity("CheckFastForwardActivity", mock.Anything, mock.MatchedBy(func(in types.CheckFastForwardInput) bool {
		return in.MainBranch == "main" && in.TaskBranch == input.SourceBranchName
	})).Return(&types.CheckFastForwardOutput{
		IsFF:        true,
		MainHeadSHA: mainHeadSHA,
		TaskHeadSHA: input.SourceHeadCommitSHA,
	}, nil)

	// Phase 3a: fast-forward with ExpectedOldSHA
	env.OnActivity("FastForwardBranchActivity", mock.Anything, mock.MatchedBy(func(in types.FastForwardBranchInput) bool {
		return in.Branch == "main" &&
			in.TargetSHA == input.SourceHeadCommitSHA &&
			in.ExpectedOldSHA == mainHeadSHA
	})).Return(nil)

	// Step result + finalize
	env.OnActivity("SaveStepResultActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineFinishedEventActivity", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(PromoteWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var output types.PromoteWorkflowOutput
	assert.NoError(t, env.GetWorkflowResult(&output))
	assert.True(t, output.Success)
	assert.Equal(t, "fast-forward", output.MergeMethod)
	assert.Equal(t, input.SourceHeadCommitSHA, output.FinalCommitSHA)

	env.AssertExpectations(t)
}

func TestPromoteWorkflow_CleanMergePath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerPromoteActivities(env)

	// Also register worktree and merge activities for non-FF path
	env.RegisterActivityWithOptions(func(context.Context, types.CreateWorktreeActivityInput) (*types.CreateWorktreeActivityOutput, error) {
		return nil, nil
	}, activity.RegisterOptions{Name: "CreateWorktreeActivity"})
	env.RegisterActivityWithOptions(func(context.Context, types.MergeInWorktreeInput) (*types.MergeInWorktreeOutput, error) {
		return nil, nil
	}, activity.RegisterOptions{Name: "MergeInWorktreeActivity"})
	env.RegisterActivityWithOptions(func(context.Context, types.RemoveWorktreeActivityInput) error {
		return nil
	}, activity.RegisterOptions{Name: "RemoveWorktreeActivity"})

	input := basePromoteInput()
	mainHeadSHA := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	mergeCommitSHA := "cccccccccccccccccccccccccccccccccccccccc"

	// Phase 1: save + publish
	env.OnActivity("SavePipelineRunActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineCreatedEventActivity", mock.Anything, mock.Anything).Return(nil)

	// Phase 2: not fast-forwardable
	env.OnActivity("CheckFastForwardActivity", mock.Anything, mock.Anything).Return(&types.CheckFastForwardOutput{
		IsFF:        false,
		MainHeadSHA: mainHeadSHA,
		TaskHeadSHA: input.SourceHeadCommitSHA,
	}, nil)

	// Phase 3b: create worktree, merge clean
	env.OnActivity("CreateWorktreeActivity", mock.Anything, mock.Anything).Return(&types.CreateWorktreeActivityOutput{
		WorktreePath: "/tmp/repo/.worktrees/promote",
		BranchName:   "promote-promote-r",
	}, nil)
	env.OnActivity("MergeInWorktreeActivity", mock.Anything, mock.Anything).Return(&types.MergeInWorktreeOutput{
		CommitSHA:    mergeCommitSHA,
		HasConflicts: false,
	}, nil)

	// Re-check main HEAD (unchanged)
	env.OnActivity("GetBranchHeadActivity", mock.Anything, mock.MatchedBy(func(in types.GetBranchHeadInput) bool {
		return in.Branch == "main"
	})).Return(&types.GetBranchHeadOutput{SHA: mainHeadSHA}, nil)

	// FF branch after clean merge with ExpectedOldSHA
	env.OnActivity("FastForwardBranchActivity", mock.Anything, mock.MatchedBy(func(in types.FastForwardBranchInput) bool {
		return in.Branch == "main" &&
			in.TargetSHA == mergeCommitSHA &&
			in.ExpectedOldSHA == mainHeadSHA
	})).Return(nil)

	// Step results + cleanup + finalize
	env.OnActivity("SaveStepResultActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("RemoveWorktreeActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineFinishedEventActivity", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(PromoteWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var output types.PromoteWorkflowOutput
	assert.NoError(t, env.GetWorkflowResult(&output))
	assert.True(t, output.Success)
	assert.Equal(t, "clean-merge", output.MergeMethod)
	assert.Equal(t, mergeCommitSHA, output.FinalCommitSHA)

	env.AssertExpectations(t)
}

func TestPromoteWorkflow_FailedFF(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerPromoteActivities(env)

	input := basePromoteInput()

	// Phase 1: save + publish
	env.OnActivity("SavePipelineRunActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineCreatedEventActivity", mock.Anything, mock.Anything).Return(nil)

	// Phase 2: FF check fails
	env.OnActivity("CheckFastForwardActivity", mock.Anything, mock.Anything).Return(
		nil, errors.New("git error: repository not found"))

	// Failure path: mark failed + publish failed event
	env.OnActivity("UpdatePipelineRunStatusActivity", mock.Anything, mock.MatchedBy(func(in types.UpdatePipelineRunStatusActivityInput) bool {
		return in.RunID == input.PromoteRunID && in.Status == models.PipelineRunStatusFailed
	})).Return(nil)
	env.OnActivity("PublishPipelineFailedEventActivity", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(PromoteWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "Failed to check fast-forward")

	env.AssertExpectations(t)
}

func TestPromoteWorkflow_ConflictAIResolutionPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerPromoteActivities(env)
	registerPromoteAIActivities(env)

	// Register worktree/merge activities for non-FF path
	env.RegisterActivityWithOptions(func(context.Context, types.CreateWorktreeActivityInput) (*types.CreateWorktreeActivityOutput, error) {
		return nil, nil
	}, activity.RegisterOptions{Name: "CreateWorktreeActivity"})
	env.RegisterActivityWithOptions(func(context.Context, types.MergeInWorktreeInput) (*types.MergeInWorktreeOutput, error) {
		return nil, nil
	}, activity.RegisterOptions{Name: "MergeInWorktreeActivity"})
	env.RegisterActivityWithOptions(func(context.Context, types.RemoveWorktreeActivityInput) error {
		return nil
	}, activity.RegisterOptions{Name: "RemoveWorktreeActivity"})

	env.RegisterWorkflow(ProcessingStepWorkflow)

	input := basePromoteInput()
	mainHeadSHA := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	resolvedSHA := "dddddddddddddddddddddddddddddddddddddddd"

	// Phase 1: save + publish
	env.OnActivity("SavePipelineRunActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineCreatedEventActivity", mock.Anything, mock.Anything).Return(nil)

	// Phase 2: not fast-forwardable
	env.OnActivity("CheckFastForwardActivity", mock.Anything, mock.Anything).Return(&types.CheckFastForwardOutput{
		IsFF:        false,
		MainHeadSHA: mainHeadSHA,
		TaskHeadSHA: input.SourceHeadCommitSHA,
	}, nil)

	// Phase 3b: create worktree, merge with conflicts
	env.OnActivity("CreateWorktreeActivity", mock.Anything, mock.Anything).Return(&types.CreateWorktreeActivityOutput{
		WorktreePath: "/tmp/repo/.worktrees/promote",
		BranchName:   "promote-promote-r",
	}, nil)
	env.OnActivity("MergeInWorktreeActivity", mock.Anything, mock.Anything).Return(&types.MergeInWorktreeOutput{
		HasConflicts: true,
	}, nil)

	// Conflict step result saved as failed
	env.OnActivity("SaveStepResultActivity", mock.Anything, mock.Anything).Return(nil)

	// Container setup for AI resolution
	env.OnActivity("CreateContainerActivity", mock.Anything, mock.Anything).Return(&types.CreateContainerActivityOutput{
		ContainerID: "container-123",
	}, nil)
	env.OnActivity("CopyClaudeConfigActivity", mock.Anything, mock.Anything).Return(&types.CopyClaudeConfigActivityOutput{
		Success: true,
	}, nil)
	env.OnActivity("CopyClaudeCredentialsActivity", mock.Anything, mock.Anything).Return(&types.CopyClaudeCredentialsActivityOutput{
		Success: true,
	}, nil)

	// AI resolution succeeds
	env.OnWorkflow(ProcessingStepWorkflow, mock.Anything, mock.Anything).Return(&types.ProcessingStepOutput{
		Success:   true,
		CommitSHA: resolvedSHA,
	}, nil)

	// Re-check main HEAD (unchanged after AI resolution)
	env.OnActivity("GetBranchHeadActivity", mock.Anything, mock.MatchedBy(func(in types.GetBranchHeadInput) bool {
		return in.Branch == "main" && in.RepoPath == input.RepositoryPath
	})).Return(&types.GetBranchHeadOutput{SHA: mainHeadSHA}, nil)

	// FF branch after AI resolution
	env.OnActivity("FastForwardBranchActivity", mock.Anything, mock.MatchedBy(func(in types.FastForwardBranchInput) bool {
		return in.Branch == "main" &&
			in.TargetSHA == resolvedSHA &&
			in.ExpectedOldSHA == mainHeadSHA
	})).Return(nil)

	// Cleanup + finalize
	env.OnActivity("RemoveWorktreeActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("StopContainerActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineFinishedEventActivity", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(PromoteWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var output types.PromoteWorkflowOutput
	assert.NoError(t, env.GetWorkflowResult(&output))
	assert.True(t, output.Success)
	assert.Equal(t, "ai-resolved", output.MergeMethod)
	assert.Equal(t, resolvedSHA, output.FinalCommitSHA)

	env.AssertExpectations(t)
}

func TestPromoteWorkflow_MainMovedDuringMerge(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerPromoteActivities(env)

	// Register worktree and merge activities for non-FF path
	env.RegisterActivityWithOptions(func(context.Context, types.CreateWorktreeActivityInput) (*types.CreateWorktreeActivityOutput, error) {
		return nil, nil
	}, activity.RegisterOptions{Name: "CreateWorktreeActivity"})
	env.RegisterActivityWithOptions(func(context.Context, types.MergeInWorktreeInput) (*types.MergeInWorktreeOutput, error) {
		return nil, nil
	}, activity.RegisterOptions{Name: "MergeInWorktreeActivity"})
	env.RegisterActivityWithOptions(func(context.Context, types.RemoveWorktreeActivityInput) error {
		return nil
	}, activity.RegisterOptions{Name: "RemoveWorktreeActivity"})

	input := basePromoteInput()
	mainHeadSHA := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	movedMainSHA := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	mergeCommitSHA := "cccccccccccccccccccccccccccccccccccccccc"

	// Phase 1: save + publish
	env.OnActivity("SavePipelineRunActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishPipelineCreatedEventActivity", mock.Anything, mock.Anything).Return(nil)

	// Phase 2: not fast-forwardable
	env.OnActivity("CheckFastForwardActivity", mock.Anything, mock.Anything).Return(&types.CheckFastForwardOutput{
		IsFF:        false,
		MainHeadSHA: mainHeadSHA,
		TaskHeadSHA: input.SourceHeadCommitSHA,
	}, nil)

	// Phase 3b: create worktree, clean merge
	env.OnActivity("CreateWorktreeActivity", mock.Anything, mock.Anything).Return(&types.CreateWorktreeActivityOutput{
		WorktreePath: "/tmp/repo/.worktrees/promote",
		BranchName:   "promote-promote-r",
	}, nil)
	env.OnActivity("MergeInWorktreeActivity", mock.Anything, mock.Anything).Return(&types.MergeInWorktreeOutput{
		CommitSHA:    mergeCommitSHA,
		HasConflicts: false,
	}, nil)

	// Re-check main HEAD returns a DIFFERENT SHA (main moved externally)
	env.OnActivity("GetBranchHeadActivity", mock.Anything, mock.MatchedBy(func(in types.GetBranchHeadInput) bool {
		return in.Branch == "main"
	})).Return(&types.GetBranchHeadOutput{SHA: movedMainSHA}, nil)

	// Failure path: mark failed + publish failed + cleanup
	env.OnActivity("UpdatePipelineRunStatusActivity", mock.Anything, mock.MatchedBy(func(in types.UpdatePipelineRunStatusActivityInput) bool {
		return in.RunID == input.PromoteRunID && in.Status == models.PipelineRunStatusFailed
	})).Return(nil)
	env.OnActivity("PublishPipelineFailedEventActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("RemoveWorktreeActivity", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(PromoteWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "main branch modified externally")

	env.AssertExpectations(t)
}

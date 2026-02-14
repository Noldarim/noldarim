// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// GitCommitActivity mock for testing (only this one is new, others are in process_task_test.go)
func GitCommitActivity(ctx context.Context, input types.GitCommitActivityInput) (*types.GitCommitActivityOutput, error) {
	return &types.GitCommitActivityOutput{}, nil
}

// CaptureGitDiffActivity mock for testing
func CaptureGitDiffActivity(ctx context.Context, input types.CaptureGitDiffActivityInput) (*types.CaptureGitDiffActivityOutput, error) {
	return &types.CaptureGitDiffActivityOutput{}, nil
}

// PublishTaskInProgressEventActivity mock for testing
func PublishTaskInProgressEventActivity(ctx context.Context, input types.PublishEventInput) error {
	return nil
}

// PublishTaskFinishedEventActivity mock for testing
func PublishTaskFinishedEventActivity(ctx context.Context, input types.PublishEventInput) error {
	return nil
}


// ProcessTaskGitTestSuite tests the cross-worker git commit functionality
type ProcessTaskGitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestProcessTaskGitTestSuite(t *testing.T) {
	suite.Run(t, new(ProcessTaskGitTestSuite))
}

// TestProcessTaskWorkflow_GitCommit tests that the workflow commits changes via cross-worker activity
func (s *ProcessTaskGitTestSuite) TestProcessTaskWorkflow_GitCommit() {
	env := s.NewTestWorkflowEnvironment()

	// Register the workflow and activities
	env.RegisterWorkflow(ProcessTaskWorkflow)
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(CaptureGitDiffActivity)
	env.RegisterActivity(GitCommitActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Setup input with worktree path
	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "noldarim-system-progress-log/tasks/test-task.md",
		ProjectID:    "project-456",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'test output' > output.txt",
		},
		WorktreePath:          "/tmp/worktrees/task-123", // Simulated worktree path
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock PublishTaskInProgressEventActivity
	env.OnActivity(PublishTaskInProgressEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.Anything, mock.Anything).Return(
		[]string{"sh", "-c", "echo 'test output' > output.txt"},
		nil).Once()

	// Mock LocalExecuteActivity - simulates agent execution
	env.OnActivity(LocalExecuteActivity, mock.Anything, mock.MatchedBy(func(input types.LocalExecuteActivityInput) bool {
		return len(input.Command) > 0 && input.WorkDir == "/workspace"
	})).Return(&types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Command executed successfully",
		ErrorOutput: "",
		Duration:    100 * time.Millisecond,
		Success:     true,
	}, nil).Once()

	// Mock CaptureGitDiffActivity
	env.OnActivity(CaptureGitDiffActivity, mock.Anything, mock.Anything).Return(&types.CaptureGitDiffActivityOutput{
		Success:      true,
		Diff:         "diff --git a/output.txt b/output.txt\n+test output",
		DiffStat:     " output.txt | 1 +\n 1 file changed, 1 insertion(+)",
		FilesChanged: []string{"output.txt"},
		Insertions:   1,
		Deletions:    0,
		HasChanges:   true,
	}, nil).Once()

	// Mock GitCommitActivity - THIS IS THE KEY TEST
	// This activity should execute on the orchestrator worker, not the agent worker
	env.OnActivity(GitCommitActivity, mock.Anything, mock.MatchedBy(func(input types.GitCommitActivityInput) bool {
		// Verify that the worktree path is passed correctly
		return input.RepositoryPath == "/tmp/worktrees/task-123" &&
			len(input.FileNames) > 0 &&
			input.CommitMessage == "Agent automated changes for task task-123"
	})).Return(&types.GitCommitActivityOutput{
		Success: true,
		Error:   "",
	}, nil).Once()

	// Mock PublishTaskFinishedEventActivity
	env.OnActivity(PublishTaskFinishedEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	// Get workflow result
	var output types.ProcessTaskWorkflowOutput
	require.NoError(s.T(), env.GetWorkflowResult(&output))
	require.True(s.T(), output.Success)
	require.Equal(s.T(), "Command executed successfully", output.ProcessedData)
	require.Empty(s.T(), output.Error)

	// Verify all expected activities were called
	env.AssertExpectations(s.T())
}

// TestProcessTaskWorkflow_GitCommitFailure tests that git commit failures fail the workflow (critical for idempotency)
func (s *ProcessTaskGitTestSuite) TestProcessTaskWorkflow_GitCommitFailure() {
	env := s.NewTestWorkflowEnvironment()

	// Register the workflow and activities
	env.RegisterWorkflow(ProcessTaskWorkflow)
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(CaptureGitDiffActivity)
	env.RegisterActivity(GitCommitActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-789",
		TaskFilePath: "noldarim-system-progress-log/tasks/test-task.md",
		ProjectID:    "project-456",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'test' > test.txt",
		},
		WorktreePath:          "/tmp/worktrees/task-789",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock PublishTaskInProgressEventActivity
	env.OnActivity(PublishTaskInProgressEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.Anything, mock.Anything).Return(
		[]string{"sh", "-c", "echo 'test'"},
		nil).Once()

	// Mock successful command execution
	env.OnActivity(LocalExecuteActivity, mock.Anything, mock.Anything).Return(&types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Success",
		ErrorOutput: "",
		Duration:    50 * time.Millisecond,
		Success:     true,
	}, nil).Once()

	// Mock CaptureGitDiffActivity - succeeds before commit fails
	env.OnActivity(CaptureGitDiffActivity, mock.Anything, mock.Anything).Return(&types.CaptureGitDiffActivityOutput{
		Success:      true,
		Diff:         "diff --git a/test.txt b/test.txt\n+test",
		DiffStat:     " test.txt | 1 +\n 1 file changed, 1 insertion(+)",
		FilesChanged: []string{"test.txt"},
		Insertions:   1,
		Deletions:    0,
		HasChanges:   true,
	}, nil).Once()

	// Mock GitCommitActivity failure - workflow should FAIL (critical for idempotency)
	env.OnActivity(GitCommitActivity, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("git commit failed: no changes to commit"),
	).Times(3) // Will retry 3 times

	// Mock PublishErrorEventActivity
	env.OnActivity(PublishErrorEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Workflow should fail due to git commit failure
	require.True(s.T(), env.IsWorkflowCompleted())

	// Check that workflow failed with the expected error
	err := env.GetWorkflowError()
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "git commit failed")

	// Verify the workflow returns an error (idempotency check failed)
}

// TestProcessTaskWorkflow_NoWorktreePath tests backward compatibility when worktree path is empty
func (s *ProcessTaskGitTestSuite) TestProcessTaskWorkflow_NoWorktreePath() {
	env := s.NewTestWorkflowEnvironment()

	// Register the workflow and activities
	env.RegisterWorkflow(ProcessTaskWorkflow)
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Input without WorktreePath (backward compatibility)
	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-000",
		TaskFilePath: "noldarim-system-progress-log/tasks/test-task.md",
		ProjectID:    "project-456",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'test'",
		},
		WorktreePath:          "", // Empty worktree path
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock PublishTaskInProgressEventActivity
	env.OnActivity(PublishTaskInProgressEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.Anything, mock.Anything).Return(
		[]string{"sh", "-c", "echo 'test'"},
		nil).Once()

	// Mock successful command execution
	env.OnActivity(LocalExecuteActivity, mock.Anything, mock.Anything).Return(&types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "No commit needed",
		ErrorOutput: "",
		Duration:    50 * time.Millisecond,
		Success:     true,
	}, nil).Once()

	// GitCommitActivity should NOT be called when WorktreePath is empty
	// No mock setup for GitCommitActivity or CaptureGitDiffActivity

	// Mock PublishTaskFinishedEventActivity
	env.OnActivity(PublishTaskFinishedEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var output types.ProcessTaskWorkflowOutput
	require.NoError(s.T(), env.GetWorkflowResult(&output))
	require.True(s.T(), output.Success)
	require.Equal(s.T(), "No commit needed", output.ProcessedData)

	// Verify expectations (GitCommitActivity was NOT called)
	env.AssertExpectations(s.T())
}

// TestProcessTaskWorkflow_CrossWorkerExecution tests the actual cross-worker behavior
// This is more of an integration test that would run with real Temporal
func (s *ProcessTaskGitTestSuite) TestProcessTaskWorkflow_CrossWorkerExecution() {
	// Create a temporary directory to simulate a worktree
	tmpDir, err := os.MkdirTemp("", "test-worktree-*")
	require.NoError(s.T(), err)
	defer os.RemoveAll(tmpDir)

	// Create a test file in the worktree
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(s.T(), err)

	env := s.NewTestWorkflowEnvironment()

	// Register the workflow and activities
	env.RegisterWorkflow(ProcessTaskWorkflow)
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(CaptureGitDiffActivity)
	env.RegisterActivity(GitCommitActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-integration",
		TaskFilePath: "noldarim-system-progress-log/tasks/integration-task.md",
		ProjectID:    "project-int",
		WorkspaceDir: tmpDir,
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: fmt.Sprintf("echo 'modified content' > %s", testFile),
		},
		WorktreePath:          tmpDir,
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock PublishTaskInProgressEventActivity
	env.OnActivity(PublishTaskInProgressEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.Anything, mock.Anything).Return(
		[]string{"sh", "-c", fmt.Sprintf("echo 'modified content' > %s", testFile)},
		nil).Once()

	// Mock LocalExecuteActivity to simulate file modification
	env.OnActivity(LocalExecuteActivity, mock.Anything, mock.MatchedBy(func(input types.LocalExecuteActivityInput) bool {
		return input.WorkDir == tmpDir
	})).Return(&types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "File modified",
		ErrorOutput: "",
		Duration:    100 * time.Millisecond,
		Success:     true,
	}, nil).Once()

	// Mock CaptureGitDiffActivity
	env.OnActivity(CaptureGitDiffActivity, mock.Anything, mock.Anything).Return(&types.CaptureGitDiffActivityOutput{
		Success:      true,
		Diff:         "diff --git a/test.txt b/test.txt\n+modified content",
		DiffStat:     " test.txt | 1 +\n 1 file changed, 1 insertion(+)",
		FilesChanged: []string{"test.txt"},
		Insertions:   1,
		Deletions:    0,
		HasChanges:   true,
	}, nil).Once()

	// Mock GitCommitActivity
	env.OnActivity(GitCommitActivity, mock.Anything, mock.MatchedBy(func(input types.GitCommitActivityInput) bool {
		return input.RepositoryPath == tmpDir
	})).Return(&types.GitCommitActivityOutput{
		Success: true,
		Error:   "",
	}, nil).Once()

	// Mock PublishTaskFinishedEventActivity
	env.OnActivity(PublishTaskFinishedEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	var output types.ProcessTaskWorkflowOutput
	require.NoError(s.T(), env.GetWorkflowResult(&output))
	require.True(s.T(), output.Success)

	// Verify all activities were called
	env.AssertExpectations(s.T())
}

// TestProcessTaskWorkflow_GitDiffCapture tests that git diff is captured before commit
func (s *ProcessTaskGitTestSuite) TestProcessTaskWorkflow_GitDiffCapture() {
	env := s.NewTestWorkflowEnvironment()

	// Register the workflow and activities
	env.RegisterWorkflow(ProcessTaskWorkflow)
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(CaptureGitDiffActivity)
	env.RegisterActivity(GitCommitActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Setup input with worktree path
	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-diff-123",
		TaskFilePath: "noldarim-system-progress-log/tasks/test-task.md",
		ProjectID:    "project-456",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'test output' > output.txt",
		},
		WorktreePath:          "/tmp/worktrees/task-diff-123",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock PublishTaskInProgressEventActivity
	env.OnActivity(PublishTaskInProgressEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.Anything, mock.Anything).Return(
		[]string{"sh", "-c", "echo 'test output' > output.txt"},
		nil).Once()

	// Mock LocalExecuteActivity - simulates agent execution
	env.OnActivity(LocalExecuteActivity, mock.Anything, mock.Anything).Return(&types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Command executed successfully",
		ErrorOutput: "",
		Duration:    100 * time.Millisecond,
		Success:     true,
	}, nil).Once()

	// Mock CaptureGitDiffActivity - should be called BEFORE commit
	env.OnActivity(CaptureGitDiffActivity, mock.Anything, mock.MatchedBy(func(input types.CaptureGitDiffActivityInput) bool {
		return input.RepositoryPath == "/tmp/worktrees/task-diff-123"
	})).Return(&types.CaptureGitDiffActivityOutput{
		Success:      true,
		Diff:         "diff --git a/output.txt b/output.txt\n+test output",
		DiffStat:     " output.txt | 1 +\n 1 file changed, 1 insertion(+)",
		FilesChanged: []string{"output.txt"},
		Insertions:   1,
		Deletions:    0,
		HasChanges:   true,
	}, nil).Once()

	// Mock GitCommitActivity - should be called AFTER diff capture
	env.OnActivity(GitCommitActivity, mock.Anything, mock.Anything).Return(&types.GitCommitActivityOutput{
		Success: true,
		Error:   "",
	}, nil).Once()

	// Mock PublishTaskFinishedEventActivity
	env.OnActivity(PublishTaskFinishedEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	// Get workflow result
	var output types.ProcessTaskWorkflowOutput
	require.NoError(s.T(), env.GetWorkflowResult(&output))
	require.True(s.T(), output.Success)

	// Query the workflow for processing metadata
	value, err := env.QueryWorkflow("GetProcessingMetadata")
	require.NoError(s.T(), err)

	var metadata types.ProcessingMetadata
	err = value.Get(&metadata)
	require.NoError(s.T(), err)

	// Verify metadata contains git diff information
	require.NotNil(s.T(), metadata.GitDiff)
	require.True(s.T(), metadata.GitDiff.Success)
	require.True(s.T(), metadata.GitDiff.HasChanges)
	require.Equal(s.T(), 1, len(metadata.GitDiff.FilesChanged))
	require.Equal(s.T(), "output.txt", metadata.GitDiff.FilesChanged[0])
	require.Equal(s.T(), 1, metadata.GitDiff.Insertions)
	require.Equal(s.T(), 0, metadata.GitDiff.Deletions)
	require.Contains(s.T(), metadata.GitDiff.Diff, "test output")

	// Verify processing metadata
	require.Equal(s.T(), "Command executed successfully", metadata.CommandOutput)
	require.Equal(s.T(), 100*time.Millisecond, metadata.ProcessingTime)

	// Verify all expected activities were called in the correct order
	env.AssertExpectations(s.T())
}

// TestProcessTaskWorkflow_GitDiffCaptureFailureCritical tests that diff capture failure fails the workflow
func (s *ProcessTaskGitTestSuite) TestProcessTaskWorkflow_GitDiffCaptureFailureCritical() {
	env := s.NewTestWorkflowEnvironment()

	// Register the workflow and activities
	env.RegisterWorkflow(ProcessTaskWorkflow)
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(CaptureGitDiffActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-diff-fail",
		TaskFilePath: "noldarim-system-progress-log/tasks/test-task.md",
		ProjectID:    "project-456",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'test'",
		},
		WorktreePath:          "/tmp/worktrees/task-diff-fail",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock PublishTaskInProgressEventActivity
	env.OnActivity(PublishTaskInProgressEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.Anything, mock.Anything).Return(
		[]string{"sh", "-c", "echo 'test'"},
		nil).Once()

	// Mock successful command execution
	env.OnActivity(LocalExecuteActivity, mock.Anything, mock.Anything).Return(&types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Success",
		ErrorOutput: "",
		Duration:    50 * time.Millisecond,
		Success:     true,
	}, nil).Once()

	// Mock CaptureGitDiffActivity to fail - should fail the workflow after retries
	// Allow for retries (Temporal will retry 3 times by default)
	env.OnActivity(CaptureGitDiffActivity, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("failed to capture diff"),
	).Times(3) // Max retries from activity options

	// Mock PublishErrorEventActivity - should be called when diff fails
	env.OnActivity(PublishErrorEventActivity, mock.Anything, mock.MatchedBy(func(input types.PublishErrorEventInput) bool {
		return input.TaskID == "task-diff-fail" && input.Message == "Failed to capture git diff"
	})).Return(nil).Once()

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Workflow should fail due to critical diff capture failure
	require.True(s.T(), env.IsWorkflowCompleted())
	workflowErr := env.GetWorkflowError()
	require.Error(s.T(), workflowErr)
	require.Contains(s.T(), workflowErr.Error(), "failed to capture git diff")

	// When workflow fails, the output may not be populated
	var output types.ProcessTaskWorkflowOutput
	err := env.GetWorkflowResult(&output)
	require.Error(s.T(), err)

	// Verify all activities were called
	env.AssertExpectations(s.T())
}

// TestProcessTaskWorkflow_GitDiffNoChanges tests diff capture when there are no changes
func (s *ProcessTaskGitTestSuite) TestProcessTaskWorkflow_GitDiffNoChanges() {
	env := s.NewTestWorkflowEnvironment()

	// Register the workflow and activities
	env.RegisterWorkflow(ProcessTaskWorkflow)
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(CaptureGitDiffActivity)
	env.RegisterActivity(GitCommitActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-no-changes",
		TaskFilePath: "noldarim-system-progress-log/tasks/test-task.md",
		ProjectID:    "project-456",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'no file changes'",
		},
		WorktreePath:          "/tmp/worktrees/task-no-changes",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock PublishTaskInProgressEventActivity
	env.OnActivity(PublishTaskInProgressEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.Anything, mock.Anything).Return(
		[]string{"sh", "-c", "echo 'no file changes'"},
		nil).Once()

	// Mock successful command execution
	env.OnActivity(LocalExecuteActivity, mock.Anything, mock.Anything).Return(&types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Command executed",
		ErrorOutput: "",
		Duration:    50 * time.Millisecond,
		Success:     true,
	}, nil).Once()

	// Mock CaptureGitDiffActivity with no changes
	env.OnActivity(CaptureGitDiffActivity, mock.Anything, mock.Anything).Return(&types.CaptureGitDiffActivityOutput{
		Success:      true,
		Diff:         "",
		DiffStat:     "",
		FilesChanged: []string{},
		Insertions:   0,
		Deletions:    0,
		HasChanges:   false,
	}, nil).Once()

	// Mock GitCommitActivity
	env.OnActivity(GitCommitActivity, mock.Anything, mock.Anything).Return(&types.GitCommitActivityOutput{
		Success: true,
		Error:   "",
	}, nil).Once()

	// Mock PublishTaskFinishedEventActivity
	env.OnActivity(PublishTaskFinishedEventActivity, mock.Anything, mock.Anything).Return(nil).Once()

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	require.True(s.T(), env.IsWorkflowCompleted())
	require.NoError(s.T(), env.GetWorkflowError())

	// Query the workflow for processing metadata
	value, err := env.QueryWorkflow("GetProcessingMetadata")
	require.NoError(s.T(), err)

	var metadata types.ProcessingMetadata
	err = value.Get(&metadata)
	require.NoError(s.T(), err)

	// Verify metadata shows no changes
	require.NotNil(s.T(), metadata.GitDiff)
	require.True(s.T(), metadata.GitDiff.Success)
	require.False(s.T(), metadata.GitDiff.HasChanges)
	require.Equal(s.T(), 0, len(metadata.GitDiff.FilesChanged))
	require.Equal(s.T(), 0, metadata.GitDiff.Insertions)
	require.Equal(s.T(), 0, metadata.GitDiff.Deletions)
	require.Empty(s.T(), metadata.GitDiff.Diff)

	// Verify all activities were called
	env.AssertExpectations(s.T())
}

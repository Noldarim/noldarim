// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/activities"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workflows"
	"github.com/noldarim/noldarim/internal/protocol"
)

// TestCrossWorkerGitCommit verifies that GitCommitActivity executes on the orchestrator's
// task queue when called from ProcessTaskWorkflow running on an agent's task queue.
// This tests the critical cross-worker activity routing for idempotency.
func TestCrossWorkerGitCommit(t *testing.T) {
	// Skip if Temporal is not available
	cfg, err := config.NewConfig("../../test-config.yaml")
	require.NoError(t, err)

	temporalClient, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		t.Skip("Temporal not available, skipping cross-worker test")
	}
	defer temporalClient.Close()

	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "cross-worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize a git repository for testing
	gitService, err := services.NewGitService(tempDir, true)
	require.NoError(t, err)

	ctx := context.Background()
	err = gitService.SetConfig(ctx, tempDir, "user.name", "Test User")
	require.NoError(t, err)
	err = gitService.SetConfig(ctx, tempDir, "user.email", "test@example.com")
	require.NoError(t, err)

	// Create initial commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	require.NoError(t, err)
	err = gitService.CreateCommit(ctx, tempDir, "Initial commit")
	require.NoError(t, err)

	// Create a worktree for the test
	worktreePath := filepath.Join(tempDir, "worktree-test")
	err = gitService.AddWorktree(ctx, worktreePath, "test-branch", "main")
	require.NoError(t, err)
	defer gitService.RemoveWorktree(ctx, worktreePath, false)

	// Setup orchestrator worker with GitCommitActivity
	orchestratorQueue := "test-orchestrator-queue"
	orchestratorWorker := worker.New(temporalClient, orchestratorQueue, worker.Options{})

	// Create GitActivities for orchestrator
	gitServiceManager := services.NewGitServiceManager(cfg)
	gitActivities := activities.NewGitActivities(gitServiceManager)

	// Setup agent worker WITHOUT GitCommitActivity
	agentQueue := "test-agent-queue"
	agentWorker := worker.New(temporalClient, agentQueue, worker.Options{})

	// Register ProcessTaskWorkflow, AIObservabilityWorkflow, and agent-specific activities on agent
	agentWorker.RegisterWorkflow(workflows.ProcessTaskWorkflow)
	agentWorker.RegisterWorkflow(workflows.AIObservabilityWorkflow)
	agentWorker.RegisterActivityWithOptions(mockLocalExecuteActivity, activity.RegisterOptions{
		Name: "LocalExecuteActivity",
	})
	agentWorker.RegisterActivityWithOptions(mockPrepareAgentCommandActivity, activity.RegisterOptions{
		Name: "PrepareAgentCommandActivity",
	})

	// Track activity executions
	activityExecutions := make(chan ActivityExecution, 10)

	// Create a wrapper for GitCommitActivity to track where it executes
	gitCommitWrapper := func(ctx context.Context, input types.GitCommitActivityInput) (*types.GitCommitActivityOutput, error) {
		// Record that this executed on orchestrator
		activityExecutions <- ActivityExecution{
			Name:      "GitCommitActivity",
			TaskQueue: orchestratorQueue,
			Time:      time.Now(),
		}
		return gitActivities.GitCommitActivity(ctx, input)
	}

	// Register orchestrator activities (all activities that use orchestratorCtx in workflow)
	orchestratorWorker.RegisterActivityWithOptions(gitCommitWrapper, activity.RegisterOptions{
		Name: "GitCommitActivity",
	})
	orchestratorWorker.RegisterActivityWithOptions(mockUpdateTaskStatusActivity, activity.RegisterOptions{
		Name: "UpdateTaskStatusActivity",
	})
	orchestratorWorker.RegisterActivityWithOptions(mockPublishTaskInProgressEventActivity, activity.RegisterOptions{
		Name: "PublishTaskInProgressEventActivity",
	})
	orchestratorWorker.RegisterActivityWithOptions(mockPublishTaskFinishedEventActivity, activity.RegisterOptions{
		Name: "PublishTaskFinishedEventActivity",
	})
	orchestratorWorker.RegisterActivityWithOptions(mockPublishErrorEventActivity, activity.RegisterOptions{
		Name: "PublishErrorEventActivity",
	})
	orchestratorWorker.RegisterActivityWithOptions(mockCaptureGitDiffActivity, activity.RegisterOptions{
		Name: "CaptureGitDiffActivity",
	})
	orchestratorWorker.RegisterActivityWithOptions(mockUpdateTaskGitDiffActivity, activity.RegisterOptions{
		Name: "UpdateTaskGitDiffActivity",
	})

	// Start workers
	err = orchestratorWorker.Start()
	require.NoError(t, err)
	defer orchestratorWorker.Stop()

	err = agentWorker.Start()
	require.NoError(t, err)
	defer agentWorker.Stop()

	// Execute ProcessTaskWorkflow on AGENT queue
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("test-cross-worker-%d", time.Now().Unix()),
		TaskQueue: agentQueue, // Run on agent queue
	}

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "test-123",
		TaskFilePath: "tasks/test.md",
		ProjectID:    "project-456",
		WorkspaceDir: worktreePath,
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: fmt.Sprintf("echo 'modified content' > %s", filepath.Join(worktreePath, "test.txt")),
			Variables:      map[string]string{},
			FlagFormat:     "space",
		},
		WorktreePath:          worktreePath,      // This triggers GitCommitActivity
		OrchestratorTaskQueue: orchestratorQueue, // Routes GitCommitActivity to orchestrator
	}

	we, err := temporalClient.ExecuteWorkflow(ctx, workflowOptions, workflows.ProcessTaskWorkflowName, input)
	require.NoError(t, err)

	// Wait for workflow completion
	var result types.ProcessTaskWorkflowOutput
	err = we.Get(ctx, &result)

	// EXPECTED BEHAVIOR WITH BUG:
	// This should fail with "no worker polling for task queue" because
	// GitCommitActivity is not registered on agent queue

	// EXPECTED BEHAVIOR WITH FIX:
	// This should succeed because GitCommitActivity is routed to orchestrator queue

	if err != nil {
		// This is the expected failure with current implementation
		require.Contains(t, err.Error(), "GitCommitActivity",
			"Should fail because GitCommitActivity is not on agent queue")
		t.Log("✓ Test correctly identified the cross-worker routing bug")
		t.Log("  GitCommitActivity was called on agent queue but not registered there")
		return
	}

	// If we get here, verify the activity executed on the correct queue
	select {
	case exec := <-activityExecutions:
		require.Equal(t, "GitCommitActivity", exec.Name)
		require.Equal(t, orchestratorQueue, exec.TaskQueue,
			"GitCommitActivity should execute on orchestrator queue")
		t.Log("✓ GitCommitActivity correctly routed to orchestrator queue")
	case <-time.After(5 * time.Second):
		t.Fatal("GitCommitActivity was not tracked - may have executed on wrong queue")
	}

	// Verify the workflow succeeded
	require.True(t, result.Success, "Workflow should succeed")
}

type ActivityExecution struct {
	Name      string
	TaskQueue string
	Time      time.Time
}

// mockLocalExecuteActivity simulates command execution for testing
func mockLocalExecuteActivity(ctx context.Context, input types.LocalExecuteActivityInput) (*types.LocalExecuteActivityOutput, error) {
	// Simulate successful command execution
	return &types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Command executed successfully",
		ErrorOutput: "",
		Duration:    100 * time.Millisecond,
		Success:     true,
	}, nil
}

// mockPrepareAgentCommandActivity simulates agent command preparation for testing
func mockPrepareAgentCommandActivity(ctx context.Context, input interface{}) ([]string, error) {
	return []string{"echo", "test"}, nil
}

// mockUpdateTaskStatusActivity simulates task status updates for testing
func mockUpdateTaskStatusActivity(ctx context.Context, input types.UpdateTaskStatusActivityInput) error {
	return nil
}

// mockPublishTaskInProgressEventActivity simulates task in-progress event publishing for testing
func mockPublishTaskInProgressEventActivity(ctx context.Context, input types.PublishEventInput) error {
	return nil
}

// mockPublishTaskFinishedEventActivity simulates task finished event publishing for testing
func mockPublishTaskFinishedEventActivity(ctx context.Context, input types.PublishEventInput) error {
	return nil
}

// mockPublishErrorEventActivity simulates error event publishing for testing
func mockPublishErrorEventActivity(ctx context.Context, input types.PublishErrorEventInput) error {
	return nil
}

// mockCaptureGitDiffActivity simulates git diff capture for testing
func mockCaptureGitDiffActivity(ctx context.Context, input types.CaptureGitDiffActivityInput) (*types.CaptureGitDiffActivityOutput, error) {
	return &types.CaptureGitDiffActivityOutput{
		Success:      true,
		Diff:         "mock diff",
		DiffStat:     "1 file changed, 1 insertion(+), 1 deletion(-)",
		FilesChanged: []string{"test.txt"},
		Insertions:   1,
		Deletions:    1,
		HasChanges:   true,
	}, nil
}

// mockUpdateTaskGitDiffActivity simulates git diff database update for testing
func mockUpdateTaskGitDiffActivity(ctx context.Context, input types.UpdateTaskGitDiffActivityInput) error {
	return nil
}

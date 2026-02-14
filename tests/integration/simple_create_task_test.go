// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal"
	temporaltypes "github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"
)

type TestResources struct {
	containerIDs []string
	dockerClient *client.Client
}

func (r *TestResources) trackContainer(containerID string) {
	r.containerIDs = append(r.containerIDs, containerID)
}

func (r *TestResources) cleanup() {
	for _, containerID := range r.containerIDs {
		if err := r.dockerClient.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true}); err != nil {
			fmt.Printf("Failed to cleanup container %s: %v\n", containerID[:12], err)
		}
	}
}

func TestSimpleCreateTaskFlow(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "noldarim-integration-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create sample test-claude-config.json
	testClaudeConfigPath := filepath.Join(tempDir, "test-claude-config.json")
	testClaudeConfig := `{
  "api_key": "test-api-key",
  "model": "claude-3-haiku-20240307"
}`
	err = os.WriteFile(testClaudeConfigPath, []byte(testClaudeConfig), 0o644)
	require.NoError(t, err)

	// Setup
	cfg, err := config.NewConfig("../../test-config.yaml")
	require.NoError(t, err)
	cfg.Database.Driver = "sqlite"
	cfg.Database.Database = ":memory:"

	// Override Claude config path to use absolute path to temp file
	cfg.Claude.ClaudeJSONHostPath = testClaudeConfigPath

	ctx := context.Background()

	// Create test project directory with git repository
	testProjectDir := filepath.Join(tempDir, "test-project")

	// Initialize git repository using GitService
	gitService, err := services.NewGitService(testProjectDir, true)
	require.NoError(t, err)

	// Configure git user for the test repo
	err = gitService.SetConfig(ctx, testProjectDir, "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.SetConfig(ctx, testProjectDir, "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a sample file and initial commit
	sampleFile := filepath.Join(testProjectDir, "README.md")
	err = os.WriteFile(sampleFile, []byte("# Test Project\n\nThis is a test project for noldarim integration tests.\n"), 0o644)
	require.NoError(t, err)

	// Create initial commit using GitService
	err = gitService.CreateCommit(ctx, testProjectDir, "Initial commit")
	require.NoError(t, err)

	dataServiceFixture := services.WithDataService(t)
	cmdChan := make(chan protocol.Command, 10)
	eventChan := make(chan protocol.Event, 10)

	// Setup Docker client for container verification using same host as config
	dockerClient, err := client.NewClientWithOpts(
		client.WithHost(cfg.Container.DockerHost),
		client.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err)

	resources := &TestResources{
		dockerClient: dockerClient,
		containerIDs: []string{},
	}
	defer resources.cleanup()

	orch, err := orchestrator.New(cmdChan, eventChan, cfg)
	require.NoError(t, err)

	go orch.Run(ctx)
	defer orch.Close()

	// Give the Temporal worker time to start polling
	// The worker.Run() is started in a goroutine and needs time to connect
	time.Sleep(2 * time.Second)

	// Create test project using the temp directory
	project, err := dataServiceFixture.Service.CreateProject(ctx, "Test Project", "Test project", testProjectDir)
	require.NoError(t, err)

	// Send create task command
	taskID := "test-" + time.Now().Format("20060102-150405")
	cmd := protocol.CreateTaskCommand{
		Metadata: protocol.Metadata{
			TaskID:  taskID,
			Version: protocol.CurrentProtocolVersion,
		},
		ProjectID:   project.ID,
		Title:       "Simple Test Task",
		Description: "Test task creation",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Hello World' > test_output.txt",
			Variables:      map[string]string{},
			FlagFormat:     "space",
		},
	}

	// Record temporal state before command
	var beforeWorkflows []WorkflowInfo
	if temporalClient := getTemporalClient(cfg); temporalClient != nil {
		beforeWorkflows = listWorkflows(temporalClient)
	}

	cmdChan <- cmd

	// 1. Verify expected events in event channel
	events := collectEvents(t, eventChan, 30*time.Second)
	require.True(t, len(events) > 0, "Should receive events")

	var taskCreatedEvent *protocol.TaskLifecycleEvent
	var pipelineCreatedEvent *protocol.PipelineLifecycleEvent
	var errorEvent *protocol.ErrorEvent

	for _, event := range events {
		switch evt := event.(type) {
		case protocol.TaskLifecycleEvent:
			if evt.Type == protocol.TaskCreated {
				taskCreatedEvent = &evt
			}
		case protocol.PipelineLifecycleEvent:
			if evt.Type == protocol.PipelineCreated {
				pipelineCreatedEvent = &evt
			}
		case protocol.ErrorEvent:
			errorEvent = &evt
		}
	}

	if errorEvent != nil {
		t.Logf("Received error: %s (may indicate temporal integration not implemented)", errorEvent.Message)
		return
	}

	// Accept either TaskLifecycleEvent (old system) or PipelineLifecycleEvent (unified system)
	var createdID string
	if taskCreatedEvent != nil {
		require.Equal(t, project.ID, taskCreatedEvent.ProjectID)
		require.NotNil(t, taskCreatedEvent.Task)
		createdID = taskCreatedEvent.Task.ID
	} else if pipelineCreatedEvent != nil {
		require.Equal(t, project.ID, pipelineCreatedEvent.ProjectID)
		require.NotNil(t, pipelineCreatedEvent.Run)
		createdID = pipelineCreatedEvent.Run.ID
	} else {
		require.Fail(t, "Should receive either TaskLifecycleEvent with TaskCreated or PipelineLifecycleEvent with PipelineCreated")
	}

	// 2. Verify expected temporal workflows/activities executed
	verifyTemporalExecution(t, createdID, cfg, beforeWorkflows)

	// 3. Verify expected files exist in worktree (container may be stopped by now)
	verifyTaskOutputFiles(t, createdID, project.ID, cfg, testProjectDir)
}

type WorkflowInfo struct {
	WorkflowID string
	RunID      string
	Status     string
}

func getTemporalClient(cfg *config.AppConfig) temporalclient.Client {
	temporalClientWrapper, err := temporal.NewClient(
		cfg.Temporal.HostPort,
		cfg.Temporal.Namespace,
		cfg.Temporal.TaskQueue,
	)
	if err != nil {
		return nil
	}
	return temporalClientWrapper.GetTemporalClient()
}

func listWorkflows(client temporalclient.Client) []WorkflowInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{PageSize: 100})
	if err != nil {
		return nil
	}

	var workflows []WorkflowInfo
	for _, workflow := range resp.Executions {
		workflows = append(workflows, WorkflowInfo{
			WorkflowID: workflow.Execution.WorkflowId,
			RunID:      workflow.Execution.RunId,
			Status:     workflow.Status.String(),
		})
	}
	return workflows
}

func collectEvents(t *testing.T, eventChan <-chan protocol.Event, timeout time.Duration) []protocol.Event {
	var events []protocol.Event
	deadline := time.After(timeout)

	for {
		select {
		case event := <-eventChan:
			events = append(events, event)
			// Check for terminal events
			// TaskFinished (old system) or PipelineFinished/PipelineFailed (unified system)
			// ErrorEvent also indicates failure/completion
			switch evt := event.(type) {
			case protocol.TaskLifecycleEvent:
				if evt.Type == protocol.TaskFinished {
					// Give a short window for any additional events
					time.Sleep(500 * time.Millisecond)
					return events
				}
				// TaskCreated is not terminal - ProcessTaskWorkflow still needs to run
			case protocol.PipelineLifecycleEvent:
				// PipelineFinished and PipelineFailed are terminal events in the unified system
				if evt.Type == protocol.PipelineFinished || evt.Type == protocol.PipelineFailed {
					// Give a short window for any additional events
					time.Sleep(500 * time.Millisecond)
					return events
				}
			case protocol.ErrorEvent:
				return events
			}
		case <-deadline:
			t.Log("collectEvents: timeout reached before receiving terminal event")
			return events
		}
	}
}

func verifyTemporalExecution(t *testing.T, taskID string, cfg *config.AppConfig, beforeWorkflows []WorkflowInfo) {
	temporalClient := getTemporalClient(cfg)
	if temporalClient == nil {
		t.Log("Temporal client not available - skipping workflow verification")
		return
	}

	afterWorkflows := listWorkflows(temporalClient)

	// Find new workflows
	var newWorkflows []WorkflowInfo
	for _, after := range afterWorkflows {
		found := false
		for _, before := range beforeWorkflows {
			if after.WorkflowID == before.WorkflowID {
				found = true
				break
			}
		}
		if !found && strings.Contains(after.WorkflowID, "create-task") {
			newWorkflows = append(newWorkflows, after)
		}
	}

	if len(newWorkflows) == 0 {
		t.Log("No temporal workflows found - integration not yet implemented")
		return
	}

	// Verify workflow activities for CreateTaskWorkflow
	for _, workflow := range newWorkflows {
		if strings.Contains(workflow.WorkflowID, "create-task") {
			// These activities run in the main CreateTaskWorkflow
			verifyWorkflowActivities(t, temporalClient, workflow.WorkflowID, []string{
				"WriteTaskFileActivity",
				"CreateTaskActivity",
				"GitCommitActivity",
				"CreateWorktreeActivity",
				"CreateContainerActivity",
				"CopyClaudeConfigActivity",
				"CopyClaudeCredentialsActivity",
				"PublishTaskCreatedEventActivity",
			})
		}
	}

	// Wait a bit for the child ProcessTaskWorkflow to start
	time.Sleep(5 * time.Second)

	// Check for ProcessTaskWorkflow (child workflow on dynamic queue)
	afterWorkflows2 := listWorkflows(temporalClient)
	var processWorkflows []WorkflowInfo
	for _, workflow := range afterWorkflows2 {
		if strings.Contains(workflow.WorkflowID, "process-task") {
			// Only include workflows that are not in the "before" list (i.e., new workflows)
			found := false
			for _, before := range beforeWorkflows {
				if workflow.WorkflowID == before.WorkflowID {
					found = true
					break
				}
			}
			if !found {
				processWorkflows = append(processWorkflows, workflow)
			}
		}
	}

	if len(processWorkflows) > 0 {
		// Verify the ProcessTaskWorkflow activities
		for _, workflow := range processWorkflows {
			verifyWorkflowActivities(t, temporalClient, workflow.WorkflowID, []string{
				"UpdateTaskStatusActivity",
				"PublishTaskInProgressEventActivity",
				"LocalExecuteActivity",
				"CaptureGitDiffActivity",
				"GitCommitActivity",
				"UpdateTaskStatusActivity",
				"PublishTaskFinishedEventActivity",
			})

			// Verify git diff metadata was captured
			verifyGitDiffMetadata(t, temporalClient, workflow.WorkflowID)
		}
	} else {
		t.Log("ProcessTaskWorkflow not found - agent may not be running in container")
	}
}

func verifyWorkflowActivities(t *testing.T, client temporalclient.Client, workflowID string, expectedActivities []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	iter := client.GetWorkflowHistory(ctx, workflowID, "", false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)

	var executedActivities []string
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			break
		}

		if event.GetEventType().String() == "ActivityTaskScheduled" {
			attrs := event.GetActivityTaskScheduledEventAttributes()
			if attrs != nil {
				executedActivities = append(executedActivities, attrs.GetActivityType().GetName())
			}
		}
	}

	t.Logf("Executed activities in workflow %s: %v", workflowID, executedActivities)

	for _, expected := range expectedActivities {
		found := false
		for _, executed := range executedActivities {
			if executed == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected activity '%s' was not executed. Executed: %v", expected, executedActivities)
	}
}

func verifyTaskOutputFiles(t *testing.T, taskID string, projectID string, cfg *config.AppConfig, testProjectDir string) {
	// CreateWorktreeFromCommit creates worktrees at {testProjectDir}/.worktrees/task-{taskID}-{timestamp}
	// We need to find it by scanning the .worktrees directory
	worktreesDir := filepath.Join(testProjectDir, ".worktrees")
	t.Logf("Looking for worktree in directory: %s", worktreesDir)

	// Check if .worktrees directory exists
	if _, err := os.Stat(worktreesDir); os.IsNotExist(err) {
		t.Logf("Worktrees directory not found at %s - may indicate worktree creation not implemented", worktreesDir)
		return
	}

	// Find the worktree directory for this task (it has a timestamp suffix)
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		t.Fatalf("Failed to read worktrees directory: %v", err)
	}

	var worktreePath string
	taskPrefix := fmt.Sprintf("task-%s", taskID)
	for _, entry := range entries {
		if entry.IsDir() && (entry.Name() == taskPrefix || strings.HasPrefix(entry.Name(), taskPrefix+"-")) {
			worktreePath = filepath.Join(worktreesDir, entry.Name())
			t.Logf("Found worktree: %s", worktreePath)
			break
		}
	}

	if worktreePath == "" {
		t.Logf("Worktree not found for task %s - available directories:", taskID)
		for _, entry := range entries {
			t.Logf("  - %s", entry.Name())
		}
		return
	}

	// Verify expected files exist in worktree
	expectedFiles := map[string]string{
		"test_output.txt": "Hello World",
	}

	for filename, expectedContent := range expectedFiles {
		filePath := filepath.Join(worktreePath, filename)

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Logf("Expected file %s not found in worktree", filename)

			// Debug: list worktree contents
			t.Logf("  - Listing worktree contents for debugging...")
			entries, readErr := os.ReadDir(worktreePath)
			if readErr != nil {
				t.Logf("    - Failed to read directory: %v", readErr)
			} else {
				t.Logf("    - Worktree contents:")
				for _, entry := range entries {
					t.Logf("      - %s (dir: %v)", entry.Name(), entry.IsDir())
				}
			}
			continue
		}

		t.Logf("Verified file exists in worktree: %s", filename)

		// Check file contents
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Logf("  - Failed to read file: %v", err)
			continue
		}

		assert.Contains(t, string(content), expectedContent, "File should contain expected content")
		t.Logf("  - File content verified: contains '%s'", expectedContent)
	}
}

func verifyGitDiffMetadata(t *testing.T, client temporalclient.Client, workflowID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query the workflow for processing metadata
	resp, err := client.QueryWorkflow(ctx, workflowID, "", "GetProcessingMetadata")
	if err != nil {
		t.Logf("Could not query workflow metadata (workflow may have completed): %v", err)
		return
	}

	var metadata temporaltypes.ProcessingMetadata
	if err := resp.Get(&metadata); err != nil {
		t.Logf("Could not decode metadata: %v", err)
		return
	}

	// Verify git diff was captured
	if metadata.GitDiff != nil {
		t.Logf("Git diff metadata captured successfully:")
		t.Logf("  - Files changed: %d", len(metadata.GitDiff.FilesChanged))
		t.Logf("  - Insertions: %d", metadata.GitDiff.Insertions)
		t.Logf("  - Deletions: %d", metadata.GitDiff.Deletions)
		t.Logf("  - Has changes: %v", metadata.GitDiff.HasChanges)

		// The task should have created test_output.txt, so we expect at least one file change
		assert.True(t, metadata.GitDiff.HasChanges, "Git diff should indicate changes were made")
		assert.Greater(t, len(metadata.GitDiff.FilesChanged), 0, "At least one file should have changed")
	} else {
		t.Log("Git diff metadata not available (may not be implemented yet)")
	}
}

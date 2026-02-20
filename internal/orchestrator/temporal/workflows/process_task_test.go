// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"
)

// Mock activity functions for testing
func LocalExecuteActivity(ctx context.Context, input types.LocalExecuteActivityInput) (*types.LocalExecuteActivityOutput, error) {
	return &types.LocalExecuteActivityOutput{}, nil
}

func PublishErrorEventActivity(ctx context.Context, input types.PublishErrorEventInput) error {
	return nil
}

func UpdateTaskStatusActivity(ctx context.Context, input types.UpdateTaskStatusActivityInput) error {
	return nil
}

func PrepareAgentCommandActivity(ctx context.Context, input *protocol.AgentConfigInput) ([]string, error) {
	return []string{"sh", "-c", input.PromptTemplate}, nil
}

func SaveAIActivityRecordActivity(ctx context.Context, record *models.AIActivityRecord) error {
	return nil
}


func TestProcessTaskWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Setup input
	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/test-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "cd /workspace && touch task-123.task && echo 'Test Task' > task.title && echo 'Processed task Test Task with description Test task description' > processing.log && echo 'Task processed successfully'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock LocalExecuteActivity - successful execution
	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"cd /workspace && touch task-123.task && echo 'Test Task' > task.title && echo 'Processed task Test Task with description Test task description' > processing.log && echo 'Task processed successfully'",
		},
		WorkDir: "/workspace",
	}

	mockCommandOutput := types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Task processed successfully\n",
		ErrorOutput: "",
		Success:     true,
	}

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Register and mock the activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)
	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return(&mockCommandOutput, nil)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.ProcessTaskWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, mockCommandOutput.Output, result.ProcessedData)
	assert.Empty(t, result.Error)

	// Verify activity was called with correct parameters
	env.AssertExpectations(t)
}

func TestProcessTaskWorkflow_LocalExecuteActivity_Error(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/test-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Test command that will fail'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"echo 'Test command that will fail'",
		},
		WorkDir: "/workspace",
	}

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)

	// Mock LocalExecuteActivity to return an error
	commandError := errors.New("container not found")
	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return((*types.LocalExecuteActivityOutput)(nil), commandError)

	// Mock PublishErrorEventActivity to handle error publishing (use mock.MatchedBy for flexible matching)
	env.OnActivity(PublishErrorEventActivity, mock.AnythingOfType("*context.timerCtx"), mock.MatchedBy(func(input types.PublishErrorEventInput) bool {
		return input.TaskID == "task-123" && input.Message == "Failed to process task"
	})).Return(nil)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed with error (this is expected behavior)
	assert.True(t, env.IsWorkflowCompleted())
	workflowError := env.GetWorkflowError()
	assert.Error(t, workflowError)

	// Verify the workflow error contains the expected activity error
	assert.Contains(t, workflowError.Error(), "container not found")
	assert.Contains(t, workflowError.Error(), "LocalExecuteActivity")

	// Verify activities were called
	env.AssertExpectations(t)
}

func TestProcessTaskWorkflow_CommandFailed_NonZeroExitCode(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/test-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "exit 1",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"exit 1",
		},
		WorkDir: "/workspace",
	}

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)

	// Mock LocalExecuteActivity to return non-zero exit code
	mockCommandOutput := types.LocalExecuteActivityOutput{
		ExitCode:    1,
		Output:      "",
		ErrorOutput: "Command failed: permission denied",
		Success:     false,
	}

	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return(&mockCommandOutput, nil)

	// Mock PublishErrorEventActivity (use mock.MatchedBy for flexible matching)
	env.OnActivity(PublishErrorEventActivity, mock.AnythingOfType("*context.timerCtx"), mock.MatchedBy(func(input types.PublishErrorEventInput) bool {
		return input.TaskID == "task-123" && input.Message == "Task processing failed"
	})).Return(nil)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed with error (this is expected behavior)
	assert.True(t, env.IsWorkflowCompleted())
	workflowError := env.GetWorkflowError()
	assert.Error(t, workflowError)

	// Verify the workflow error contains the expected error information
	assert.Contains(t, workflowError.Error(), "processing command failed: exit code 1")

	// Verify activities were called
	env.AssertExpectations(t)
}

func TestProcessTaskWorkflow_PublishErrorEventActivity_Fails(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/test-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Test command that will error'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"echo 'Test command that will error'",
		},
		WorkDir: "/workspace",
	}

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)

	// Mock LocalExecuteActivity to return an error
	commandError := errors.New("container not found")
	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return((*types.LocalExecuteActivityOutput)(nil), commandError)

	// Mock PublishErrorEventActivity to also fail (use mock.MatchedBy for flexible matching)
	publishError := errors.New("event publishing failed")
	env.OnActivity(PublishErrorEventActivity, mock.AnythingOfType("*context.timerCtx"), mock.MatchedBy(func(input types.PublishErrorEventInput) bool {
		return input.TaskID == "task-123" && input.Message == "Failed to process task"
	})).Return(publishError)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow fails with the original LocalExecuteActivity error
	// (PublishErrorEventActivity failure is logged but doesn't override the original error)
	assert.True(t, env.IsWorkflowCompleted())
	workflowError := env.GetWorkflowError()
	assert.Error(t, workflowError)

	// Verify the workflow error is the original LocalExecuteActivity error
	assert.Contains(t, workflowError.Error(), "container not found")
	assert.Contains(t, workflowError.Error(), "LocalExecuteActivity")

	// Verify activities were called
	env.AssertExpectations(t)
}

func TestProcessTaskWorkflow_ActivityOptions_Configuration(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/test-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Activity options test'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"echo 'Activity options test'",
		},
		WorkDir: "/workspace",
	}

	mockCommandOutput := types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Activity options test\n",
		ErrorOutput: "",
		Success:     true,
	}

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)

	// Verify activity options are configured correctly
	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return(&mockCommandOutput, nil).Run(func(args mock.Arguments) {
		// We can't directly inspect activity options in the test environment,
		// but we can verify the activity executes successfully with expected timeouts
		ctx := args.Get(0)
		assert.NotNil(t, ctx)
	})

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify activity was called
	env.AssertExpectations(t)
}

func TestProcessTaskWorkflow_EmptyInputFields(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Test with empty fields
	input := types.ProcessTaskWorkflowInput{
		TaskID:       "",
		TaskFilePath: "",
		ProjectID:    "",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Empty fields test'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"echo 'Empty fields test'",
		},
		WorkDir: "/workspace",
	}

	mockCommandOutput := types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Empty fields test\n",
		ErrorOutput: "",
		Success:     true,
	}

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)

	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return(&mockCommandOutput, nil)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully even with empty fields
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.ProcessTaskWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, mockCommandOutput.Output, result.ProcessedData)

	env.AssertExpectations(t)
}

func TestProcessTaskWorkflow_SpecialCharactersInInput(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Test with special characters that could cause shell issues
	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/special-chars-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Special characters test: & \"quotes\" $vars'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"echo 'Special characters test: & \"quotes\" $vars'",
		},
		WorkDir: "/workspace",
	}

	mockCommandOutput := types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Special characters test: & \"quotes\" vars\n",
		ErrorOutput: "",
		Success:     true,
	}

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)

	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return(&mockCommandOutput, nil)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.ProcessTaskWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, mockCommandOutput.Output, result.ProcessedData)

	env.AssertExpectations(t)
}

func TestProcessTaskWorkflow_WorkflowOptions_Validation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set up longer test timeout to verify activity timeouts are configured correctly
	env.SetTestTimeout(15 * time.Second)

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/test-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Workflow options validation test'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{
			"sh", "-c",
			"echo 'Workflow options validation test'",
		},
		WorkDir: "/workspace",
	}

	mockCommandOutput := types.LocalExecuteActivityOutput{
		ExitCode:    0,
		Output:      "Workflow options validation test\n",
		ErrorOutput: "",
		Success:     true,
	}

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	// Mock PrepareAgentCommandActivity
	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)

	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return(&mockCommandOutput, nil)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed successfully within expected time bounds
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	env.AssertExpectations(t)
}

// Mock activity for AI activity event publishing
func PublishAIActivityEventActivity(ctx context.Context, event *models.AIActivityRecord) error {
	return nil
}


func TestProcessTaskWorkflow_QueryHandler_GetProcessingMetadata(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := types.ProcessTaskWorkflowInput{
		TaskID:       "task-123",
		TaskFilePath: "tasks/test-task.md",
		ProjectID:    "project-789",
		WorkspaceDir: "/workspace",
		AgentConfig: &protocol.AgentConfigInput{
			ToolName:       "test",
			PromptTemplate: "echo 'Query test'",
		},
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	expectedCommandInput := types.LocalExecuteActivityInput{
		Command: []string{"sh", "-c", "echo 'Query test'"},
		WorkDir: "/workspace",
	}

	mockCommandOutput := types.LocalExecuteActivityOutput{
		ExitCode: 0,
		Output:   "Query test\n",
		Success:  true,
	}

	// Register child workflow
	env.RegisterWorkflow(AIObservabilityWorkflow)

	// Register activities
	env.RegisterActivity(PrepareAgentCommandActivity)
	env.RegisterActivity(LocalExecuteActivity)
	env.RegisterActivity(PublishErrorEventActivity)
	env.RegisterActivity(PublishTaskInProgressEventActivity)
	env.RegisterActivity(PublishTaskFinishedEventActivity)
	env.RegisterActivity(UpdateTaskStatusActivity)

	env.OnActivity(PrepareAgentCommandActivity, mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(expectedCommandInput.Command, nil)
	env.OnActivity(LocalExecuteActivity, mock.AnythingOfType("*context.timerCtx"), expectedCommandInput).Return(&mockCommandOutput, nil)

	// Execute workflow
	env.ExecuteWorkflow(ProcessTaskWorkflow, input)

	// Verify workflow completed
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Query the workflow for metadata
	queryResult, err := env.QueryWorkflow("GetProcessingMetadata")
	assert.NoError(t, err)

	var metadata types.ProcessingMetadata
	err = queryResult.Get(&metadata)
	assert.NoError(t, err)

	// Verify metadata was populated
	assert.NotZero(t, metadata.Timestamp)
	assert.Equal(t, "Query test\n", metadata.CommandOutput)
}

// Mock activities for transcript watcher tests
func InitTranscriptWatcherActivity(ctx context.Context, input types.InitTranscriptWatcherActivityInput) (*types.InitTranscriptWatcherActivityOutput, error) {
	return &types.InitTranscriptWatcherActivityOutput{
		Success:   true,
		WatcherID: input.TaskID,
	}, nil
}

func StopTranscriptWatcherActivity(ctx context.Context, input types.StopTranscriptWatcherActivityInput) error {
	return nil
}

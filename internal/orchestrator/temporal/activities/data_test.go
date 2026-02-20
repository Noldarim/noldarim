// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"
)

func TestUpdateTaskStatusActivity(t *testing.T) {
	tests := []struct {
		name          string
		input         types.UpdateTaskStatusActivityInput
		setupTask     func(t *testing.T, ds *services.DataService) string // Returns task ID
		expectedError bool
		errorContains string
	}{
		{
			name: "successfully update task status to in_progress",
			input: types.UpdateTaskStatusActivityInput{
				ProjectID: "test-project-1",
				TaskID:    "test-task-1",
				Status:    models.TaskStatusInProgress,
			},
			setupTask: func(t *testing.T, ds *services.DataService) string {
				// Create a project
				project, err := ds.CreateProject(context.Background(), "Test Project", "Test description", "/test/repo")
				require.NoError(t, err)

				// Create a task
				task, err := ds.CreateTask(context.Background(), project.ID, "test-task-1", "Test Task", "Test description", "")
				require.NoError(t, err)
				assert.Equal(t, models.TaskStatusPending, task.Status, "Initial task status should be pending")

				return project.ID
			},
			expectedError: false,
		},
		{
			name: "successfully update task status to completed",
			input: types.UpdateTaskStatusActivityInput{
				ProjectID: "test-project-2",
				TaskID:    "test-task-2",
				Status:    models.TaskStatusCompleted,
			},
			setupTask: func(t *testing.T, ds *services.DataService) string {
				// Create a project
				project, err := ds.CreateProject(context.Background(), "Test Project 2", "Test description", "/test/repo2")
				require.NoError(t, err)

				// Create a task
				task, err := ds.CreateTask(context.Background(), project.ID, "test-task-2", "Test Task 2", "Test description", "")
				require.NoError(t, err)
				assert.Equal(t, models.TaskStatusPending, task.Status, "Initial task status should be pending")

				return project.ID
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			// Setup test database
			dataServiceFixture := services.WithDataService(t)
			dataService := dataServiceFixture.Service

			// Setup task if needed
			if tt.setupTask != nil {
				projectID := tt.setupTask(t, dataService)
				// Update the input with the actual project ID
				tt.input.ProjectID = projectID
			}

			// Create activities
			dataActivities := NewDataActivities(dataService, nil)

			// Register activity
			env.RegisterActivity(dataActivities.UpdateTaskStatusActivity)

			// Execute activity
			_, err := env.ExecuteActivity(dataActivities.UpdateTaskStatusActivity, tt.input)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)

				// Verify the task status was actually updated in the database
				tasks, err := dataService.LoadTasks(context.Background(), tt.input.ProjectID)
				require.NoError(t, err)

				task, exists := tasks[tt.input.TaskID]
				require.True(t, exists, "Task should exist in database")
				assert.Equal(t, tt.input.Status, task.Status, "Task status should be updated")
			}
		})
	}
}

func TestUpdateTaskStatusActivity_Idempotency(t *testing.T) {
	// Setup test environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Setup test database
	dataServiceFixture := services.WithDataService(t)
	dataService := dataServiceFixture.Service

	// Create a project and task
	project, err := dataService.CreateProject(context.Background(), "Test Project", "Test description", "/test/repo")
	require.NoError(t, err)

	task, err := dataService.CreateTask(context.Background(), project.ID, "test-task-idempotent", "Test Task", "Test description", "")
	require.NoError(t, err)
	assert.Equal(t, models.TaskStatusPending, task.Status)

	// Create activities
	dataActivities := NewDataActivities(dataService, nil)
	env.RegisterActivity(dataActivities.UpdateTaskStatusActivity)

	// Execute activity multiple times with same input
	input := types.UpdateTaskStatusActivityInput{
		ProjectID: project.ID,
		TaskID:    task.ID,
		Status:    models.TaskStatusInProgress,
	}

	// First execution
	_, err = env.ExecuteActivity(dataActivities.UpdateTaskStatusActivity, input)
	assert.NoError(t, err)

	// Verify status changed
	tasks, err := dataService.LoadTasks(context.Background(), project.ID)
	require.NoError(t, err)
	updatedTask := tasks[task.ID]
	assert.Equal(t, models.TaskStatusInProgress, updatedTask.Status)

	// Second execution (idempotent)
	_, err = env.ExecuteActivity(dataActivities.UpdateTaskStatusActivity, input)
	assert.NoError(t, err)

	// Verify status is still in_progress
	tasks, err = dataService.LoadTasks(context.Background(), project.ID)
	require.NoError(t, err)
	updatedTask = tasks[task.ID]
	assert.Equal(t, models.TaskStatusInProgress, updatedTask.Status)

	// Third execution with different status
	input.Status = models.TaskStatusCompleted
	_, err = env.ExecuteActivity(dataActivities.UpdateTaskStatusActivity, input)
	assert.NoError(t, err)

	// Verify status changed to completed
	tasks, err = dataService.LoadTasks(context.Background(), project.ID)
	require.NoError(t, err)
	updatedTask = tasks[task.ID]
	assert.Equal(t, models.TaskStatusCompleted, updatedTask.Status)
}

func TestSaveAIActivityRecordActivity(t *testing.T) {
	trueVal := true
	tests := []struct {
		name          string
		record        *models.AIActivityRecord
		expectedError bool
	}{
		{
			name: "successfully save tool_use record",
			record: &models.AIActivityRecord{
				EventID:          "evt-save-tool-use-001",
				TaskID:           "test-task-save-1",
				SessionID:        "session-123",
				EventType:        models.AIEventToolUse,
				ToolName:         "Bash",
				ToolInputSummary: "ls -la",
			},
			expectedError: false,
		},
		{
			name: "successfully save tool_result record",
			record: &models.AIActivityRecord{
				EventID:        "evt-save-tool-result-002",
				TaskID:         "test-task-save-2",
				SessionID:      "session-123",
				EventType:      models.AIEventToolResult,
				ToolName:       "Bash",
				ToolSuccess:    &trueVal,
				ContentPreview: "file1.txt\nfile2.txt",
			},
			expectedError: false,
		},
		{
			name: "successfully save session_end record",
			record: &models.AIActivityRecord{
				EventID:      "evt-save-stop-003",
				TaskID:       "test-task-save-3",
				SessionID:    "session-123",
				EventType:    models.AIEventSessionEnd,
				StopReason:   "completed",
				InputTokens:  1000,
				OutputTokens: 4000,
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			// Setup test database
			dataServiceFixture := services.WithDataService(t)
			dataService := dataServiceFixture.Service

			// Create activities
			dataActivities := NewDataActivities(dataService, nil)
			env.RegisterActivity(dataActivities.SaveAIActivityRecordActivity)

			// Execute activity
			_, err := env.ExecuteActivity(dataActivities.SaveAIActivityRecordActivity, tt.record)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Verify the record was saved by loading it back
			records, err := dataService.GetAIActivityByTask(context.Background(), tt.record.TaskID)
			require.NoError(t, err)
			assert.Len(t, records, 1)
			assert.Equal(t, tt.record.EventID, records[0].EventID)
			assert.Equal(t, tt.record.EventType, records[0].EventType)
		})
	}
}

func TestSaveAIActivityRecordActivity_Idempotency(t *testing.T) {
	// Setup test environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Setup test database
	dataServiceFixture := services.WithDataService(t)
	dataService := dataServiceFixture.Service

	// Create activities
	dataActivities := NewDataActivities(dataService, nil)
	env.RegisterActivity(dataActivities.SaveAIActivityRecordActivity)

	// Create a record
	record := &models.AIActivityRecord{
		EventID:          "evt-idempotent-001",
		TaskID:           "test-task-idempotent",
		SessionID:        "session-123",
		EventType:        models.AIEventToolUse,
		ToolName:         "Read",
		ToolInputSummary: "/test.go",
	}

	// Execute activity first time
	_, err := env.ExecuteActivity(dataActivities.SaveAIActivityRecordActivity, record)
	assert.NoError(t, err)

	// Execute activity second time (idempotent - should not create duplicate)
	_, err = env.ExecuteActivity(dataActivities.SaveAIActivityRecordActivity, record)
	assert.NoError(t, err)

	// Verify only one record exists
	records, err := dataService.GetAIActivityByTask(context.Background(), record.TaskID)
	require.NoError(t, err)
	assert.Len(t, records, 1, "Should only have one record despite two save attempts")
}

func TestLoadAIActivityByTaskActivity(t *testing.T) {
	tests := []struct {
		name          string
		taskID        string
		setupRecords  func(t *testing.T, ds *services.DataService)
		expectedCount int
		expectedError bool
	}{
		{
			name:   "load records for task with records",
			taskID: "test-task-load-1",
			setupRecords: func(t *testing.T, ds *services.DataService) {
				// Create some records with explicit unique IDs
				for i := 1; i <= 3; i++ {
					record := &models.AIActivityRecord{
						EventID:          fmt.Sprintf("evt-load-%03d", i),
						TaskID:           "test-task-load-1",
						SessionID:        "session-123",
						EventType:        models.AIEventToolUse,
						ToolName:         "Read",
						ToolInputSummary: "/test.go",
					}
					err := ds.SaveAIActivityRecord(context.Background(), record)
					require.NoError(t, err)
				}
			},
			expectedCount: 3,
			expectedError: false,
		},
		{
			name:          "load records for task with no records",
			taskID:        "test-task-load-empty",
			setupRecords:  nil,
			expectedCount: 0,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			// Setup test database
			dataServiceFixture := services.WithDataService(t)
			dataService := dataServiceFixture.Service

			// Setup records if needed
			if tt.setupRecords != nil {
				tt.setupRecords(t, dataService)
			}

			// Create activities
			dataActivities := NewDataActivities(dataService, nil)
			env.RegisterActivity(dataActivities.LoadAIActivityByTaskActivity)

			// Execute activity
			val, err := env.ExecuteActivity(dataActivities.LoadAIActivityByTaskActivity, tt.taskID)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Get result
			var records []*models.AIActivityRecord
			err = val.Get(&records)
			require.NoError(t, err)

			assert.Len(t, records, tt.expectedCount)
		})
	}
}

func TestLoadAIActivityByTaskActivity_EventTypes(t *testing.T) {
	// Setup test environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Setup test database
	dataServiceFixture := services.WithDataService(t)
	dataService := dataServiceFixture.Service

	taskID := "test-task-event-types"
	trueVal := true

	// Create different event types with explicit unique IDs to avoid timestamp collision
	records := []*models.AIActivityRecord{
		{
			EventID:   "evt-types-001",
			TaskID:    taskID,
			SessionID: "session-123",
			EventType: models.AIEventSessionStart,
		},
		{
			EventID:          "evt-types-002",
			TaskID:           taskID,
			SessionID:        "session-123",
			EventType:        models.AIEventToolUse,
			ToolName:         "Bash",
			ToolInputSummary: "ls",
		},
		{
			EventID:        "evt-types-003",
			TaskID:         taskID,
			SessionID:      "session-123",
			EventType:      models.AIEventToolResult,
			ToolName:       "Bash",
			ToolSuccess:    &trueVal,
			ContentPreview: "file1.txt",
		},
		{
			EventID:      "evt-types-004",
			TaskID:       taskID,
			SessionID:    "session-123",
			EventType:    models.AIEventStop,
			StopReason:   "completed",
			InputTokens:  300,
			OutputTokens: 700,
		},
		{
			EventID:   "evt-types-005",
			TaskID:    taskID,
			SessionID: "session-123",
			EventType: models.AIEventSessionEnd,
		},
	}

	// Save all records
	for _, record := range records {
		err := dataService.SaveAIActivityRecord(context.Background(), record)
		require.NoError(t, err)
	}

	// Create activities
	dataActivities := NewDataActivities(dataService, nil)
	env.RegisterActivity(dataActivities.LoadAIActivityByTaskActivity)

	// Execute activity
	val, err := env.ExecuteActivity(dataActivities.LoadAIActivityByTaskActivity, taskID)
	require.NoError(t, err)

	// Get result
	var loadedRecords []*models.AIActivityRecord
	err = val.Get(&loadedRecords)
	require.NoError(t, err)

	// Verify all event types are preserved
	assert.Len(t, loadedRecords, 5)

	// Check each event type is present
	eventTypes := make(map[models.AIEventType]bool)
	for _, r := range loadedRecords {
		eventTypes[r.EventType] = true
	}

	assert.True(t, eventTypes[models.AIEventSessionStart], "Should have SessionStart record")
	assert.True(t, eventTypes[models.AIEventToolUse], "Should have ToolUse record")
	assert.True(t, eventTypes[models.AIEventToolResult], "Should have ToolResult record")
	assert.True(t, eventTypes[models.AIEventStop], "Should have Stop record")
	assert.True(t, eventTypes[models.AIEventSessionEnd], "Should have SessionEnd record")
}

func TestPrepareAgentCommandActivity(t *testing.T) {
	tests := []struct {
		name          string
		input         *protocol.AgentConfigInput
		wantCommand   []string
		expectedError bool
	}{
		{
			name: "valid claude config",
			input: &protocol.AgentConfigInput{
				ToolName:       "claude",
				ToolVersion:    "4.5",
				PromptTemplate: "Analyze {{.file}}",
				Variables: map[string]string{
					"file": "main.go",
				},
			},
			wantCommand: []string{
				"claude",
				"--print",
				"Analyze main.go",
			},
			expectedError: false,
		},
		{
			name: "claude config with model option",
			input: &protocol.AgentConfigInput{
				ToolName:       "claude",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{},
				ToolOptions: map[string]interface{}{
					"model": "claude-sonnet-4-5",
				},
			},
			wantCommand: []string{
				"claude",
				"--print",
				"--model",
				"claude-sonnet-4-5",
				"Test prompt",
			},
			expectedError: false,
		},
		{
			name:          "nil config",
			input:         nil,
			expectedError: true,
		},
		{
			name: "invalid config",
			input: &protocol.AgentConfigInput{
				PromptTemplate: "Test",
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			// Create data activities
			dataActivities := NewDataActivities(nil, nil)
			env.RegisterActivity(dataActivities.PrepareAgentCommandActivity)

			// Execute activity
			val, err := env.ExecuteActivity(dataActivities.PrepareAgentCommandActivity, tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Get result - activity returns []string directly
			var result []string
			err = val.Get(&result)
			require.NoError(t, err)

			// Verify command
			assert.Equal(t, tt.wantCommand, result)
		})
	}
}

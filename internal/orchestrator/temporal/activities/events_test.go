// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"testing"
	"time"

	"github.com/noldarim/noldarim/internal/common"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// Helper to create standard test input
func testEventInput(projectID, taskID string) types.PublishEventInput {
	return types.PublishEventInput{
		ProjectID: projectID,
		TaskID:    taskID,
	}
}

func TestPublishTaskCreatedEventActivity(t *testing.T) {
	tests := []struct {
		name          string
		input         types.PublishEventInput
		expectedError string
		expectEvent   bool
	}{
		{
			name: "successful TaskCreated event",
			input: types.PublishEventInput{
				ProjectID: "proj-123",
				TaskID:    "task-456",
				Task: &models.Task{
					ID:        "task-456",
					Title:     "Test Task",
					ProjectID: "proj-123",
				},
			},
			expectEvent: true,
		},
		{
			name: "TaskCreated without Task field",
			input: types.PublishEventInput{
				ProjectID: "proj-123",
				TaskID:    "task-456",
			},
			expectedError: "Task field is required",
			expectEvent:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			eventChan := make(chan protocol.Event, 10)
			eventActivities := NewEventActivities(eventChan)
			env.RegisterActivity(eventActivities.PublishTaskCreatedEventActivity)

			_, err := env.ExecuteActivity(eventActivities.PublishTaskCreatedEventActivity, tt.input)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}

			assert.NoError(t, err)

			if tt.expectEvent {
				select {
				case event := <-eventChan:
					lifecycleEvent, ok := event.(protocol.TaskLifecycleEvent)
					require.True(t, ok, "Expected TaskLifecycleEvent")

					assert.Equal(t, protocol.TaskCreated, lifecycleEvent.Type)
					assert.Equal(t, tt.input.ProjectID, lifecycleEvent.ProjectID)
					assert.NotNil(t, lifecycleEvent.Task)

					metadata := event.GetMetadata()
					assert.NotEmpty(t, metadata.IdempotencyKey)
					assert.Equal(t, protocol.CurrentProtocolVersion, metadata.Version)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("Expected event not received within timeout")
				}
			}
		})
	}
}

func TestPublishTaskDeletedEventActivity(t *testing.T) {
	tests := []struct {
		name        string
		input       types.PublishEventInput
		expectEvent bool
	}{
		{
			name:        "successful TaskDeleted event",
			input:       testEventInput("proj-123", "task-456"),
			expectEvent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			eventChan := make(chan protocol.Event, 10)
			eventActivities := NewEventActivities(eventChan)
			env.RegisterActivity(eventActivities.PublishTaskDeletedEventActivity)

			_, err := env.ExecuteActivity(eventActivities.PublishTaskDeletedEventActivity, tt.input)
			assert.NoError(t, err)

			if tt.expectEvent {
				select {
				case event := <-eventChan:
					lifecycleEvent, ok := event.(protocol.TaskLifecycleEvent)
					require.True(t, ok, "Expected TaskLifecycleEvent")

					assert.Equal(t, protocol.TaskDeleted, lifecycleEvent.Type)
					assert.Equal(t, tt.input.ProjectID, lifecycleEvent.ProjectID)
					assert.Equal(t, tt.input.TaskID, lifecycleEvent.TaskID)

					metadata := event.GetMetadata()
					assert.NotEmpty(t, metadata.IdempotencyKey)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("Expected event not received within timeout")
				}
			}
		})
	}
}

func TestPublishTaskStatusUpdatedEventActivity(t *testing.T) {
	tests := []struct {
		name        string
		input       types.PublishEventInput
		expectEvent bool
	}{
		{
			name: "successful TaskStatusUpdated event",
			input: types.PublishEventInput{
				ProjectID: "proj-123",
				TaskID:    "task-456",
				Status:    models.TaskStatusInProgress,
			},
			expectEvent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			eventChan := make(chan protocol.Event, 10)
			eventActivities := NewEventActivities(eventChan)
			env.RegisterActivity(eventActivities.PublishTaskStatusUpdatedEventActivity)

			_, err := env.ExecuteActivity(eventActivities.PublishTaskStatusUpdatedEventActivity, tt.input)
			assert.NoError(t, err)

			if tt.expectEvent {
				select {
				case event := <-eventChan:
					lifecycleEvent, ok := event.(protocol.TaskLifecycleEvent)
					require.True(t, ok, "Expected TaskLifecycleEvent")

					assert.Equal(t, protocol.TaskStatusUpdated, lifecycleEvent.Type)
					assert.Equal(t, tt.input.ProjectID, lifecycleEvent.ProjectID)
					assert.Equal(t, tt.input.TaskID, lifecycleEvent.TaskID)
					assert.Equal(t, tt.input.Status, lifecycleEvent.NewStatus)

					metadata := event.GetMetadata()
					assert.NotEmpty(t, metadata.IdempotencyKey)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("Expected event not received within timeout")
				}
			}
		})
	}
}

func TestPublishTaskRequestedEventActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	eventChan := make(chan protocol.Event, 10)
	eventActivities := NewEventActivities(eventChan)
	env.RegisterActivity(eventActivities.PublishTaskRequestedEventActivity)

	input := testEventInput("proj-123", "task-456")
	_, err := env.ExecuteActivity(eventActivities.PublishTaskRequestedEventActivity, input)
	assert.NoError(t, err)

	select {
	case event := <-eventChan:
		lifecycleEvent, ok := event.(protocol.TaskLifecycleEvent)
		require.True(t, ok, "Expected TaskLifecycleEvent")

		assert.Equal(t, protocol.TaskRequested, lifecycleEvent.Type)
		assert.Equal(t, input.ProjectID, lifecycleEvent.ProjectID)
		assert.Equal(t, input.TaskID, lifecycleEvent.TaskID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event not received within timeout")
	}
}

func TestPublishTaskInProgressEventActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	eventChan := make(chan protocol.Event, 10)
	eventActivities := NewEventActivities(eventChan)
	env.RegisterActivity(eventActivities.PublishTaskInProgressEventActivity)

	input := testEventInput("proj-123", "task-456")
	_, err := env.ExecuteActivity(eventActivities.PublishTaskInProgressEventActivity, input)
	assert.NoError(t, err)

	select {
	case event := <-eventChan:
		lifecycleEvent, ok := event.(protocol.TaskLifecycleEvent)
		require.True(t, ok, "Expected TaskLifecycleEvent")

		assert.Equal(t, protocol.TaskInProgress, lifecycleEvent.Type)
		assert.Equal(t, input.ProjectID, lifecycleEvent.ProjectID)
		assert.Equal(t, input.TaskID, lifecycleEvent.TaskID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event not received within timeout")
	}
}

func TestPublishTaskFinishedEventActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	eventChan := make(chan protocol.Event, 10)
	eventActivities := NewEventActivities(eventChan)
	env.RegisterActivity(eventActivities.PublishTaskFinishedEventActivity)

	input := testEventInput("proj-123", "task-456")
	_, err := env.ExecuteActivity(eventActivities.PublishTaskFinishedEventActivity, input)
	assert.NoError(t, err)

	select {
	case event := <-eventChan:
		lifecycleEvent, ok := event.(protocol.TaskLifecycleEvent)
		require.True(t, ok, "Expected TaskLifecycleEvent")

		assert.Equal(t, protocol.TaskFinished, lifecycleEvent.Type)
		assert.Equal(t, input.ProjectID, lifecycleEvent.ProjectID)
		assert.Equal(t, input.TaskID, lifecycleEvent.TaskID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event not received within timeout")
	}
}

func TestPublishErrorEventActivity(t *testing.T) {
	tests := []struct {
		name          string
		input         types.PublishErrorEventInput
		expectedError string
		expectEvent   bool
	}{
		{
			name: "successful error event",
			input: types.PublishErrorEventInput{
				Message:      "Test error message",
				ErrorContext: "Test context",
				TaskID:       "task-123",
			},
			expectEvent: true,
		},
		{
			name: "error event without message",
			input: types.PublishErrorEventInput{
				ErrorContext: "Test context",
			},
			expectedError: "message is required",
			expectEvent:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			eventChan := make(chan protocol.Event, 1)
			eventActivities := NewEventActivities(eventChan)
			env.RegisterActivity(eventActivities.PublishErrorEventActivity)

			_, err := env.ExecuteActivity(eventActivities.PublishErrorEventActivity, tt.input)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}

			assert.NoError(t, err)

			if tt.expectEvent {
				select {
				case event := <-eventChan:
					errorEvent, ok := event.(protocol.ErrorEvent)
					require.True(t, ok)
					assert.Equal(t, tt.input.Message, errorEvent.Message)
					assert.Equal(t, tt.input.ErrorContext, errorEvent.Context)

					metadata := errorEvent.GetMetadata()
					assert.NotEmpty(t, metadata.IdempotencyKey)
				case <-time.After(100 * time.Millisecond):
					t.Fatal("Expected error event not received within timeout")
				}
			}
		})
	}
}

func TestPublishAIActivityEventActivity(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	eventChan := make(chan common.Event, 10)
	eventActivities := NewEventActivities(eventChan)
	env.RegisterActivity(eventActivities.PublishAIActivityEventActivity)

	aiRecord := &models.AIActivityRecord{
		EventID:          "event-456",
		TaskID:           "task-789",
		SessionID:        "session-abc",
		Timestamp:        time.Now(),
		EventType:        models.AIEventToolUse,
		ToolName:         "Bash",
		ToolInputSummary: "ls -la",
	}

	_, err := env.ExecuteActivity(eventActivities.PublishAIActivityEventActivity, aiRecord)
	require.NoError(t, err)

	select {
	case event := <-eventChan:
		publishedRecord, ok := event.(*models.AIActivityRecord)
		require.True(t, ok, "Expected *models.AIActivityRecord")

		assert.Equal(t, "task-789", publishedRecord.TaskID)
		assert.Equal(t, "event-456", publishedRecord.EventID)
		assert.Equal(t, models.AIEventToolUse, publishedRecord.EventType)

		metadata := publishedRecord.GetMetadata()
		assert.NotEmpty(t, metadata.IdempotencyKey)
		assert.Equal(t, "event-456", metadata.IdempotencyKey) // Uses EventID as idempotency key
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected event not received within timeout")
	}
}

func TestPublishAIActivityEventActivity_MissingRecord(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	eventChan := make(chan common.Event, 10)
	eventActivities := NewEventActivities(eventChan)
	env.RegisterActivity(eventActivities.PublishAIActivityEventActivity)

	// Pass nil record
	var nilRecord *models.AIActivityRecord

	_, err := env.ExecuteActivity(eventActivities.PublishAIActivityEventActivity, nilRecord)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record is required")
}

func TestPublishAIActivityEventActivity_AllEventTypes(t *testing.T) {
	eventTypes := []models.AIEventType{
		models.AIEventSessionStart,
		models.AIEventSessionEnd,
		models.AIEventToolUse,
		models.AIEventToolResult,
		models.AIEventStop,
		models.AIEventError,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()

			eventChan := make(chan common.Event, 10)
			eventActivities := NewEventActivities(eventChan)
			env.RegisterActivity(eventActivities.PublishAIActivityEventActivity)

			aiRecord := &models.AIActivityRecord{
				EventID:   "event-456",
				TaskID:    "task-789",
				EventType: eventType,
			}

			_, err := env.ExecuteActivity(eventActivities.PublishAIActivityEventActivity, aiRecord)
			require.NoError(t, err)

			select {
			case event := <-eventChan:
				receivedRecord, ok := event.(*models.AIActivityRecord)
				require.True(t, ok)
				assert.Equal(t, eventType, receivedRecord.EventType)
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Expected event not received within timeout")
			}
		})
	}
}

// Test channel timeout behavior
func TestEventActivity_ChannelTimeout(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Create unbuffered channel to simulate timeout
	eventChan := make(chan protocol.Event)
	eventActivities := NewEventActivities(eventChan)
	env.RegisterActivity(eventActivities.PublishTaskInProgressEventActivity)

	input := testEventInput("proj-123", "task-456")

	_, err := env.ExecuteActivity(eventActivities.PublishTaskInProgressEventActivity, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout publishing")
}

// Test validation
func TestValidatePublishEventInput(t *testing.T) {
	tests := []struct {
		name          string
		input         types.PublishEventInput
		expectedError string
	}{
		{
			name:  "valid input",
			input: testEventInput("proj-123", "task-456"),
		},
		{
			name: "missing projectID",
			input: types.PublishEventInput{
				TaskID: "task-456",
			},
			expectedError: "projectID is required",
		},
		{
			name: "missing taskID",
			input: types.PublishEventInput{
				ProjectID: "proj-123",
			},
			expectedError: "taskID is required",
		},
		{
			name:          "empty input",
			input:         types.PublishEventInput{},
			expectedError: "projectID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePublishEventInput(tt.input)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateErrorEventInput(t *testing.T) {
	tests := []struct {
		name          string
		input         types.PublishErrorEventInput
		expectedError string
	}{
		{
			name: "valid input",
			input: types.PublishErrorEventInput{
				Message: "Error message",
			},
		},
		{
			name:          "missing message",
			input:         types.PublishErrorEventInput{},
			expectedError: "message is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorEventInput(tt.input)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

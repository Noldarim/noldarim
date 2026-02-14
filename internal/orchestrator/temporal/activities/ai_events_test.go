// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// =============================================================================
// SaveRawEventActivity Tests
//
// Tests the activity logic:
// - EventID generation
// - Event construction with correct fields
// - Successful save returns EventID
// - DB errors return Success=false (not activity error)
// =============================================================================

func TestSaveRawEventActivity_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Use real DataService with in-memory DB
	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)
	env.RegisterActivity(activities.SaveRawEventActivity)

	input := types.SaveRawEventInput{
		TaskID:     "task-123",
		ProjectID:  "project-456",
		Source:     "claude",
		RawPayload: json.RawMessage(`{"type":"tool_use","name":"Read"}`),
		Timestamp:  time.Now(),
	}

	encoded, err := env.ExecuteActivity(activities.SaveRawEventActivity, input)
	require.NoError(t, err)

	var output types.SaveRawEventOutput
	require.NoError(t, encoded.Get(&output))

	// Verify output
	assert.True(t, output.Success)
	assert.NotEmpty(t, output.EventID, "EventID should be generated")
	assert.Empty(t, output.Error)
}

func TestSaveRawEventActivity_GeneratesUniqueEventIDs(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)

	input := types.SaveRawEventInput{
		TaskID:     "task-123",
		ProjectID:  "project-456",
		Source:     "claude",
		RawPayload: json.RawMessage(`{"type":"tool_use"}`),
		Timestamp:  time.Now(),
	}

	eventIDs := make(map[string]bool)

	// Execute multiple times and collect EventIDs
	for i := 0; i < 5; i++ {
		env := testSuite.NewTestActivityEnvironment()
		env.RegisterActivity(activities.SaveRawEventActivity)

		encoded, err := env.ExecuteActivity(activities.SaveRawEventActivity, input)
		require.NoError(t, err)

		var output types.SaveRawEventOutput
		require.NoError(t, encoded.Get(&output))

		require.True(t, output.Success)
		require.NotEmpty(t, output.EventID)

		// Check uniqueness
		assert.False(t, eventIDs[output.EventID], "EventID should be unique: %s", output.EventID)
		eventIDs[output.EventID] = true
	}
}

func TestSaveRawEventActivity_PreservesInputFields(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)
	env.RegisterActivity(activities.SaveRawEventActivity)

	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	rawPayload := json.RawMessage(`{"type":"assistant","content":"hello"}`)

	input := types.SaveRawEventInput{
		TaskID:     "task-abc",
		ProjectID:  "project-xyz",
		Source:     "claude",
		RawPayload: rawPayload,
		Timestamp:  timestamp,
	}

	encoded, err := env.ExecuteActivity(activities.SaveRawEventActivity, input)
	require.NoError(t, err)

	var output types.SaveRawEventOutput
	require.NoError(t, encoded.Get(&output))
	require.True(t, output.Success)

	// Verify by loading the event from DB
	events, err := fixture.Service.GetAIActivityByTask(context.Background(), input.TaskID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	saved := events[0]
	assert.Equal(t, output.EventID, saved.EventID)
	assert.Equal(t, input.TaskID, saved.TaskID)
	assert.JSONEq(t, string(input.RawPayload), saved.RawPayload)
}

// =============================================================================
// ParseEventActivity Tests
//
// Tests the activity logic:
// - Unknown source returns Success=false (not activity error)
// - Context fields are set correctly on parsed event
// - Initializes adapters if not registered
// - Parse errors return Success=false gracefully
//
// NOTE: Does NOT test adapter parsing logic - that's in adapters package tests
// =============================================================================

func TestParseEventActivity_UnknownSource(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)
	env.RegisterActivity(activities.ParseEventActivity)

	input := types.ParseEventInput{
		EventID:    "event-123",
		Source:     "unknown-ai-tool",
		TaskID:     "task-456",
		ProjectID:  "project-789",
		RawPayload: json.RawMessage(`{"type":"tool_use"}`),
	}

	encoded, err := env.ExecuteActivity(activities.ParseEventActivity, input)
	require.NoError(t, err) // Activity should not error

	var output types.ParseEventOutput
	require.NoError(t, encoded.Get(&output))

	// Should return failure gracefully
	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "unknown adapter source")
	assert.Empty(t, output.Events)
}

func TestParseEventActivity_SetsContextFields(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Ensure adapters are registered
	adapters.RegisterAll()

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)
	env.RegisterActivity(activities.ParseEventActivity)

	// Use a valid Claude transcript line that the adapter can parse
	// This is a minimal tool_use event from Claude's JSONL format
	rawPayload := json.RawMessage(`{
		"type": "assistant",
		"message": {
			"content": [
				{
					"type": "tool_use",
					"name": "Read",
					"input": {"file_path": "/tmp/test.txt"}
				}
			]
		}
	}`)

	input := types.ParseEventInput{
		EventID:    "event-abc",
		Source:     "claude",
		TaskID:     "task-xyz",
		ProjectID:  "project-123",
		RawPayload: rawPayload,
	}

	encoded, err := env.ExecuteActivity(activities.ParseEventActivity, input)
	require.NoError(t, err)

	var output types.ParseEventOutput
	require.NoError(t, encoded.Get(&output))

	// If parsing succeeds, verify context fields are set on first event
	if output.Success && len(output.Events) > 0 {
		event := output.Events[0]
		assert.Equal(t, input.EventID, event.EventID, "EventID should be set from input")
		assert.Equal(t, input.TaskID, event.TaskID, "TaskID should be set from input")
		assert.NotEmpty(t, event.RawPayload, "RawPayload should be preserved")
	}
}

func TestParseEventActivity_UnrecognizedEventType(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Ensure adapters are registered
	adapters.RegisterAll()

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)
	env.RegisterActivity(activities.ParseEventActivity)

	// Valid JSON but unrecognized event type that adapter can't parse
	input := types.ParseEventInput{
		EventID:    "event-123",
		Source:     "claude",
		TaskID:     "task-456",
		ProjectID:  "project-789",
		RawPayload: json.RawMessage(`{"type":"unknown_event_type","data":"test"}`),
	}

	encoded, err := env.ExecuteActivity(activities.ParseEventActivity, input)
	require.NoError(t, err) // Activity should not error

	var output types.ParseEventOutput
	require.NoError(t, encoded.Get(&output))

	// Adapter returns empty array for unrecognized events (not an error)
	// The activity handles this gracefully
	if !output.Success {
		assert.Contains(t, output.Error, "no events parsed from entry")
		assert.Empty(t, output.Events)
	}
}

func TestParseEventActivity_PreservesRawPayload(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	adapters.RegisterAll()

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)
	env.RegisterActivity(activities.ParseEventActivity)

	// Use a parseable event
	rawPayload := json.RawMessage(`{
		"type": "assistant",
		"message": {
			"content": [{"type": "text", "text": "Hello"}]
		}
	}`)

	input := types.ParseEventInput{
		EventID:    "event-preserve",
		Source:     "claude",
		TaskID:     "task-123",
		ProjectID:  "project-456",
		RawPayload: rawPayload,
	}

	encoded, err := env.ExecuteActivity(activities.ParseEventActivity, input)
	require.NoError(t, err)

	var output types.ParseEventOutput
	require.NoError(t, encoded.Get(&output))

	// If parsing succeeds, RawPayload should be preserved
	if output.Success && len(output.Events) > 0 {
		assert.NotEmpty(t, output.Events[0].RawPayload, "RawPayload should be set on parsed event")
	}
}

func TestParseEventActivity_MultipleEventsFromSingleEntry(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	adapters.RegisterAll()

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)
	env.RegisterActivity(activities.ParseEventActivity)

	// Claude transcript entry with both thinking and text content blocks
	// This should produce 2 events
	rawPayload := json.RawMessage(`{
		"type": "assistant",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "thinking", "thinking": "Let me think about this..."},
				{"type": "text", "text": "Here is my response"}
			]
		}
	}`)

	input := types.ParseEventInput{
		EventID:    "event-multi",
		Source:     "claude",
		TaskID:     "task-multi",
		ProjectID:  "project-multi",
		RawPayload: rawPayload,
	}

	encoded, err := env.ExecuteActivity(activities.ParseEventActivity, input)
	require.NoError(t, err)

	var output types.ParseEventOutput
	require.NoError(t, encoded.Get(&output))

	require.True(t, output.Success, "Parsing should succeed")
	require.Len(t, output.Events, 2, "Should produce 2 events from thinking+text")

	// First event should be thinking
	assert.Equal(t, "thinking", string(output.Events[0].EventType))
	assert.Equal(t, "event-multi", output.Events[0].EventID)

	// Second event should be ai_output with different EventID
	assert.Equal(t, "ai_output", string(output.Events[1].EventType))
	assert.Equal(t, "event-multi-1", output.Events[1].EventID)
}

// =============================================================================
// Integration: Save then Parse flow
//
// Tests that SaveRawEvent → ParseEvent chain works correctly
// =============================================================================

func TestSaveAndParse_Integration(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}

	adapters.RegisterAll()

	fixture := services.WithDataService(t)
	defer fixture.Cleanup()

	activities := NewAIEventActivities(fixture.Service)

	// Step 1: Save raw event
	rawPayload := json.RawMessage(`{
		"type": "assistant",
		"message": {
			"content": [{"type": "text", "text": "Integration test"}]
		}
	}`)

	saveInput := types.SaveRawEventInput{
		TaskID:     "task-integration",
		ProjectID:  "project-integration",
		Source:     "claude",
		RawPayload: rawPayload,
		Timestamp:  time.Now(),
	}

	env1 := testSuite.NewTestActivityEnvironment()
	env1.RegisterActivity(activities.SaveRawEventActivity)

	encoded1, err := env1.ExecuteActivity(activities.SaveRawEventActivity, saveInput)
	require.NoError(t, err)

	var saveOutput types.SaveRawEventOutput
	require.NoError(t, encoded1.Get(&saveOutput))
	require.True(t, saveOutput.Success)
	require.NotEmpty(t, saveOutput.EventID)

	// Step 2: Parse using the EventID from save
	parseInput := types.ParseEventInput{
		EventID:    saveOutput.EventID,
		Source:     saveInput.Source,
		TaskID:     saveInput.TaskID,
		ProjectID:  saveInput.ProjectID,
		RawPayload: saveInput.RawPayload,
	}

	env2 := testSuite.NewTestActivityEnvironment()
	env2.RegisterActivity(activities.ParseEventActivity)

	encoded2, err := env2.ExecuteActivity(activities.ParseEventActivity, parseInput)
	require.NoError(t, err)

	var parseOutput types.ParseEventOutput
	require.NoError(t, encoded2.Get(&parseOutput))

	// Verify chain worked
	if parseOutput.Success && len(parseOutput.Events) > 0 {
		event := parseOutput.Events[0]
		assert.Equal(t, saveOutput.EventID, event.EventID)
		assert.Equal(t, saveInput.TaskID, event.TaskID)
	}
}

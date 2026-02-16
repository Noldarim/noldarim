// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// Stub activities for testing - these provide the correct function signatures
// so the test environment can register them, then we mock their behavior

func WatchTranscriptActivity(ctx context.Context, input types.WatchTranscriptActivityInput) (*types.WatchTranscriptActivityOutput, error) {
	return nil, nil
}

func SaveRawEventActivity(ctx context.Context, input types.SaveRawEventInput) (*types.SaveRawEventOutput, error) {
	return nil, nil
}

func ParseEventActivity(ctx context.Context, input types.ParseEventInput) (*types.ParseEventOutput, error) {
	return nil, nil
}

func UpdateParsedEventActivity(ctx context.Context, record *models.AIActivityRecord) error {
	return nil
}

// Note: PublishAIActivityEventActivity is already defined in process_task_test.go

// registerAIObsActivities registers all activities needed for AIObservability workflow tests
func registerAIObsActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivity(WatchTranscriptActivity)
	env.RegisterActivity(SaveRawEventActivity)
	env.RegisterActivity(ParseEventActivity)
	env.RegisterActivity(UpdateParsedEventActivity)
	env.RegisterActivity(PublishAIActivityEventActivity)
}

func TestAIObservabilityWorkflow_Success_ActivityCompletesNaturally(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities before mocking
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-123",
		ProjectID:             "project-789",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-123",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock WatchTranscriptActivity to complete successfully
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.MatchedBy(func(actInput types.WatchTranscriptActivityInput) bool {
		return actInput.TaskID == "task-123" &&
			actInput.ProjectID == "project-789" &&
			actInput.TranscriptDir == "/home/noldarim/.claude/projects/-workspace" &&
			actInput.Source == "claude"
	})).Return(&types.WatchTranscriptActivityOutput{
		Success:     true,
		EventsCount: 0, // Activity no longer counts - signals do
	}, nil)

	// Mock orchestrator activities for event forwarding (they may be called if signals arrive)
	env.OnActivity("SaveRawEventActivity", mock.Anything, mock.Anything).Return(&types.SaveRawEventOutput{
		EventID: "test-event-id",
		Success: true,
	}, nil).Maybe()
	env.OnActivity("ParseEventActivity", mock.Anything, mock.Anything).Return(&types.ParseEventOutput{
		Events:  []*models.AIActivityRecord{{EventType: models.AIEventToolUse}},
		Success: true,
	}, nil).Maybe()
	env.OnActivity("UpdateParsedEventActivity", mock.Anything, mock.Anything).Return(nil).Maybe()
	env.OnActivity("PublishAIActivityEventActivity", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, result.Error)
	// EventsCount will be 0 since no raw transcript signals were sent
	assert.Equal(t, 0, result.EventsCount)

	env.AssertExpectations(t)
}

func TestAIObservabilityWorkflow_ActivityCancelled_WorkflowCompletes(t *testing.T) {
	// Test that workflow handles activity cancellation gracefully
	// (happens when parent workflow terminates via PARENT_CLOSE_POLICY_TERMINATE)
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-cancel",
		ProjectID:             "project-cancel",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-cancel",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock WatchTranscriptActivity to complete (simulates cancellation/completion)
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{
			Success:     true,
			EventsCount: 0,
		}, nil,
	)

	// Mock orchestrator activities
	env.OnActivity("SaveRawEventActivity", mock.Anything, mock.Anything).Return(&types.SaveRawEventOutput{
		EventID: "test-event-id",
		Success: true,
	}, nil).Maybe()
	env.OnActivity("ParseEventActivity", mock.Anything, mock.Anything).Return(&types.ParseEventOutput{
		Events:  []*models.AIActivityRecord{{EventType: models.AIEventToolUse}},
		Success: true,
	}, nil).Maybe()
	env.OnActivity("UpdateParsedEventActivity", mock.Anything, mock.Anything).Return(nil).Maybe()
	env.OnActivity("PublishAIActivityEventActivity", mock.Anything, mock.Anything).Return(nil).Maybe()

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)

	env.AssertExpectations(t)
}

func TestAIObservabilityWorkflow_RawEventsForwardedViaSignals(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-events",
		ProjectID:             "project-events",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-events",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Track how many times the activities were called
	saveCallCount := 0
	parseCallCount := 0
	updateCallCount := 0
	publishCallCount := 0

	// Mock WatchTranscriptActivity - completes after a delay to allow signals to be processed
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{
			Success:     true,
			EventsCount: 0,
		}, nil,
	).After(100 * time.Millisecond) // Give time for signals to be processed

	// Mock SaveRawEventActivity - track calls
	env.OnActivity("SaveRawEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saveCallCount++
	}).Return(&types.SaveRawEventOutput{
		EventID: "test-event-id",
		Success: true,
	}, nil)

	// Mock ParseEventActivity - track calls
	env.OnActivity("ParseEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		parseCallCount++
	}).Return(&types.ParseEventOutput{
		Events: []*models.AIActivityRecord{{
			EventID:   "test-event-id",
			TaskID:    "task-events",
			EventType: models.AIEventToolUse,
		}},
		Success: true,
	}, nil)

	// Mock UpdateParsedEventActivity - track calls
	env.OnActivity("UpdateParsedEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		updateCallCount++
	}).Return(nil)

	// Mock PublishAIActivityEventActivity - track calls
	env.OnActivity("PublishAIActivityEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		publishCallCount++
	}).Return(nil)

	// Send raw transcript signals during workflow execution (simulating WatchTranscriptActivity behavior)
	env.RegisterDelayedCallback(func() {
		// Send 3 raw transcript events
		for i := 0; i < 3; i++ {
			rawEvent := types.RawTranscriptEvent{
				Source:    "claude",
				RawLine:   json.RawMessage(`{"type":"tool_use","name":"Read"}`),
				Timestamp: time.Now(),
				TaskID:    "task-events",
				ProjectID: "project-events",
			}
			env.SignalWorkflow(RawTranscriptLineSignal, rawEvent)
		}
	}, 10*time.Millisecond)

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, result.Error)

	// Verify events were processed through the pipeline (3 events sent via signals)
	assert.Equal(t, 3, result.EventsCount, "Should have processed 3 events")
	assert.Equal(t, 3, saveCallCount, "SaveRawEventActivity should be called 3 times")
	assert.Equal(t, 3, parseCallCount, "ParseEventActivity should be called 3 times")
	assert.Equal(t, 3, updateCallCount, "UpdateParsedEventActivity should be called 3 times")
	assert.Equal(t, 3, publishCallCount, "PublishAIActivityEventActivity should be called 3 times")

	env.AssertExpectations(t)
}

func TestAIObservabilityWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-error",
		ProjectID:             "project-error",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-error",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock WatchTranscriptActivity to return an error
	watcherError := errors.New("transcript file not found")
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{
			Success: false,
			Error:   watcherError.Error(),
		},
		watcherError,
	)

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow completed with error
	assert.True(t, env.IsWorkflowCompleted())
	workflowError := env.GetWorkflowError()
	assert.Error(t, workflowError)
	assert.Contains(t, workflowError.Error(), "transcript file not found")
}

func TestAIObservabilityWorkflow_InputValidation(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	// Test with empty fields - workflow should still work
	input := types.AIObservabilityWorkflowInput{
		TaskID:                "",
		ProjectID:             "",
		TranscriptDir:         "",
		ProcessTaskWorkflowID: "",
		OrchestratorTaskQueue: "",
	}

	// Mock activity to complete
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{
			Success:     true,
			EventsCount: 0,
		},
		nil,
	)

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow completed (activity handles empty inputs)
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Verify result
	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
}

func TestAIObservabilityWorkflow_ActivityOptions_Configured(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-options",
		ProjectID:             "project-options",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-options",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Verify activity receives expected input parameters
	activityCalled := false
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.MatchedBy(func(actInput types.WatchTranscriptActivityInput) bool {
		activityCalled = true
		// Verify the activity input is correctly passed
		return actInput.TaskID == "task-options" &&
			actInput.ProjectID == "project-options" &&
			actInput.TranscriptDir == "/home/noldarim/.claude/projects/-workspace" &&
			actInput.Source == "claude"
	})).Return(&types.WatchTranscriptActivityOutput{
		Success:     true,
		EventsCount: 100,
	}, nil)

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())
	assert.True(t, activityCalled, "Activity should have been called with expected input")

	// Get and verify result
	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	// EventsCount is now from signals (0 in unit tests without signal simulation)

	env.AssertExpectations(t)
}

func TestAIObservabilityWorkflow_WorkflowName(t *testing.T) {
	// Verify the workflow name constant is correct
	assert.Equal(t, "AIObservabilityWorkflow", AIObservabilityWorkflowName)
}

func TestAIObservabilityWorkflow_SignalNames(t *testing.T) {
	// Verify signal name constants are correct
	assert.Equal(t, "raw-transcript-line", RawTranscriptLineSignal)
	assert.Equal(t, "step-change", StepChangeSignal)
}

func TestAIObservabilityWorkflow_StepChangeSignal_TagsEvents(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-step-signal",
		RunID:                 "run-step-signal",
		ProjectID:             "project-step-signal",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-step-signal",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Capture StepID values passed to SaveRawEventActivity and ParseEventActivity
	var saveStepIDs []string
	var parseStepIDs []string

	// Mock WatchTranscriptActivity - completes after signals are processed
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{Success: true}, nil,
	).After(200 * time.Millisecond)

	// Mock SaveRawEventActivity - capture StepID from each call
	env.OnActivity("SaveRawEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saveInput := args.Get(1).(types.SaveRawEventInput)
		saveStepIDs = append(saveStepIDs, saveInput.StepID)
	}).Return(&types.SaveRawEventOutput{
		EventID: "test-event-id",
		Success: true,
	}, nil)

	// Mock ParseEventActivity - capture StepID from each call
	env.OnActivity("ParseEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		parseInput := args.Get(1).(types.ParseEventInput)
		parseStepIDs = append(parseStepIDs, parseInput.StepID)
	}).Return(&types.ParseEventOutput{
		Events:  []*models.AIActivityRecord{{EventID: "test-event-id", EventType: models.AIEventToolUse}},
		Success: true,
	}, nil)

	env.OnActivity("UpdateParsedEventActivity", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("PublishAIActivityEventActivity", mock.Anything, mock.Anything).Return(nil)

	makeRawEvent := func() types.RawTranscriptEvent {
		return types.RawTranscriptEvent{
			Source:    "claude",
			RawLine:   json.RawMessage(`{"type":"tool_use","name":"Read"}`),
			Timestamp: time.Now(),
			TaskID:    "task-step-signal",
			ProjectID: "project-step-signal",
		}
	}

	// Simulate PipelineWorkflow signaling step changes interleaved with transcript events.
	// Each batch is in a separate callback so the workflow fully processes one step's
	// signals before the next step's arrive (avoids the step-change goroutine racing
	// ahead and consuming all step signals before raw events are processed).

	// Batch 1: step-a with 2 events
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(StepChangeSignal, "step-a")
		env.SignalWorkflow(RawTranscriptLineSignal, makeRawEvent())
		env.SignalWorkflow(RawTranscriptLineSignal, makeRawEvent())
	}, 10*time.Millisecond)

	// Batch 2: step-b with 1 event (arrives after batch 1 activities complete)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(StepChangeSignal, "step-b")
		env.SignalWorkflow(RawTranscriptLineSignal, makeRawEvent())
	}, 50*time.Millisecond)

	// Batch 3: clear step context (like pipeline does after loop ends)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(StepChangeSignal, "")
	}, 80*time.Millisecond)

	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 3, result.EventsCount)

	// Verify step IDs were correctly propagated to activities
	assert.Equal(t, []string{"step-a", "step-a", "step-b"}, saveStepIDs,
		"SaveRawEventActivity should receive the correct StepID for each event")
	assert.Equal(t, []string{"step-a", "step-a", "step-b"}, parseStepIDs,
		"ParseEventActivity should receive the correct StepID for each event")

	env.AssertExpectations(t)
}

func TestAIObservabilityWorkflow_SaveEventFailure_ContinuesProcessing(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-save-fail",
		ProjectID:             "project-save-fail",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-save-fail",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock WatchTranscriptActivity - completes after signals are processed
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{
			Success:     true,
			EventsCount: 0,
		}, nil,
	).After(100 * time.Millisecond)

	// Mock SaveRawEventActivity to fail
	env.OnActivity("SaveRawEventActivity", mock.Anything, mock.Anything).Return(
		&types.SaveRawEventOutput{
			Success: false,
			Error:   "database connection failed",
		}, nil,
	)

	// ParseEventActivity and PublishAIActivityEventActivity should NOT be called when save fails
	// (we skip the rest of the pipeline on save failure)

	// Send a raw transcript signal
	env.RegisterDelayedCallback(func() {
		rawEvent := types.RawTranscriptEvent{
			Source:    "claude",
			RawLine:   json.RawMessage(`{"type":"tool_use"}`),
			Timestamp: time.Now(),
			TaskID:    "task-save-fail",
			ProjectID: "project-save-fail",
		}
		env.SignalWorkflow(RawTranscriptLineSignal, rawEvent)
	}, 10*time.Millisecond)

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow still completes successfully (save failure is non-fatal)
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	// EventsCount should be 0 since save failed
	assert.Equal(t, 0, result.EventsCount)

	env.AssertExpectations(t)
}

func TestAIObservabilityWorkflow_ParseEventFailure_ContinuesProcessing(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-parse-fail",
		ProjectID:             "project-parse-fail",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-parse-fail",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock WatchTranscriptActivity - completes after signals are processed
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{
			Success:     true,
			EventsCount: 0,
		}, nil,
	).After(100 * time.Millisecond)

	// Mock SaveRawEventActivity to succeed
	env.OnActivity("SaveRawEventActivity", mock.Anything, mock.Anything).Return(
		&types.SaveRawEventOutput{
			EventID: "test-event-id",
			Success: true,
		}, nil,
	)

	// Mock ParseEventActivity to fail
	env.OnActivity("ParseEventActivity", mock.Anything, mock.Anything).Return(
		&types.ParseEventOutput{
			Success: false,
			Error:   "unknown event type",
		}, nil,
	)

	// PublishAIActivityEventActivity should NOT be called when parse fails

	// Send a raw transcript signal
	env.RegisterDelayedCallback(func() {
		rawEvent := types.RawTranscriptEvent{
			Source:    "claude",
			RawLine:   json.RawMessage(`{"type":"unknown"}`),
			Timestamp: time.Now(),
			TaskID:    "task-parse-fail",
			ProjectID: "project-parse-fail",
		}
		env.SignalWorkflow(RawTranscriptLineSignal, rawEvent)
	}, 10*time.Millisecond)

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow still completes successfully (parse failure is non-fatal)
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	// EventsCount should be 0 since parse failed
	assert.Equal(t, 0, result.EventsCount)

	env.AssertExpectations(t)
}

func TestAIObservabilityWorkflow_MultipleEventsWithMixedResults(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerAIObsActivities(env)

	input := types.AIObservabilityWorkflowInput{
		TaskID:                "task-mixed",
		ProjectID:             "project-mixed",
		TranscriptDir:         "/home/noldarim/.claude/projects/-workspace",
		ProcessTaskWorkflowID: "process-task-task-mixed",
		OrchestratorTaskQueue: "noldarim-task-queue",
	}

	// Mock WatchTranscriptActivity - completes after signals are processed
	env.OnActivity("WatchTranscriptActivity", mock.Anything, mock.Anything).Return(
		&types.WatchTranscriptActivityOutput{
			Success:     true,
			EventsCount: 0,
		}, nil,
	).After(200 * time.Millisecond)

	// Track call counts
	saveCount := 0
	parseCount := 0
	updateCount := 0
	publishCount := 0

	// Mock SaveRawEventActivity - all succeed
	env.OnActivity("SaveRawEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saveCount++
	}).Return(&types.SaveRawEventOutput{
		EventID: "test-event-id",
		Success: true,
	}, nil)

	// Mock ParseEventActivity - succeed
	env.OnActivity("ParseEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		parseCount++
	}).Return(&types.ParseEventOutput{
		Events: []*models.AIActivityRecord{{
			EventID:   "test-event-id",
			TaskID:    "task-mixed",
			EventType: models.AIEventToolUse,
		}},
		Success: true,
	}, nil)

	// Mock UpdateParsedEventActivity - succeed
	env.OnActivity("UpdateParsedEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		updateCount++
	}).Return(nil)

	// Mock PublishAIActivityEventActivity
	env.OnActivity("PublishAIActivityEventActivity", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		publishCount++
	}).Return(nil)

	// Send multiple raw transcript signals
	env.RegisterDelayedCallback(func() {
		for i := 0; i < 5; i++ {
			rawEvent := types.RawTranscriptEvent{
				Source:    "claude",
				RawLine:   json.RawMessage(`{"type":"tool_use"}`),
				Timestamp: time.Now(),
				TaskID:    "task-mixed",
				ProjectID: "project-mixed",
			}
			env.SignalWorkflow(RawTranscriptLineSignal, rawEvent)
		}
	}, 10*time.Millisecond)

	// Execute workflow
	env.ExecuteWorkflow(AIObservabilityWorkflow, input)

	// Verify workflow completed successfully
	assert.True(t, env.IsWorkflowCompleted())
	assert.NoError(t, env.GetWorkflowError())

	// Get and verify result
	var result types.AIObservabilityWorkflowOutput
	err := env.GetWorkflowResult(&result)
	assert.NoError(t, err)
	assert.True(t, result.Success)

	// All 5 events should be processed through the full pipeline
	assert.Equal(t, 5, saveCount, "SaveRawEventActivity should be called 5 times")
	assert.Equal(t, 5, parseCount, "ParseEventActivity should be called 5 times")
	assert.Equal(t, 5, updateCount, "UpdateParsedEventActivity should be called 5 times")
	assert.Equal(t, 5, publishCount, "PublishAIActivityEventActivity should be called 5 times")
	assert.Equal(t, 5, result.EventsCount, "Should have processed 5 events")

	env.AssertExpectations(t)
}

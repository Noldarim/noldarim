// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	AIObservabilityWorkflowName = "AIObservabilityWorkflow"
	// continueAsNewThreshold limits events per workflow run to cap Temporal history growth.
	continueAsNewThreshold = 5000
)

// AIObservabilityWorkflow watches transcript sources and processes events via orchestrator.
// It runs as a CHILD workflow of ProcessTaskWorkflow with PARENT_CLOSE_POLICY_TERMINATE,
// meaning it automatically terminates when the parent completes.
//
// Architecture (new, when RuntimeName is set):
// - WatchTranscriptActivity runs in agent container, uses Observer/Parser pipeline
// - Activity signals this workflow with PARSED events via ParsedTranscriptBatchSignal
// - This workflow executes 2 activities on ORCHESTRATOR queue:
//  1. SaveCompleteEventActivity - saves fully-parsed record to DB
//  2. PublishAIActivityEventActivity - sends to TUI channel
//
// Architecture (legacy, when RuntimeName is empty):
// - WatchTranscriptActivity forwards RAW transcript lines
// - This workflow executes 4 activities on ORCHESTRATOR queue:
//  1. SaveRawEventActivity → 2. ParseEventActivity → 3. UpdateParsedEventActivity → 4. PublishAIActivityEventActivity
func AIObservabilityWorkflow(ctx workflow.Context, input types.AIObservabilityWorkflowInput) (*types.AIObservabilityWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AIObservability workflow",
		"taskID", input.TaskID,
		"projectID", input.ProjectID,
		"transcriptDir", input.TranscriptDir,
		"orchestratorQueue", input.OrchestratorTaskQueue,
		"runtimeName", input.RuntimeName)

	output := &types.AIObservabilityWorkflowOutput{
		Success: false,
	}

	// Configure activity options for the long-running watch activity (runs on agent queue)
	watchActivityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute, // Match ProcessTask timeout
		HeartbeatTimeout:    30 * time.Second,  // Activity must heartbeat regularly
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	watchCtx := workflow.WithActivityOptions(ctx, watchActivityOptions)

	// Configure activity options for orchestrator activities (save, parse, publish)
	// These run on the orchestrator queue, not the agent queue
	orchestratorActivityOptions := workflow.ActivityOptions{
		TaskQueue:           input.OrchestratorTaskQueue,
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    500 * time.Millisecond,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    3,
		},
	}
	orchestratorCtx := workflow.WithActivityOptions(ctx, orchestratorActivityOptions)

	// Track pending event processing to ensure all events are forwarded before completion
	pendingEvents := 0
	eventsProcessed := 0
	failedEvents := 0
	shouldContinueAsNew := false

	// Track which pipeline step is currently executing (set via StepChangeSignal from PipelineWorkflow)
	currentStepID := input.InitialStepID

	// Set up signal handler for step changes from PipelineWorkflow
	stepChangeChan := workflow.GetSignalChannel(ctx, types.StepChangeSignal)
	workflow.Go(ctx, func(gCtx workflow.Context) {
		for {
			var stepID string
			more := stepChangeChan.Receive(gCtx, &stepID)
			if !more {
				return
			}
			currentStepID = stepID
			logger.Info("Step context changed", "stepID", stepID)
		}
	})

	// =========================================================================
	// New: ParsedTranscriptBatchSignal handler (Observer/Parser pipeline)
	// =========================================================================
	parsedBatchChan := workflow.GetSignalChannel(ctx, types.ParsedTranscriptBatchSignal)
	workflow.Go(ctx, func(gCtx workflow.Context) {
		for {
			var batch types.ParsedTranscriptBatch
			more := parsedBatchChan.Receive(gCtx, &batch)
			if !more {
				logger.Info("Parsed transcript batch signal channel closed")
				return
			}

			for _, parsedEvent := range batch.Events {
				pendingEvents++
				stepID := currentStepID
				processedDelta, failedDelta := processParsedBatch(gCtx, orchestratorCtx, parsedEvent, stepID, logger)
				eventsProcessed += processedDelta
				failedEvents += failedDelta
				if input.EventsOffset+eventsProcessed >= continueAsNewThreshold {
					shouldContinueAsNew = true
				}
				pendingEvents--
			}
		}
	})

	// =========================================================================
	// Legacy: RawTranscriptLineSignal + RawTranscriptBatchSignal handlers
	// Deprecated: kept for backward compat when RuntimeName is empty
	// =========================================================================
	rawLineChan := workflow.GetSignalChannel(ctx, types.RawTranscriptLineSignal)
	batchChan := workflow.GetSignalChannel(ctx, types.RawTranscriptBatchSignal)

	// Process raw lines in a goroutine
	workflow.Go(ctx, func(gCtx workflow.Context) {
		for {
			var rawEvent types.RawTranscriptEvent
			more := rawLineChan.Receive(gCtx, &rawEvent)
			if !more {
				logger.Info("Raw transcript line signal channel closed")
				return
			}

			pendingEvents++
			stepID := currentStepID
			processedDelta, failedDelta := processRawEvent(gCtx, orchestratorCtx, rawEvent, stepID, input.RunID, logger)
			eventsProcessed += processedDelta
			failedEvents += failedDelta
			if input.EventsOffset+eventsProcessed >= continueAsNewThreshold {
				shouldContinueAsNew = true
			}
			pendingEvents--
		}
	})

	workflow.Go(ctx, func(gCtx workflow.Context) {
		for {
			var batch types.RawTranscriptBatch
			more := batchChan.Receive(gCtx, &batch)
			if !more {
				logger.Info("Raw transcript batch signal channel closed")
				return
			}

			for _, rawEvent := range batch.Events {
				pendingEvents++
				stepID := currentStepID
				processedDelta, failedDelta := processRawEvent(gCtx, orchestratorCtx, rawEvent, stepID, input.RunID, logger)
				eventsProcessed += processedDelta
				failedEvents += failedDelta
				if input.EventsOffset+eventsProcessed >= continueAsNewThreshold {
					shouldContinueAsNew = true
				}
				pendingEvents--
			}
		}
	})

	// Start the blocking watch activity (runs on agent queue)
	// This activity runs until the parent workflow terminates (PARENT_CLOSE_POLICY_TERMINATE)
	var activityResult types.WatchTranscriptActivityOutput
	activityErr := workflow.ExecuteActivity(watchCtx, "WatchTranscriptActivity", types.WatchTranscriptActivityInput{
		TaskID:        input.TaskID,
		RunID:         input.RunID,
		ProjectID:     input.ProjectID,
		TranscriptDir: input.TranscriptDir,
		Source:        "claude",
		RuntimeName:   input.RuntimeName,
	}).Get(ctx, &activityResult)

	// Activity completed (either naturally or via parent termination)
	logger.Info("Watch activity completed",
		"success", activityResult.Success,
		"error", activityErr,
		"eventsProcessed", input.EventsOffset+eventsProcessed,
		"failedEvents", failedEvents)

	// Set output - cancelled errors are expected when parent terminates
	if activityErr != nil && !temporal.IsCanceledError(activityErr) {
		output.Error = activityErr.Error()
		output.FailedEventsCount = failedEvents
		logger.Error("AIObservability workflow failed", "error", activityErr)
		return output, activityErr
	}

	if shouldContinueAsNew {
		nextInput := input
		nextInput.InitialStepID = currentStepID
		nextInput.EventsOffset = input.EventsOffset + eventsProcessed

		logger.Info("ContinueAsNew triggered",
			"eventsProcessed", nextInput.EventsOffset,
			"stepID", nextInput.InitialStepID)

		return output, workflow.NewContinueAsNewError(ctx, AIObservabilityWorkflowName, nextInput)
	}

	output.Success = true
	output.EventsCount = input.EventsOffset + eventsProcessed
	output.FailedEventsCount = failedEvents
	logger.Info("AIObservability workflow completed",
		"taskID", input.TaskID,
		"eventsProcessed", output.EventsCount,
		"failedEvents", failedEvents)

	return output, nil
}

// processParsedBatch handles a single ParsedTranscriptEvent from the Observer/Parser pipeline.
// For each ParsedEvent: Save + Publish (2 activities instead of 4).
func processParsedBatch(
	gCtx workflow.Context,
	orchestratorCtx workflow.Context,
	parsedEvent types.ParsedTranscriptEvent,
	stepID string,
	logger log.Logger,
) (int, int) {
	processed := 0
	failed := 0

	for _, parsed := range parsedEvent.ParsedEvents {
		record := models.NewAIActivityRecordFromParsed(parsed, parsedEvent.TaskID, parsedEvent.RunID, stepID)

		// Save complete event (single DB write)
		saveErr := workflow.ExecuteActivity(orchestratorCtx, "SaveCompleteEventActivity", record).Get(gCtx, nil)
		if saveErr != nil {
			logger.Warn("Failed to save complete event",
				"error", saveErr,
				"eventID", record.EventID,
				"taskID", parsedEvent.TaskID)
			failed++
			continue
		}

		// Publish to TUI
		publishErr := workflow.ExecuteActivity(orchestratorCtx, "PublishAIActivityEventActivity", record).Get(gCtx, nil)
		if publishErr != nil {
			logger.Warn("Failed to publish AI activity event",
				"error", publishErr,
				"eventType", record.EventType,
				"eventID", record.EventID)
		}

		processed++
	}

	return processed, failed
}

// processRawEvent handles a single RawTranscriptEvent (legacy 4-activity pipeline).
// Deprecated: New callers should use the Observer/Parser pipeline with ParsedTranscriptBatchSignal.
func processRawEvent(
	gCtx workflow.Context,
	orchestratorCtx workflow.Context,
	rawEvent types.RawTranscriptEvent,
	stepID string,
	runID string,
	logger log.Logger,
) (int, int) {
	var saveOutput types.SaveRawEventOutput
	saveErr := workflow.ExecuteActivity(orchestratorCtx, "SaveRawEventActivity", types.SaveRawEventInput{
		TaskID:     rawEvent.TaskID,
		RunID:      runID,
		StepID:     stepID,
		ProjectID:  rawEvent.ProjectID,
		Source:     rawEvent.Source,
		RawPayload: rawEvent.RawLine,
		Timestamp:  rawEvent.Timestamp,
	}).Get(gCtx, &saveOutput)
	if saveErr != nil {
		logger.Warn("Failed to save raw event to database",
			"error", saveErr,
			"taskID", rawEvent.TaskID)
		return 0, 1
	}

	if !saveOutput.Success {
		logger.Warn("Save raw event returned failure",
			"error", saveOutput.Error,
			"taskID", rawEvent.TaskID)
		return 0, 1
	}

	var parseOutput types.ParseEventOutput
	parseErr := workflow.ExecuteActivity(orchestratorCtx, "ParseEventActivity", types.ParseEventInput{
		EventID:    saveOutput.EventID,
		Source:     rawEvent.Source,
		TaskID:     rawEvent.TaskID,
		RunID:      runID,
		StepID:     stepID,
		ProjectID:  rawEvent.ProjectID,
		RawPayload: rawEvent.RawLine,
	}).Get(gCtx, &parseOutput)
	if parseErr != nil {
		logger.Warn("Failed to parse event",
			"error", parseErr,
			"taskID", rawEvent.TaskID,
			"eventID", saveOutput.EventID)
		return 0, 1
	}

	if !parseOutput.Success || len(parseOutput.Events) == 0 {
		logger.Warn("Parse event returned failure or empty",
			"error", parseOutput.Error,
			"taskID", rawEvent.TaskID,
			"eventID", saveOutput.EventID)
		return 0, 1
	}

	for _, event := range parseOutput.Events {
		updateErr := workflow.ExecuteActivity(orchestratorCtx, "UpdateParsedEventActivity", event).Get(gCtx, nil)
		if updateErr != nil {
			logger.Warn("Failed to update parsed event in database",
				"error", updateErr,
				"eventID", event.EventID,
				"taskID", rawEvent.TaskID)
		}

		publishErr := workflow.ExecuteActivity(orchestratorCtx, "PublishAIActivityEventActivity", event).Get(gCtx, nil)
		if publishErr != nil {
			logger.Warn("Failed to publish AI activity event to TUI",
				"error", publishErr,
				"eventType", event.EventType,
				"eventID", event.EventID)
		}
	}

	return 1, 0
}

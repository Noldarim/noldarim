// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows

import (
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	AIObservabilityWorkflowName = "AIObservabilityWorkflow"
	// RawTranscriptLineSignal is the signal for raw transcript lines from the watcher
	RawTranscriptLineSignal = "raw-transcript-line"
	// StepChangeSignal is sent by PipelineWorkflow to communicate the current step ID
	StepChangeSignal = "step-change"
)

// AIObservabilityWorkflow watches Claude's transcript and processes events via orchestrator.
// It runs as a CHILD workflow of ProcessTaskWorkflow with PARENT_CLOSE_POLICY_TERMINATE,
// meaning it automatically terminates when the parent completes.
//
// Architecture:
// - WatchTranscriptActivity runs in agent container, reads transcript files
// - Activity signals this workflow with RAW transcript lines (no parsing in agent)
// - This workflow executes activities on ORCHESTRATOR queue:
//  1. SaveRawEventActivity - saves raw payload to DB (persistence first)
//  2. ParseEventActivity - parses with adapter (single source of truth)
//  3. PublishAIActivityEventActivity - sends to TUI channel
//
// Lifecycle:
// - Started by ProcessTaskWorkflow at the beginning
// - Runs while Claude is executing
// - Automatically terminated when ProcessTaskWorkflow completes (after 5s grace period)
func AIObservabilityWorkflow(ctx workflow.Context, input types.AIObservabilityWorkflowInput) (*types.AIObservabilityWorkflowOutput, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AIObservability workflow (raw mode)",
		"taskID", input.TaskID,
		"projectID", input.ProjectID,
		"transcriptDir", input.TranscriptDir,
		"orchestratorQueue", input.OrchestratorTaskQueue)

	output := &types.AIObservabilityWorkflowOutput{
		Success: false,
	}

	// Configure activity options for the long-running watch activity (runs on agent queue)
	watchActivityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute, // Match ProcessTask timeout
		HeartbeatTimeout:    30 * time.Second, // Activity must heartbeat regularly
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

	// Track which pipeline step is currently executing (set via StepChangeSignal from PipelineWorkflow)
	currentStepID := ""

	// Set up signal handler for step changes from PipelineWorkflow
	stepChangeChan := workflow.GetSignalChannel(ctx, StepChangeSignal)
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

	// Set up signal handler for raw transcript lines from WatchTranscriptActivity
	rawLineChan := workflow.GetSignalChannel(ctx, RawTranscriptLineSignal)

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

			// Process event: Save Raw → Parse → Publish
			// Each step runs on the orchestrator queue

			// Step 1: Save raw event to database (persistence first)
			// Capture current step ID at event receive time
			stepID := currentStepID

			var saveOutput types.SaveRawEventOutput
			saveErr := workflow.ExecuteActivity(orchestratorCtx, "SaveRawEventActivity", types.SaveRawEventInput{
				TaskID:     rawEvent.TaskID,
				RunID:      input.RunID,
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
				failedEvents++
				pendingEvents--
				continue // Continue processing other events
			}

			if !saveOutput.Success {
				logger.Warn("Save raw event returned failure",
					"error", saveOutput.Error,
					"taskID", rawEvent.TaskID)
				// Cannot continue without a valid EventID - skip to next event
				failedEvents++
				pendingEvents--
				continue
			}

			// Step 2: Parse the event using the appropriate adapter
			var parseOutput types.ParseEventOutput
			parseErr := workflow.ExecuteActivity(orchestratorCtx, "ParseEventActivity", types.ParseEventInput{
				EventID:    saveOutput.EventID,
				Source:     rawEvent.Source,
				TaskID:     rawEvent.TaskID,
				RunID:      input.RunID,
				StepID:     stepID,
				ProjectID:  rawEvent.ProjectID,
				RawPayload: rawEvent.RawLine,
			}).Get(gCtx, &parseOutput)

			if parseErr != nil {
				logger.Warn("Failed to parse event",
					"error", parseErr,
					"taskID", rawEvent.TaskID,
					"eventID", saveOutput.EventID)
				failedEvents++
				pendingEvents--
				continue
			}

			if !parseOutput.Success || len(parseOutput.Events) == 0 {
				logger.Warn("Parse event returned failure or empty",
					"error", parseOutput.Error,
					"taskID", rawEvent.TaskID,
					"eventID", saveOutput.EventID)
				failedEvents++
				pendingEvents--
				continue
			} else {
				logger.Info("Parse event data", "Parse Output len", len(parseOutput.Events), "parseOutput", parseOutput)
			}

			// Process all parsed events (one entry can produce multiple events)
			for _, event := range parseOutput.Events {
				// Step 3: Update DB record with parsed data (enables historical retrieval)
				updateErr := workflow.ExecuteActivity(orchestratorCtx, "UpdateParsedEventActivity", event).Get(gCtx, nil)
				if updateErr != nil {
					logger.Warn("Failed to update parsed event in database",
						"error", updateErr,
						"eventID", event.EventID,
						"taskID", rawEvent.TaskID)
					// Non-fatal - raw data is still saved, parsing just won't persist
				}

				// Step 4: Publish parsed event to TUI for real-time display
				publishErr := workflow.ExecuteActivity(orchestratorCtx, "PublishAIActivityEventActivity", event).Get(gCtx, nil)
				if publishErr != nil {
					logger.Warn("Failed to publish AI activity event to TUI",
						"error", publishErr,
						"eventType", event.EventType,
						"eventID", event.EventID)
					// Non-fatal - continue processing
				}
			}

			pendingEvents--
			eventsProcessed++
		}
	})

	// Start the blocking watch activity (runs on agent queue)
	// This activity runs until the parent workflow terminates (PARENT_CLOSE_POLICY_TERMINATE)
	var activityResult types.WatchTranscriptActivityOutput
	activityErr := workflow.ExecuteActivity(watchCtx, "WatchTranscriptActivity", types.WatchTranscriptActivityInput{
		TaskID:        input.TaskID,
		ProjectID:     input.ProjectID,
		TranscriptDir: input.TranscriptDir,
		Source:        "claude",
	}).Get(ctx, &activityResult)

	// Activity completed (either naturally or via parent termination)
	logger.Info("Watch activity completed",
		"success", activityResult.Success,
		"error", activityErr,
		"eventsProcessed", eventsProcessed,
		"failedEvents", failedEvents)

	// Set output - cancelled errors are expected when parent terminates
	if activityErr != nil && !temporal.IsCanceledError(activityErr) {
		output.Error = activityErr.Error()
		output.FailedEventsCount = failedEvents
		logger.Error("AIObservability workflow failed", "error", activityErr)
		return output, activityErr
	}

	output.Success = true
	output.EventsCount = eventsProcessed
	output.FailedEventsCount = failedEvents
	logger.Info("AIObservability workflow completed",
		"taskID", input.TaskID,
		"eventsProcessed", eventsProcessed,
		"failedEvents", failedEvents)

	return output, nil
}

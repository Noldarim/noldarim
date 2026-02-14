// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"encoding/json"
	"time"

	"github.com/noldarim/noldarim/internal/aiobs/watcher"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

// RawTranscriptLineSignal is the signal name for raw transcript lines.
// The workflow listens for this signal to receive unparsed transcript data.
const RawTranscriptLineSignal = "raw-transcript-line"

// TranscriptWatcherActivities provides the blocking WatchTranscriptActivity.
type TranscriptWatcherActivities struct {
	temporalClient client.Client
}

// NewTranscriptWatcherActivities creates a new instance of TranscriptWatcherActivities.
func NewTranscriptWatcherActivities(temporalClient client.Client) *TranscriptWatcherActivities {
	return &TranscriptWatcherActivities{
		temporalClient: temporalClient,
	}
}

// WatchTranscriptActivity is a blocking activity that watches Claude's transcript directory
// and forwards RAW lines to AIObservabilityWorkflow via Temporal signals.
//
// This activity is a "dumb forwarder" - it does NOT parse transcript data.
// Parsing is done on the orchestrator side, allowing:
// - Single source of truth for adapter logic (orchestrator only)
// - Re-parsing historical data with improved adapters
// - Generic agent that works with any AI tool
//
// The activity:
// - Watches a directory for UUID-named .jsonl transcript files (Claude session files)
// - Signals its parent workflow (AIObservabilityWorkflow) with each RAW line
// - AIObservabilityWorkflow then executes orchestrator activities to save, parse, and publish
// - Sends heartbeats to Temporal every 10 seconds
// - Returns when context is cancelled (workflow received stop signal)
//
// The activity is designed to run for the duration of Claude's execution.
func (a *TranscriptWatcherActivities) WatchTranscriptActivity(
	ctx context.Context,
	input types.WatchTranscriptActivityInput,
) (*types.WatchTranscriptActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting transcript watcher activity (raw mode)",
		"taskID", input.TaskID,
		"transcriptDir", input.TranscriptDir,
		"source", input.Source)

	output := &types.WatchTranscriptActivityOutput{
		Success: false,
	}

	// Get parent workflow ID (AIObservabilityWorkflow) for signaling events
	// The activity signals its parent workflow, which handles forwarding to orchestrator
	activityInfo := activity.GetInfo(ctx)
	parentWorkflowID := activityInfo.WorkflowExecution.ID
	logger.Info("Will signal parent workflow with raw lines", "parentWorkflowID", parentWorkflowID)

	// Set defaults
	source := input.Source
	if source == "" {
		source = "claude"
	}

	// Create watcher configuration with UUID discovery and RAW MODE enabled
	// Raw mode means the watcher emits unparsed lines - no adapter needed in agent
	cfg := watcher.Config{
		FilePath:        input.TranscriptDir, // Directory for UUID discovery
		Source:          source,
		EventBufferSize: 1000,
		PollInterval:    100 * time.Millisecond,
		DiscoverUUID:    true, // Enable UUID file discovery
		RawMode:         true, // Enable raw mode - emit lines without parsing
	}

	// Create the transcript watcher
	w, err := watcher.NewTranscriptWatcher(ctx, cfg)
	if err != nil {
		output.Error = err.Error()
		logger.Error("Failed to create transcript watcher", "error", err)
		return output, err
	}

	// Start watching
	if err := w.Start(); err != nil {
		output.Error = err.Error()
		logger.Error("Failed to start transcript watcher", "error", err)
		return output, err
	}
	defer w.Stop()

	logger.Info("Transcript watcher started in raw mode, waiting for lines",
		"taskID", input.TaskID,
		"transcriptDir", input.TranscriptDir)

	// Get raw event and error channels (raw mode uses RawEvents())
	rawEventChan := w.RawEvents()
	errorChan := w.Errors()
	doneChan := w.Done()

	// Heartbeat ticker - send heartbeat every 10 seconds
	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	eventsCount := 0

	// Main event loop - runs until context is cancelled
	for {
		select {
		case <-ctx.Done():
			// Context cancelled - workflow received stop signal
			logger.Info("Context cancelled, stopping watcher",
				"taskID", input.TaskID,
				"linesForwarded", eventsCount)
			output.Success = true
			output.EventsCount = eventsCount
			return output, nil

		case <-doneChan:
			// Watcher stopped (file closed or error)
			stats := w.Stats()
			logger.Info("Watcher done",
				"taskID", input.TaskID,
				"linesRead", stats.LinesRead,
				"linesForwarded", eventsCount)
			output.Success = true
			output.EventsCount = eventsCount
			return output, nil

		case rawLine, ok := <-rawEventChan:
			if !ok {
				// Channel closed
				logger.Info("Raw event channel closed",
					"taskID", input.TaskID,
					"linesForwarded", eventsCount)
				output.Success = true
				output.EventsCount = eventsCount
				return output, nil
			}

			// Create RawTranscriptEvent to signal to workflow
			rawEvent := types.RawTranscriptEvent{
				Source:    source,
				RawLine:   json.RawMessage(rawLine.Line),
				Timestamp: rawLine.Timestamp,
				TaskID:    input.TaskID,
				ProjectID: input.ProjectID,
			}

			// Signal parent workflow (AIObservabilityWorkflow) with the raw line
			// The workflow will execute orchestrator activities to save, parse, and publish
			signalCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := a.temporalClient.SignalWorkflow(signalCtx, parentWorkflowID, "", RawTranscriptLineSignal, rawEvent)
			cancel()

			if err != nil {
				logger.Warn("Failed to signal parent workflow with raw line",
					"error", err,
					"taskID", input.TaskID)
				// Continue processing - don't fail on signal errors
			} else {
				eventsCount++
			}

		case err := <-errorChan:
			// Log watcher errors but don't fail
			logger.Warn("Watcher error", "error", err, "taskID", input.TaskID)

		case <-heartbeatTicker.C:
			// Send heartbeat with current stats
			stats := w.Stats()
			activity.RecordHeartbeat(ctx, map[string]interface{}{
				"taskID":          input.TaskID,
				"linesForwarded":  eventsCount,
				"linesRead":       stats.LinesRead,
				"activeFiles":     stats.ActiveFiles,
				"activeFileCount": stats.ActiveFileCount,
			})
		}
	}
}

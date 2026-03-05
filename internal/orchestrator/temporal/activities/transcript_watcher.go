// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"encoding/json"
	"time"

	"github.com/noldarim/noldarim/internal/aiobs/watcher"
	aiobsTypes "github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/orchestrator/agents"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
)

const (
	maxBatchSize       = 50
	batchFlushInterval = 250 * time.Millisecond
)

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

// WatchTranscriptActivity is a blocking activity that watches transcript sources
// and forwards events to AIObservabilityWorkflow via Temporal signals.
//
// When RuntimeName is set, uses the Observer/Parser pipeline:
// - Looks up the runtime via agents.GetRuntime(RuntimeName)
// - Calls observer.Discover() to find transcript sources
// - Creates a stateful Parser for tool correlation, session tracking, etc.
// - Parses events in-activity and signals ParsedTranscriptBatchSignal
//
// When RuntimeName is empty (backward compat), falls back to raw mode:
// - Forwards unparsed JSONL lines via RawTranscriptBatchSignal
// - Parsing happens on the orchestrator side
func (a *TranscriptWatcherActivities) WatchTranscriptActivity(
	ctx context.Context,
	input types.WatchTranscriptActivityInput,
) (*types.WatchTranscriptActivityOutput, error) {
	logger := activity.GetLogger(ctx)

	// Get parent workflow ID for signaling
	activityInfo := activity.GetInfo(ctx)
	parentWorkflowID := activityInfo.WorkflowExecution.ID

	// If RuntimeName is set, use the Observer/Parser pipeline
	if input.RuntimeName != "" {
		return a.watchWithParser(ctx, input, parentWorkflowID)
	}

	// Fallback: raw mode (backward compat)
	logger.Info("Starting transcript watcher activity (raw mode)",
		"taskID", input.TaskID,
		"transcriptDir", input.TranscriptDir,
		"source", input.Source)
	return a.watchRawMode(ctx, input, parentWorkflowID)
}

// watchWithParser uses the Observer/Parser pipeline for stateful event parsing.
func (a *TranscriptWatcherActivities) watchWithParser(
	ctx context.Context,
	input types.WatchTranscriptActivityInput,
	parentWorkflowID string,
) (*types.WatchTranscriptActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting transcript watcher activity (parser mode)",
		"taskID", input.TaskID,
		"runtimeName", input.RuntimeName,
		"transcriptDir", input.TranscriptDir)

	output := &types.WatchTranscriptActivityOutput{Success: false}

	// Look up the runtime
	runtime, ok := agents.GetRuntime(input.RuntimeName)
	if !ok {
		logger.Warn("Runtime not found, falling back to raw mode",
			"runtimeName", input.RuntimeName)
		return a.watchRawMode(ctx, input, parentWorkflowID)
	}

	observer := runtime.Observability()
	if observer == nil {
		logger.Warn("Runtime has no observer, falling back to raw mode",
			"runtimeName", input.RuntimeName)
		return a.watchRawMode(ctx, input, parentWorkflowID)
	}

	// Discover transcript sources
	runCtx := aiobsTypes.RunContext{
		TaskID:    input.TaskID,
		RunID:     input.RunID,
		ProjectID: input.ProjectID,
		WorkDir:   input.TranscriptDir,
	}

	spec, err := observer.Discover(ctx, runCtx)
	if err != nil {
		logger.Error("Failed to discover transcript sources", "error", err)
		output.Error = err.Error()
		return output, err
	}

	// Create stateful parser
	parser := observer.NewParser(runCtx)

	// Determine transcript dir from spec or input
	transcriptDir := input.TranscriptDir
	for _, stream := range spec.Streams {
		if stream.Type == "fs-jsonl" && transcriptDir == "" {
			transcriptDir = stream.Root
		}
	}

	// Check if we have any fs-jsonl streams to watch
	hasFSStream := false
	for _, stream := range spec.Streams {
		switch stream.Type {
		case "fs-jsonl":
			hasFSStream = true
		case "sse":
			logger.Info("SSE stream discovered, will start SSE reader",
				"name", stream.Name, "url", stream.Root)
		default:
			logger.Warn("Unknown stream type", "type", stream.Type, "name", stream.Name)
		}
	}

	if !hasFSStream && !hasSSEStream(spec.Streams) {
		output.Error = "no watchable streams discovered"
		logger.Error("No streams to watch", "runtimeName", input.RuntimeName)
		return output, nil
	}

	// Start file watcher for fs-jsonl streams
	var w *watcher.TranscriptWatcher
	var rawEventChan <-chan watcher.RawLine
	var errorChan <-chan error
	var doneChan <-chan struct{}

	if hasFSStream {
		cfg := watcher.Config{
			FilePath:        transcriptDir,
			Source:          input.RuntimeName,
			EventBufferSize: 1000,
			PollInterval:    100 * time.Millisecond,
			DiscoverUUID:    true,
			RawMode:         true,
		}

		w, err = watcher.NewTranscriptWatcher(ctx, cfg)
		if err != nil {
			output.Error = err.Error()
			logger.Error("Failed to create transcript watcher", "error", err)
			return output, err
		}

		if err := w.Start(); err != nil {
			output.Error = err.Error()
			logger.Error("Failed to start transcript watcher", "error", err)
			return output, err
		}
		defer w.Stop()

		rawEventChan = w.RawEvents()
		errorChan = w.Errors()
		doneChan = w.Done()
	}

	// Start SSE readers for sse streams
	sseLineChan := make(chan watcher.RawLine, 1000)
	for _, stream := range spec.Streams {
		if stream.Type == "sse" {
			reader := watcher.NewSSEReader(stream.Root, stream.Name)
			go func() {
				if err := reader.Start(ctx, sseLineChan); err != nil && ctx.Err() == nil {
					logger.Warn("SSE reader stopped", "error", err, "url", stream.Root)
				}
			}()
		}
	}

	logger.Info("Transcript watcher started (parser mode)",
		"taskID", input.TaskID,
		"transcriptDir", transcriptDir)

	// Heartbeat ticker
	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	eventsCount := 0
	batchesSent := 0

	parsedBatch := make([]types.ParsedTranscriptEvent, 0, maxBatchSize)
	batchTicker := time.NewTicker(batchFlushInterval)
	defer batchTicker.Stop()

	flushParsedBatch := func() {
		if len(parsedBatch) == 0 {
			return
		}

		batchPayload := types.ParsedTranscriptBatch{Events: parsedBatch}
		signalParentCtx := ctx
		if ctx.Err() != nil {
			signalParentCtx = context.Background()
		}
		signalCtx, cancel := context.WithTimeout(signalParentCtx, 5*time.Second)
		err := a.temporalClient.SignalWorkflow(signalCtx, parentWorkflowID, "", types.ParsedTranscriptBatchSignal, batchPayload)
		cancel()
		if err != nil {
			logger.Warn("Failed to signal parent workflow with parsed batch",
				"error", err,
				"taskID", input.TaskID,
				"batchSize", len(parsedBatch))
		} else {
			eventsCount += len(parsedBatch)
			batchesSent++
		}

		parsedBatch = parsedBatch[:0]
	}

	// processLine parses a raw line through the Parser and appends to the batch.
	processLine := func(rawLine watcher.RawLine, streamType string) {
		streamID := aiobsTypes.StreamID{
			Name:       rawLine.SourceFile,
			StreamType: streamType,
		}

		parsedEvents, err := parser.OnLine(ctx, streamID, rawLine.Line)
		if err != nil {
			logger.Warn("Parser error", "error", err, "sourceFile", rawLine.SourceFile)
			return
		}

		if len(parsedEvents) > 0 {
			parsedBatch = append(parsedBatch, types.ParsedTranscriptEvent{
				ParsedEvents: parsedEvents,
				TaskID:       input.TaskID,
				RunID:        input.RunID,
				ProjectID:    input.ProjectID,
				Timestamp:    rawLine.Timestamp,
			})

			if len(parsedBatch) >= maxBatchSize {
				flushParsedBatch()
			}
		}
	}

	flushParser := func() {
		remaining, err := parser.Flush(ctx)
		if err != nil {
			logger.Warn("Parser flush error", "error", err)
		}
		if len(remaining) > 0 {
			parsedBatch = append(parsedBatch, types.ParsedTranscriptEvent{
				ParsedEvents: remaining,
				TaskID:       input.TaskID,
				RunID:        input.RunID,
				ProjectID:    input.ProjectID,
				Timestamp:    time.Now(),
			})
		}
		flushParsedBatch()
	}

	// Make nil channels that will block forever if not initialized
	if rawEventChan == nil {
		ch := make(chan watcher.RawLine)
		rawEventChan = ch
	}
	if errorChan == nil {
		ch := make(chan error)
		errorChan = ch
	}
	if doneChan == nil {
		ch := make(chan struct{})
		doneChan = ch
	}

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			flushParser()
			logger.Info("Context cancelled, stopping watcher",
				"taskID", input.TaskID,
				"eventsForwarded", eventsCount)
			output.Success = true
			output.EventsCount = eventsCount
			return output, nil

		case <-doneChan:
			flushParser()
			if w != nil {
				stats := w.Stats()
				logger.Info("Watcher done",
					"taskID", input.TaskID,
					"linesRead", stats.LinesRead,
					"eventsForwarded", eventsCount)
			}
			output.Success = true
			output.EventsCount = eventsCount
			return output, nil

		case rawLine, ok := <-rawEventChan:
			if !ok {
				flushParser()
				logger.Info("Raw event channel closed",
					"taskID", input.TaskID,
					"eventsForwarded", eventsCount)
				output.Success = true
				output.EventsCount = eventsCount
				return output, nil
			}
			processLine(rawLine, "fs-jsonl")

		case rawLine, ok := <-sseLineChan:
			if !ok {
				continue
			}
			processLine(rawLine, "sse")

		case err := <-errorChan:
			logger.Warn("Watcher error", "error", err, "taskID", input.TaskID)

		case <-batchTicker.C:
			flushParsedBatch()

		case <-heartbeatTicker.C:
			heartbeatData := map[string]interface{}{
				"taskID":          input.TaskID,
				"eventsForwarded": eventsCount,
				"batchesSent":     batchesSent,
				"mode":            "parser",
			}
			if w != nil {
				stats := w.Stats()
				heartbeatData["linesRead"] = stats.LinesRead
				heartbeatData["activeFiles"] = stats.ActiveFiles
				heartbeatData["activeFileCount"] = stats.ActiveFileCount
			}
			activity.RecordHeartbeat(ctx, heartbeatData)
		}
	}
}

// watchRawMode is the original raw mode watcher (backward compat).
func (a *TranscriptWatcherActivities) watchRawMode(
	ctx context.Context,
	input types.WatchTranscriptActivityInput,
	parentWorkflowID string,
) (*types.WatchTranscriptActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Will signal parent workflow with raw transcript batches", "parentWorkflowID", parentWorkflowID)

	output := &types.WatchTranscriptActivityOutput{Success: false}

	source := input.Source
	if source == "" {
		source = "claude"
	}

	cfg := watcher.Config{
		FilePath:        input.TranscriptDir,
		Source:          source,
		EventBufferSize: 1000,
		PollInterval:    100 * time.Millisecond,
		DiscoverUUID:    true,
		RawMode:         true,
	}

	w, err := watcher.NewTranscriptWatcher(ctx, cfg)
	if err != nil {
		output.Error = err.Error()
		logger.Error("Failed to create transcript watcher", "error", err)
		return output, err
	}

	if err := w.Start(); err != nil {
		output.Error = err.Error()
		logger.Error("Failed to start transcript watcher", "error", err)
		return output, err
	}
	defer w.Stop()

	logger.Info("Transcript watcher started in raw mode, waiting for lines",
		"taskID", input.TaskID,
		"transcriptDir", input.TranscriptDir)

	rawEventChan := w.RawEvents()
	errorChan := w.Errors()
	doneChan := w.Done()

	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()

	eventsCount := 0
	batchesSent := 0

	batch := make([]types.RawTranscriptEvent, 0, maxBatchSize)
	batchTicker := time.NewTicker(batchFlushInterval)
	defer batchTicker.Stop()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		batchPayload := types.RawTranscriptBatch{Events: batch}
		signalParentCtx := ctx
		if ctx.Err() != nil {
			signalParentCtx = context.Background()
		}
		signalCtx, cancel := context.WithTimeout(signalParentCtx, 5*time.Second)
		err := a.temporalClient.SignalWorkflow(signalCtx, parentWorkflowID, "", types.RawTranscriptBatchSignal, batchPayload)
		cancel()
		if err != nil {
			logger.Warn("Failed to signal parent workflow with batch",
				"error", err,
				"taskID", input.TaskID,
				"batchSize", len(batch))
		} else {
			eventsCount += len(batch)
			batchesSent++
		}

		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flushBatch()
			logger.Info("Context cancelled, stopping watcher",
				"taskID", input.TaskID,
				"linesForwarded", eventsCount)
			output.Success = true
			output.EventsCount = eventsCount
			return output, nil

		case <-doneChan:
			flushBatch()
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
				flushBatch()
				logger.Info("Raw event channel closed",
					"taskID", input.TaskID,
					"linesForwarded", eventsCount)
				output.Success = true
				output.EventsCount = eventsCount
				return output, nil
			}

			rawEvent := types.RawTranscriptEvent{
				Source:     source,
				RawLine:    json.RawMessage(rawLine.Line),
				Timestamp:  rawLine.Timestamp,
				TaskID:     input.TaskID,
				ProjectID:  input.ProjectID,
				SourceFile: rawLine.SourceFile,
			}
			batch = append(batch, rawEvent)

			if len(batch) >= maxBatchSize {
				flushBatch()
			}

		case err := <-errorChan:
			logger.Warn("Watcher error", "error", err, "taskID", input.TaskID)

		case <-batchTicker.C:
			flushBatch()

		case <-heartbeatTicker.C:
			stats := w.Stats()
			activity.RecordHeartbeat(ctx, map[string]interface{}{
				"taskID":          input.TaskID,
				"linesForwarded":  eventsCount,
				"linesRead":       stats.LinesRead,
				"activeFiles":     stats.ActiveFiles,
				"activeFileCount": stats.ActiveFileCount,
				"batchesSent":     batchesSent,
			})
		}
	}
}

// hasSSEStream checks if any stream spec has type "sse".
func hasSSEStream(streams []aiobsTypes.StreamSpec) bool {
	for _, s := range streams {
		if s.Type == "sse" {
			return true
		}
	}
	return false
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	aiobsTypes "github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"

	"go.temporal.io/sdk/activity"
)

// AIEventActivities provides activities for processing AI events on the orchestrator.
// These activities run on the orchestrator queue and handle:
// - Saving raw event data to the database
// - Parsing raw events using the appropriate adapter
// - Publishing events to the TUI
type AIEventActivities struct {
	dataService *services.DataService
}

// NewAIEventActivities creates a new instance of AIEventActivities.
func NewAIEventActivities(dataService *services.DataService) *AIEventActivities {
	return &AIEventActivities{
		dataService: dataService,
	}
}

// SaveRawEventActivity saves a raw transcript event to the database.
// This is called BEFORE parsing to ensure raw data is always persisted.
//
// The activity:
// - Generates a unique EventID
// - Creates an AIActivityRecord with the raw payload (unparsed)
// - Saves to database
// - Returns the EventID for subsequent operations
//
// This enables:
// - Raw data preservation for debugging/analysis
// - Re-parsing historical data with improved adapters
// - Graceful handling of parsing failures (raw is still saved)
func (a *AIEventActivities) SaveRawEventActivity(
	ctx context.Context,
	input types.SaveRawEventInput,
) (*types.SaveRawEventOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Saving raw event",
		"taskID", input.TaskID,
		"runID", input.RunID,
		"source", input.Source)

	activity.RecordHeartbeat(ctx, "Saving raw event to database")

	// Generate unique event ID
	eventID := models.GenerateEventID()

	// Create minimal AIActivityRecord with raw data
	// EventType is left empty - will be filled after parsing
	record := &models.AIActivityRecord{
		EventID:    eventID,
		TaskID:     input.TaskID,
		RunID:      input.RunID,
		Timestamp:  input.Timestamp,
		RawPayload: string(input.RawPayload),
		// EventType intentionally empty - set during parsing
	}

	// Save to database
	if err := a.dataService.SaveAIActivityRecord(ctx, record); err != nil {
		logger.Error("Failed to save raw event", "error", err, "taskID", input.TaskID)
		return &types.SaveRawEventOutput{
			Success: false,
			Error:   err.Error(),
		}, nil // Return nil error to not retry - data issues shouldn't block
	}

	logger.Info("Raw event saved", "eventID", eventID, "taskID", input.TaskID, "runID", input.RunID)

	return &types.SaveRawEventOutput{
		EventID: eventID,
		Success: true,
	}, nil
}

// ParseEventActivity parses a raw transcript line using the appropriate adapter.
// This runs on the orchestrator, keeping adapter logic centralized.
//
// The activity:
// - Gets the adapter for the specified source (e.g., "claude")
// - Parses the raw JSON line into structured AIActivityRecords
// - Sets the EventID (from SaveRawEventActivity) on the parsed records
// - Returns the fully parsed event for publishing
//
// Note: This activity receives the raw payload directly in the input,
// avoiding a database round-trip. The raw data was already saved by SaveRawEventActivity.
func (a *AIEventActivities) ParseEventActivity(
	ctx context.Context,
	input types.ParseEventInput,
) (*types.ParseEventOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Debug("Parsing event",
		"eventID", input.EventID,
		"source", input.Source,
		"taskID", input.TaskID)

	activity.RecordHeartbeat(ctx, "Parsing event with adapter")

	// Get the adapter for this source
	adapter, ok := adapters.Get(input.Source)
	if !ok {
		// Check if adapters are registered
		if !adapters.IsInitialized() {
			// Register adapters - this should normally be done at startup
			// but we handle it here for robustness
			adapters.RegisterAll()
			adapter, ok = adapters.Get(input.Source)
		}
		if !ok {
			errMsg := fmt.Sprintf("unknown adapter source: %s (registered: %v)", input.Source, adapters.RegisteredAdapters())
			logger.Error("Adapter not found", "source", input.Source, "registered", adapters.RegisteredAdapters())
			return &types.ParseEventOutput{
				Success: false,
				Error:   errMsg,
			}, nil
		}
	}

	rawEntry := aiobsTypes.RawEntry{
		Line:      0, // Line number not available in this context
		Data:      input.RawPayload,
		SessionID: aiobsTypes.ExtractSessionID(input.RawPayload),
	}

	// Parse the raw JSON line using new adapter API
	parsedEvents, err := adapter.ParseEntry(rawEntry)
	if err != nil {
		logger.Warn("Failed to parse event", "error", err, "taskID", input.TaskID, "eventID", input.EventID)
		// Return failure but don't error - parsing failures shouldn't block the workflow
		return &types.ParseEventOutput{
			Success: false,
			Error:   fmt.Sprintf("parsing failed: %v", err),
		}, nil
	}

	// Handle case where no events were parsed (e.g., unknown entry type)
	if len(parsedEvents) == 0 {
		return &types.ParseEventOutput{
			Success: false,
			Error:   "no events parsed from entry",
		}, nil
	}

	// Convert all parsed events to AIActivityRecords using the shared conversion function
	events := make([]*models.AIActivityRecord, 0, len(parsedEvents))
	for i, parsed := range parsedEvents {
		// Generate unique event ID for each event from this entry
		eventID := input.EventID
		if i > 0 {
			eventID = fmt.Sprintf("%s-%d", input.EventID, i)
		}
		// Override the EventID from parsed with our workflow-assigned ID
		parsed.EventID = eventID
		events = append(events, models.NewAIActivityRecordFromParsed(parsed, input.TaskID, input.RunID))
	}

	logger.Debug("Events parsed successfully",
		"count", len(events),
		"taskID", input.TaskID)

	return &types.ParseEventOutput{
		Events:  events,
		Success: true,
	}, nil
}

// UpdateParsedEventActivity updates an existing raw event record with parsed data.
// This is called after ParseEventActivity succeeds to persist the parsed event_type
// and other fields to the database, enabling historical event retrieval with full data.
func (a *AIEventActivities) UpdateParsedEventActivity(
	ctx context.Context,
	record *models.AIActivityRecord,
) error {
	logger := activity.GetLogger(ctx)
	logger.Debug("Updating parsed event in database",
		"eventID", record.EventID,
		"eventType", record.EventType,
		"taskID", record.TaskID)

	activity.RecordHeartbeat(ctx, "Updating parsed event in database")

	// Use UpdateAIActivityRecord which does a proper UPDATE
	if err := a.dataService.UpdateAIActivityRecord(ctx, record); err != nil {
		logger.Error("Failed to update parsed event", "error", err, "eventID", record.EventID)
		return err
	}

	logger.Debug("Parsed event updated", "eventID", record.EventID, "eventType", record.EventType)
	return nil
}


// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"encoding/json"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// RawTranscriptEvent represents a raw transcript line from the agent.
// This is the signal payload sent from WatchTranscriptActivity to AIObservabilityWorkflow.
// The agent does NOT parse - just forwards raw bytes to orchestrator for processing.
type RawTranscriptEvent struct {
	// Source identifies the AI tool (e.g., "claude", "gemini")
	Source string `json:"source"`

	// RawLine is the unparsed JSON line from the transcript file
	RawLine json.RawMessage `json:"raw_line"`

	// Timestamp when the line was read from the file
	Timestamp time.Time `json:"timestamp"`

	// TaskID for correlation (set by activity)
	TaskID string `json:"task_id"`

	// ProjectID for event context (set by activity)
	ProjectID string `json:"project_id"`
}

// SaveRawEventInput is the input for SaveRawEventActivity.
// This activity saves the raw event to the database before parsing.
type SaveRawEventInput struct {
	TaskID     string          `json:"task_id"`
	RunID      string          `json:"run_id"`  // Pipeline run ID for aggregating all steps
	StepID     string          `json:"step_id"` // Pipeline step ID this event belongs to
	ProjectID  string          `json:"project_id"`
	Source     string          `json:"source"`
	RawPayload json.RawMessage `json:"raw_payload"`
	Timestamp  time.Time       `json:"timestamp"`
}

// SaveRawEventOutput is the output from SaveRawEventActivity.
type SaveRawEventOutput struct {
	// EventID is the generated unique ID for this event
	EventID string `json:"event_id"`

	// Success indicates if save was successful
	Success bool `json:"success"`

	// Error message if save failed
	Error string `json:"error,omitempty"`
}

// ParseEventInput is the input for ParseEventActivity.
// This activity parses raw event data using the appropriate adapter.
type ParseEventInput struct {
	// EventID of the event (from SaveRawEventActivity)
	EventID string `json:"event_id"`

	// Source identifies which adapter to use (e.g., "claude")
	Source string `json:"source"`

	// TaskID for context
	TaskID string `json:"task_id"`

	// RunID for pipeline run aggregation
	RunID string `json:"run_id"`

	// StepID for pipeline step association
	StepID string `json:"step_id"`

	// ProjectID for context
	ProjectID string `json:"project_id"`

	// RawPayload is the raw JSON line to parse
	// This is passed directly to avoid a database round-trip
	RawPayload json.RawMessage `json:"raw_payload"`
}

// ParseEventOutput is the output from ParseEventActivity.
type ParseEventOutput struct {
	// Events contains all parsed AIActivityRecords from the entry.
	// One transcript entry can produce multiple events (e.g., thinking + tool_use).
	Events []*models.AIActivityRecord `json:"events,omitempty"`

	// Success indicates if parsing was successful
	Success bool `json:"success"`

	// Error message if parsing failed
	Error string `json:"error,omitempty"`
}

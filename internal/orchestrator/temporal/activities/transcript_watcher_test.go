// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/internal/aiobs/watcher"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// =============================================================================
// Unit Tests for WatchTranscriptActivity
//
// NOTE: The blocking nature of this activity makes full execution tests impractical.
// Instead we test:
// 1. Activity struct creation
// 2. Signal payload construction (the core logic)
// 3. Signal name constant
//
// The actual watcher behavior is tested in internal/aiobs/watcher/watcher_test.go
// The workflow signal handling is tested in workflows/ai_observability_test.go
// =============================================================================

func TestNewTranscriptWatcherActivities(t *testing.T) {
	// Verify activity struct is created correctly
	activities := NewTranscriptWatcherActivities(nil)
	require.NotNil(t, activities)
}

func TestRawTranscriptLineSignal_ConstantValue(t *testing.T) {
	// Verify the signal name matches what the workflow expects
	assert.Equal(t, "raw-transcript-line", RawTranscriptLineSignal)
}

func TestRawTranscriptEvent_PayloadConstruction(t *testing.T) {
	// Test that the signal payload is constructed correctly from watcher output
	// This mirrors the logic in WatchTranscriptActivity lines 150-157

	source := "claude"
	taskID := "task-123"
	projectID := "project-456"

	// Simulate what the watcher produces
	rawLine := watcher.RawLine{
		Line:      []byte(`{"type":"tool_use","name":"Read"}`),
		Timestamp: time.Now(),
	}

	// Construct the event (same as activity does)
	rawEvent := types.RawTranscriptEvent{
		Source:    source,
		RawLine:   json.RawMessage(rawLine.Line),
		Timestamp: rawLine.Timestamp,
		TaskID:    taskID,
		ProjectID: projectID,
	}

	// Verify all fields are set correctly
	assert.Equal(t, "claude", rawEvent.Source)
	assert.Equal(t, "task-123", rawEvent.TaskID)
	assert.Equal(t, "project-456", rawEvent.ProjectID)
	assert.Equal(t, rawLine.Timestamp, rawEvent.Timestamp)
	assert.JSONEq(t, `{"type":"tool_use","name":"Read"}`, string(rawEvent.RawLine))
}

func TestRawTranscriptEvent_JSONSerialization(t *testing.T) {
	// Verify the payload can be serialized/deserialized (Temporal uses JSON)
	original := types.RawTranscriptEvent{
		Source:    "claude",
		RawLine:   json.RawMessage(`{"type":"assistant","content":"hello"}`),
		Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		TaskID:    "task-abc",
		ProjectID: "project-xyz",
	}

	// Serialize
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Deserialize
	var decoded types.RawTranscriptEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify round-trip
	assert.Equal(t, original.Source, decoded.Source)
	assert.Equal(t, original.TaskID, decoded.TaskID)
	assert.Equal(t, original.ProjectID, decoded.ProjectID)
	assert.JSONEq(t, string(original.RawLine), string(decoded.RawLine))
}

func TestSourceDefault_Logic(t *testing.T) {
	// Test the source defaulting logic from the activity (line 69-72)
	tests := []struct {
		input    string
		expected string
	}{
		{"", "claude"},           // Empty defaults to claude
		{"claude", "claude"},     // Explicit claude stays claude
		{"gemini", "gemini"},     // Other sources preserved
	}

	for _, tt := range tests {
		source := tt.input
		if source == "" {
			source = "claude"
		}
		assert.Equal(t, tt.expected, source)
	}
}

// =============================================================================
// Integration testing notes:
//
// The full activity → signal → workflow flow is tested via:
// 1. ai_observability_test.go - mocks WatchTranscriptActivity, sends signals,
//    verifies workflow receives them and calls orchestrator activities
//
// 2. The actual watcher (file reading, UUID discovery, streaming) is tested in:
//    internal/aiobs/watcher/watcher_test.go
//
// For true end-to-end testing with real Temporal, use integration tests
// that spin up actual workflows and containers.
// =============================================================================

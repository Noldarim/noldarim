// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

func TestGenerateEventID(t *testing.T) {
	id1 := GenerateEventID()
	id2 := GenerateEventID()

	// Should not be empty
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)

	// Should have expected format (timestamp-based)
	assert.Len(t, id1, 24) // "20060102150405.000000000"
}

func TestAIActivityRecord_Basic(t *testing.T) {
	trueVal := true
	record := &AIActivityRecord{
		EventID:          "evt-123",
		TaskID:           "task-456",
		SessionID:        "session-789",
		Timestamp:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EventType:        AIEventToolUse,
		ToolName:         "Bash",
		ToolInputSummary: "ls -la",
		ToolSuccess:      &trueVal,
	}

	assert.Equal(t, "evt-123", record.EventID)
	assert.Equal(t, "task-456", record.TaskID)
	assert.Equal(t, "session-789", record.SessionID)
	assert.Equal(t, AIEventToolUse, record.EventType)
	assert.Equal(t, "Bash", record.ToolName)
	assert.Equal(t, "ls -la", record.ToolInputSummary)
}

func TestAIActivityRecord_GetMetadata(t *testing.T) {
	record := &AIActivityRecord{
		EventID:   "evt-123",
		TaskID:    "task-456",
		SessionID: "session-789",
		EventType: AIEventToolUse,
	}

	meta := record.GetMetadata()
	assert.Equal(t, "task-456", meta.TaskID)
	assert.Equal(t, "evt-123", meta.IdempotencyKey)
}

func TestAIActivityRecord_WithStopReason(t *testing.T) {
	record := &AIActivityRecord{
		EventID:      "evt-stop-1",
		TaskID:       "task-456",
		SessionID:    "session-789",
		Timestamp:    time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC),
		EventType:    AIEventSessionEnd,
		StopReason:   "completed",
		InputTokens:  1000,
		OutputTokens: 4000,
	}

	assert.Equal(t, "completed", record.StopReason)
	assert.Equal(t, 1000, record.InputTokens)
	assert.Equal(t, 4000, record.OutputTokens)
}

func TestAIActivityRecord_WithToolResult(t *testing.T) {
	trueVal := true
	record := &AIActivityRecord{
		EventID:        "evt-result-1",
		TaskID:         "task-456",
		SessionID:      "session-789",
		Timestamp:      time.Date(2024, 1, 15, 10, 30, 5, 0, time.UTC),
		EventType:      AIEventToolResult,
		ToolName:       "Bash",
		ToolSuccess:    &trueVal,
		ContentPreview: "file1.txt\nfile2.txt",
	}

	assert.Equal(t, "Bash", record.ToolName)
	assert.True(t, *record.ToolSuccess)
	assert.Equal(t, "file1.txt\nfile2.txt", record.ContentPreview)
}

func TestAIActivityRecord_TokenTracking(t *testing.T) {
	record := &AIActivityRecord{
		EventID:           "evt-tokens-1",
		TaskID:            "task-456",
		EventType:         AIEventAIOutput,
		InputTokens:       5000,
		OutputTokens:      1500,
		CacheReadTokens:   3000,
		CacheCreateTokens: 500,
		ContextTokens:     5000,
	}

	assert.Equal(t, 5000, record.InputTokens)
	assert.Equal(t, 1500, record.OutputTokens)
	assert.Equal(t, 3000, record.CacheReadTokens)
	assert.Equal(t, 500, record.CacheCreateTokens)
	assert.Equal(t, 5000, record.ContextTokens)
}

func TestAIActivityRecord_ConversationTracking(t *testing.T) {
	record := &AIActivityRecord{
		EventID:     "evt-conv-1",
		TaskID:      "task-456",
		EventType:   AIEventAIOutput,
		MessageUUID: "msg-uuid-123",
		ParentUUID:  "parent-uuid-456",
		RequestID:   "req-789",
	}

	assert.Equal(t, "msg-uuid-123", record.MessageUUID)
	assert.Equal(t, "parent-uuid-456", record.ParentUUID)
	assert.Equal(t, "req-789", record.RequestID)
}

func TestNewAIActivityRecordFromParsed(t *testing.T) {
	trueVal := true
	ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	parsed := types.ParsedEvent{
		EventID:           "evt-parsed-123",
		SessionID:         "session-abc",
		MessageUUID:       "msg-uuid-xyz",
		ParentUUID:        "parent-uuid-def",
		RequestID:         "req-456",
		EventType:         types.EventTypeToolUse,
		IsHumanInput:      false,
		Timestamp:         ts,
		Model:             "claude-opus-4",
		StopReason:        "",
		InputTokens:       1000,
		OutputTokens:      500,
		CacheReadTokens:   800,
		CacheCreateTokens: 200,
		ToolName:          "Bash",
		ToolInputSummary:  "ls -la /tmp",
		ToolSuccess:       &trueVal,
		ToolError:         "",
		FilePath:          "/tmp",
		ContentPreview:    "command output preview",
		ContentLength:     1500,
		RawPayload:        json.RawMessage(`{"type":"tool_use"}`),
	}

	record := NewAIActivityRecordFromParsed(parsed, "task-789", "abc123def45678ab")

	// Identity
	assert.Equal(t, "evt-parsed-123", record.EventID)
	assert.Equal(t, "session-abc", record.SessionID)
	assert.Equal(t, "task-789", record.TaskID)
	assert.Equal(t, "abc123def45678ab", record.RunID)

	// Conversation structure
	assert.Equal(t, "msg-uuid-xyz", record.MessageUUID)
	assert.Equal(t, "parent-uuid-def", record.ParentUUID)
	assert.Equal(t, "req-456", record.RequestID)

	// Classification
	assert.Equal(t, AIEventToolUse, record.EventType)
	assert.NotNil(t, record.IsHumanInput)
	assert.False(t, *record.IsHumanInput)
	assert.Equal(t, ts, record.Timestamp)

	// Model info
	assert.Equal(t, "claude-opus-4", record.Model)

	// Token usage
	assert.Equal(t, 1000, record.InputTokens)
	assert.Equal(t, 500, record.OutputTokens)
	assert.Equal(t, 800, record.CacheReadTokens)
	assert.Equal(t, 200, record.CacheCreateTokens)
	assert.Equal(t, 1000, record.ContextTokens) // Should equal InputTokens

	// Tool info
	assert.Equal(t, "Bash", record.ToolName)
	assert.Equal(t, "ls -la /tmp", record.ToolInputSummary)
	assert.True(t, *record.ToolSuccess)
	assert.Equal(t, "/tmp", record.FilePath)

	// Content
	assert.Equal(t, "command output preview", record.ContentPreview)
	assert.Equal(t, 1500, record.ContentLength)

	// Raw payload stored as string
	assert.Equal(t, `{"type":"tool_use"}`, record.RawPayload)
}

func TestNewAIActivityRecordFromParsed_NilToolSuccess(t *testing.T) {
	parsed := types.ParsedEvent{
		EventID:      "evt-nil-success",
		EventType:    types.EventTypeAIOutput,
		IsHumanInput: false,
		ToolSuccess:  nil, // Intentionally nil
	}

	record := NewAIActivityRecordFromParsed(parsed, "task-123", "xyz789def01234ab")

	// ToolSuccess should still be nil (not dereferenced)
	assert.Nil(t, record.ToolSuccess)
}

func TestAIActivityRecord_GetRawPayloadJSON(t *testing.T) {
	t.Run("valid JSON payload", func(t *testing.T) {
		record := &AIActivityRecord{
			RawPayload: `{"foo":"bar","count":42}`,
		}

		raw := record.GetRawPayloadJSON()
		assert.NotNil(t, raw)

		var data map[string]interface{}
		err := json.Unmarshal(raw, &data)
		assert.NoError(t, err)
		assert.Equal(t, "bar", data["foo"])
		assert.Equal(t, float64(42), data["count"])
	})

	t.Run("empty payload", func(t *testing.T) {
		record := &AIActivityRecord{
			RawPayload: "",
		}

		raw := record.GetRawPayloadJSON()
		assert.Nil(t, raw)
	})

	t.Run("array payload", func(t *testing.T) {
		record := &AIActivityRecord{
			RawPayload: `[1,2,3]`,
		}

		raw := record.GetRawPayloadJSON()
		assert.NotNil(t, raw)

		var data []int
		err := json.Unmarshal(raw, &data)
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, data)
	})
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

func TestAdapter_Name(t *testing.T) {
	adapter := &Adapter{}
	assert.Equal(t, "claude", adapter.Name())
}

func parseEntry(t *testing.T, adapter *Adapter, rawJSON []byte) []types.ParsedEvent {
	entry := types.RawEntry{
		Line: 1,
		Data: json.RawMessage(rawJSON),
	}
	events, err := adapter.ParseEntry(entry)
	require.NoError(t, err)
	return events
}

func TestAdapter_ParseUserMessage(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "user",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [
				{"type": "text", "text": "Hello, Claude!"}
			]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeUserPrompt, event.EventType)
	assert.True(t, event.IsHumanInput)
	assert.Equal(t, "session-123", event.SessionID)
	assert.Equal(t, "test-uuid", event.MessageUUID)
	assert.Equal(t, "Hello, Claude!", event.ContentPreview)
}

func TestAdapter_ParseAssistantResponse(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "assistant",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"requestId": "request-456",
		"message": {
			"role": "assistant",
			"model": "claude-opus-4-5-20251101",
			"content": [
				{"type": "text", "text": "Hello! How can I help you today?"}
			],
			"stop_reason": "end_turn"
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeAIOutput, event.EventType)
	assert.False(t, event.IsHumanInput)
	assert.Equal(t, "claude-opus-4-5-20251101", event.Model)
	assert.Equal(t, "end_turn", event.StopReason)
	assert.Equal(t, "request-456", event.RequestID)
	assert.Equal(t, "Hello! How can I help you today?", event.ContentPreview)
}

func TestAdapter_ParseToolUse(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "assistant",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [
				{
					"type": "tool_use",
					"id": "tool-123",
					"name": "Bash",
					"input": {
						"command": "ls -la"
					}
				}
			]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeToolUse, event.EventType)
	assert.False(t, event.IsHumanInput)
	assert.Equal(t, "Bash", event.ToolName)
	assert.Equal(t, "ls -la", event.ToolInputSummary)
}

func TestAdapter_ParseToolUse_FileOperations(t *testing.T) {
	adapter := &Adapter{}

	testCases := []struct {
		name          string
		toolName      string
		input         string
		expectedInput string
		expectedPath  string
	}{
		{
			name:          "Read",
			toolName:      "Read",
			input:         `"file_path": "/path/to/file.go"`,
			expectedInput: "/path/to/file.go",
			expectedPath:  "/path/to/file.go",
		},
		{
			name:          "Write",
			toolName:      "Write",
			input:         `"file_path": "/path/to/output.txt"`,
			expectedInput: "/path/to/output.txt",
			expectedPath:  "/path/to/output.txt",
		},
		{
			name:          "Edit",
			toolName:      "Edit",
			input:         `"file_path": "/path/to/edit.js"`,
			expectedInput: "/path/to/edit.js",
			expectedPath:  "/path/to/edit.js",
		},
		{
			name:          "Glob",
			toolName:      "Glob",
			input:         `"pattern": "**/*.go"`,
			expectedInput: "**/*.go",
			expectedPath:  "",
		},
		{
			name:          "Grep",
			toolName:      "Grep",
			input:         `"pattern": "func main"`,
			expectedInput: "func main",
			expectedPath:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawJSON := []byte(`{
				"type": "assistant",
				"uuid": "test-uuid",
				"timestamp": "2025-01-15T10:30:00.000Z",
				"sessionId": "session-123",
				"message": {
					"role": "assistant",
					"content": [
						{
							"type": "tool_use",
							"id": "tool-123",
							"name": "` + tc.toolName + `",
							"input": {` + tc.input + `}
						}
					]
				}
			}`)

			events := parseEntry(t, adapter, rawJSON)
			require.Len(t, events, 1)

			event := events[0]
			assert.Equal(t, types.EventTypeToolUse, event.EventType)
			assert.Equal(t, tc.toolName, event.ToolName)
			assert.Equal(t, tc.expectedInput, event.ToolInputSummary)
			assert.Equal(t, tc.expectedPath, event.FilePath)
		})
	}
}

func TestAdapter_ParseToolResult(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "user",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [
				{
					"type": "tool_result",
					"tool_use_id": "tool-123",
					"content": "file1.txt\nfile2.txt",
					"is_error": false
				}
			]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeToolResult, event.EventType)
	assert.False(t, event.IsHumanInput)
	require.NotNil(t, event.ToolSuccess)
	assert.True(t, *event.ToolSuccess)
	assert.Contains(t, event.ContentPreview, "file1.txt")
}

func TestAdapter_ParseToolResult_Error(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "user",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [
				{
					"type": "tool_result",
					"tool_use_id": "tool-123",
					"content": "command not found: foo",
					"is_error": true
				}
			]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeToolResult, event.EventType)
	require.NotNil(t, event.ToolSuccess)
	assert.False(t, *event.ToolSuccess)
	assert.Equal(t, "command not found: foo", event.ToolError)
}

func TestAdapter_ParseThinking(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "assistant",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [
				{
					"type": "thinking",
					"thinking": "Let me analyze this request..."
				}
			]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeThinking, event.EventType)
	assert.Contains(t, event.ContentPreview, "Let me analyze this request...")
}

func TestAdapter_ParseSummary(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "summary",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"summary": "Session completed successfully"
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeSessionEnd, event.EventType)
	assert.False(t, event.IsHumanInput)
}

func TestAdapter_ParseSystem(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "system",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "System error occurred"}]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeError, event.EventType)
}

func TestAdapter_ParseUnknownType(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "unknown-type",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123"
	}`)

	events := parseEntry(t, adapter, rawJSON)
	// Unknown types should return empty - they're skipped
	assert.Len(t, events, 0)
}

func TestAdapter_ParseWithTokenUsage(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "assistant",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"model": "claude-opus-4-5-20251101",
			"content": [{"type": "text", "text": "Response"}],
			"usage": {
				"input_tokens": 1000,
				"output_tokens": 500,
				"cache_read_input_tokens": 200,
				"cache_creation_input_tokens": 100
			}
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, 1000, event.InputTokens)
	assert.Equal(t, 500, event.OutputTokens)
	assert.Equal(t, 200, event.CacheReadTokens)
	assert.Equal(t, 100, event.CacheCreateTokens)
}

func TestAdapter_ParseToolUseResult(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "user",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"toolUseResult": {
			"type": "text",
			"file": {
				"filePath": "/path/to/file.go",
				"content": "package main\n\nfunc main() {\n}",
				"numLines": 4,
				"startLine": 1,
				"totalLines": 4
			}
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, types.EventTypeToolResult, event.EventType)
	assert.Equal(t, "Read", event.ToolName)
	assert.Equal(t, "/path/to/file.go", event.FilePath)
	assert.Contains(t, event.ContentPreview, "/path/to/file.go")
}

func TestAdapter_ParseMultipleContentBlocks(t *testing.T) {
	adapter := &Adapter{}

	rawJSON := []byte(`{
		"type": "assistant",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "thinking", "thinking": "Thinking..."},
				{"type": "text", "text": "Here is my response"}
			]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	// Should produce 2 events - one for thinking, one for text
	require.Len(t, events, 2)

	assert.Equal(t, types.EventTypeThinking, events[0].EventType)
	assert.Equal(t, types.EventTypeAIOutput, events[1].EventType)
}

func TestAdapter_TimestampParsing(t *testing.T) {
	adapter := &Adapter{}

	testCases := []struct {
		name      string
		timestamp string
	}{
		{"RFC3339Nano", "2025-01-15T10:30:00.123456789Z"},
		{"RFC3339", "2025-01-15T10:30:00Z"},
		{"RFC3339 with offset", "2025-01-15T10:30:00+05:30"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rawJSON := []byte(`{
				"type": "user",
				"uuid": "test-uuid",
				"timestamp": "` + tc.timestamp + `",
				"sessionId": "session-123",
				"message": {
					"role": "user",
					"content": [{"type": "text", "text": "test"}]
				}
			}`)

			events := parseEntry(t, adapter, rawJSON)
			require.Len(t, events, 1)
			assert.False(t, events[0].Timestamp.IsZero())
		})
	}
}

func TestAdapter_ContentLength(t *testing.T) {
	adapter := &Adapter{}

	longContent := "This is a fairly long message that should be tracked for its full length in the ContentLength field."

	rawJSON := []byte(`{
		"type": "user",
		"uuid": "test-uuid",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "` + longContent + `"}]
		}
	}`)

	events := parseEntry(t, adapter, rawJSON)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, len(longContent), event.ContentLength)
}

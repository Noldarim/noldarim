// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

func TestClaudeParser_ToolResultCorrelatesToolName(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "session.jsonl", StreamType: "fs-jsonl"}

	toolUseLine := []byte(`{
		"type": "assistant",
		"uuid": "assistant-1",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "tool_use", "id": "toolu_123", "name": "Bash", "input": {"command": "ls -la"}}
			]
		}
	}`)

	_, err := parser.OnLine(context.Background(), stream, toolUseLine)
	require.NoError(t, err)

	toolResultLine := []byte(`{
		"type": "user",
		"uuid": "user-1",
		"timestamp": "2025-01-15T10:30:01.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [
				{"type": "tool_result", "tool_use_id": "toolu_123", "content": "ok", "is_error": false}
			]
		}
	}`)

	events, err := parser.OnLine(context.Background(), stream, toolResultLine)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, types.EventTypeToolResult, events[0].EventType)
	assert.Equal(t, "Bash", events[0].ToolName)
}

func TestClaudeParser_GeneratesSessionStart(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "session.jsonl", StreamType: "fs-jsonl"}

	line := []byte(`{
		"type": "user",
		"uuid": "user-1",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "hello"}]
		}
	}`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, types.EventTypeSessionStart, events[0].EventType)
	assert.Equal(t, types.EventTypeUserPrompt, events[1].EventType)
	assert.Equal(t, "session.jsonl", events[0].SourceFile)
	assert.Equal(t, "session.jsonl", events[1].SourceFile)
}

func TestClaudeParser_AssignsKindAndLevel(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "session.jsonl", StreamType: "fs-jsonl"}

	toolUseLine := []byte(`{
		"type": "assistant",
		"uuid": "assistant-1",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [{"type": "tool_use", "id": "toolu_123", "name": "Bash", "input": {"command": "ls"}}]
		}
	}`)
	toolEvents, err := parser.OnLine(context.Background(), stream, toolUseLine)
	require.NoError(t, err)
	require.Len(t, toolEvents, 2)
	assert.Equal(t, types.KindTool, toolEvents[1].Kind)
	assert.Equal(t, types.LevelInfo, toolEvents[1].Level)

	thinkingLine := []byte(`{
		"type": "assistant",
		"uuid": "assistant-2",
		"timestamp": "2025-01-15T10:30:01.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [{"type": "thinking", "thinking": "working"}]
		}
	}`)
	thinkingEvents, err := parser.OnLine(context.Background(), stream, thinkingLine)
	require.NoError(t, err)
	require.Len(t, thinkingEvents, 1)
	assert.Equal(t, types.KindMessage, thinkingEvents[0].Kind)
	assert.Equal(t, types.LevelInfo, thinkingEvents[0].Level)

	errorLine := []byte(`{
		"type": "system",
		"uuid": "system-1",
		"timestamp": "2025-01-15T10:30:02.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "failure"}]
		}
	}`)
	errorEvents, err := parser.OnLine(context.Background(), stream, errorLine)
	require.NoError(t, err)
	require.Len(t, errorEvents, 1)
	assert.Equal(t, types.KindError, errorEvents[0].Kind)
	assert.Equal(t, types.LevelError, errorEvents[0].Level)
}

func TestClaudeParser_SequenceMonotonicPerSession(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "session.jsonl", StreamType: "fs-jsonl"}

	line1 := []byte(`{
		"type": "user",
		"uuid": "user-1",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "hello"}]
		}
	}`)
	line2 := []byte(`{
		"type": "assistant",
		"uuid": "assistant-1",
		"timestamp": "2025-01-15T10:30:01.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [{"type": "text", "text": "hi"}]
		}
	}`)

	events1, err := parser.OnLine(context.Background(), stream, line1)
	require.NoError(t, err)
	events2, err := parser.OnLine(context.Background(), stream, line2)
	require.NoError(t, err)

	sequences := []int64{events1[0].Sequence, events1[1].Sequence, events2[0].Sequence}
	assert.True(t, sequences[0] < sequences[1] && sequences[1] < sequences[2], "sequences must be strictly increasing")
}

func TestClaudeParser_AgentFileGetsParentSessionID(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})

	// First: send a line from the main session file to establish mainSessionID
	mainStream := types.StreamID{Name: "88ad3a71-1234-5678-9abc-71a7b027ee63.jsonl", StreamType: "fs-jsonl"}
	mainLine := []byte(`{
		"type": "user",
		"uuid": "user-1",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "main-session-id",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "hello"}]
		}
	}`)

	mainEvents, err := parser.OnLine(context.Background(), mainStream, mainLine)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(mainEvents), 1)

	// Main session events should NOT have ParentSessionID
	for _, e := range mainEvents {
		assert.Empty(t, e.ParentSessionID, "Main session events should not have ParentSessionID")
		assert.False(t, e.IsSidechain, "Main session events should not be sidechain")
	}

	// Now: send a line from an agent file
	agentStream := types.StreamID{Name: "agent-abc123.jsonl", StreamType: "fs-jsonl"}
	agentLine := []byte(`{
		"type": "assistant",
		"uuid": "assistant-agent-1",
		"timestamp": "2025-01-15T10:30:01.000Z",
		"sessionId": "agent-session-id",
		"message": {
			"role": "assistant",
			"content": [{"type": "text", "text": "working on it"}]
		}
	}`)

	agentEvents, err := parser.OnLine(context.Background(), agentStream, agentLine)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(agentEvents), 1)

	// Agent file events should have ParentSessionID = main session ID
	for _, e := range agentEvents {
		assert.Equal(t, "main-session-id", e.ParentSessionID,
			"Agent file events should have ParentSessionID set to main session")
		assert.True(t, e.IsSidechain, "Agent file events should be sidechain")
	}

	// Verify the session_start event for the agent also has ParentSessionID
	assert.Equal(t, types.EventTypeSessionStart, agentEvents[0].EventType)
	assert.Equal(t, "main-session-id", agentEvents[0].ParentSessionID)
}

func TestClaudeParser_MainSessionDoesNotGetParentSessionID(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})

	// Send multiple lines from a regular (non-agent) file
	stream := types.StreamID{Name: "session.jsonl", StreamType: "fs-jsonl"}

	line1 := []byte(`{
		"type": "user",
		"uuid": "user-1",
		"timestamp": "2025-01-15T10:30:00.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "user",
			"content": [{"type": "text", "text": "hello"}]
		}
	}`)
	line2 := []byte(`{
		"type": "assistant",
		"uuid": "assistant-1",
		"timestamp": "2025-01-15T10:30:01.000Z",
		"sessionId": "session-123",
		"message": {
			"role": "assistant",
			"content": [{"type": "text", "text": "hi"}]
		}
	}`)

	events1, err := parser.OnLine(context.Background(), stream, line1)
	require.NoError(t, err)
	events2, err := parser.OnLine(context.Background(), stream, line2)
	require.NoError(t, err)

	allEvents := append(events1, events2...)
	for _, e := range allEvents {
		assert.Empty(t, e.ParentSessionID, "Regular file events should never have ParentSessionID")
		assert.False(t, e.IsSidechain, "Regular file events should not be sidechain")
	}
}

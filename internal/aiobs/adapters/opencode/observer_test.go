// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package opencode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

func TestOpenCodeParser_SessionCreated(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	line := []byte(`{"type":"session.created","properties":{"sessionId":"sess-abc","id":"sess-abc"}}`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, types.EventTypeSessionStart, events[0].EventType)
	assert.Equal(t, types.KindLifecycle, events[0].Kind)
	assert.Equal(t, types.LevelInfo, events[0].Level)
	assert.Equal(t, "sess-abc", events[0].SessionID)
	assert.Equal(t, "opencode-events", events[0].SourceFile)
	assert.Equal(t, "OpenCode session created", events[0].ContentPreview)
}

func TestOpenCodeParser_SessionUpdated_WithContent(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	line := []byte(`{"type":"session.updated","properties":{"sessionId":"sess-abc","content":"Hello, world!","model":"gpt-4"}}`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, types.EventTypeAIOutput, events[0].EventType)
	assert.Equal(t, types.KindMessage, events[0].Kind)
	assert.Equal(t, "sess-abc", events[0].SessionID)
	assert.Equal(t, "Hello, world!", events[0].ContentPreview)
	assert.Equal(t, 13, events[0].ContentLength)
	assert.Equal(t, "gpt-4", events[0].Model)
}

func TestOpenCodeParser_SessionError(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	line := []byte(`{"type":"session.error","properties":{"sessionId":"sess-abc","error":"rate limit exceeded"}}`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, types.EventTypeError, events[0].EventType)
	assert.Equal(t, types.KindError, events[0].Kind)
	assert.Equal(t, types.LevelError, events[0].Level)
	assert.Equal(t, "sess-abc", events[0].SessionID)
	assert.Equal(t, "rate limit exceeded", events[0].ContentPreview)
	assert.Equal(t, "rate limit exceeded", events[0].ToolError)
}

func TestOpenCodeParser_SessionCompleted(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	line := []byte(`{"type":"session.completed","properties":{"sessionId":"sess-abc"}}`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, types.EventTypeSessionEnd, events[0].EventType)
	assert.Equal(t, types.KindLifecycle, events[0].Kind)
	assert.Equal(t, "sess-abc", events[0].SessionID)
}

func TestOpenCodeParser_ToolStartAndResult(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	// Tool start
	startLine := []byte(`{"type":"tool.start","properties":{"sessionId":"sess-abc","name":"bash","input":"ls -la"}}`)
	events, err := parser.OnLine(context.Background(), stream, startLine)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, types.EventTypeToolUse, events[0].EventType)
	assert.Equal(t, types.KindTool, events[0].Kind)
	assert.Equal(t, "bash", events[0].ToolName)
	assert.Equal(t, "ls -la", events[0].ToolInputSummary)

	// Tool result
	resultLine := []byte(`{"type":"tool.result","properties":{"sessionId":"sess-abc","name":"bash","success":true}}`)
	events, err = parser.OnLine(context.Background(), stream, resultLine)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, types.EventTypeToolResult, events[0].EventType)
	assert.Equal(t, types.KindTool, events[0].Kind)
	assert.Equal(t, "bash", events[0].ToolName)
	assert.NotNil(t, events[0].ToolSuccess)
	assert.True(t, *events[0].ToolSuccess)
}

func TestOpenCodeParser_SessionIDFallsBackToID(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	// Event with only "id" field, no "sessionId"
	line := []byte(`{"type":"session.created","properties":{"id":"fallback-id"}}`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.Equal(t, "fallback-id", events[0].SessionID)
}

func TestOpenCodeParser_SequenceMonotonicPerSession(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	lines := [][]byte{
		[]byte(`{"type":"session.created","properties":{"sessionId":"sess-1"}}`),
		[]byte(`{"type":"session.updated","properties":{"sessionId":"sess-1","content":"a"}}`),
		[]byte(`{"type":"session.created","properties":{"sessionId":"sess-2"}}`),
		[]byte(`{"type":"session.updated","properties":{"sessionId":"sess-1","content":"b"}}`),
		[]byte(`{"type":"session.updated","properties":{"sessionId":"sess-2","content":"c"}}`),
	}

	sess1Sequences := []int64{}
	sess2Sequences := []int64{}

	for _, line := range lines {
		events, err := parser.OnLine(context.Background(), stream, line)
		require.NoError(t, err)
		for _, e := range events {
			if e.SessionID == "sess-1" {
				sess1Sequences = append(sess1Sequences, e.Sequence)
			} else if e.SessionID == "sess-2" {
				sess2Sequences = append(sess2Sequences, e.Sequence)
			}
		}
	}

	// Verify monotonic ordering within each session
	for i := 1; i < len(sess1Sequences); i++ {
		assert.Greater(t, sess1Sequences[i], sess1Sequences[i-1],
			"sess-1 sequences must be strictly increasing")
	}
	for i := 1; i < len(sess2Sequences); i++ {
		assert.Greater(t, sess2Sequences[i], sess2Sequences[i-1],
			"sess-2 sequences must be strictly increasing")
	}
}

func TestOpenCodeParser_UnknownEventType(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	line := []byte(`{"type":"unknown.event","properties":{"sessionId":"sess-abc"}}`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err)
	assert.Empty(t, events, "Unknown event types should produce no events")
}

func TestOpenCodeParser_InvalidJSON(t *testing.T) {
	observer := NewObserver()
	parser := observer.NewParser(types.RunContext{})
	stream := types.StreamID{Name: "opencode-events", StreamType: "sse"}

	line := []byte(`not valid json`)

	events, err := parser.OnLine(context.Background(), stream, line)
	require.NoError(t, err) // Should not error, just skip
	assert.Empty(t, events)
}

func TestOpenCodeObserver_Discover(t *testing.T) {
	observer := NewObserver()
	spec, err := observer.Discover(context.Background(), types.RunContext{})
	require.NoError(t, err)
	require.Len(t, spec.Streams, 1)
	assert.Equal(t, "opencode-events", spec.Streams[0].Name)
	assert.Equal(t, "sse", spec.Streams[0].Type)
}

func TestOpenCodeObserver_DiscoverWithEnvVar(t *testing.T) {
	t.Setenv(envOpenCodeURL, "http://custom:4000/events")
	observer := NewObserver()
	spec, err := observer.Discover(context.Background(), types.RunContext{})
	require.NoError(t, err)
	require.Len(t, spec.Streams, 1)
	assert.Equal(t, "http://custom:4000/events", spec.Streams[0].Root)
}

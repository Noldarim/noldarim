// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package types defines the core types for the AI observability system.
// This package is designed to have no dependencies to avoid import cycles.
package types

import (
	"encoding/json"
	"time"
)

// RawEntry is the unparsed transcript line with minimal extraction for routing.
type RawEntry struct {
	Line      int             // Line number in the file
	Data      json.RawMessage // Raw JSON data
	SessionID string          // Extracted session ID for routing
}

// ObsKind is the stable category of an observability event.
// New event types can be added without changing this enum.
// The UI uses ObsKind for default rendering (icon, color, collapsibility).
type ObsKind string

const (
	KindMessage   ObsKind = "message"   // user_prompt, ai_output, thinking
	KindTool      ObsKind = "tool"      // tool_use, tool_result, tool_blocked
	KindLifecycle ObsKind = "lifecycle" // session_start, session_end, subagent_start/stop
	KindError     ObsKind = "error"     // error events
	KindMetric    ObsKind = "metric"    // token usage summaries, timing
)

// ObsLevel represents the severity/importance of an observability event.
type ObsLevel string

const (
	LevelDebug ObsLevel = "debug" // streaming, queue-operation
	LevelInfo  ObsLevel = "info"  // user_prompt, ai_output, tool_use, tool_result
	LevelWarn  ObsLevel = "warn"  // tool_blocked, stop
	LevelError ObsLevel = "error" // error events
)

// ParsedEvent is the normalized output from any adapter.
// This is the canonical format for all AI transcript events, regardless of source.
type ParsedEvent struct {
	// Identity
	EventID   string `json:"event_id"`   // Unique ID for this event
	SessionID string `json:"session_id"` // AI session ID

	// Conversation structure (from transcript)
	MessageUUID string `json:"message_uuid,omitempty"` // Unique message ID from transcript
	ParentUUID  string `json:"parent_uuid,omitempty"`  // Links to parent message
	RequestID   string `json:"request_id,omitempty"`   // Groups streaming chunks (assistant only)

	// Classification
	EventType    string    `json:"event_type"`     // user_prompt, thinking, ai_output, tool_use, tool_result
	Kind         ObsKind   `json:"kind"`           // Stable event category (message, tool, lifecycle, error, metric)
	Level        ObsLevel  `json:"level"`          // Event severity (debug, info, warn, error)
	IsHumanInput bool      `json:"is_human_input"` // true = human typed, false = tool_result or AI
	Timestamp    time.Time `json:"timestamp"`
	Sequence     int64     `json:"sequence"` // Monotonic sequence number within session

	// Model info (from assistant entries)
	Model      string `json:"model,omitempty"`       // e.g., "claude-opus-4-5-20251101"
	StopReason string `json:"stop_reason,omitempty"` // "tool_use", "end_turn", null

	// Token usage
	InputTokens       int `json:"input_tokens,omitempty"`
	OutputTokens      int `json:"output_tokens,omitempty"`
	CacheReadTokens   int `json:"cache_read_tokens,omitempty"`
	CacheCreateTokens int `json:"cache_create_tokens,omitempty"`

	// Tool info (for tool_use and tool_result events)
	ToolName         string `json:"tool_name,omitempty"`
	ToolInputSummary string `json:"tool_input_summary,omitempty"` // Human-readable truncated
	ToolSuccess      *bool  `json:"tool_success,omitempty"`       // nil if not applicable
	ToolError        string `json:"tool_error,omitempty"`
	FilePath         string `json:"file_path,omitempty"` // Extracted for file operations

	IsSidechain     bool   `json:"is_sidechain,omitempty"`
	AgentID         string `json:"agent_id,omitempty"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
	SourceFile      string `json:"source_file,omitempty"`

	// Content
	ContentPreview string `json:"content_preview,omitempty"` // First 500 chars
	ContentLength  int    `json:"content_length,omitempty"`  // Full content length

	// Raw data for debugging
	RawPayload json.RawMessage `json:"raw_payload,omitempty"`
}

// Event type constants for ParsedEvent.EventType
// These mirror models.AIEventType values for consistency.
const (
	// Session events
	EventTypeSessionStart = "session_start"
	EventTypeSessionEnd   = "session_end"

	// User events
	EventTypeUserPrompt = "user_prompt"

	// AI processing events
	EventTypeThinking  = "thinking"
	EventTypeAIOutput  = "ai_output"
	EventTypeStreaming = "streaming"

	// Tool events
	EventTypeToolUse     = "tool_use"
	EventTypeToolResult  = "tool_result"
	EventTypeToolBlocked = "tool_blocked"

	// Status events
	EventTypeError = "error"
	EventTypeStop  = "stop"

	// Subagent events
	EventTypeSubagentStart = "subagent_start"
	EventTypeSubagentStop  = "subagent_stop"
)

// EventKindMap maps event types to their ObsKind for classification.
var EventKindMap = map[string]ObsKind{
	EventTypeSessionStart:  KindLifecycle,
	EventTypeSessionEnd:    KindLifecycle,
	EventTypeUserPrompt:    KindMessage,
	EventTypeThinking:      KindMessage,
	EventTypeAIOutput:      KindMessage,
	EventTypeStreaming:     KindMessage,
	EventTypeToolUse:       KindTool,
	EventTypeToolResult:    KindTool,
	EventTypeToolBlocked:   KindTool,
	EventTypeError:         KindError,
	EventTypeStop:          KindLifecycle,
	EventTypeSubagentStart: KindLifecycle,
	EventTypeSubagentStop:  KindLifecycle,
}

// EventLevelMap maps event types to their ObsLevel for filtering.
var EventLevelMap = map[string]ObsLevel{
	EventTypeSessionStart:  LevelInfo,
	EventTypeSessionEnd:    LevelInfo,
	EventTypeUserPrompt:    LevelInfo,
	EventTypeThinking:      LevelInfo,
	EventTypeAIOutput:      LevelInfo,
	EventTypeStreaming:     LevelDebug,
	EventTypeToolUse:       LevelInfo,
	EventTypeToolResult:    LevelInfo,
	EventTypeToolBlocked:   LevelWarn,
	EventTypeError:         LevelError,
	EventTypeStop:          LevelInfo,
	EventTypeSubagentStart: LevelInfo,
	EventTypeSubagentStop:  LevelInfo,
}

// KindForEvent returns the ObsKind for a given event type, defaulting to KindMessage.
func KindForEvent(eventType string) ObsKind {
	if kind, ok := EventKindMap[eventType]; ok {
		return kind
	}
	return KindMessage
}

// LevelForEvent returns the ObsLevel for a given event type, defaulting to LevelInfo.
func LevelForEvent(eventType string) ObsLevel {
	if level, ok := EventLevelMap[eventType]; ok {
		return level
	}
	return LevelInfo
}

// Adapter parses AI tool transcripts into normalized events.
// Each AI tool (Claude, Gemini, Aider) has its own adapter implementation.
type Adapter interface {
	// Name returns adapter identifier (e.g., "claude", "aider")
	Name() string

	// ParseEntry converts one transcript line to events.
	// Returns multiple events because one entry can have multiple content blocks.
	ParseEntry(raw RawEntry) ([]ParsedEvent, error)
}

// ExtractSessionID extracts the sessionId field from a raw JSON payload.
// This is a common operation used across adapters and watchers for routing.
// Returns empty string if the field is not present or extraction fails.
func ExtractSessionID(raw json.RawMessage) string {
	var probe struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	return probe.SessionID
}

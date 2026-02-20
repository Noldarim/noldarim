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
	IsHumanInput bool      `json:"is_human_input"` // true = human typed, false = tool_result or AI
	Timestamp    time.Time `json:"timestamp"`

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

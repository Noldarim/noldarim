// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"time"
)

// AIEventType defines the type of AI activity event
type AIEventType string

const (
	// Session events
	AIEventSessionStart AIEventType = "session_start"
	AIEventSessionEnd   AIEventType = "session_end"

	// Tool events
	AIEventToolUse     AIEventType = "tool_use"     // Before tool execution
	AIEventToolResult  AIEventType = "tool_result"  // After tool execution
	AIEventToolBlocked AIEventType = "tool_blocked" // Tool was blocked by user/policy

	// Processing events
	AIEventThinking  AIEventType = "thinking"  // AI thinking/reasoning
	AIEventAIOutput  AIEventType = "ai_output" // AI text output (canonical name)
	AIEventStreaming AIEventType = "streaming" // Partial streaming output

	// Status events
	AIEventError AIEventType = "error"
	AIEventStop  AIEventType = "stop" // AI stopped (completed or interrupted)

	// Subagent events
	AIEventSubagentStart AIEventType = "subagent_start"
	AIEventSubagentStop  AIEventType = "subagent_stop"

	// User events
	AIEventUserPrompt AIEventType = "user_prompt"
)

// GenerateEventID creates a unique event ID
func GenerateEventID() string {
	return time.Now().Format("20060102150405.000000000")
}

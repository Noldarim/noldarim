// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package adapters provides source-specific parsers for AI event data.
package adapters

import (
	"github.com/noldarim/noldarim/internal/aiobs/types"
)

// Re-export types from the types package for convenience.
// This allows code to import just the adapters package.
type (
	RawEntry    = types.RawEntry
	ParsedEvent = types.ParsedEvent
	Adapter     = types.Adapter
)

// Re-export event type constants
const (
	EventTypeSessionStart  = types.EventTypeSessionStart
	EventTypeSessionEnd    = types.EventTypeSessionEnd
	EventTypeUserPrompt    = types.EventTypeUserPrompt
	EventTypeThinking      = types.EventTypeThinking
	EventTypeAIOutput      = types.EventTypeAIOutput
	EventTypeStreaming     = types.EventTypeStreaming
	EventTypeToolUse       = types.EventTypeToolUse
	EventTypeToolResult    = types.EventTypeToolResult
	EventTypeToolBlocked   = types.EventTypeToolBlocked
	EventTypeError         = types.EventTypeError
	EventTypeStop          = types.EventTypeStop
	EventTypeSubagentStart = types.EventTypeSubagentStart
	EventTypeSubagentStop  = types.EventTypeSubagentStop
)

// ExtractSessionID re-exports the helper for extracting sessionId from raw JSON.
var ExtractSessionID = types.ExtractSessionID

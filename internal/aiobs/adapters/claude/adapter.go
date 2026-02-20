// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package claude provides an adapter for parsing Claude Code transcript.jsonl files.
package claude

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/logger"
)

var (
	log     *zerolog.Logger
	logOnce sync.Once
)

func getLog() *zerolog.Logger {
	logOnce.Do(func() {
		l := logger.GetAIObsLogger()
		log = &l
	})
	return log
}

// Adapter implements the types.Adapter interface for Claude Code transcripts.
type Adapter struct{}

// New creates a new Claude adapter instance.
func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Name() string {
	return "claude"
}

// ParseEntry converts a raw Claude transcript entry to ParsedEvents.
// One entry can produce multiple events (e.g., thinking + tool_use in same message).
func (a *Adapter) ParseEntry(raw types.RawEntry) ([]types.ParsedEvent, error) {
	var entry TranscriptEntry
	if err := json.Unmarshal(raw.Data, &entry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transcript entry: %w", err)
	}

	// Parse timestamp once
	timestamp := parseTimestamp(entry.Timestamp)

	// Extract common base fields for all events from this entry
	base := types.ParsedEvent{
		SessionID:   entry.SessionID,
		MessageUUID: entry.UUID,
		ParentUUID:  entry.ParentUUID,
		RequestID:   entry.RequestID,
		Timestamp:   timestamp,
		RawPayload:  raw.Data,
	}

	// Extract model and usage info if present (for assistant messages)
	if entry.Message != nil {
		base.Model = entry.Message.Model
		if entry.Message.StopReason != nil {
			base.StopReason = *entry.Message.StopReason
		}
	}

	// Get usage from either entry level or message level
	usage := entry.Usage
	if usage == nil && entry.Message != nil {
		usage = entry.Message.Usage
	}
	if usage != nil {
		base.InputTokens = usage.InputTokens
		base.OutputTokens = usage.OutputTokens
		base.CacheReadTokens = usage.CacheReadInputTokens
		base.CacheCreateTokens = usage.CacheCreationInputTokens
	}

	// Route based on entry type
	switch entry.Type {
	case "user":
		return a.parseUserEntry(entry, base)
	case "assistant":
		return a.parseAssistantEntry(entry, base)
	case "summary":
		return a.parseSummaryEntry(entry, base)
	case "system":
		return a.parseSystemEntry(entry, base)
	default:
		// Skip unknown entry types (e.g., "queue-operation", "file-history-snapshot")
		return nil, nil
	}
}

// parseUserEntry handles user entries - either human prompts or tool results.
func (a *Adapter) parseUserEntry(entry TranscriptEntry, base types.ParsedEvent) ([]types.ParsedEvent, error) {
	// Check if this is a tool result wrapped in user message.
	// Claude sends tool outputs back as user messages with toolUseResult field.
	if entry.ToolUseResult != nil {
		return a.parseToolUseResultField(entry.ToolUseResult, base)
	}

	// Check message content for tool_result type items
	if entry.Message != nil {
		for _, item := range entry.Message.Content {
			if item.Type == "tool_result" {
				return a.parseToolResultContent(item, base)
			}
		}
	}

	// This is an actual human prompt
	base.EventID = generateEventID()
	base.EventType = types.EventTypeUserPrompt
	base.IsHumanInput = true

	// Extract content preview
	content := extractTextContent(entry.Message)
	base.ContentPreview = truncateString(content, 500)
	base.ContentLength = len(content)

	return []types.ParsedEvent{base}, nil
}

// parseAssistantEntry handles assistant entries - thinking, text output, or tool use.
func (a *Adapter) parseAssistantEntry(entry TranscriptEntry, base types.ParsedEvent) ([]types.ParsedEvent, error) {
	if entry.Message == nil {
		return nil, nil
	}

	var events []types.ParsedEvent

	// Process each content block - one entry can have multiple blocks
	for _, item := range entry.Message.Content {
		event := base // Copy base for each event
		event.EventID = generateEventID()
		event.IsHumanInput = false

		switch item.Type {
		case "thinking":
			event.EventType = types.EventTypeThinking
			event.ContentPreview = truncateString(item.Thinking, 500)
			event.ContentLength = len(item.Thinking)
			events = append(events, event)

		case "text":
			event.EventType = types.EventTypeAIOutput
			event.ContentPreview = truncateString(item.Text, 500)
			event.ContentLength = len(item.Text)
			events = append(events, event)

		case "tool_use":
			event.EventType = types.EventTypeToolUse
			event.ToolName = item.Name
			event.ToolInputSummary = extractToolInputSummary(item.Name, item.Input)
			event.FilePath = extractFilePath(item.Name, item.Input)

			// For tool_use, content preview shows the input summary
			inputJSON, _ := json.Marshal(item.Input)
			event.ContentPreview = truncateString(string(inputJSON), 500)
			event.ContentLength = len(inputJSON)
			events = append(events, event)

		case "tool_result":
			// Tool results in assistant entries (less common)
			resultEvents, err := a.parseToolResultContent(item, base)
			if err != nil {
				getLog().Warn().Err(err).Str("sessionID", base.SessionID).Msg("failed to parse tool_result content block, skipping")
				continue
			}
			events = append(events, resultEvents...)
		}
	}

	// If no specific content blocks, treat as generic output
	if len(events) == 0 {
		base.EventID = generateEventID()
		base.EventType = types.EventTypeAIOutput
		base.IsHumanInput = false
		content := extractTextContent(entry.Message)
		base.ContentPreview = truncateString(content, 500)
		base.ContentLength = len(content)
		return []types.ParsedEvent{base}, nil
	}

	return events, nil
}

// parseToolResultContent handles tool_result content items.
// TODO(aiobs): ToolName is not set here because tool_result only contains tool_use_id,
// not the tool name. To correlate tool results with their tool names, we would need
// stateful parsing that tracks tool_use IDs from earlier in the conversation.
// For now, ToolName remains empty for tool results unless inferred from toolUseResult field.
func (a *Adapter) parseToolResultContent(item ContentItem, base types.ParsedEvent) ([]types.ParsedEvent, error) {
	event := base
	event.EventID = generateEventID()
	event.EventType = types.EventTypeToolResult
	event.IsHumanInput = false

	// Tool success/error
	success := !item.IsError
	event.ToolSuccess = &success
	if item.IsError {
		if content, ok := item.Content.(string); ok {
			event.ToolError = content
		}
	}

	// Extract content preview
	contentStr := ""
	if content, ok := item.Content.(string); ok {
		contentStr = content
	} else if item.Content != nil {
		if contentJSON, err := json.Marshal(item.Content); err == nil {
			contentStr = string(contentJSON)
		}
	}
	event.ContentPreview = truncateString(contentStr, 500)
	event.ContentLength = len(contentStr)

	return []types.ParsedEvent{event}, nil
}

// parseToolUseResultField handles the toolUseResult convenience field.
// Claude Code adds this to user entries with pre-parsed tool output metadata.
// The format varies by tool type - we detect the format by examining which fields are present.
func (a *Adapter) parseToolUseResultField(raw json.RawMessage, base types.ParsedEvent) ([]types.ParsedEvent, error) {
	event := base
	event.EventID = generateEventID()
	event.EventType = types.EventTypeToolResult
	event.IsHumanInput = false

	success := true
	event.ToolSuccess = &success

	// Try plain string first (error messages come as plain strings)
	var textContent string
	if json.Unmarshal(raw, &textContent) == nil {
		event.ContentPreview = truncateString(textContent, 500)
		event.ContentLength = len(textContent)
		return []types.ParsedEvent{event}, nil
	}

	// Parse as object
	var result ToolUseResult
	if err := json.Unmarshal(raw, &result); err != nil {
		// Failed to parse - show raw snippet as last resort
		event.ContentPreview = truncateString(string(raw), 100)
		event.ContentLength = len(raw)
		return []types.ParsedEvent{event}, nil
	}

	// Route by detected format (check fields, not just type)
	switch {
	case isBashResult(raw):
		// Bash result (has stdout/stderr fields, may be empty)
		event.ToolName = "Bash"
		output := result.Stdout
		if result.Stderr != "" {
			if output != "" {
				output += "\n" + result.Stderr
			} else {
				output = result.Stderr
			}
		}
		if output == "" {
			output = "(no output)"
		}
		event.ContentPreview = truncateString(output, 500)
		event.ContentLength = len(output)

	case result.Type == "text" && result.File != nil:
		// Read result
		event.ToolName = "Read"
		event.FilePath = result.File.FilePath
		event.ContentPreview = fmt.Sprintf("[%s] %d lines", result.File.FilePath, result.File.NumLines)
		event.ContentLength = len(result.File.Content)

	case result.Type == "create":
		// Write result (new file)
		event.ToolName = "Write"
		event.FilePath = result.FilePath
		event.ContentPreview = fmt.Sprintf("Created %s", result.FilePath)
		event.ContentLength = len(result.Content)

	case result.Type == "update":
		// Edit result
		event.ToolName = "Edit"
		event.FilePath = result.FilePath
		event.ContentPreview = fmt.Sprintf("Updated %s", result.FilePath)
		event.ContentLength = len(result.Content)

	case result.Type == "delete":
		// Delete result
		event.FilePath = result.FilePath
		event.ContentPreview = fmt.Sprintf("Deleted %s", result.FilePath)

	case result.NewTodos != nil || result.OldTodos != nil:
		// TodoWrite result
		event.ToolName = "TodoWrite"
		event.ContentPreview = fmt.Sprintf("Updated todos (%d items)", len(result.NewTodos))

	case result.Filenames != nil:
		// Glob result
		event.ToolName = "Glob"
		if result.NumFiles > 0 {
			event.ContentPreview = fmt.Sprintf("Found %d files", result.NumFiles)
		} else {
			event.ContentPreview = "No files found"
		}

	case result.Content != "":
		// Generic content fallback
		event.ContentPreview = truncateString(result.Content, 500)
		event.ContentLength = len(result.Content)

	default:
		// Last resort: show raw snippet
		event.ContentPreview = truncateString(string(raw), 100)
		event.ContentLength = len(raw)
	}

	return []types.ParsedEvent{event}, nil
}

// parseSummaryEntry handles session summary entries.
func (a *Adapter) parseSummaryEntry(entry TranscriptEntry, base types.ParsedEvent) ([]types.ParsedEvent, error) {
	event := base
	event.EventID = generateEventID()
	event.EventType = types.EventTypeSessionEnd
	event.IsHumanInput = false
	event.ContentPreview = truncateString(entry.Summary, 500)
	event.ContentLength = len(entry.Summary)

	return []types.ParsedEvent{event}, nil
}

// parseSystemEntry handles system/error entries.
func (a *Adapter) parseSystemEntry(entry TranscriptEntry, base types.ParsedEvent) ([]types.ParsedEvent, error) {
	event := base
	event.EventID = generateEventID()
	event.EventType = types.EventTypeError
	event.IsHumanInput = false

	content := extractTextContent(entry.Message)
	event.ContentPreview = truncateString(content, 500)
	event.ContentLength = len(content)

	return []types.ParsedEvent{event}, nil
}

// Helper functions

func parseTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Now()
	}
	// Try RFC3339Nano first (most common)
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t
	}
	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t
	}
	return time.Now()
}

func extractTextContent(msg *Message) string {
	if msg == nil {
		return ""
	}
	for _, item := range msg.Content {
		if item.Type == "text" && item.Text != "" {
			return item.Text
		}
	}
	return ""
}

func extractToolInputSummary(toolName string, input map[string]interface{}) string {
	if input == nil {
		return ""
	}

	// Tool-specific extraction for human-readable summary
	switch toolName {
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			return truncateString(cmd, 100)
		}
	case "Read":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Write":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Edit":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Glob":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			return pattern
		}
	case "WebFetch":
		if url, ok := input["url"].(string); ok {
			return url
		}
	case "WebSearch":
		if query, ok := input["query"].(string); ok {
			return query
		}
	case "Task":
		if prompt, ok := input["prompt"].(string); ok {
			if agentType, ok := input["subagent_type"].(string); ok {
				return "[" + agentType + "] " + truncateString(prompt, 80)
			}
			return truncateString(prompt, 100)
		}
	case "TodoWrite":
		return "[todo list update]"
	case "AskUserQuestion":
		if questions, ok := input["questions"].([]interface{}); ok && len(questions) > 0 {
			return fmt.Sprintf("[%d questions]", len(questions))
		}
	}

	// Fallback: look for common keys
	for _, key := range []string{"command", "path", "file_path", "pattern", "query", "url", "content"} {
		if val, ok := input[key].(string); ok {
			return truncateString(val, 100)
		}
	}

	return ""
}

func extractFilePath(toolName string, input map[string]interface{}) string {
	if input == nil {
		return ""
	}

	// Only extract file path for file-related tools
	switch toolName {
	case "Read", "Write", "Edit":
		if path, ok := input["file_path"].(string); ok {
			return path
		}
	case "Glob":
		if path, ok := input["path"].(string); ok {
			return path
		}
	case "Grep":
		if path, ok := input["path"].(string); ok {
			return path
		}
	}
	return ""
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// isBashResult checks if the raw JSON represents a Bash tool result.
// Bash results have stdout/stderr fields but no type field.
func isBashResult(raw json.RawMessage) bool {
	var probe struct {
		Stdout *string `json:"stdout"`
		Type   *string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	// Has stdout field but no type field = Bash result
	return probe.Stdout != nil && probe.Type == nil
}

// eventCounter provides uniqueness within the same nanosecond (thread-safe).
// The counter is masked to 16 bits (65535 max) since:
// 1. Nanosecond precision in timestamp provides primary uniqueness
// 2. 16 bits is sufficient for events within the same nanosecond
// 3. Keeps event IDs at reasonable length
var eventCounter atomic.Uint32

func generateEventID() string {
	count := eventCounter.Add(1)
	// Mask to 16 bits - combined with nanosecond timestamp, collision is negligible
	return fmt.Sprintf("%s-%04x", time.Now().Format("20060102150405.000000000"), count&0xFFFF)
}

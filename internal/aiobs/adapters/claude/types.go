// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"encoding/json"
	"fmt"
)

// TranscriptEntry represents a single entry in Claude Code's transcript.jsonl file.
type TranscriptEntry struct {
	// Common fields for all entry types
	Type        string `json:"type"`                  // "user", "assistant", "summary", "system", "queue-operation"
	UUID        string `json:"uuid"`                  // Unique message ID
	ParentUUID  string `json:"parentUuid,omitempty"`  // Links messages in conversation
	SessionID   string `json:"sessionId,omitempty"`   // Session identifier
	RequestID   string `json:"requestId,omitempty"`   // Groups assistant entries from same API call
	Timestamp   string `json:"timestamp"`             // ISO 8601 timestamp
	CWD         string `json:"cwd,omitempty"`         // Working directory
	Version     string `json:"version,omitempty"`     // Claude Code version
	IsSidechain bool   `json:"isSidechain,omitempty"` // Side conversation
	UserType    string `json:"userType,omitempty"`    // "external", "internal"

	// Message content (for user/assistant types)
	Message *Message `json:"message,omitempty"`

	// Tool use result (for user messages that are tool results)
	ToolUseResult json.RawMessage `json:"toolUseResult,omitempty"`

	// Summary fields (for summary type)
	Summary  string `json:"summary,omitempty"`
	LeafUUID string `json:"leafUuid,omitempty"`

	// Usage/cost info (for assistant messages)
	CostUSD    float64    `json:"costUSD,omitempty"`
	DurationMs int64      `json:"durationMs,omitempty"`
	Usage      *UsageInfo `json:"usage,omitempty"`
}

// Message represents a Claude message with role and content.
type Message struct {
	ID         string        `json:"id,omitempty"`
	Type       string        `json:"type,omitempty"` // "message"
	Role       string        `json:"role"`           // "user", "assistant"
	Model      string        `json:"model,omitempty"`
	Content    []ContentItem `json:"-"` // Custom unmarshaling handles both string and array
	StopReason *string       `json:"stop_reason,omitempty"`
	Usage      *UsageInfo    `json:"usage,omitempty"` // Token usage (sometimes at message level)
}

// UnmarshalJSON handles Claude's variable content format.
// Content can be either a string or an array of ContentItems.
func (m *Message) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type MessageAlias Message
	type messageWithRawContent struct {
		MessageAlias
		Content json.RawMessage `json:"content"`
	}

	var raw messageWithRawContent
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Copy non-content fields
	*m = Message(raw.MessageAlias)

	// Handle empty content
	if len(raw.Content) == 0 {
		m.Content = nil
		return nil
	}

	// Try to unmarshal as array first (most common case)
	var contentArray []ContentItem
	if err := json.Unmarshal(raw.Content, &contentArray); err == nil {
		m.Content = contentArray
		return nil
	}

	// Try to unmarshal as string
	var contentString string
	if err := json.Unmarshal(raw.Content, &contentString); err == nil {
		m.Content = []ContentItem{{Type: "text", Text: contentString}}
		return nil
	}

	// If neither works, return error with details
	return fmt.Errorf("content field is neither array nor string: %s", string(raw.Content[:min(100, len(raw.Content))]))
}

// ContentItem represents a single content block in a message.
type ContentItem struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result", "thinking", "image"

	// For text content
	Text string `json:"text,omitempty"`

	// For tool_use
	ID    string                 `json:"id,omitempty"`   // Tool use ID
	Name  string                 `json:"name,omitempty"` // Tool name
	Input map[string]interface{} `json:"input,omitempty"`

	// For tool_result
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"` // string or structured content
	IsError   bool        `json:"is_error,omitempty"`

	// For thinking
	Thinking string `json:"thinking,omitempty"`

	// For image
	Source *ImageSource `json:"source,omitempty"`
}

// ImageSource represents image data in a message.
type ImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g., "image/png"
	Data      string `json:"data"`       // Base64 encoded data
}

// UsageInfo contains token usage information.
type UsageInfo struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ToolUseResult represents the result of a tool execution in a user message.
// Claude Code adds this as a convenience field with pre-parsed metadata.
// The structure varies by tool type - we capture all known fields.
type ToolUseResult struct {
	// Generic type (absent for Bash results)
	Type string `json:"type,omitempty"` // "text", "create", "update", "delete"

	// Bash tool results (no type field present)
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
	IsImage     bool   `json:"isImage,omitempty"`

	// File operation results (Write, Edit)
	FilePath string `json:"filePath,omitempty"`
	Content  string `json:"content,omitempty"`

	// Read results
	File *FileResult `json:"file,omitempty"`

	// TodoWrite results
	OldTodos []interface{} `json:"oldTodos,omitempty"`
	NewTodos []interface{} `json:"newTodos,omitempty"`

	// Glob results
	Filenames []string `json:"filenames,omitempty"`
	NumFiles  int      `json:"numFiles,omitempty"`
}

// FileResult contains file content and metadata from Read tool.
type FileResult struct {
	Content    string `json:"content"`
	FilePath   string `json:"filePath"`
	NumLines   int    `json:"numLines"`
	StartLine  int    `json:"startLine"`
	TotalLines int    `json:"totalLines"`
}

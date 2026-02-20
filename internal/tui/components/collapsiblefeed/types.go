// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package collapsiblefeed

// GroupState represents the state of an activity group
type GroupState int

const (
	StatePending   GroupState = iota // Tool use initiated, waiting for result
	StateCompleted                   // Tool completed successfully
	StateFailed                      // Tool failed
)

// ActivityGroup pairs a tool_use with its eventual tool_result.
// This is the fundamental display unit - not individual events.
type ActivityGroup struct {
	ID       string     // Unique ID for matching (tool_use_id or sequence number)
	ToolName string     // "Read", "Write", "Bash", "TodoWrite", etc.
	Input    ToolInput  // Parsed tool input
	Result   *ToolResult // nil while pending
	State    GroupState
	Expanded bool // User toggled expansion
}

// ToolInput holds parsed input for different tool types
type ToolInput struct {
	// Common fields
	Raw string // Original input for fallback display

	// File operations (Read, Write, Edit, Glob)
	FilePath string

	// Bash specific
	Command     string
	Description string

	// TodoWrite - extracted for special handling
	Todos []TodoItem

	// Search operations (Grep, Glob)
	Pattern string

	// Generic preview
	Preview string
}

// ToolResult holds the outcome of a tool execution
type ToolResult struct {
	Success bool
	Error   string
	Output  string // Result preview/summary

	// Tool-specific result data
	LineCount int // For Read
	TodoCount int // For TodoWrite
}

// TodoItem represents a single todo entry from TodoWrite
type TodoItem struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status"` // "pending", "in_progress", "completed"
}

// TodoWriteInput is the JSON structure for TodoWrite tool input
type TodoWriteInput struct {
	Todos []TodoItem `json:"todos"`
}

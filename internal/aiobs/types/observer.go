// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import "context"

// RunContext provides context about the current pipeline run to the observer.
type RunContext struct {
	TaskID    string
	RunID     string
	ProjectID string
	WorkDir   string // Working directory inside the container
}

// StreamID identifies a specific transcript stream (e.g., a file).
type StreamID struct {
	Name       string // e.g., filename "88ad3a71-...-71a7b027ee63.jsonl"
	StreamType string // e.g., "fs-jsonl", "sse", "stdout"
}

// StreamSpec describes a transcript source to watch.
type StreamSpec struct {
	Name string // Human-readable name (e.g., "claude-transcripts")
	Type string // "fs-jsonl", "fs-glob", "sse", "stdout"
	Root string // Root directory or URL
	Glob string // For fs types: file pattern (e.g., "**/*.jsonl")
}

// DiscoverySpec describes all transcript sources for an agent runtime.
type DiscoverySpec struct {
	Streams []StreamSpec
}

// Observer is the observability contract for an agent runtime.
// Each runtime (Claude, OpenCode, etc.) provides an Observer that knows
// where to find transcript data and how to parse it.
type Observer interface {
	// RuntimeName returns the identifier for this runtime (e.g., "claude", "opencode")
	RuntimeName() string

	// Discover returns what transcript sources this runtime produces.
	// Called once per pipeline run to configure the watcher.
	Discover(ctx context.Context, run RunContext) (DiscoverySpec, error)

	// NewParser creates a stateful parser for a specific run.
	// The parser maintains internal state for tool correlation, session tracking, etc.
	NewParser(run RunContext) Parser
}

// Parser processes raw transcript lines into structured observability events.
// Parsers are stateful — they track tool correlations, session boundaries, etc.
// One Parser instance is created per pipeline run.
type Parser interface {
	// OnLine processes one raw line from a transcript stream.
	// Returns zero or more events. The parser can use internal state to correlate
	// events (e.g., tool_use → tool_result name resolution).
	OnLine(ctx context.Context, stream StreamID, line []byte) ([]ParsedEvent, error)

	// Flush emits any pending events (e.g., unclosed tool spans, session-end markers).
	// Called when the watcher is shutting down.
	Flush(ctx context.Context) ([]ParsedEvent, error)
}

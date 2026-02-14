// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package common provides shared types used across multiple packages.
package common

// Metadata contains common fields for all messages that interact with the UI.
// This includes Commands (UI → Orchestrator) and Events (Orchestrator → UI).
type Metadata struct {
	// TaskID serves as the correlation ID for task-related operations
	// Optional - only present for task-related commands/events
	TaskID string `json:"task_id,omitempty"`

	// IdempotencyKey is used for event deduplication to handle workflow retries
	// Optional - events without this key will always be processed
	IdempotencyKey string `json:"idempotency_key,omitempty"`

	// Version indicates the protocol version for backward compatibility.
	// Format: "v{major}.{minor}.{patch}" (e.g., "v1.0.0")
	Version string `json:"version"`
}

// CurrentProtocolVersion defines the current version of the protocol.
// This should be updated when making breaking changes to the protocol.
const CurrentProtocolVersion = "v1.0.0"

// Event represents events that can be sent from the orchestrator to the TUI.
// Any type implementing this interface can be sent through the event channel.
type Event interface {
	GetMetadata() Metadata
}

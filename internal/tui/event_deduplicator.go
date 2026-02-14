// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"sync"
	"time"

	"github.com/noldarim/noldarim/internal/protocol"
)

// EventDeduplicator handles deduplication of events at the TUI level
type EventDeduplicator struct {
	processedEvents sync.Map // idempotencyKey -> time.Time
	ttl             time.Duration
}

// NewEventDeduplicator creates a new event deduplicator
func NewEventDeduplicator() *EventDeduplicator {
	ed := &EventDeduplicator{
		processedEvents: sync.Map{},
		ttl:             10 * time.Minute, // Keep track of events for 10 minutes
	}

	// Start cleanup goroutine
	go ed.cleanupExpiredEvents()

	return ed
}

// ShouldProcess returns true if the event should be processed (not a duplicate)
func (ed *EventDeduplicator) ShouldProcess(event protocol.Event) bool {
	idempotencyKey := protocol.GetIdempotencyKey(event)
	if idempotencyKey == "" {
		// No idempotency key, always process
		return true
	}

	// Check if already processed
	if _, exists := ed.processedEvents.Load(idempotencyKey); exists {
		return false // Duplicate, skip
	}

	// Mark as processed
	ed.processedEvents.Store(idempotencyKey, time.Now())
	return true
}

// cleanupExpiredEvents periodically removes expired event records
func (ed *EventDeduplicator) cleanupExpiredEvents() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		ed.processedEvents.Range(func(key, value interface{}) bool {
			if timestamp, ok := value.(time.Time); ok {
				if now.Sub(timestamp) > ed.ttl {
					ed.processedEvents.Delete(key)
				}
			}
			return true
		})
	}
}

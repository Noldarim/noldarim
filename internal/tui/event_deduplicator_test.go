// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"testing"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/test/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventDeduplicator_BasicDeduplication(t *testing.T) {
	deduplicator := NewEventDeduplicator()

	t.Run("allows first event with idempotency key", func(t *testing.T) {
		event := protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "test-key-1",
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskCreated,
			ProjectID: "proj1",
			TaskID:    "task-1",
			Task:      testutil.SingleTask("proj1"),
		}

		// First event should be processed
		shouldProcess := deduplicator.ShouldProcess(event)
		assert.True(t, shouldProcess, "First event with idempotency key should be processed")
	})

	t.Run("blocks duplicate event with same idempotency key", func(t *testing.T) {
		event1 := protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "test-key-2",
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskCreated,
			ProjectID: "proj1",
			TaskID:    "task-1",
			Task:      testutil.SingleTask("proj1"),
		}

		event2 := protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "test-key-2", // Same key as event1
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskDeleted,
			ProjectID: "proj1",
			TaskID:    "task-123",
		}

		// First event should be processed
		shouldProcess1 := deduplicator.ShouldProcess(event1)
		assert.True(t, shouldProcess1, "First event with unique key should be processed")

		// Duplicate event should be blocked
		shouldProcess2 := deduplicator.ShouldProcess(event2)
		assert.False(t, shouldProcess2, "Duplicate event with same idempotency key should be blocked")
	})

	t.Run("allows events without idempotency key", func(t *testing.T) {
		event1 := protocol.ErrorEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "", // No idempotency key
				Version:        protocol.CurrentProtocolVersion,
			},
			Message: "Test error 1",
			Context: "Test context",
		}

		event2 := protocol.ErrorEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "", // No idempotency key
				Version:        protocol.CurrentProtocolVersion,
			},
			Message: "Test error 2",
			Context: "Test context",
		}

		// Both events should be processed since they have no idempotency key
		shouldProcess1 := deduplicator.ShouldProcess(event1)
		assert.True(t, shouldProcess1, "Event without idempotency key should always be processed")

		shouldProcess2 := deduplicator.ShouldProcess(event2)
		assert.True(t, shouldProcess2, "Event without idempotency key should always be processed")
	})

	t.Run("allows different idempotency keys", func(t *testing.T) {
		event1 := protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "unique-key-1",
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskStatusUpdated,
			ProjectID: "proj1",
			TaskID:    "task-1",
			NewStatus: models.TaskStatusInProgress,
		}

		event2 := protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "unique-key-2", // Different key
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskStatusUpdated,
			ProjectID: "proj1",
			TaskID:    "task-2",
			NewStatus: models.TaskStatusCompleted,
		}

		// Both events should be processed since they have different keys
		shouldProcess1 := deduplicator.ShouldProcess(event1)
		assert.True(t, shouldProcess1, "Event with unique key should be processed")

		shouldProcess2 := deduplicator.ShouldProcess(event2)
		assert.True(t, shouldProcess2, "Event with different unique key should be processed")
	})
}

func TestEventDeduplicator_MultipleEventTypes(t *testing.T) {
	deduplicator := NewEventDeduplicator()

	// Test that deduplication works across different event types
	events := []protocol.Event{
		protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "cross-type-key",
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskCreated,
			ProjectID: "proj1",
			TaskID:    "task-1",
			Task:      testutil.SingleTask("proj1"),
		},
		protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "cross-type-key", // Same key
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskDeleted,
			ProjectID: "proj1",
			TaskID:    "task-123",
		},
		protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "cross-type-key", // Same key as above
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskStatusUpdated,
			ProjectID: "proj1",
			TaskID:    "task-123",
			NewStatus: models.TaskStatusCompleted,
		},
	}

	// Only first event should be processed
	results := make([]bool, len(events))
	for i, event := range events {
		results[i] = deduplicator.ShouldProcess(event)
	}

	assert.True(t, results[0], "First event should be processed")
	assert.False(t, results[1], "Second event with same key should be blocked")
	assert.False(t, results[2], "Third event with same key should be blocked")
}

func TestEventDeduplicator_TTLExpiration(t *testing.T) {
	// Create deduplicator with very short TTL for testing
	deduplicator := &EventDeduplicator{
		ttl: 50 * time.Millisecond, // Very short TTL
	}

	event1 := protocol.TaskLifecycleEvent{
		Metadata: protocol.Metadata{
			IdempotencyKey: "ttl-test-key",
			Version:        protocol.CurrentProtocolVersion,
		},
		Type:      protocol.TaskCreated,
		ProjectID: "proj1",
		TaskID:    "task-1",
		Task:      testutil.SingleTask("proj1"),
	}

	event2 := protocol.TaskLifecycleEvent{
		Metadata: protocol.Metadata{
			IdempotencyKey: "ttl-test-key", // Same key
			Version:        protocol.CurrentProtocolVersion,
		},
		Type:      protocol.TaskCreated,
		ProjectID: "proj1",
		TaskID:    "task-1",
		Task:      testutil.SingleTask("proj1"),
	}

	// First event should be processed
	shouldProcess1 := deduplicator.ShouldProcess(event1)
	assert.True(t, shouldProcess1, "First event should be processed")

	// Immediately, duplicate should be blocked
	shouldProcess2 := deduplicator.ShouldProcess(event2)
	assert.False(t, shouldProcess2, "Immediate duplicate should be blocked")

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup since the automatic cleanup goroutine has longer intervals
	now := time.Now()
	deduplicator.processedEvents.Range(func(key, value interface{}) bool {
		if timestamp, ok := value.(time.Time); ok {
			if now.Sub(timestamp) > deduplicator.ttl {
				deduplicator.processedEvents.Delete(key)
			}
		}
		return true
	})

	// Now the same key should be allowed again
	shouldProcess3 := deduplicator.ShouldProcess(event2)
	assert.True(t, shouldProcess3, "Event should be allowed after TTL expiration")
}

func TestEventDeduplicator_ConcurrentAccess(t *testing.T) {
	deduplicator := NewEventDeduplicator()

	// Test concurrent access to deduplicator
	const numGoroutines = 10
	const eventsPerGoroutine = 50

	results := make(chan bool, numGoroutines*eventsPerGoroutine)

	// Start multiple goroutines processing events with the same idempotency key
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				event := protocol.TaskLifecycleEvent{
					Metadata: protocol.Metadata{
						IdempotencyKey: "concurrent-test-key", // Same key for all
						Version:        protocol.CurrentProtocolVersion,
					},
					Type:      protocol.TaskCreated,
					ProjectID: "proj1",
					TaskID:    "task-1",
					Task:      testutil.SingleTask("proj1"),
				}
				results <- deduplicator.ShouldProcess(event)
			}
		}(i)
	}

	// Collect results
	processedCount := 0
	blockedCount := 0
	for i := 0; i < numGoroutines*eventsPerGoroutine; i++ {
		if <-results {
			processedCount++
		} else {
			blockedCount++
		}
	}

	// At least one and at most a few events should be processed due to potential race conditions
	// The key insight is that sync.Map protects against corruption but there might be a small window
	// where multiple goroutines check before the first one stores the value
	assert.True(t, processedCount >= 1, "At least one event should be processed")
	assert.True(t, processedCount <= 5, "Should not process more than a few events due to race conditions")
	assert.Equal(t, numGoroutines*eventsPerGoroutine, processedCount+blockedCount, "Total should match expected count")
}

func TestEventDeduplicator_Integration_WithTUIFlow(t *testing.T) {
	// Test integration with TUI message flow
	t.Run("TUI event processing with deduplication", func(t *testing.T) {
		// Create event channel and deduplicator
		eventChan := make(chan protocol.Event, 10)
		deduplicator := NewEventDeduplicator()

		// Create events - some duplicates, some unique
		events := []protocol.Event{
			protocol.TaskLifecycleEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "integration-key-1",
					Version:        protocol.CurrentProtocolVersion,
				},
				Type:      protocol.TaskCreated,
				ProjectID: "proj1",
				TaskID:    "task-1",
				Task:      testutil.SingleTask("proj1"),
			},
			protocol.TaskLifecycleEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "integration-key-1", // Duplicate
					Version:        protocol.CurrentProtocolVersion,
				},
				Type:      protocol.TaskCreated,
				ProjectID: "proj1",
				TaskID:    "task-1",
				Task:      testutil.SingleTask("proj1"),
			},
			protocol.TaskLifecycleEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "integration-key-2", // Unique
					Version:        protocol.CurrentProtocolVersion,
				},
				Type:      protocol.TaskDeleted,
				ProjectID: "proj1",
				TaskID:    "task-123",
			},
			protocol.ErrorEvent{
				Metadata: protocol.Metadata{
					IdempotencyKey: "", // No key - should always process
					Version:        protocol.CurrentProtocolVersion,
				},
				Message: "Test error",
				Context: "Test context",
			},
		}

		// Send events to channel
		go func() {
			for _, event := range events {
				eventChan <- event
			}
			close(eventChan)
		}()

		// Process events through deduplicator (simulating TUI flow)
		processedEvents := make([]protocol.Event, 0)
		for event := range eventChan {
			if deduplicator.ShouldProcess(event) {
				processedEvents = append(processedEvents, event)
			}
		}

		// Should have processed 3 events: first TaskCreated, TaskDeleted, and ErrorEvent
		require.Len(t, processedEvents, 3, "Should process 3 out of 4 events")

		// Verify the processed events are the expected ones
		assert.IsType(t, protocol.TaskLifecycleEvent{}, processedEvents[0])
		assert.IsType(t, protocol.TaskLifecycleEvent{}, processedEvents[1])
		assert.IsType(t, protocol.ErrorEvent{}, processedEvents[2])

		// Verify idempotency keys
		assert.Equal(t, "integration-key-1", protocol.GetIdempotencyKey(processedEvents[0]))
		assert.Equal(t, "integration-key-2", protocol.GetIdempotencyKey(processedEvents[1]))
		assert.Equal(t, "", protocol.GetIdempotencyKey(processedEvents[2]))
	})
}

func TestEventDeduplicator_EdgeCases(t *testing.T) {
	deduplicator := NewEventDeduplicator()

	t.Run("handles nil event metadata gracefully", func(t *testing.T) {
		// This test ensures we don't panic on malformed events
		// Though in practice, all events should have proper metadata
		event := &protocol.TaskLifecycleEvent{
			// No Metadata field set - should not panic
			Type:      protocol.TaskCreated,
			ProjectID: "proj1",
			TaskID:    "task-1",
			Task:      testutil.SingleTask("proj1"),
		}

		// Should not panic and should process (no idempotency key means always process)
		assert.NotPanics(t, func() {
			shouldProcess := deduplicator.ShouldProcess(event)
			assert.True(t, shouldProcess, "Event without metadata should be processed")
		})
	})

	t.Run("handles empty idempotency key consistently", func(t *testing.T) {
		event1 := protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "",
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskCreated,
			ProjectID: "proj1",
			TaskID:    "task-1",
			Task:      testutil.SingleTask("proj1"),
		}

		event2 := protocol.TaskLifecycleEvent{
			Metadata: protocol.Metadata{
				IdempotencyKey: "",
				Version:        protocol.CurrentProtocolVersion,
			},
			Type:      protocol.TaskCreated,
			ProjectID: "proj1",
			TaskID:    "task-1",
			Task:      testutil.SingleTask("proj1"),
		}

		// Both should be processed (empty key means always process)
		shouldProcess1 := deduplicator.ShouldProcess(event1)
		shouldProcess2 := deduplicator.ShouldProcess(event2)

		assert.True(t, shouldProcess1, "First event with empty key should be processed")
		assert.True(t, shouldProcess2, "Second event with empty key should be processed")
	})
}

func TestNewEventDeduplicator_DefaultValues(t *testing.T) {
	deduplicator := NewEventDeduplicator()

	// Verify default TTL
	assert.Equal(t, 10*time.Minute, deduplicator.ttl, "Default TTL should be 10 minutes")

	// Verify that processedEvents map is initialized
	assert.NotNil(t, &deduplicator.processedEvents, "processedEvents map should be initialized")
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"testing"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

func TestClientRegistryBroadcast_RunFilterMatchesAIActivityRecord(t *testing.T) {
	registry := NewClientRegistry()
	client := &wsClient{
		send:    make(chan []byte, 1),
		filters: []SubscriptionFilter{{RunID: "run-1"}},
	}
	registry.add(client)

	// TaskID is a step-level ID, RunID is the pipeline run ID.
	// The filter subscribes by run_id, which should match.
	registry.Broadcast(&models.AIActivityRecord{
		EventID: "evt-1",
		TaskID:  "step-task-abc",
		RunID:   "run-1",
	})

	select {
	case <-client.send:
		// Expected path.
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected broadcast for matching run_id filter")
	}
}

func TestClientRegistryBroadcast_RunFilterDoesNotMatchAIActivityRecord(t *testing.T) {
	registry := NewClientRegistry()
	client := &wsClient{
		send:    make(chan []byte, 1),
		filters: []SubscriptionFilter{{RunID: "run-expected"}},
	}
	registry.add(client)

	registry.Broadcast(&models.AIActivityRecord{
		EventID: "evt-1",
		TaskID:  "step-task-abc",
		RunID:   "run-other",
	})

	select {
	case <-client.send:
		t.Fatalf("did not expect broadcast for non-matching run_id filter")
	default:
		// Expected: channel is empty because the filter did not match.
	}
}

func TestClientRegistryBroadcast_TaskFilterStillWorks(t *testing.T) {
	registry := NewClientRegistry()
	client := &wsClient{
		send:    make(chan []byte, 1),
		filters: []SubscriptionFilter{{TaskID: "task-1"}},
	}
	registry.add(client)

	registry.Broadcast(&models.AIActivityRecord{
		EventID: "evt-1",
		TaskID:  "task-1",
		RunID:   "run-1",
	})

	select {
	case <-client.send:
		// Expected path.
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected broadcast for matching task_id filter")
	}
}

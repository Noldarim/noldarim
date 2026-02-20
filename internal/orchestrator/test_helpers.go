// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestrator

import (
	"testing"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/database"
	"github.com/noldarim/noldarim/internal/protocol"

	"github.com/stretchr/testify/require"
)

// OrchestratorFixture represents an orchestrator setup with channels and cleanup
type OrchestratorFixture struct {
	Orchestrator *Orchestrator
	CmdChan      chan protocol.Command
	EventChan    chan protocol.Event
	Cleanup      func()
}

// WithOrchestrator sets up an orchestrator with the given config
func WithOrchestrator(t *testing.T, cfg *config.AppConfig) *OrchestratorFixture {
	cmdChan := make(chan protocol.Command, 10)
	eventChan := make(chan protocol.Event, 10)

	orch, err := New(cmdChan, eventChan, cfg)
	require.NoError(t, err, "Failed to create orchestrator")

	cleanup := func() {
		orch.Close()
		close(cmdChan)
		close(eventChan)
	}

	return &OrchestratorFixture{
		Orchestrator: orch,
		CmdChan:      cmdChan,
		EventChan:    eventChan,
		Cleanup:      cleanup,
	}
}

// WithInMemoryOrchestrator creates a complete orchestrator setup with in-memory database
func WithInMemoryOrchestrator(t *testing.T) *OrchestratorFixture {
	// Use the database fixture to create and migrate the database
	dbFixture := database.UseFreshInMemoryDatabase(t)

	cfg := database.WithInMemoryConfig()

	fixture := WithOrchestrator(t, cfg)

	// Wrap the cleanup to include database cleanup
	originalCleanup := fixture.Cleanup
	fixture.Cleanup = func() {
		originalCleanup()
		dbFixture.Cleanup()
	}

	return fixture
}

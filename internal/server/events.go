// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server provides a REST + WebSocket API. Handlers call PipelineService
// directly for mutations and broadcast resulting orchestrator events to
// connected WebSocket clients.
package server

import (
	"context"
	"sync"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/protocol"

	"github.com/rs/zerolog"
)

var (
	log     *zerolog.Logger
	logOnce sync.Once
)

func getLog() *zerolog.Logger {
	logOnce.Do(func() {
		l := logger.GetLogger("api")
		log = &l
	})
	return log
}

// EventBroadcaster reads every event from the orchestrator's eventChan and
// fans them out to all connected WebSocket clients.
type EventBroadcaster struct {
	eventChan <-chan protocol.Event
	clients   *ClientRegistry
}

// NewEventBroadcaster creates a broadcaster that fans out events from the
// orchestrator's event channel.
func NewEventBroadcaster(eventChan <-chan protocol.Event, clients *ClientRegistry) *EventBroadcaster {
	return &EventBroadcaster{
		eventChan: eventChan,
		clients:   clients,
	}
}

// Run reads events until the channel is closed or context is cancelled.
func (b *EventBroadcaster) Run(ctx context.Context) {
	for {
		select {
		case event, ok := <-b.eventChan:
			if !ok {
				getLog().Info().Msg("Event broadcaster stopped (channel closed)")
				return
			}
			b.dispatch(event)
		case <-ctx.Done():
			getLog().Info().Msg("Event broadcaster stopped (context cancelled)")
			return
		}
	}
}

func (b *EventBroadcaster) dispatch(event protocol.Event) {
	if b.clients != nil {
		b.clients.Broadcast(event)
	}
}

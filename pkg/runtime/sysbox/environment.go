// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package sysbox

import (
	"context"
	"fmt"
	"time"

	"github.com/noldarim/noldarim/pkg/containers"
	dockerclient "github.com/noldarim/noldarim/pkg/containers/docker"
	"github.com/noldarim/noldarim/pkg/runtime"
)

// environment is a Sysbox container running its own Docker daemon.
// The Backend connects to the inner daemon via TCP.
type environment struct {
	id          string
	containerID string
	dockerHost  string // tcp://localhost:<port>
	hostBackend containers.Backend
	backend     containers.Backend // inner Docker client, created lazily by WaitReady
	destroyed   bool
}

var _ runtime.Environment = (*environment)(nil)

func (e *environment) ID() string         { return e.id }
func (e *environment) DockerHost() string  { return e.dockerHost }

// ContainerBackend returns a Docker client connected to the inner daemon.
// WaitReady must be called first; panics if the backend is not yet initialized.
func (e *environment) ContainerBackend() containers.Backend {
	if e.backend == nil {
		panic("sysbox: ContainerBackend() called before WaitReady()")
	}
	return e.backend
}

// WaitReady polls the inner Docker daemon until it responds or the context
// is cancelled. Creates the Backend client on success.
func (e *environment) WaitReady(ctx context.Context) error {
	// Poll interval and overall timeout are governed by the context.
	// If the caller doesn't set a deadline, default to 60s.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	// Create the client once; retry only the ping (ListContainers).
	// If client creation fails it's a config error — retrying won't help.
	client, err := dockerclient.NewClientWithHost(e.dockerHost)
	if err != nil {
		return fmt.Errorf("sysbox: failed to create Docker client for %s: %w", e.dockerHost, err)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = client.Close()
			return fmt.Errorf("sysbox: inner Docker daemon not ready at %s: %w", e.dockerHost, ctx.Err())
		case <-ticker.C:
			if _, err := client.ListContainers(ctx); err != nil {
				continue
			}
			e.backend = client
			return nil
		}
	}
}

// Destroy stops and removes the Sysbox container. All agent containers
// running inside the environment's Docker are destroyed with it.
// Safe to call multiple times.
func (e *environment) Destroy(ctx context.Context) error {
	if e.destroyed {
		return nil
	}
	e.destroyed = true

	if e.backend != nil {
		_ = e.backend.Close()
		e.backend = nil
	}

	// Force-remove the Sysbox container (kills it if running).
	if err := e.hostBackend.RemoveContainer(ctx, e.containerID, true); err != nil {
		return fmt.Errorf("sysbox: failed to remove environment container %s: %w", e.containerID, err)
	}
	return nil
}

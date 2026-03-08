// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package runtime

import (
	"context"

	"github.com/noldarim/noldarim/pkg/containers"
)

// Provider name constants.
const (
	ProviderLocal  = "local"
	ProviderSysbox = "sysbox"
)

// Provider provisions isolated environments for pipeline execution.
type Provider interface {
	// Provision creates a new isolated environment.
	Provision(ctx context.Context, opts ProvisionOpts) (Environment, error)

	// Name returns the provider name (e.g., "local", "sysbox", "firecracker").
	Name() string

	// Close releases any provider-level resources.
	Close() error
}

// Environment represents a provisioned isolated execution environment.
type Environment interface {
	// ID returns the environment identifier.
	ID() string

	// ContainerBackend returns a client for managing containers inside this environment.
	ContainerBackend() containers.Backend

	// DockerHost returns the Docker socket/host address for this environment.
	DockerHost() string

	// WaitReady blocks until the environment is ready to accept work.
	WaitReady(ctx context.Context) error

	// Destroy tears down the environment and releases resources.
	Destroy(ctx context.Context) error
}

// ProvisionOpts configures environment provisioning.
type ProvisionOpts struct {
	// ID is a unique identifier for this environment (e.g., pipeline run ID).
	ID string
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package local

import (
	"context"

	"github.com/noldarim/noldarim/pkg/containers"
	"github.com/noldarim/noldarim/pkg/containers/docker"
	"github.com/noldarim/noldarim/pkg/runtime"
)

// Provider is a RuntimeProvider that uses the host Docker daemon directly.
// No isolation — this is the current behavior before runtime providers were introduced.
type Provider struct {
	dockerHost string
	backend    containers.Backend
}

var _ runtime.Provider = (*Provider)(nil)

// New creates a LocalProvider that will create a docker.Client on Provision.
// Pass backendOverride for testing; nil uses a real Docker client.
func New(dockerHost string, backendOverride containers.Backend) *Provider {
	return &Provider{
		dockerHost: dockerHost,
		backend:    backendOverride,
	}
}

// NewWithBackend creates a LocalProvider with an explicit backend (for testing).
func NewWithBackend(dockerHost string, backend containers.Backend) *Provider {
	return &Provider{
		dockerHost: dockerHost,
		backend:    backend,
	}
}

func (p *Provider) Name() string { return "local" }

func (p *Provider) Provision(_ context.Context, opts runtime.ProvisionOpts) (runtime.Environment, error) {
	backend := p.backend
	if backend == nil {
		client, err := docker.NewClientWithHost(p.dockerHost)
		if err != nil {
			return nil, err
		}
		backend = client
	}
	return &localEnvironment{
		id:         opts.ID,
		dockerHost: p.dockerHost,
		backend:    backend,
	}, nil
}

func (p *Provider) Close() error { return nil }

// localEnvironment is the host itself — no isolation boundary.
type localEnvironment struct {
	id         string
	dockerHost string
	backend    containers.Backend
}

var _ runtime.Environment = (*localEnvironment)(nil)

func (e *localEnvironment) ID() string                           { return e.id }
func (e *localEnvironment) ContainerBackend() containers.Backend { return e.backend }
func (e *localEnvironment) DockerHost() string                   { return e.dockerHost }
func (e *localEnvironment) WaitReady(_ context.Context) error    { return nil }
func (e *localEnvironment) Destroy(_ context.Context) error      { return nil }

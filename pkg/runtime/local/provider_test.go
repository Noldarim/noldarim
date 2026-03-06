// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package local

import (
	"context"
	"testing"

	"github.com/noldarim/noldarim/pkg/containers"
	"github.com/noldarim/noldarim/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend is a minimal containers.Backend for testing.
type mockBackend struct{ containers.Backend }

func (m *mockBackend) Close() error { return nil }

func TestLocalProvider_Name(t *testing.T) {
	p := New("unix:///var/run/docker.sock", nil)
	assert.Equal(t, "local", p.Name())
}

func TestLocalProvider_ImplementsProvider(t *testing.T) {
	var _ runtime.Provider = (*Provider)(nil)
}

func TestLocalProvider_Provision(t *testing.T) {
	backend := &mockBackend{}
	p := New("unix:///var/run/docker.sock", backend)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{
		ID: "test-env-1",
	})
	require.NoError(t, err)

	assert.Equal(t, "test-env-1", env.ID())
	assert.Equal(t, "unix:///var/run/docker.sock", env.DockerHost())
	assert.Same(t, backend, env.ContainerBackend())
}

func TestLocalProvider_WaitReady(t *testing.T) {
	backend := &mockBackend{}
	p := New("unix:///test.sock", backend)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "ready-test"})
	require.NoError(t, err)

	// Local environments are always ready immediately.
	err = env.WaitReady(context.Background())
	assert.NoError(t, err)
}

func TestLocalProvider_Destroy(t *testing.T) {
	backend := &mockBackend{}
	p := New("unix:///test.sock", backend)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "destroy-test"})
	require.NoError(t, err)

	// Destroy is a no-op for local — nothing to tear down.
	err = env.Destroy(context.Background())
	assert.NoError(t, err)
}

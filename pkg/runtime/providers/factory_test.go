// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider_Local(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Name:       "local",
		DockerHost: "unix:///var/run/docker.sock",
	})
	require.NoError(t, err)
	assert.Equal(t, "local", p.Name())
}

func TestNewProvider_EmptyDefaultsToLocal(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		DockerHost: "unix:///var/run/docker.sock",
	})
	require.NoError(t, err)
	assert.Equal(t, "local", p.Name())
}

func TestNewProvider_Sysbox(t *testing.T) {
	// Sysbox provider creation succeeds (creates host Docker client).
	// This test may fail if Docker is not available, which is expected
	// in CI without Docker.
	p, err := NewProvider(ProviderConfig{
		Name:       "sysbox",
		DockerHost: "unix:///var/run/docker.sock",
	})
	if err != nil {
		t.Skipf("Sysbox provider creation failed (Docker unavailable?): %v", err)
	}
	assert.Equal(t, "sysbox", p.Name())
	_ = p.Close()
}

func TestNewProvider_UnknownReturnsError(t *testing.T) {
	_, err := NewProvider(ProviderConfig{Name: "quantum-vm"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown runtime provider")
}

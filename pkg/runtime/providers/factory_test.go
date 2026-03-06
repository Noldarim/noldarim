// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider_Local(t *testing.T) {
	p, err := NewProvider("local", "unix:///var/run/docker.sock")
	require.NoError(t, err)
	assert.Equal(t, "local", p.Name())
}

func TestNewProvider_EmptyDefaultsToLocal(t *testing.T) {
	p, err := NewProvider("", "unix:///var/run/docker.sock")
	require.NoError(t, err)
	assert.Equal(t, "local", p.Name())
}

func TestNewProvider_UnknownReturnsError(t *testing.T) {
	_, err := NewProvider("quantum-vm", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown runtime provider")
}

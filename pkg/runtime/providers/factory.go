// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"fmt"

	"github.com/noldarim/noldarim/pkg/runtime"
	"github.com/noldarim/noldarim/pkg/runtime/local"
	"github.com/noldarim/noldarim/pkg/runtime/sysbox"
)

// ProviderConfig holds all fields needed by the provider factory.
// The orchestrator maps from AppConfig to this struct, keeping pkg/
// decoupled from internal/config.
type ProviderConfig struct {
	// Name selects the provider: "local" (default), "sysbox".
	Name string

	// DockerHost is the host Docker socket (e.g., "unix:///var/run/docker.sock").
	DockerHost string

	// SysboxImage is the Docker image for Sysbox environment containers.
	// Only used when Name == "sysbox". Default: "docker:27-dind".
	SysboxImage string

	// WorktreeBasePath is the absolute path to the worktree directory.
	// Mounted into Sysbox containers for code delivery.
	WorktreeBasePath string
}

// NewProvider creates a RuntimeProvider based on the config.
func NewProvider(cfg ProviderConfig) (runtime.Provider, error) {
	switch cfg.Name {
	case runtime.ProviderLocal, "":
		return local.New(cfg.DockerHost, nil), nil

	case runtime.ProviderSysbox:
		return sysbox.New(sysbox.Config{
			Image:            cfg.SysboxImage,
			HostDockerHost:   cfg.DockerHost,
			WorktreeBasePath: cfg.WorktreeBasePath,
		}, nil)

	default:
		return nil, fmt.Errorf("unknown runtime provider: %q", cfg.Name)
	}
}

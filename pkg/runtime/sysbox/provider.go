// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package sysbox

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/noldarim/noldarim/pkg/containers"
	dockerclient "github.com/noldarim/noldarim/pkg/containers/docker"
	"github.com/noldarim/noldarim/pkg/containers/models"
	"github.com/noldarim/noldarim/pkg/runtime"
)

// Config holds Sysbox provider configuration.
type Config struct {
	// Image is the Docker image for the Sysbox environment container.
	// Must support running a Docker daemon (e.g., "docker:27-dind").
	Image string

	// HostDockerHost is the Docker socket on the host, used to create
	// the Sysbox container itself.
	HostDockerHost string

	// WorktreeBasePath is the absolute path to the worktree directory.
	// Mounted into the Sysbox container so agent containers can bind-mount
	// individual worktrees at the same absolute paths.
	WorktreeBasePath string
}

// Provider is a RuntimeProvider that provisions Sysbox containers, each
// running its own Docker daemon. Agent containers created through the
// environment's Backend are fully isolated from the host Docker.
type Provider struct {
	cfg         Config
	hostBackend containers.Backend
	envs        map[string]*environment
	mu          sync.Mutex
}

var _ runtime.Provider = (*Provider)(nil)

// New creates a SysboxProvider. It connects to the host Docker daemon
// to create Sysbox containers. Pass a non-nil hostBackend for testing.
func New(cfg Config, hostBackend containers.Backend) (*Provider, error) {
	if hostBackend == nil {
		client, err := dockerclient.NewClientWithHost(cfg.HostDockerHost)
		if err != nil {
			return nil, fmt.Errorf("sysbox: failed to create host Docker client: %w", err)
		}
		hostBackend = client
	}

	if cfg.Image == "" {
		cfg.Image = "docker:27-dind"
	}

	return &Provider{
		cfg:         cfg,
		hostBackend: hostBackend,
		envs:        make(map[string]*environment),
	}, nil
}

func (p *Provider) Name() string { return runtime.ProviderSysbox }

// Provision creates a Sysbox container running a Docker daemon and returns
// an Environment whose Backend talks to the inner daemon.
func (p *Provider) Provision(ctx context.Context, opts runtime.ProvisionOpts) (runtime.Environment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find an ephemeral port for the inner Docker daemon.
	port, err := findFreePort()
	if err != nil {
		return nil, fmt.Errorf("sysbox: failed to find free port: %w", err)
	}

	containerName := fmt.Sprintf("noldarim-sysbox-%s", opts.ID)

	envConfig := models.ContainerConfig{
		Name:    containerName,
		Image:   p.cfg.Image,
		Runtime: "sysbox-runc",
		Environment: map[string]string{
			"DOCKER_TLS_CERTDIR": "", // Disable TLS; daemon listens on tcp://0.0.0.0:2375
		},
		Ports: []models.PortMapping{
			{
				HostPort:      port,
				ContainerPort: 2375,
				Protocol:      "tcp",
			},
		},
		Labels: map[string]string{
			"noldarim.sysbox.env": opts.ID,
			"noldarim.managed":    "true",
		},
	}

	// Mount worktree base path so agent container bind mounts resolve correctly.
	if p.cfg.WorktreeBasePath != "" {
		envConfig.Volumes = append(envConfig.Volumes, models.VolumeMapping{
			HostPath:      p.cfg.WorktreeBasePath,
			ContainerPath: p.cfg.WorktreeBasePath,
			ReadOnly:      false,
		})
	}

	ctr, err := p.hostBackend.CreateContainer(ctx, envConfig)
	if err != nil {
		return nil, fmt.Errorf("sysbox: failed to create environment container: %w", err)
	}

	if err := p.hostBackend.StartContainer(ctx, ctr.ID); err != nil {
		_ = p.hostBackend.RemoveContainer(ctx, ctr.ID, true)
		return nil, fmt.Errorf("sysbox: failed to start environment container: %w", err)
	}

	dockerHost := fmt.Sprintf("tcp://localhost:%d", port)

	env := &environment{
		id:          opts.ID,
		containerID: ctr.ID,
		dockerHost:  dockerHost,
		hostBackend: p.hostBackend,
	}

	p.envs[opts.ID] = env
	return env, nil
}

// Close destroys all provisioned environments and releases the host client.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	for _, env := range p.envs {
		if err := env.Destroy(context.Background()); err != nil {
			errs = append(errs, err)
		}
	}
	p.envs = make(map[string]*environment)

	if err := p.hostBackend.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("sysbox: errors during close: %v", errs)
	}
	return nil
}

// findFreePort asks the OS for an available TCP port.
func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

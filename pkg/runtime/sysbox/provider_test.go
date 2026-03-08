// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package sysbox

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/pkg/containers"
	"github.com/noldarim/noldarim/pkg/containers/models"
	"github.com/noldarim/noldarim/pkg/runtime"
)

// ---------------------------------------------------------------------------
// recordingBackend — simple mock that records calls and supports error injection
// ---------------------------------------------------------------------------

type recordingBackend struct {
	mu              sync.Mutex
	createdConfigs  []models.ContainerConfig
	startedIDs      []string
	removedIDs      []string
	nextContainerID int
	closed          bool

	// Error injection
	createErr error
	startErr  error
	removeErr error
}

var _ containers.Backend = (*recordingBackend)(nil)

func newRecordingBackend() *recordingBackend {
	return &recordingBackend{}
}

func (b *recordingBackend) CreateContainer(_ context.Context, config models.ContainerConfig) (*models.Container, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.createErr != nil {
		return nil, b.createErr
	}
	b.nextContainerID++
	id := fmt.Sprintf("container-%d", b.nextContainerID)
	b.createdConfigs = append(b.createdConfigs, config)
	return &models.Container{
		ID:     id,
		Name:   config.Name,
		Image:  config.Image,
		Status: models.StatusCreated,
	}, nil
}

func (b *recordingBackend) StartContainer(_ context.Context, containerID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.startErr != nil {
		return b.startErr
	}
	b.startedIDs = append(b.startedIDs, containerID)
	return nil
}

func (b *recordingBackend) RemoveContainer(_ context.Context, containerID string, _ bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.removeErr != nil {
		return b.removeErr
	}
	b.removedIDs = append(b.removedIDs, containerID)
	return nil
}

func (b *recordingBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

// Unused Backend methods — minimal stubs.
func (b *recordingBackend) StopContainer(context.Context, string, *time.Duration) error { return nil }
func (b *recordingBackend) InspectContainer(context.Context, string) (*models.Container, error) {
	return nil, fmt.Errorf("not found")
}
func (b *recordingBackend) ListContainers(context.Context) ([]*models.Container, error) {
	return nil, nil
}
func (b *recordingBackend) ListContainersByLabels(context.Context, map[string]string) ([]*models.Container, error) {
	return nil, nil
}
func (b *recordingBackend) KillContainer(context.Context, string) error                 { return nil }
func (b *recordingBackend) CopyToContainer(context.Context, string, string, string) error   { return nil }
func (b *recordingBackend) CopyFromContainer(context.Context, string, string, string) error { return nil }
func (b *recordingBackend) WriteToContainer(context.Context, string, string, string) error  { return nil }
func (b *recordingBackend) ExecContainer(context.Context, string, []string, string) (*models.ExecResult, error) {
	return nil, nil
}
func (b *recordingBackend) GetContainerLogs(context.Context, string, string) (string, string, error) {
	return "", "", nil
}

// ---------------------------------------------------------------------------
// Provider tests
// ---------------------------------------------------------------------------

func TestProviderName(t *testing.T) {
	p, err := New(Config{}, newRecordingBackend())
	require.NoError(t, err)
	assert.Equal(t, runtime.ProviderSysbox, p.Name())
}

func TestProviderDefaultImage(t *testing.T) {
	p, err := New(Config{}, newRecordingBackend())
	require.NoError(t, err)
	assert.Equal(t, "docker:27-dind", p.cfg.Image)
}

func TestProviderCustomImage(t *testing.T) {
	p, err := New(Config{Image: "my-custom-dind:latest"}, newRecordingBackend())
	require.NoError(t, err)
	assert.Equal(t, "my-custom-dind:latest", p.cfg.Image)
}

func TestProvision_CreatesContainerWithSysboxRuntime(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{
		WorktreeBasePath: "/home/user/worktrees",
	}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "test-env"})
	require.NoError(t, err)

	// Verify container config
	require.Len(t, mock.createdConfigs, 1)
	created := mock.createdConfigs[0]
	assert.Equal(t, "sysbox-runc", created.Runtime)
	assert.Equal(t, "noldarim-sysbox-test-env", created.Name)
	assert.Equal(t, "docker:27-dind", created.Image)
	assert.Equal(t, "", created.Environment["DOCKER_TLS_CERTDIR"])
	assert.Equal(t, "true", created.Labels["noldarim.managed"])
	assert.Equal(t, "test-env", created.Labels["noldarim.sysbox.env"])

	// Verify port mapping
	require.Len(t, created.Ports, 1)
	assert.Equal(t, 2375, created.Ports[0].ContainerPort)
	assert.Equal(t, "tcp", created.Ports[0].Protocol)
	assert.Greater(t, created.Ports[0].HostPort, 0, "host port should be an ephemeral port")

	// Verify worktree mount
	require.Len(t, created.Volumes, 1)
	assert.Equal(t, "/home/user/worktrees", created.Volumes[0].HostPath)
	assert.Equal(t, "/home/user/worktrees", created.Volumes[0].ContainerPath)
	assert.False(t, created.Volumes[0].ReadOnly)

	// Verify container was started
	require.Len(t, mock.startedIDs, 1)

	// Clean up
	err = env.Destroy(context.Background())
	assert.NoError(t, err)
}

func TestProvision_NoWorktreeMount(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	_, err = p.Provision(context.Background(), runtime.ProvisionOpts{ID: "no-wt"})
	require.NoError(t, err)

	require.Len(t, mock.createdConfigs, 1)
	assert.Empty(t, mock.createdConfigs[0].Volumes)
}

func TestProvision_CreateContainerFails(t *testing.T) {
	mock := newRecordingBackend()
	mock.createErr = fmt.Errorf("image not found")

	p, err := New(Config{}, mock)
	require.NoError(t, err)

	_, err = p.Provision(context.Background(), runtime.ProvisionOpts{ID: "fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create environment container")
	assert.Contains(t, err.Error(), "image not found")

	// No start or cleanup should have been attempted
	assert.Empty(t, mock.startedIDs)
	assert.Empty(t, mock.removedIDs)
}

func TestProvision_StartContainerFails_CleansUp(t *testing.T) {
	mock := newRecordingBackend()
	mock.startErr = fmt.Errorf("insufficient resources")

	p, err := New(Config{}, mock)
	require.NoError(t, err)

	_, err = p.Provision(context.Background(), runtime.ProvisionOpts{ID: "start-fail"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start environment container")

	// Container was created but start failed — should be cleaned up
	require.Len(t, mock.createdConfigs, 1)
	require.Len(t, mock.removedIDs, 1, "failed container should be removed")
}

func TestClose_DestroysAllEnvironments(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	_, err = p.Provision(context.Background(), runtime.ProvisionOpts{ID: "env-1"})
	require.NoError(t, err)
	_, err = p.Provision(context.Background(), runtime.ProvisionOpts{ID: "env-2"})
	require.NoError(t, err)

	require.Len(t, mock.createdConfigs, 2)

	err = p.Close()
	assert.NoError(t, err)

	// Both environments destroyed + host backend closed
	assert.Len(t, mock.removedIDs, 2)
	assert.True(t, mock.closed)
}

func TestClose_ReportsDestroyErrors(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	_, err = p.Provision(context.Background(), runtime.ProvisionOpts{ID: "err-env"})
	require.NoError(t, err)

	// Inject remove error after provision
	mock.removeErr = fmt.Errorf("permission denied")

	err = p.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "errors during close")
}

// ---------------------------------------------------------------------------
// Environment tests
// ---------------------------------------------------------------------------

func TestEnvironment_ID(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "my-id"})
	require.NoError(t, err)

	assert.Equal(t, "my-id", env.ID())
}

func TestEnvironment_DockerHost(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "host-test"})
	require.NoError(t, err)

	host := env.DockerHost()
	assert.True(t, strings.HasPrefix(host, "tcp://localhost:"), "DockerHost should be tcp://localhost:<port>, got %s", host)
}

func TestEnvironment_ContainerBackendPanicsBeforeWaitReady(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "panic-test"})
	require.NoError(t, err)

	assert.Panics(t, func() {
		env.ContainerBackend()
	})
}

func TestEnvironment_WaitReady_TimesOut(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "timeout-test"})
	require.NoError(t, err)

	// Use a very short timeout — no real Docker daemon, so it will fail
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	err = env.WaitReady(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not ready")
}

func TestEnvironment_Destroy_CleansUpBackend(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "destroy-test"})
	require.NoError(t, err)

	// Manually set a mock backend to simulate post-WaitReady state
	sysEnv := env.(*environment)
	innerMock := newRecordingBackend()
	sysEnv.backend = innerMock

	err = env.Destroy(context.Background())
	require.NoError(t, err)

	// Inner backend should be closed
	assert.True(t, innerMock.closed)
	// Sysbox container should be removed via host backend
	assert.Contains(t, mock.removedIDs, sysEnv.containerID)
}

func TestEnvironment_Destroy_NilBackend(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "nil-backend"})
	require.NoError(t, err)

	// Destroy before WaitReady (backend is nil) — should not panic
	err = env.Destroy(context.Background())
	require.NoError(t, err)
	assert.Len(t, mock.removedIDs, 1)
}

func TestEnvironment_ContainerBackendAfterManualSet(t *testing.T) {
	mock := newRecordingBackend()
	p, err := New(Config{}, mock)
	require.NoError(t, err)

	env, err := p.Provision(context.Background(), runtime.ProvisionOpts{ID: "backend-test"})
	require.NoError(t, err)

	// Simulate WaitReady success by setting backend directly
	sysEnv := env.(*environment)
	innerMock := newRecordingBackend()
	sysEnv.backend = innerMock

	// Should not panic, should return the inner backend
	backend := env.ContainerBackend()
	assert.Equal(t, innerMock, backend)
}

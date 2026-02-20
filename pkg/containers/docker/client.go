// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/noldarim/noldarim/pkg/containers/models"
)

// ClientInterface defines what we need from Docker
type ClientInterface interface {
	CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	InspectContainer(ctx context.Context, containerID string) (*models.Container, error)
	ListContainers(ctx context.Context) ([]*models.Container, error)
	ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error)
	KillContainer(ctx context.Context, containerID string) error
	CopyToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error
	CopyFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error
	WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error
	ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error)
	Close() error
}

// Client implements ClientInterface using real Docker
type Client struct {
	docker *client.Client
}

// Compile-time check that Client implements ClientInterface
var _ ClientInterface = (*Client)(nil)

// NewClient creates a new Docker client using default environment settings
func NewClient() (*Client, error) {
	return NewClientWithHost("")
}

// NewClientWithHost creates a new Docker client with a specific host
// If dockerHost is empty, uses environment variables (FromEnv)
func NewClientWithHost(dockerHost string) (*Client, error) {
	var opts []client.Opt

	if dockerHost != "" {
		opts = append(opts, client.WithHost(dockerHost))
	} else {
		opts = append(opts, client.FromEnv)
	}

	opts = append(opts, client.WithAPIVersionNegotiation())

	dockerClient, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Client{
		docker: dockerClient,
	}, nil
}

// CreateContainer creates a new container from the given configuration
func (c *Client) CreateContainer(ctx context.Context, config models.ContainerConfig) (*models.Container, error) {
	// Convert port mappings
	portBindings := nat.PortMap{}
	exposedPorts := nat.PortSet{}
	for _, port := range config.Ports {
		containerPort := nat.Port(fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol))
		hostBinding := nat.PortBinding{
			HostPort: strconv.Itoa(port.HostPort),
		}
		portBindings[containerPort] = []nat.PortBinding{hostBinding}
		exposedPorts[containerPort] = struct{}{}
	}

	// Convert volume mappings
	binds := make([]string, 0, len(config.Volumes))
	for _, volume := range config.Volumes {
		bind := fmt.Sprintf("%s:%s", volume.HostPath, volume.ContainerPath)
		if volume.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	// Create container configuration
	containerConfig := &container.Config{
		Image:        config.Image,
		Env:          envMapToSlice(config.Environment),
		ExposedPorts: exposedPorts,
		WorkingDir:   config.WorkingDir,
		Cmd:          config.Command,
		Labels:       config.Labels,
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        binds,
		NetworkMode:  container.NetworkMode(config.NetworkMode),
		Resources: container.Resources{
			Memory:    config.MemoryMB * 1024 * 1024, // Memory is in bytes
			CPUShares: config.CPUShares,
		},
	}

	networkingConfig := &network.NetworkingConfig{}

	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	now := time.Now()
	return &models.Container{
		ID:          resp.ID,
		Name:        config.Name,
		Image:       config.Image,
		Status:      models.StatusCreated,
		Environment: config.Environment,
		Ports:       config.Ports,
		Volumes:     config.Volumes,
		CreatedAt:   now,
		UpdatedAt:   now,
		TaskID:      config.TaskID,
	}, nil
}

// StartContainer starts an existing container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.docker.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a running container
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout *time.Duration) error {
	var timeoutSeconds *int
	if timeout != nil {
		seconds := int(timeout.Seconds())
		timeoutSeconds = &seconds
	}
	return c.docker.ContainerStop(ctx, containerID, container.StopOptions{Timeout: timeoutSeconds})
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return c.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: force,
	})
}

// InspectContainer gets detailed information about a container
func (c *Client) InspectContainer(ctx context.Context, containerID string) (*models.Container, error) {
	resp, err := c.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	status := models.StatusCreated
	if resp.State.Running {
		status = models.StatusRunning
	} else if resp.State.Dead || resp.State.OOMKilled {
		status = models.StatusFailed
	} else if !resp.State.Running && resp.State.ExitCode == 0 {
		status = models.StatusStopped
	}

	// Convert port bindings back to our model
	var ports []models.PortMapping
	for port, bindings := range resp.NetworkSettings.Ports {
		for _, binding := range bindings {
			hostPort, _ := strconv.Atoi(binding.HostPort)
			containerPort, _ := strconv.Atoi(port.Port())
			ports = append(ports, models.PortMapping{
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Protocol:      port.Proto(),
			})
		}
	}

	// Convert mounts to volume mappings
	var volumes []models.VolumeMapping
	for _, mount := range resp.Mounts {
		volumes = append(volumes, models.VolumeMapping{
			HostPath:      mount.Source,
			ContainerPath: mount.Destination,
			ReadOnly:      !mount.RW,
		})
	}

	createdTime, _ := time.Parse(time.RFC3339Nano, resp.Created)

	return &models.Container{
		ID:          resp.ID,
		Name:        resp.Name,
		Image:       resp.Config.Image,
		Status:      status,
		Environment: envSliceToMap(resp.Config.Env),
		Ports:       ports,
		Volumes:     volumes,
		CreatedAt:   createdTime,
		UpdatedAt:   time.Now(),
	}, nil
}

// ListContainers lists all containers (running and stopped)
func (c *Client) ListContainers(ctx context.Context) ([]*models.Container, error) {
	containers, err := c.docker.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	result := make([]*models.Container, 0, len(containers))
	for _, container := range containers {
		inspected, err := c.InspectContainer(ctx, container.ID)
		if err != nil {
			continue // Skip containers we can't inspect
		}
		result = append(result, inspected)
	}

	return result, nil
}

// ListContainersByLabels lists containers filtered by labels
func (c *Client) ListContainersByLabels(ctx context.Context, labels map[string]string) ([]*models.Container, error) {
	// Build label filters
	filterArgs := filters.NewArgs()
	for key, value := range labels {
		filterArgs.Add("label", fmt.Sprintf("%s=%s", key, value))
	}

	// List containers with filters
	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers by labels: %w", err)
	}

	// Convert to models.Container
	result := make([]*models.Container, 0, len(containers))
	for _, dockerContainer := range containers {
		// Get full container details
		inspected, err := c.InspectContainer(ctx, dockerContainer.ID)
		if err != nil {
			// Skip containers we can't inspect
			continue
		}
		result = append(result, inspected)
	}

	return result, nil
}

// KillContainer sends SIGKILL to a container
func (c *Client) KillContainer(ctx context.Context, containerID string) error {
	// Send SIGKILL signal
	err := c.docker.ContainerKill(ctx, containerID, "SIGKILL")
	if err != nil {
		// Check if the error is because container doesn't exist
		if client.IsErrNotFound(err) {
			// Container not found is not an error for idempotency
			return nil
		}
		return fmt.Errorf("failed to kill container: %w", err)
	}

	return nil
}

// CopyToContainer copies a file from the host to a container
func (c *Client) CopyToContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	// Read the source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	// Get file info for the tar header
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %w", srcPath, err)
	}

	// Create a tar archive containing the file
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Use base filename for the tar entry
	fileName := filepath.Base(srcPath)
	header := &tar.Header{
		Name: fileName,
		Mode: int64(srcInfo.Mode()),
		Size: srcInfo.Size(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Copy file contents to tar
	if _, err := io.Copy(tw, srcFile); err != nil {
		return fmt.Errorf("failed to write file to tar: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Copy to container using Docker API
	// The destination path should be the directory where we want to place the file
	destDir := filepath.Dir(dstPath)
	if err := c.docker.CopyToContainer(ctx, containerID, destDir, &buf, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy to container: %w", err)
	}

	return nil
}

// CopyFromContainer copies a file from a container to the host
func (c *Client) CopyFromContainer(ctx context.Context, containerID string, srcPath string, dstPath string) error {
	// Copy from container using Docker API
	reader, _, err := c.docker.CopyFromContainer(ctx, containerID, srcPath)
	if err != nil {
		return fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	// Extract the file from the tar archive
	tr := tar.NewReader(reader)

	// Read the first (and should be only) entry
	_, err = tr.Next()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("no file found in container at path %s", srcPath)
		}
		return fmt.Errorf("failed to read tar header: %w", err)
	}

	// Create the destination file
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	// Copy the file contents
	if _, err := io.Copy(dstFile, tr); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// WriteToContainer writes string content directly to a container file
func (c *Client) WriteToContainer(ctx context.Context, containerID string, content string, dstPath string) error {
	// Create a tar archive containing the content
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Use base filename for the tar entry
	fileName := filepath.Base(dstPath)
	header := &tar.Header{
		Name: fileName,
		Mode: 0644,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	// Write content to tar
	if _, err := tw.Write([]byte(content)); err != nil {
		return fmt.Errorf("failed to write content to tar: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Copy to container using Docker API
	// The destination path should be the directory where we want to place the file
	destDir := filepath.Dir(dstPath)
	if err := c.docker.CopyToContainer(ctx, containerID, destDir, &buf, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy content to container: %w", err)
	}

	return nil
}

// ExecContainer executes a command in a running container
func (c *Client) ExecContainer(ctx context.Context, containerID string, cmd []string, workDir string) (*models.ExecResult, error) {
	// Create exec configuration
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		WorkingDir:   workDir,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create the exec instance
	execResp, err := c.docker.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec instance: %w", err)
	}

	// Start the exec instance and capture output
	hijackedResp, err := c.docker.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer hijackedResp.Close()

	// Read all output
	var stdout, stderr strings.Builder
	outputDone := make(chan error, 1)

	go func() {
		// Docker multiplexes stdout and stderr in the response
		// We need to demultiplex it
		_, err := io.Copy(&stdout, hijackedResp.Reader)
		outputDone <- err
	}()

	// Wait for output to be read
	if err := <-outputDone; err != nil {
		return nil, fmt.Errorf("failed to read exec output: %w", err)
	}

	// Get the exit code
	inspectResp, err := c.docker.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	// For now, we put all output in stdout since Docker multiplexes the streams
	// In a more sophisticated implementation, we could demultiplex stdout and stderr
	return &models.ExecResult{
		ExitCode: inspectResp.ExitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

// Close closes the Docker client connection
func (c *Client) Close() error {
	return c.docker.Close()
}

// Helper functions
func envMapToSlice(envMap map[string]string) []string {
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func envSliceToMap(envSlice []string) map[string]string {
	envMap := make(map[string]string)
	for _, env := range envSlice {
		parts := []string{"", ""}
		if len(env) > 0 {
			for i, char := range env {
				if char == '=' {
					parts[0] = env[:i]
					parts[1] = env[i+1:]
					break
				}
			}
		}
		if parts[0] != "" {
			envMap[parts[0]] = parts[1]
		}
	}
	return envMap
}

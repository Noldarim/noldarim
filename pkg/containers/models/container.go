// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// ContainerStatus represents the current state of a container
type ContainerStatus string

const (
	StatusCreated ContainerStatus = "created"
	StatusRunning ContainerStatus = "running"
	StatusStopped ContainerStatus = "stopped"
	StatusFailed  ContainerStatus = "failed"
	StatusDeleted ContainerStatus = "deleted"
)

// Container represents a development environment container
type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Status      ContainerStatus   `json:"status"`
	Environment map[string]string `json:"environment"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMapping   `json:"volumes"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	TaskID      string            `json:"task_id,omitempty"`
	AgentID     string            `json:"agent_id,omitempty"`
}

// PortMapping defines port forwarding configuration
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeMapping defines volume mount configuration
type VolumeMapping struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// ContainerConfig holds configuration for creating a container
type ContainerConfig struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Environment map[string]string `json:"environment"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMapping   `json:"volumes"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Command     []string          `json:"command,omitempty"`
	TaskID      string            `json:"task_id,omitempty"`
	AgentID     string            `json:"agent_id,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	MemoryMB    int64             `json:"memory_mb,omitempty"`
	CPUShares   int64             `json:"cpu_shares,omitempty"`
	NetworkMode string            `json:"network_mode,omitempty"`
}

// ExecResult holds the result of executing a command in a container
type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

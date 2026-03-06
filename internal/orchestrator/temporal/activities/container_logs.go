// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/noldarim/noldarim/internal/orchestrator/models"

	"go.temporal.io/sdk/activity"
)

// ContainerLogSaver persists container log records.
type ContainerLogSaver interface {
	SaveContainerLog(ctx context.Context, log *models.ContainerLog) error
}

// ContainerLogSource retrieves logs from a container runtime.
type ContainerLogSource interface {
	GetContainerLogs(ctx context.Context, containerID string, tail string) (stdout, stderr string, err error)
}

// ContainerLogActivities provides the CaptureContainerLogsActivity.
type ContainerLogActivities struct {
	source ContainerLogSource
	saver  ContainerLogSaver
}

// NewContainerLogActivities creates a new instance.
func NewContainerLogActivities(source ContainerLogSource, saver ContainerLogSaver) *ContainerLogActivities {
	return &ContainerLogActivities{
		source: source,
		saver:  saver,
	}
}

// CaptureContainerLogsInput holds parameters for the capture activity.
type CaptureContainerLogsInput struct {
	RunID       string
	StepID      string
	ContainerID string
	Tail        string // Number of lines, or "all"
}

// CaptureContainerLogsOutput holds the result of the capture activity.
type CaptureContainerLogsOutput struct {
	StdoutLines int
	StderrLines int
}

// CaptureContainerLogsActivity retrieves container logs and persists them.
func (a *ContainerLogActivities) CaptureContainerLogsActivity(ctx context.Context, input CaptureContainerLogsInput) (*CaptureContainerLogsOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Capturing container logs", "containerID", input.ContainerID, "runID", input.RunID, "stepID", input.StepID)

	activity.RecordHeartbeat(ctx, "Fetching container logs")

	tail := input.Tail
	if tail == "" {
		tail = "1000"
	}

	stdout, stderr, err := a.source.GetContainerLogs(ctx, input.ContainerID, tail)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	now := time.Now()
	ts := now.UnixMilli()
	output := &CaptureContainerLogsOutput{}

	if stdout != "" {
		if err := a.saver.SaveContainerLog(ctx, &models.ContainerLog{
			ID:          fmt.Sprintf("cl-%s-%s-stdout-%d", input.RunID, input.StepID, ts),
			RunID:       input.RunID,
			StepID:      input.StepID,
			ContainerID: input.ContainerID,
			Stream:      "stdout",
			Content:     stdout,
			Timestamp:   now,
		}); err != nil {
			return nil, fmt.Errorf("failed to save stdout log: %w", err)
		}
		output.StdoutLines = strings.Count(stdout, "\n") + 1
	}

	if stderr != "" {
		if err := a.saver.SaveContainerLog(ctx, &models.ContainerLog{
			ID:          fmt.Sprintf("cl-%s-%s-stderr-%d", input.RunID, input.StepID, ts),
			RunID:       input.RunID,
			StepID:      input.StepID,
			ContainerID: input.ContainerID,
			Stream:      "stderr",
			Content:     stderr,
			Timestamp:   now,
		}); err != nil {
			return nil, fmt.Errorf("failed to save stderr log: %w", err)
		}
		output.StderrLines = strings.Count(stderr, "\n") + 1
	}

	logger.Info("Container logs captured", "stdout_lines", output.StdoutLines, "stderr_lines", output.StderrLines)
	return output, nil
}

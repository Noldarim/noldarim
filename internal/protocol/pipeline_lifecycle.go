// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// PipelineLifecycleType defines the type of pipeline lifecycle event
type PipelineLifecycleType string

const (
	// PipelineCreated - pipeline run has been created and setup completed
	PipelineCreated PipelineLifecycleType = "created"
	// PipelineStepStarted - a step within the pipeline has started processing
	PipelineStepStarted PipelineLifecycleType = "step_started"
	// PipelineStepCompleted - a step within the pipeline completed successfully
	PipelineStepCompleted PipelineLifecycleType = "step_completed"
	// PipelineStepFailed - a step within the pipeline failed
	PipelineStepFailed PipelineLifecycleType = "step_failed"
	// PipelineFinished - pipeline run completed successfully
	PipelineFinished PipelineLifecycleType = "finished"
	// PipelineFailed - pipeline run failed
	PipelineFailed PipelineLifecycleType = "failed"
)

// PipelineLifecycleEvent represents any pipeline lifecycle state change.
// This provides TUI visibility into pipeline progress and step completion.
type PipelineLifecycleEvent struct {
	Metadata
	Type      PipelineLifecycleType
	ProjectID string
	RunID     string
	Name      string // Pipeline/run name for display

	// Step info (populated for step-related events)
	StepID    string
	StepIndex int
	StepName  string

	// Run data (populated for PipelineCreated and status change events)
	Run *models.PipelineRun

	// Step result (populated for step completion events)
	StepResult *models.StepResult
}

func (e PipelineLifecycleEvent) GetMetadata() Metadata {
	return e.Metadata
}

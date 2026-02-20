// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/noldarim/noldarim/internal/common"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"

	"go.temporal.io/sdk/activity"
)

// EventActivities provides event publishing activities with distinct names
// for Temporal UI visibility, but shared implementation logic.
type EventActivities struct {
	eventChan chan<- common.Event
}

// NewEventActivities creates a new instance
func NewEventActivities(eventChan chan<- common.Event) *EventActivities {
	return &EventActivities{
		eventChan: eventChan,
	}
}

// ============================================================================
// Task Lifecycle Events - Thin wrappers for Temporal UI visibility
// ============================================================================

// PublishTaskCreatedEventActivity publishes a TaskCreated lifecycle event
func (a *EventActivities) PublishTaskCreatedEventActivity(ctx context.Context, input types.PublishEventInput) error {
	if input.Task == nil {
		return fmt.Errorf("Task field is required for TaskCreated events")
	}

	event := protocol.TaskLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.Task.ID, "task-created"),
		Type:      protocol.TaskCreated,
		ProjectID: input.ProjectID,
		TaskID:    input.Task.ID,
		Task:      input.Task,
	}
	return a.publish(ctx, event, "TaskCreated")
}

// PublishTaskDeletedEventActivity publishes a TaskDeleted lifecycle event
func (a *EventActivities) PublishTaskDeletedEventActivity(ctx context.Context, input types.PublishEventInput) error {
	event := protocol.TaskLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.TaskID, "task-deleted"),
		Type:      protocol.TaskDeleted,
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
	}
	return a.publish(ctx, event, "TaskDeleted")
}

// PublishTaskStatusUpdatedEventActivity publishes a TaskStatusUpdated lifecycle event
func (a *EventActivities) PublishTaskStatusUpdatedEventActivity(ctx context.Context, input types.PublishEventInput) error {
	event := protocol.TaskLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.TaskID, fmt.Sprintf("task-status-%s", input.Status.String())),
		Type:      protocol.TaskStatusUpdated,
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
		NewStatus: input.Status,
	}
	return a.publish(ctx, event, "TaskStatusUpdated")
}

// PublishTaskRequestedEventActivity publishes a TaskRequested lifecycle event
func (a *EventActivities) PublishTaskRequestedEventActivity(ctx context.Context, input types.PublishEventInput) error {
	event := protocol.TaskLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.TaskID, "task-requested"),
		Type:      protocol.TaskRequested,
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
	}
	return a.publish(ctx, event, "TaskRequested")
}

// PublishTaskInProgressEventActivity publishes a TaskInProgress lifecycle event
func (a *EventActivities) PublishTaskInProgressEventActivity(ctx context.Context, input types.PublishEventInput) error {
	event := protocol.TaskLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.TaskID, "task-in-progress"),
		Type:      protocol.TaskInProgress,
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
	}
	return a.publish(ctx, event, "TaskInProgress")
}

// PublishTaskFinishedEventActivity publishes a TaskFinished lifecycle event
func (a *EventActivities) PublishTaskFinishedEventActivity(ctx context.Context, input types.PublishEventInput) error {
	event := protocol.TaskLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.TaskID, "task-finished"),
		Type:      protocol.TaskFinished,
		ProjectID: input.ProjectID,
		TaskID:    input.TaskID,
	}
	return a.publish(ctx, event, "TaskFinished")
}

// ============================================================================
// Pipeline Lifecycle Events
// ============================================================================

// PublishPipelineCreatedEventActivity publishes a PipelineCreated lifecycle event
func (a *EventActivities) PublishPipelineCreatedEventActivity(ctx context.Context, input types.PublishPipelineEventInput) error {
	if input.Run == nil {
		return fmt.Errorf("Run field is required for PipelineCreated events")
	}

	event := protocol.PipelineLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.RunID, "pipeline-created"),
		Type:      protocol.PipelineCreated,
		ProjectID: input.ProjectID,
		RunID:     input.RunID,
		Name:      input.Name,
		Run:       input.Run,
	}
	return a.publish(ctx, event, "PipelineCreated")
}

// PublishPipelineStepStartedEventActivity publishes a PipelineStepStarted lifecycle event
func (a *EventActivities) PublishPipelineStepStartedEventActivity(ctx context.Context, input types.PublishPipelineEventInput) error {
	event := protocol.PipelineLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.RunID, fmt.Sprintf("step-started-%s", input.StepID)),
		Type:      protocol.PipelineStepStarted,
		ProjectID: input.ProjectID,
		RunID:     input.RunID,
		Name:      input.Name,
		StepID:    input.StepID,
		StepIndex: input.StepIndex,
		StepName:  input.StepName,
	}
	return a.publish(ctx, event, "PipelineStepStarted")
}

// PublishPipelineStepCompletedEventActivity publishes a PipelineStepCompleted lifecycle event
func (a *EventActivities) PublishPipelineStepCompletedEventActivity(ctx context.Context, input types.PublishPipelineEventInput) error {
	event := protocol.PipelineLifecycleEvent{
		Metadata:   a.metadata(input.ProjectID, input.RunID, fmt.Sprintf("step-completed-%s", input.StepID)),
		Type:       protocol.PipelineStepCompleted,
		ProjectID:  input.ProjectID,
		RunID:      input.RunID,
		Name:       input.Name,
		StepID:     input.StepID,
		StepIndex:  input.StepIndex,
		StepResult: input.StepResult,
	}
	return a.publish(ctx, event, "PipelineStepCompleted")
}

// PublishPipelineStepFailedEventActivity publishes a PipelineStepFailed lifecycle event
func (a *EventActivities) PublishPipelineStepFailedEventActivity(ctx context.Context, input types.PublishPipelineEventInput) error {
	event := protocol.PipelineLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.RunID, fmt.Sprintf("step-failed-%s", input.StepID)),
		Type:      protocol.PipelineStepFailed,
		ProjectID: input.ProjectID,
		RunID:     input.RunID,
		Name:      input.Name,
		StepID:    input.StepID,
		StepIndex: input.StepIndex,
		StepName:  input.StepName,
	}
	return a.publish(ctx, event, "PipelineStepFailed")
}

// PublishPipelineFinishedEventActivity publishes a PipelineFinished lifecycle event
func (a *EventActivities) PublishPipelineFinishedEventActivity(ctx context.Context, input types.PublishPipelineEventInput) error {
	event := protocol.PipelineLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.RunID, "pipeline-finished"),
		Type:      protocol.PipelineFinished,
		ProjectID: input.ProjectID,
		RunID:     input.RunID,
		Name:      input.Name,
		Run:       input.Run,
	}
	return a.publish(ctx, event, "PipelineFinished")
}

// PublishPipelineFailedEventActivity publishes a PipelineFailed lifecycle event
func (a *EventActivities) PublishPipelineFailedEventActivity(ctx context.Context, input types.PublishPipelineEventInput) error {
	event := protocol.PipelineLifecycleEvent{
		Metadata:  a.metadata(input.ProjectID, input.RunID, "pipeline-failed"),
		Type:      protocol.PipelineFailed,
		ProjectID: input.ProjectID,
		RunID:     input.RunID,
		Name:      input.Name,
	}
	return a.publish(ctx, event, "PipelineFailed")
}

// ============================================================================
// Error Events
// ============================================================================

// PublishErrorEventActivity publishes an ErrorEvent
func (a *EventActivities) PublishErrorEventActivity(ctx context.Context, input types.PublishErrorEventInput) error {
	if err := ValidateErrorEventInput(input); err != nil {
		return err
	}

	idempotencyKey := fmt.Sprintf("error-event-%s-%d", input.TaskID, time.Now().UnixNano())

	event := protocol.ErrorEvent{
		Metadata: protocol.Metadata{
			IdempotencyKey: idempotencyKey,
			Version:        protocol.CurrentProtocolVersion,
		},
		Message: input.Message,
		Context: input.ErrorContext,
		TaskID:  input.TaskID,
	}
	return a.publish(ctx, event, "Error")
}

// ============================================================================
// AI Activity Events
// ============================================================================

// PublishAIActivityEventActivity publishes an AIActivityRecord for real-time streaming.
// AIActivityRecord implements common.Event directly via GetMetadata().
func (a *EventActivities) PublishAIActivityEventActivity(ctx context.Context, record *models.AIActivityRecord) error {
	if record == nil {
		return fmt.Errorf("record is required")
	}

	// AIActivityRecord.GetMetadata() provides the idempotency key (using EventID)
	return a.publish(ctx, record, "AIActivity")
}

// ============================================================================
// Shared Implementation
// ============================================================================

// metadata creates a Metadata struct with idempotency key for events.
// Works for both task events (projectID + taskID) and pipeline events (projectID + runID).
func (a *EventActivities) metadata(projectID, entityID, keyPrefix string) protocol.Metadata {
	return protocol.Metadata{
		IdempotencyKey: fmt.Sprintf("%s-%s-%s", keyPrefix, projectID, entityID),
		Version:        protocol.CurrentProtocolVersion,
	}
}

// publish is the shared implementation for all event publishing
func (a *EventActivities) publish(ctx context.Context, event common.Event, eventType string) error {
	logger := activity.GetLogger(ctx)
	info := activity.GetInfo(ctx)

	logger.Info("Publishing event",
		"type", eventType,
		"idempotencyKey", event.GetMetadata().IdempotencyKey,
		"workflowID", info.WorkflowExecution.ID)

	activity.RecordHeartbeat(ctx, fmt.Sprintf("Publishing %s event", eventType))

	// Send to channel with context cancellation support
	select {
	case a.eventChan <- event:
		logger.Info("Successfully published event", "type", eventType)
		return nil
	case <-ctx.Done():
		logger.Error("Context cancelled while publishing event", "type", eventType)
		return ctx.Err()
	case <-time.After(5 * time.Second):
		logger.Error("Event publish timeout - channel may be full", "type", eventType)
		return fmt.Errorf("timeout publishing %s event after 5 seconds", eventType)
	}
}

// ============================================================================
// Input Validation (simplified - only needed for complex payloads)
// ============================================================================

// ValidatePublishEventInput validates a PublishEventInput
func ValidatePublishEventInput(input types.PublishEventInput) error {
	if input.ProjectID == "" {
		return fmt.Errorf("projectID is required")
	}
	if input.TaskID == "" {
		return fmt.Errorf("taskID is required")
	}
	return nil
}

// ValidateErrorEventInput validates PublishErrorEventInput
func ValidateErrorEventInput(input types.PublishErrorEventInput) error {
	if input.Message == "" {
		return fmt.Errorf("message is required")
	}
	return nil
}

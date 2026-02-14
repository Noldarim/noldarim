// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Here lies the definition of the data that orchestrator can send to the UI
// All data that UI can receive from the orchestrator will be named: Event
// Event can be originated directly from Commands, eg. CreateTask command can result in TaskCreatedEvent (names are just imaginary examples here)
// Events can also origante from "independent" sources, eg. TaskUpdatedEvent can be originated from TaskUpdatedSignal
// Above example abviously, would have origin in some root type command like CreateTaskCommand, but the relation is not so direct
// Tough if possible even if the realtion between Commands and far upstream events is not direct it would be good to register it it via Temporal workflows
package protocol

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// PipelineStatus represents the status of a pipeline run
type PipelineStatus string

// Pipeline status constants
const (
	PipelineStatusRunning   PipelineStatus = "running"
	PipelineStatusCompleted PipelineStatus = "completed"
	PipelineStatusFailed    PipelineStatus = "failed"
)

// NOTE: Event interface is now defined in common package and re-exported from base.go

// GetIdempotencyKey extracts the idempotency key from any event
func GetIdempotencyKey(event Event) string {
	return event.GetMetadata().IdempotencyKey
}

// ProjectsLoadedEvent is sent when projects have been loaded
type ProjectsLoadedEvent struct {
	Metadata
	Projects map[string]*models.Project
}

func (e ProjectsLoadedEvent) GetMetadata() Metadata {
	return e.Metadata
}

// TasksLoadedEvent is sent when tasks have been loaded
type TasksLoadedEvent struct {
	Metadata
	ProjectID      string
	ProjectName    string
	RepositoryPath string
	Tasks          map[string]*models.Task
}

func (e TasksLoadedEvent) GetMetadata() Metadata {
	return e.Metadata
}

// CommitInfo represents a single commit in the event
type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	Parents []string
}

// CommitsLoadedEvent is sent when commit history has been loaded
type CommitsLoadedEvent struct {
	Metadata
	ProjectID      string
	RepositoryPath string
	Commits        []CommitInfo
}

func (e CommitsLoadedEvent) GetMetadata() Metadata {
	return e.Metadata
}

// TaskCreationStartedEvent is sent when a task creation workflow starts
// This is kept separate from TaskLifecycleEvent as it contains workflow-specific info
type TaskCreationStartedEvent struct {
	Metadata
	ProjectID  string
	WorkflowID string
}

func (e TaskCreationStartedEvent) GetMetadata() Metadata {
	return e.Metadata
}

// NOTE: The following events have been consolidated into TaskLifecycleEvent
// (see task_lifecycle.go):
// - TaskStatusUpdatedEvent -> TaskLifecycleEvent{Type: TaskStatusUpdated}
// - TaskCreationCompletedEvent -> TaskLifecycleEvent{Type: TaskCreated}
// - TaskRequestedEvent -> TaskLifecycleEvent{Type: TaskRequested}
// - TaskInProgressEvent -> TaskLifecycleEvent{Type: TaskInProgress}
// - TaskFinishedEvent -> TaskLifecycleEvent{Type: TaskFinished}
// - TaskDeletedEvent -> TaskLifecycleEvent{Type: TaskDeleted}

type ErrorEvent struct {
	Metadata
	Message string
	Context string
	TaskID  string // Optional - identifies which task the error is related to
}

func (e ErrorEvent) GetMetadata() Metadata {
	return e.Metadata
}

type CriticalErrorEvent struct {
	Metadata
	Message string
	Context string
}

func (e CriticalErrorEvent) GetMetadata() Metadata {
	return e.Metadata
}

// ProjectCreatedEvent is sent when a project has been created
type ProjectCreatedEvent struct {
	Metadata
	Project *models.Project
}

func (e ProjectCreatedEvent) GetMetadata() Metadata {
	return e.Metadata
}

// NOTE: AIActivityRecord implements common.Event directly via GetMetadata().
// Send *models.AIActivityRecord directly through the event channel.

// AIActivityBatchEvent is sent when multiple AI activity records are available
// Used for catching up on events or bulk updates
type AIActivityBatchEvent struct {
	Metadata
	TaskID     string
	ProjectID  string
	Activities []*models.AIActivityRecord
}

func (e AIActivityBatchEvent) GetMetadata() Metadata {
	return e.Metadata
}

// AIStreamStartEvent is sent when AI activity streaming begins for a task
type AIStreamStartEvent struct {
	Metadata
	TaskID    string
	ProjectID string
}

func (e AIStreamStartEvent) GetMetadata() Metadata {
	return e.Metadata
}

// AIStreamEndEvent is sent when AI activity streaming ends for a task
type AIStreamEndEvent struct {
	Metadata
	TaskID        string
	ProjectID     string
	TotalEvents   int    // Total number of events streamed
	FinalStatus   string // "completed", "error", "cancelled"
}

func (e AIStreamEndEvent) GetMetadata() Metadata {
	return e.Metadata
}

// PipelineRunStartedEvent is sent when a pipeline workflow starts.
// If AlreadyExists is true, the workflow was already running or completed.
type PipelineRunStartedEvent struct {
	Metadata
	RunID         string
	ProjectID     string
	Name          string
	WorkflowID    string
	AlreadyExists bool           // True if workflow already exists (running or completed)
	Status        PipelineStatus // Empty for new workflows
	// Fork information (if forking from a previous run)
	ForkFromRunID   string // Source run ID if forking
	ForkAfterStepID string // Step ID to fork after
	SkippedSteps    int    // Number of steps being skipped
}

func (e PipelineRunStartedEvent) GetMetadata() Metadata {
	return e.Metadata
}

// PipelineRunsLoadedEvent is sent when pipeline runs have been loaded for a project
type PipelineRunsLoadedEvent struct {
	Metadata
	ProjectID      string
	ProjectName    string
	RepositoryPath string
	Runs           map[string]*models.PipelineRun // keyed by run ID
}

func (e PipelineRunsLoadedEvent) GetMetadata() Metadata {
	return e.Metadata
}

// PipelineCancelledEvent confirms a pipeline was cancelled and workflow has stopped
type PipelineCancelledEvent struct {
	Metadata
	RunID          string
	Reason         string
	WorkflowStatus string // Final status: "canceled", "terminated", "failed", "completed", or "timeout" if we gave up waiting
}

func (e PipelineCancelledEvent) GetMetadata() Metadata {
	return e.Metadata
}

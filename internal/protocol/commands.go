// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Here lies the definition of the data that orchestrator can receive from the UI
// All data that is recieved by orchestrator from UI will be named: Command
//
// Commands should be simple high level objects which tell orchestrator which end goal is required
// Commands should NOT specify in "Write" requests any details eg. specific ids, timestamps ...
//
// Commands should be separated into Read and ReadWrite commands. In this file they should be grouped together
package protocol

// Command represents commands that can be sent to the orchestrator
type Command interface {
	// All commands must embed BaseMessage for correlation and versioning
	GetBaseMessage() Metadata
}

// Read commands

// LoadProjectsCommand requests loading all projects
type LoadProjectsCommand struct {
	Metadata
}

func (c LoadProjectsCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// LoadTasksCommand requests loading tasks for a specific project
type LoadTasksCommand struct {
	Metadata
	ProjectID string
}

func (c LoadTasksCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// LoadCommitsCommand requests loading commit history for a specific project
type LoadCommitsCommand struct {
	Metadata
	ProjectID string
	Limit     int // Maximum number of commits to load
}

func (c LoadCommitsCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// LoadAIActivityCommand requests loading AI activity events for a specific task
type LoadAIActivityCommand struct {
	Metadata
	ProjectID string
	TaskID    string
}

func (c LoadAIActivityCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// ReadWrite commands

// ToggleTaskCommand toggles a task's completion status
type ToggleTaskCommand struct {
	Metadata
	ProjectID string
	TaskID    string
}

func (c ToggleTaskCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// DeleteTaskCommand deletes a task
type DeleteTaskCommand struct {
	Metadata
	ProjectID string
	TaskID    string
}

func (c DeleteTaskCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// CreateTaskCommand creates a new task
type CreateTaskCommand struct {
	Metadata             // TaskID is now in Metadata for correlation
	ProjectID     string
	Title         string
	Description   string
	BaseCommitSHA string            // Commit SHA to create worktree from (for content-based task ID)
	AgentConfig   *AgentConfigInput // Structured agent configuration for task processing
}

func (c CreateTaskCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// UpdateTaskCommand updates task details
type UpdateTaskCommand struct {
	Metadata
	ProjectID   string
	TaskID      string
	Title       string
	Description string
}

func (c UpdateTaskCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// CreateProjectCommand creates a new project
type CreateProjectCommand struct {
	Metadata
	Name           string
	Description    string
	RepositoryPath string
}

func (c CreateProjectCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// StartPipelineCommand starts a pipeline workflow
type StartPipelineCommand struct {
	Metadata
	ProjectID     string
	Name          string      // Pipeline/run name (typically the task description)
	Steps         []StepInput // Step definitions (single step for simple runs)
	BaseCommitSHA string      // Optional: commit to start from (defaults to HEAD)
	// Fork options for smart step reuse
	ForkFromRunID   string // Explicitly fork from a previous run ID
	ForkAfterStepID string // Fork after this step ID (reuse steps up to and including this one)
	NoAutoFork      bool   // Disable automatic fork detection
}

func (c StartPipelineCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// StepInput defines a single pipeline step
type StepInput struct {
	StepID      string
	Name        string
	AgentConfig *AgentConfigInput
}

// LoadPipelineRunsCommand requests loading pipeline runs for a specific project
type LoadPipelineRunsCommand struct {
	Metadata
	ProjectID string
}

func (c LoadPipelineRunsCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// CancelPipelineCommand requests cancellation of a running pipeline
type CancelPipelineCommand struct {
	Metadata
	RunID  string // Pipeline run ID
	Reason string // Optional: why cancelled
}

func (c CancelPipelineCommand) GetBaseMessage() Metadata {
	return c.Metadata
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"time"
)

// CreateTaskWorkflowInput represents the input for CreateTask workflow
type CreateTaskWorkflowInput struct {
	ProjectID             string
	TaskID                string            // Task ID (content-based hash computed by orchestrator)
	Title                 string
	Description           string
	RepositoryPath        string            // Path to the git repository
	BaseCommitSHA         string            // Commit SHA to create worktree from
	ClaudeConfigPath      string            // Path to the Claude config file on host
	AgentConfig           *protocol.AgentConfigInput // Structured agent configuration
	WorkspaceDir          string                     // Workspace directory from config
	OrchestratorTaskQueue string                     // Task queue where orchestrator is listening (for passing to child workflows)
	HooksConfig           *HooksConfigInput          // Optional hooks configuration
}

// HooksConfigInput holds hooks configuration passed to workflows
type HooksConfigInput struct {
	EnableLogging bool   // Enable debug logging in hook script
	ScriptPath    string // Custom script path (empty = use default)
}

// CreateTaskWorkflowOutput represents the output from CreateTask workflow
type CreateTaskWorkflowOutput struct {
	Task    *models.Task
	Success bool
	Error   string
}

// CreateWorktreeActivityInput represents input for git worktree creation
type CreateWorktreeActivityInput struct {
	TaskID         string
	BranchName     string
	RepositoryPath string // Path to the git repository
	BaseCommitSHA  string // Commit SHA to create worktree from (empty = HEAD)
}

// CreateWorktreeActivityOutput represents output from git worktree creation
type CreateWorktreeActivityOutput struct {
	WorktreePath string
	BranchName   string
}

// RemoveWorktreeActivityInput represents input for git worktree removal
type RemoveWorktreeActivityInput struct {
	WorktreePath   string
	RepositoryPath string
}

// CreateContainerActivityInput represents input for container creation
type CreateContainerActivityInput struct {
	TaskID            string
	WorktreePath      string
	ProjectID         string
	TaskQueue         string // Task queue name for the temporal worker inside container
	OrchestratorQueue string // Orchestrator queue for event forwarding
}

// CreateContainerActivityOutput represents output from container creation
type CreateContainerActivityOutput struct {
	ContainerID string
	Status      string
}

// CreateTaskActivityInput represents input for task database creation
type CreateTaskActivityInput struct {
	ProjectID    string
	TaskID       string // Task ID from TUI
	Title        string
	Description  string
	TaskFilePath string
}

// CreateTaskActivityOutput represents output from task database creation
type CreateTaskActivityOutput struct {
	Task *models.Task
}

// UpdateTaskStatusActivityInput represents input for updating task status in database
type UpdateTaskStatusActivityInput struct {
	ProjectID string
	TaskID    string
	Status    models.TaskStatus
}

// NOTE: Event activity inputs have been consolidated into PublishEventInput
// in event_inputs.go. The following types are deprecated:
// - PublishTaskCreatedEventInput
// - PublishTaskDeletedEventInput
// - PublishTaskStatusUpdatedEventInput
// - PublishTaskRequestedEventInput
// - PublishTaskInProgressEventInput
// - PublishTaskFinishedEventInput
// - PublishAIActivityEventInput
//
// Use PublishEventInput instead for all task lifecycle events.
// PublishErrorEventInput remains in event_inputs.go for error events.

// WorkflowState represents the state of a workflow execution
type WorkflowState struct {
	WorkflowID  string
	Status      string
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       string
}

// ActivityResult represents a generic activity result
type ActivityResult struct {
	Success bool
	Error   string
	Data    any
}

// CopyClaudeConfigActivityInput represents input for Claude config copy activity
type CopyClaudeConfigActivityInput struct {
	ContainerID    string
	HostConfigPath string // Path to ~/.claude.json on host
}

// CopyClaudeConfigActivityOutput represents output from Claude config copy activity
type CopyClaudeConfigActivityOutput struct {
	Success bool
	Error   string
}

// CopyClaudeCredentialsActivityInput represents input for Claude credentials copy activity
type CopyClaudeCredentialsActivityInput struct {
	ContainerID string
}

// CopyClaudeCredentialsActivityOutput represents output from Claude credentials copy activity
type CopyClaudeCredentialsActivityOutput struct {
	Success bool
	Error   string
}

// ExecuteCommandActivityInput represents input for container command execution activity
type ExecuteCommandActivityInput struct {
	ContainerID string
	Command     []string
	WorkDir     string
}

// ExecuteCommandActivityOutput represents output from container command execution activity
type ExecuteCommandActivityOutput struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Success  bool
	Error    string
}

// ProcessTaskWorkflowInput represents the input for ProcessTask workflow
type ProcessTaskWorkflowInput struct {
	TaskID                string
	TaskFilePath          string                     // Path to the task file relative to workspace
	ProjectID             string
	WorkspaceDir          string
	AgentConfig           *protocol.AgentConfigInput // Structured agent configuration
	WorktreePath          string                     // Path to the git worktree (for git operations)
	OrchestratorTaskQueue string                     // Task queue where orchestrator is listening (for cross-worker activities)
	TranscriptDir         string                     // Directory where Claude writes transcript files (host path, for watcher)
}

// ProcessTaskWorkflowOutput represents the output from ProcessTask workflow
type ProcessTaskWorkflowOutput struct {
	Success       bool
	ProcessedData string
	Error         string
}

// LocalExecuteActivityInput represents input for local command execution
type LocalExecuteActivityInput struct {
	Command []string // Command and arguments to execute
	WorkDir string   // Working directory for command execution
}

// LocalExecuteActivityOutput represents output from local command execution
type LocalExecuteActivityOutput struct {
	ExitCode    int           // Exit code of the command
	Output      string        // Combined stdout output
	ErrorOutput string        // Stderr output
	Duration    time.Duration // How long the command took to execute
	Success     bool          // Whether command succeeded (exitCode == 0)
	Error       string        // Error message if execution failed
}

// GitCommitActivityInput represents input for git commit activity
type GitCommitActivityInput struct {
	RepositoryPath string   // Path to the git repository
	FileNames      []string // List of file names to commit (relative to repository root)
	CommitMessage  string   // Commit message
}

// GitCommitActivityOutput represents output from git commit activity
type GitCommitActivityOutput struct {
	Success   bool   // Whether the commit was successful
	Error     string // Error message if failed
	CommitSHA string // SHA of the created commit (if successful)
}

// WriteTaskFileActivityInput represents input for writing task details to file
type WriteTaskFileActivityInput struct {
	TaskID         string
	Title          string
	Description    string
	RepositoryPath string
}

// WriteTaskFileActivityOutput represents output from writing task details to file
type WriteTaskFileActivityOutput struct {
	FilePath string // Relative path from repository root
	FileName string // Just the filename (e.g., "20250906-131548-test-task-4.md")
}

// CaptureGitDiffActivityInput represents input for capturing git diff
type CaptureGitDiffActivityInput struct {
	RepositoryPath string // Path to the git repository (worktree)
}

// CaptureGitDiffActivityOutput represents output from capturing git diff
type CaptureGitDiffActivityOutput struct {
	Success      bool
	Error        string
	Diff         string   // Full git diff output (raw text)
	DiffStat     string   // Git diff --stat output
	FilesChanged []string // List of changed file paths
	Insertions   int      // Number of lines inserted
	Deletions    int      // Number of lines deleted
	HasChanges   bool     // Whether there are any changes
}

// UpdateTaskGitDiffActivityInput represents input for updating task git diff
type UpdateTaskGitDiffActivityInput struct {
	TaskID  string // ID of the task to update
	GitDiff string // Git diff content to save
}

// ProcessingMetadata represents metadata collected during task processing
// This data is available via Temporal queries
type ProcessingMetadata struct {
	GitDiff        *CaptureGitDiffActivityOutput // Git diff information
	ProcessingTime time.Duration                 // Time taken for processing
	CommandOutput  string                        // Output from the processing command
	Timestamp      time.Time                     // When metadata was captured
}

// NOTE: AIActivitySignal has been removed.
// Signal directly with *models.AIActivityEvent which now has ProjectID field.

// NOTE: PublishAIActivityEventInput has been replaced by PublishEventInput
// in event_inputs.go. Use PublishEventInput with Payload set to *models.AIActivityEvent.

// SetupClaudeHooksActivityInput represents input for setting up Claude hooks in container
type SetupClaudeHooksActivityInput struct {
	ContainerID       string // Container ID to setup hooks in
	TaskID            string // Task ID for event correlation
	WorkspaceDir      string // Container workspace directory (default: /workspace)
	HookScriptPath    string // Path for hook script (default: /usr/local/bin/noldarim-hook.sh)
	EventSocketPath   string // Unix socket path for events (default: /tmp/noldarim-events.sock)
	EnableLogging     bool   // Whether to enable additional debug logging
}

// SetupClaudeHooksActivityOutput represents output from setting up Claude hooks
type SetupClaudeHooksActivityOutput struct {
	Success         bool   // Whether setup was successful
	Error           string // Error message if failed
	HookScriptPath  string // Path where hook script was written
	SettingsPath    string // Path where settings.json was written
	EventSocketPath string // Unix socket path configured
}

// InitTranscriptWatcherActivityInput represents input for initializing a transcript watcher
type InitTranscriptWatcherActivityInput struct {
	TaskID          string // Task ID for correlation
	TranscriptPath  string // Path to transcript.jsonl file
	Source          string // AI tool source ("claude", "gemini", etc.)
	EventBufferSize int    // Size of event channel buffer (default: 1000)
}

// InitTranscriptWatcherActivityOutput represents output from initializing a transcript watcher
type InitTranscriptWatcherActivityOutput struct {
	Success   bool   // Whether initialization was successful
	Error     string // Error message if failed
	WatcherID string // Unique ID for the watcher (equals TaskID)
}

// StopTranscriptWatcherActivityInput represents input for stopping a transcript watcher
type StopTranscriptWatcherActivityInput struct {
	TaskID string // Task ID / watcher ID to stop
}

// NOTE: TranscriptEventSignal has been removed.
// Signal directly with *models.AIActivityEvent.

// AIObservabilityWorkflowInput represents input for the AI observability workflow
type AIObservabilityWorkflowInput struct {
	TaskID                string // Task ID for correlation (step-specific: runID-stepID)
	RunID                 string // Pipeline run ID for aggregating all steps
	ProjectID             string // Project ID for event context
	TranscriptDir         string // Directory where Claude writes transcripts (e.g., /home/noldarim/.claude/projects/-workspace)
	ProcessTaskWorkflowID string // Workflow ID of ProcessTaskWorkflow (for signaling events)
	OrchestratorTaskQueue string // Queue for orchestrator activities (save/publish events)
}

// AIObservabilityWorkflowOutput represents output from the AI observability workflow
type AIObservabilityWorkflowOutput struct {
	Success           bool   // Whether observation completed successfully
	Error             string // Error message if failed
	EventsCount       int    // Number of events successfully processed
	FailedEventsCount int    // Number of events that failed to save or parse
}

// WatchTranscriptActivityInput represents input for the blocking transcript watch activity
type WatchTranscriptActivityInput struct {
	TaskID        string // Task ID for correlation
	ProjectID     string // Project ID for event correlation
	TranscriptDir string // Directory to watch for transcript files
	Source        string // AI tool source ("claude", "gemini", etc.)
	// Note: Activity signals its parent workflow (AIObservabilityWorkflow) directly
	// using activity.GetInfo(ctx).WorkflowExecution.ID
}

// WatchTranscriptActivityOutput represents output from the transcript watch activity
type WatchTranscriptActivityOutput struct {
	Success     bool   // Whether watching completed successfully
	Error       string // Error message if failed
	EventsCount int    // Number of events captured
}

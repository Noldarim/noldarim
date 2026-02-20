// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"time"
)

// ============================================================================
// PipelineWorkflow Types
// ============================================================================

// PipelineWorkflowInput represents the input for the PipelineWorkflow
type PipelineWorkflowInput struct {
	// Run identification
	RunID      string `json:"run_id"`
	PipelineID string `json:"pipeline_id"` // Optional - can use inline steps instead
	ProjectID  string `json:"project_id"`

	// Pipeline definition (inline or loaded from DB)
	Name  string                  `json:"name"`
	Steps []models.StepDefinition `json:"steps"`

	// Prompt composition - applied to all steps
	PromptPrefix string `json:"prompt_prefix,omitempty"` // Prepended to all step prompts
	PromptSuffix string `json:"prompt_suffix,omitempty"` // Appended to all step prompts

	// Repository configuration
	RepositoryPath string `json:"repository_path"`
	BaseCommitSHA  string `json:"base_commit_sha"` // Starting commit (HEAD if empty)
	BranchName     string `json:"branch_name"`     // Branch to create for this run

	// Fork configuration (optional - for branching from previous run)
	ForkFromRunID   string `json:"fork_from_run_id,omitempty"`
	ForkAfterStepID string `json:"fork_after_step_id,omitempty"`
	StartCommitSHA  string `json:"start_commit_sha,omitempty"` // Derived from fork or explicit

	// Agent configuration
	ClaudeConfigPath      string `json:"claude_config_path"`
	WorkspaceDir          string `json:"workspace_dir"`
	OrchestratorTaskQueue string `json:"orchestrator_task_queue"`
}

// PipelineWorkflowOutput represents the output from the PipelineWorkflow
type PipelineWorkflowOutput struct {
	Success       bool                   `json:"success"`
	RunID         string                 `json:"run_id"`
	BranchName    string                 `json:"branch_name"`
	HeadCommitSHA string                 `json:"head_commit_sha"`
	StepResults   []ProcessingStepOutput `json:"step_results"`
	Error         string                 `json:"error,omitempty"`
	Duration      time.Duration          `json:"duration"`
}

// ============================================================================
// Setup Phase Types
// ============================================================================

// PipelineSetupInput represents input for the setup phase
// SetupWorkflow handles ALL setup: DB record creation + infrastructure
type PipelineSetupInput struct {
	// Run identification
	RunID      string                  `json:"run_id"`
	PipelineID string                  `json:"pipeline_id,omitempty"` // Optional
	ProjectID  string                  `json:"project_id"`
	Name       string                  `json:"name"` // Pipeline/run name
	Steps      []models.StepDefinition `json:"steps"`

	// Repository configuration
	RepositoryPath string `json:"repository_path"`
	BranchName     string `json:"branch_name"`
	BaseCommitSHA  string `json:"base_commit_sha"`  // Original base before any steps
	StartCommitSHA string `json:"start_commit_sha"` // Actual start (may differ if forked)

	// Fork configuration (optional)
	ForkFromRunID   string `json:"fork_from_run_id,omitempty"`
	ForkAfterStepID string `json:"fork_after_step_id,omitempty"`

	// Container configuration
	ClaudeConfigPath string `json:"claude_config_path"`
	WorkspaceDir     string `json:"workspace_dir"`
	TaskQueue        string `json:"task_queue"` // For worker inside container

	// Cross-worker communication
	OrchestratorTaskQueue string `json:"orchestrator_task_queue"`

	// Temporal tracking (parent workflow ID for DB record)
	ParentWorkflowID string `json:"parent_workflow_id"`
}

// PipelineSetupOutput represents output from the setup phase
type PipelineSetupOutput struct {
	Success        bool   `json:"success"`
	WorktreePath   string `json:"worktree_path"`
	ContainerID    string `json:"container_id"`
	BranchName     string `json:"branch_name"`
	StartCommitSHA string `json:"start_commit_sha"` // Resolved start commit (after fork handling)
	Error          string `json:"error,omitempty"`
}

// ============================================================================
// ProcessingStep Types (reusable for each step)
// ============================================================================

// ProcessingStepInput represents input for a single processing step
type ProcessingStepInput struct {
	// Step identification
	RunID     string `json:"run_id"`
	ProjectID string `json:"project_id"`
	StepID    string `json:"step_id"`    // e.g., "1a", "1b"
	StepIndex int    `json:"step_index"` // 0, 1, 2...
	StepName  string `json:"step_name"`

	// Agent configuration for this step
	AgentConfig *protocol.AgentConfigInput `json:"agent_config"`

	// Execution context
	WorktreePath          string `json:"worktree_path"`
	WorkspaceDir          string `json:"workspace_dir"`
	OrchestratorTaskQueue string `json:"orchestrator_task_queue"`

	// Previous step's commit (for chaining)
	PreviousCommitSHA string `json:"previous_commit_sha,omitempty"`

	// Observability (optional)
	TranscriptDir string `json:"transcript_dir,omitempty"`
}

// ProcessingStepOutput represents output from a single processing step
type ProcessingStepOutput struct {
	Success bool   `json:"success"`
	StepID  string `json:"step_id"`

	// Git results
	CommitSHA     string `json:"commit_sha"`
	CommitMessage string `json:"commit_message"`
	GitDiff       string `json:"git_diff"`

	// Diff statistics
	FilesChanged int `json:"files_changed"`
	Insertions   int `json:"insertions"`
	Deletions    int `json:"deletions"`

	// Token usage
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CacheReadTokens   int `json:"cache_read_tokens"`
	CacheCreateTokens int `json:"cache_create_tokens"`

	// Execution metadata
	AgentOutput string        `json:"agent_output"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}

// TotalTokens returns total tokens used in this step
func (o *ProcessingStepOutput) TotalTokens() int {
	return o.InputTokens + o.OutputTokens
}

// ============================================================================
// Step Documentation Types
// ============================================================================

// StepSummary represents the structured summary extracted from agent output
type StepSummary struct {
	Reason  string   `json:"reason"`  // Why changes were made
	Changes []string `json:"changes"` // List of changes made
}

// GenerateStepDocumentationActivityInput for doc generation activity
type GenerateStepDocumentationActivityInput struct {
	RunID        string   `json:"run_id"`
	StepID       string   `json:"step_id"`
	StepName     string   `json:"step_name"`
	StepIndex    int      `json:"step_index"`
	WorktreePath string   `json:"worktree_path"`
	PromptUsed   string   `json:"prompt_used"`  // The prompt given to the agent
	AgentOutput  string   `json:"agent_output"` // Full output from agent
	GitDiff      string   `json:"git_diff"`
	DiffStat     string   `json:"diff_stat"`
	FilesChanged []string `json:"files_changed"`
	Insertions   int      `json:"insertions"`
	Deletions    int      `json:"deletions"`
}

// GenerateStepDocumentationActivityOutput from doc generation activity
type GenerateStepDocumentationActivityOutput struct {
	Success      bool         `json:"success"`
	DocumentPath string       `json:"document_path"` // Relative path in worktree
	Summary      *StepSummary `json:"summary,omitempty"`
	Error        string       `json:"error,omitempty"`
}

// ============================================================================
// Cleanup Phase Types
// ============================================================================

// PipelineCleanupInput represents input for the cleanup phase
type PipelineCleanupInput struct {
	RunID        string `json:"run_id"`
	ContainerID  string `json:"container_id"`
	WorktreePath string `json:"worktree_path"`
	KeepWorktree bool   `json:"keep_worktree"` // For debugging
}

// PipelineCleanupOutput represents output from the cleanup phase
type PipelineCleanupOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ============================================================================
// Activity Input/Output Types for Pipeline
// ============================================================================

// SavePipelineRunActivityInput represents input for saving pipeline run to DB
type SavePipelineRunActivityInput struct {
	Run *models.PipelineRun `json:"run"`
}

// SaveStepResultActivityInput represents input for saving step result to DB
type SaveStepResultActivityInput struct {
	Result *models.StepResult `json:"result"`
}

// SaveRunStepSnapshotsActivityInput represents input for saving run step snapshots to DB.
type SaveRunStepSnapshotsActivityInput struct {
	RunID string                  `json:"run_id"`
	Steps []models.StepDefinition `json:"steps"`
}

// UpdatePipelineRunStatusActivityInput represents input for updating run status
type UpdatePipelineRunStatusActivityInput struct {
	RunID        string                   `json:"run_id"`
	Status       models.PipelineRunStatus `json:"status"`
	ErrorMessage string                   `json:"error_message,omitempty"`
}

// GetPipelineRunActivityInput represents input for getting a pipeline run
type GetPipelineRunActivityInput struct {
	RunID string `json:"run_id"`
}

// GetPipelineRunActivityOutput represents output from getting a pipeline run
type GetPipelineRunActivityOutput struct {
	Run *models.PipelineRun `json:"run"`
}

// GetTokenTotalsActivityInput represents input for getting token totals
type GetTokenTotalsActivityInput struct {
	TaskID string `json:"task_id"`
}

// GetTokenTotalsActivityOutput represents output from getting token totals
type GetTokenTotalsActivityOutput struct {
	InputTokens       int `json:"input_tokens"`
	OutputTokens      int `json:"output_tokens"`
	CacheReadTokens   int `json:"cache_read_tokens"`
	CacheCreateTokens int `json:"cache_create_tokens"`
}

// PublishPipelineEventInput represents input for pipeline lifecycle event activities
type PublishPipelineEventInput struct {
	ProjectID string `json:"project_id"`
	RunID     string `json:"run_id"`
	Name      string `json:"name"` // Pipeline/run name for display

	// Step info (for step-related events)
	StepID    string `json:"step_id,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	StepName  string `json:"step_name,omitempty"`

	// Run data (for created/finished events)
	Run *models.PipelineRun `json:"run,omitempty"`

	// Step result (for step completion events)
	StepResult *models.StepResult `json:"step_result,omitempty"`
}

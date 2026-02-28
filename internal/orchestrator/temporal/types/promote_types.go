// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"fmt"
	"time"
)

// ============================================================================
// Promote Activity I/O Types
// ============================================================================

// CheckFastForwardInput is the input for CheckFastForwardActivity.
type CheckFastForwardInput struct {
	RepoPath   string `json:"repo_path"`
	MainBranch string `json:"main_branch"`
	TaskBranch string `json:"task_branch"`
}

// CheckFastForwardOutput is the output from CheckFastForwardActivity.
type CheckFastForwardOutput struct {
	IsFF        bool   `json:"is_ff"`
	MainHeadSHA string `json:"main_head_sha"`
	TaskHeadSHA string `json:"task_head_sha"`
}

// FastForwardBranchInput is the input for FastForwardBranchActivity.
type FastForwardBranchInput struct {
	RepoPath       string `json:"repo_path"`
	Branch         string `json:"branch"`
	TargetSHA      string `json:"target_sha"`
	ExpectedOldSHA string `json:"expected_old_sha,omitempty"`
}

// MergeInWorktreeInput is the input for MergeInWorktreeActivity.
type MergeInWorktreeInput struct {
	WorktreePath  string `json:"worktree_path"`
	BranchToMerge string `json:"branch_to_merge"`
}

// MergeInWorktreeOutput is the output from MergeInWorktreeActivity.
type MergeInWorktreeOutput struct {
	CommitSHA    string `json:"commit_sha"`
	HasConflicts bool   `json:"has_conflicts"`
}

// GetBranchHeadInput is the input for GetBranchHeadActivity.
type GetBranchHeadInput struct {
	RepoPath string `json:"repo_path"`
	Branch   string `json:"branch"`
}

// GetBranchHeadOutput is the output from GetBranchHeadActivity.
type GetBranchHeadOutput struct {
	SHA string `json:"sha"`
}

// ============================================================================
// Merge Queue Activity Types
// ============================================================================

// EnsureMergeQueueAndSignalInput is the input for EnsureMergeQueueAndSignalActivity.
// It atomically signals the merge queue workflow and starts it if it doesn't exist.
type EnsureMergeQueueAndSignalInput struct {
	ProjectID             string         `json:"project_id"`
	RepositoryPath        string         `json:"repository_path"`
	MainBranch            string         `json:"main_branch"`
	ClaudeConfigPath      string         `json:"claude_config_path"`
	WorkspaceDir          string         `json:"workspace_dir"`
	OrchestratorTaskQueue string         `json:"orchestrator_task_queue"`
	SignalName            string         `json:"signal_name"`
	Item                  MergeQueueItem `json:"item"`
}

// ============================================================================
// PromoteWorkflow Types
// ============================================================================

// PromoteWorkflowInput is the input for the PromoteWorkflow.
type PromoteWorkflowInput struct {
	PromoteRunID        string `json:"promote_run_id"`
	SourceRunID         string `json:"source_run_id"`
	ProjectID           string `json:"project_id"`
	RepositoryPath      string `json:"repository_path"`
	MainBranch          string `json:"main_branch"`
	SourceBranchName    string `json:"source_branch_name"`
	SourceHeadCommitSHA string `json:"source_head_commit_sha"`
	ClaudeConfigPath    string `json:"claude_config_path"`
	WorkspaceDir        string `json:"workspace_dir"`
	OrchestratorTaskQueue string `json:"orchestrator_task_queue"`
}

// PromoteWorkflowOutput is the output from the PromoteWorkflow.
type PromoteWorkflowOutput struct {
	Success        bool   `json:"success"`
	RunID          string `json:"run_id"`
	MergeMethod    string `json:"merge_method"` // "fast-forward" | "clean-merge" | "ai-resolved"
	FinalCommitSHA string `json:"final_commit_sha"`
	Error          string `json:"error,omitempty"`
}

// ============================================================================
// MergeQueueWorkflow Types
// ============================================================================

// MergeQueueWorkflowInput is the input for the MergeQueueWorkflow.
type MergeQueueWorkflowInput struct {
	ProjectID             string           `json:"project_id"`
	RepositoryPath        string           `json:"repository_path"`
	MainBranch            string           `json:"main_branch"`
	ClaudeConfigPath      string           `json:"claude_config_path"`
	WorkspaceDir          string           `json:"workspace_dir"`
	OrchestratorTaskQueue string           `json:"orchestrator_task_queue"`
	PendingItems          []MergeQueueItem `json:"pending_items,omitempty"`
	ProcessedCount        int              `json:"processed_count"`
}

// MergeQueueItem represents a single item in the merge queue.
type MergeQueueItem struct {
	RunID               string    `json:"run_id"`
	SourceBranchName    string    `json:"source_branch_name"`
	SourceHeadCommitSHA string    `json:"source_head_commit_sha"`
	QueuedAt            time.Time `json:"queued_at"`
}

// MergeQueueWorkflowID returns the canonical workflow ID for a project's merge queue.
func MergeQueueWorkflowID(projectID string) string {
	return fmt.Sprintf("merge-queue-%s", projectID)
}

// MergeQueueState is the state returned by the merge queue query.
type MergeQueueState struct {
	Items               []MergeQueueItem `json:"items"`
	CurrentlyProcessing string           `json:"currently_processing,omitempty"`
}

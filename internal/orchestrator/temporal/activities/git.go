// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// GitActivities provides git-related activities
type GitActivities struct {
	manager *services.GitServiceManager
}

// NewGitActivities creates a new instance of GitActivities
func NewGitActivities(manager *services.GitServiceManager) *GitActivities {
	return &GitActivities{
		manager: manager,
	}
}

// CreateWorktreeActivity creates a new git worktree for a task with idempotency
func (a *GitActivities) CreateWorktreeActivity(ctx context.Context, input types.CreateWorktreeActivityInput) (*types.CreateWorktreeActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating git worktree", "taskID", input.TaskID, "branchName", input.BranchName)

	// Record heartbeat for long-running operations
	activity.RecordHeartbeat(ctx, "Creating worktree")

	// Repository path must be explicitly provided
	repoPath := input.RepositoryPath
	if repoPath == "" {
		return nil, fmt.Errorf("repository path must be provided")
	}

	// Get git service handle for this repository
	handle, err := a.manager.GetService(repoPath)
	if err != nil {
		logger.Error("Failed to get git service handle", "error", err)
		return nil, fmt.Errorf("failed to get git service handle: %w", err)
	}
	defer handle.Release()

	var worktreePath string
	var branchName string

	// Use read lock to check if worktree already exists
	err = handle.WithReadLock(ctx, func(gs *services.GitService) error {
		expectedPath := gs.GetWorktreePath(input.TaskID)
		if gs.WorktreeExists(expectedPath) {
			// Verify it's on the correct branch
			currentBranch, err := gs.GetWorktreeBranch(expectedPath)
			if err == nil && currentBranch == input.BranchName {
				logger.Info("Worktree already exists with correct branch", "path", expectedPath, "branch", currentBranch)
				worktreePath = expectedPath
				branchName = currentBranch
				return nil
			}
			// Wrong branch - need to clean up and recreate
			logger.Info("Worktree exists with wrong branch, will recreate", "currentBranch", currentBranch, "expectedBranch", input.BranchName)
		}
		return fmt.Errorf("need to create worktree")
	})

	// If worktree exists with correct branch, return success
	if err == nil && worktreePath != "" {
		return &types.CreateWorktreeActivityOutput{
			WorktreePath: worktreePath,
			BranchName:   branchName,
		}, nil
	}

	// Need to create or recreate worktree - use write lock
	err = handle.WithWriteLock(ctx, func(gs *services.GitService) error {
		// Clean up existing worktree if needed
		expectedPath := gs.GetWorktreePath(input.TaskID)
		if gs.WorktreeExists(expectedPath) {
			worktreeManager := services.NewWorktreeManager(gs, gs.GetWorkDir())
			if err := worktreeManager.CleanupAgentWorktrees(ctx, input.TaskID); err != nil {
				logger.Warn("Failed to cleanup existing worktree", "error", err)
			}
		}

		// Use provided BaseCommitSHA or fallback to current HEAD
		commitSHA := input.BaseCommitSHA
		if commitSHA == "" {
			var err error
			commitSHA, err = gs.GetCurrentCommit(ctx, gs.GetWorkDir())
			if err != nil {
				return fmt.Errorf("failed to get current commit: %w", err)
			}
			logger.Info("No base commit provided, using HEAD", "commit", commitSHA)
		} else {
			logger.Info("Creating worktree from specified base commit", "commit", commitSHA)
		}

		// Create worktree manager and create worktree from commit
		worktreeManager := services.NewWorktreeManager(gs, gs.GetWorkDir())
		path, err := worktreeManager.CreateWorktreeFromCommit(ctx, input.TaskID, commitSHA)
		if err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		// Generate branch name for consistency
		branch := services.GenerateTaskBranchName(input.TaskID)

		worktreePath = path
		branchName = branch
		return nil
	})

	if err != nil {
		logger.Error("Failed to create worktree", "error", err)
		return nil, err
	}

	// Register the worktree in the manager's tracking system
	handle.RegisterWorktree(input.TaskID, worktreePath, branchName)

	logger.Info("Successfully created worktree", "path", worktreePath, "branch", branchName)
	return &types.CreateWorktreeActivityOutput{
		WorktreePath: worktreePath,
		BranchName:   branchName,
	}, nil
}

// RemoveWorktreeActivity removes a git worktree (compensation activity)
func (a *GitActivities) RemoveWorktreeActivity(ctx context.Context, input types.RemoveWorktreeActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Removing git worktree", "worktreePath", input.WorktreePath, "repositoryPath", input.RepositoryPath)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Removing worktree")

	// Use the repository path to get the git service handle
	repoPath := input.RepositoryPath

	// Get git service handle
	handle, err := a.manager.GetService(repoPath)
	if err != nil {
		// If we can't get a handle, check if worktree still exists
		// It might already be removed (idempotency)
		logger.Warn("Failed to get git service handle, assuming worktree already removed", "error", err)
		return nil
	}
	defer handle.Release()

	// Use write lock for removal
	err = handle.WithWriteLock(ctx, func(gs *services.GitService) error {
		// Extract task ID from worktree path
		taskID := services.ExtractTaskIDFromPath(input.WorktreePath)
		if taskID == "" {
			logger.Warn("Could not extract task ID from path, checking if worktree exists", "path", input.WorktreePath)
			// Check if worktree exists - if not, consider it already removed (idempotent)
			if !gs.WorktreeExists(input.WorktreePath) {
				logger.Info("Worktree already removed or doesn't exist", "path", input.WorktreePath)
				return nil
			}
			return fmt.Errorf("could not extract task ID from worktree path: %s", input.WorktreePath)
		}

		// Use worktree manager to cleanup the worktree
		worktreeManager := services.NewWorktreeManager(gs, gs.GetWorkDir())
		if err := worktreeManager.CleanupAgentWorktrees(ctx, taskID); err != nil {
			// Check if worktree doesn't exist - that's OK for idempotency
			if !gs.WorktreeExists(input.WorktreePath) {
				logger.Info("Worktree already removed", "path", input.WorktreePath)
				return nil
			}
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to remove worktree", "error", err)
		return err
	}

	// Extract task ID and unregister from tracking system
	taskID := services.ExtractTaskIDFromPath(input.WorktreePath)
	if taskID != "" {
		handle.UnregisterWorktree(taskID)
	}

	logger.Info("Successfully removed worktree", "path", input.WorktreePath)
	return nil
}

// CommitChangesActivity commits changes in a worktree
func (a *GitActivities) CommitChangesActivity(ctx context.Context, worktreePath, message, agentID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Committing changes", "path", worktreePath, "agentID", agentID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Committing changes")

	// Get git service handle for the worktree path (it's a git repo itself)
	handle, err := a.manager.GetService(worktreePath)
	if err != nil {
		logger.Error("Failed to get git service handle", "error", err)
		return fmt.Errorf("failed to get git service handle: %w", err)
	}
	defer handle.Release()

	// Use write lock for commit
	err = handle.WithWriteLock(ctx, func(gs *services.GitService) error {
		// Commit changes using the git service
		if err := gs.CreateCommit(ctx, worktreePath, message); err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}
		return nil
	})

	if err != nil {
		logger.Error("Failed to commit changes", "error", err)
		return err
	}

	logger.Info("Successfully committed changes")
	return nil
}

// GetWorktreeStatusActivity gets the status of a worktree
func (a *GitActivities) GetWorktreeStatusActivity(ctx context.Context, worktreePath string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting worktree status", "path", worktreePath)

	// Get git service handle for the worktree path
	handle, err := a.manager.GetService(worktreePath)
	if err != nil {
		logger.Error("Failed to get git service handle", "error", err)
		return "", fmt.Errorf("failed to get git service handle: %w", err)
	}
	defer handle.Release()

	var status string

	// Use read lock for status check
	err = handle.WithReadLock(ctx, func(gs *services.GitService) error {
		// Check if working directory is clean
		isClean, err := gs.IsWorkingDirectoryClean(ctx, worktreePath)
		if err != nil {
			return fmt.Errorf("failed to check working directory status: %w", err)
		}

		if isClean {
			status = "clean"
		} else {
			status = "dirty"
		}
		return nil
	})

	if err != nil {
		logger.Error("Failed to get worktree status", "error", err)
		return "", err
	}

	logger.Info("Got worktree status", "status", status)
	return status, nil
}

// GitCommitActivity commits specific files in a repository
func (a *GitActivities) GitCommitActivity(ctx context.Context, input types.GitCommitActivityInput) (*types.GitCommitActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Committing files to git repository", "repositoryPath", input.RepositoryPath, "fileNames", input.FileNames, "commitMessage", input.CommitMessage)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Committing files")

	// Validate input
	if input.RepositoryPath == "" {
		return &types.GitCommitActivityOutput{
			Success: false,
			Error:   "repository path must be provided",
		}, fmt.Errorf("repository path must be provided")
	}

	if len(input.FileNames) == 0 {
		return &types.GitCommitActivityOutput{
			Success: false,
			Error:   "at least one file name must be provided",
		}, fmt.Errorf("at least one file name must be provided")
	}

	if input.CommitMessage == "" {
		return &types.GitCommitActivityOutput{
			Success: false,
			Error:   "commit message must be provided",
		}, fmt.Errorf("commit message must be provided")
	}

	// Get git service handle for this repository
	handle, err := a.manager.GetService(input.RepositoryPath)
	if err != nil {
		logger.Error("Failed to get git service handle", "error", err)
		return &types.GitCommitActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to get git service handle: %v", err),
		}, fmt.Errorf("failed to get git service handle: %w", err)
	}
	defer handle.Release()

	// Variable to capture the commit SHA
	var commitSHA string

	// Use write lock for commit operation
	err = handle.WithWriteLock(ctx, func(gs *services.GitService) error {
		// Use the new CommitSpecificFiles method
		if err := gs.CommitSpecificFiles(ctx, input.RepositoryPath, input.FileNames, input.CommitMessage); err != nil {
			return fmt.Errorf("failed to commit specific files: %w", err)
		}

		// Get the commit SHA after successful commit
		sha, err := gs.GetHeadCommitSHA(ctx, input.RepositoryPath)
		if err != nil {
			logger.Warn("Failed to get commit SHA after commit", "error", err)
			// Don't fail the commit, just log warning
		} else {
			commitSHA = sha
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to commit files", "error", err)
		return &types.GitCommitActivityOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	logger.Info("Successfully committed files to repository", "commitSHA", commitSHA)
	return &types.GitCommitActivityOutput{
		Success:   true,
		Error:     "",
		CommitSHA: commitSHA,
	}, nil
}

// CaptureGitDiffActivity captures git diff information from a repository
func (a *GitActivities) CaptureGitDiffActivity(ctx context.Context, input types.CaptureGitDiffActivityInput) (*types.CaptureGitDiffActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Capturing git diff", "repositoryPath", input.RepositoryPath)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Capturing git diff")

	// Validate input
	if input.RepositoryPath == "" {
		return &types.CaptureGitDiffActivityOutput{
			Success: false,
			Error:   "repository path must be provided",
		}, fmt.Errorf("repository path must be provided")
	}

	// Get git service handle for this repository
	handle, err := a.manager.GetService(input.RepositoryPath)
	if err != nil {
		logger.Error("Failed to get git service handle", "error", err)
		return &types.CaptureGitDiffActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to get git service handle: %v", err),
		}, fmt.Errorf("failed to get git service handle: %w", err)
	}
	defer handle.Release()

	output := &types.CaptureGitDiffActivityOutput{
		Success:      true,
		FilesChanged: make([]string, 0),
	}

	// Use read lock for diff operation
	err = handle.WithReadLock(ctx, func(gs *services.GitService) error {
		// Get full diff output
		diff, err := gs.GetDiff(ctx, input.RepositoryPath)
		if err != nil {
			return fmt.Errorf("failed to get git diff: %w", err)
		}
		output.Diff = diff

		// Get diff stat
		diffStat, err := gs.GetDiffStat(ctx, input.RepositoryPath)
		if err != nil {
			return fmt.Errorf("failed to get git diff stat: %w", err)
		}
		output.DiffStat = diffStat

		// Get changed files
		changedFiles, err := gs.GetChangedFiles(ctx, input.RepositoryPath)
		if err != nil {
			return fmt.Errorf("failed to get changed files: %w", err)
		}
		output.FilesChanged = changedFiles

		// Parse diff stat for insertions/deletions
		insertions, deletions := gs.ParseDiffStat(diffStat)
		output.Insertions = insertions
		output.Deletions = deletions

		// Determine if there are changes
		output.HasChanges = len(changedFiles) > 0 || diff != ""

		return nil
	})

	if err != nil {
		logger.Error("Failed to capture git diff", "error", err)
		return &types.CaptureGitDiffActivityOutput{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	logger.Info("Successfully captured git diff",
		"filesChanged", len(output.FilesChanged),
		"insertions", output.Insertions,
		"deletions", output.Deletions,
		"hasChanges", output.HasChanges)

	return output, nil
}

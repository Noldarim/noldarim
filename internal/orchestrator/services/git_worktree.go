// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/logger"
)

var (
	worktreeLog     *zerolog.Logger
	worktreeLogOnce sync.Once
)

func getWorktreeLog() *zerolog.Logger {
	worktreeLogOnce.Do(func() {
		l := logger.GetGitLogger().With().Str("component", "worktree").Logger()
		worktreeLog = &l
	})
	return worktreeLog
}

// WorktreeManager handles git worktree operations
type WorktreeManager struct {
	gitService *GitService
	baseRepo   string
}

// NewWorktreeManager creates a new worktree manager
func NewWorktreeManager(gitService *GitService, baseRepo string) *WorktreeManager {
	return &WorktreeManager{
		gitService: gitService,
		baseRepo:   baseRepo,
	}
}

// WorktreeInfo represents comprehensive information about a worktree
type WorktreeInfo struct {
	// Git-specific fields
	Path     string
	Branch   string
	Commit   string
	Locked   bool
	Prunable bool

	// Tracking fields for active worktrees
	TaskID       string
	CreatedAt    time.Time
	LastAccessed time.Time
}

// CreateWorktree creates a new worktree for agent isolation
func (wm *WorktreeManager) CreateWorktree(ctx context.Context, agentID, branchName string) (string, error) {
	getWorktreeLog().Debug().Str("agent_id", agentID).Str("branch_name", branchName).Msg("Creating worktree for agent")

	// Validate inputs using GitService validation
	if err := validateAgentID(agentID); err != nil {
		return "", fmt.Errorf("invalid agent ID: %w", err)
	}
	if err := validateBranchName(branchName); err != nil {
		return "", fmt.Errorf("invalid branch name: %w", err)
	}

	// Generate unique worktree path using GitService method
	worktreeName := GenerateTaskWorktreeName(agentID)
	worktreePath := filepath.Join(wm.baseRepo, ".worktrees", worktreeName)

	// Validate the path using GitService
	validatedPath, err := wm.gitService.validateRepoPath(worktreePath)
	if err != nil {
		return "", fmt.Errorf("invalid worktree path: %w", err)
	}
	worktreePath = validatedPath

	// Ensure worktrees directory exists
	worktreesDir := filepath.Dir(worktreePath)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Check if worktree already exists using GitService method
	if wm.gitService.WorktreeExists(worktreePath) {
		getWorktreeLog().Debug().Msgf("Worktree already exists at: %s", worktreePath)
		return worktreePath, nil
	}

	// Use GitService AddWorktree method for consistency
	if err := wm.gitService.AddWorktree(ctx, worktreePath, branchName, ""); err != nil {
		return "", fmt.Errorf("failed to create worktree: %w", err)
	}

	getWorktreeLog().Info().Msgf("Successfully created worktree for agent %s at: %s", agentID, worktreePath)
	return worktreePath, nil
}

// RemoveWorktree removes a worktree and cleans up resources
func (wm *WorktreeManager) RemoveWorktree(ctx context.Context, worktreePath string) error {
	getWorktreeLog().Debug().Msgf("Removing worktree at: %s", worktreePath)

	// Validate path using GitService
	validatedPath, err := wm.gitService.validateRepoPath(worktreePath)
	if err != nil {
		return fmt.Errorf("invalid worktree path: %w", err)
	}

	// Check if worktree exists using GitService method
	if !wm.gitService.WorktreeExists(validatedPath) {
		getWorktreeLog().Debug().Msgf("Worktree does not exist, skipping removal: %s", validatedPath)
		return nil
	}

	// Use GitService RemoveWorktree method for consistency
	if err := wm.gitService.RemoveWorktree(ctx, validatedPath, true); err != nil {
		// Try to remove manually if git command fails
		getWorktreeLog().Warn().Msgf("Git worktree remove failed: %v. Attempting manual cleanup.", err)

		// Remove directory manually
		if err := os.RemoveAll(validatedPath); err != nil {
			return fmt.Errorf("failed to remove worktree directory: %w", err)
		}

		// Prune worktrees to clean up git references using GitService method
		if err := wm.gitService.PruneWorktrees(ctx); err != nil {
			getWorktreeLog().Warn().Msgf("Failed to prune worktrees: %v", err)
		}
	}

	getWorktreeLog().Info().Msgf("Successfully removed worktree at: %s", validatedPath)
	return nil
}

// ListWorktrees lists all worktrees for the repository
func (wm *WorktreeManager) ListWorktrees(ctx context.Context) ([]WorktreeInfo, error) {
	getWorktreeLog().Debug().Msgf("Listing worktrees for repository: %s", wm.baseRepo)

	// Use GitService method to get porcelain output for detailed parsing
	cmd, err := wm.gitService.buildSafeGitCommand(ctx, wm.baseRepo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to build git command: %w", err)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return wm.parseWorktreeList(string(output))
}

// PruneWorktrees removes stale worktree references
func (wm *WorktreeManager) PruneWorktrees(ctx context.Context) error {
	getWorktreeLog().Debug().Msgf("Pruning worktrees for repository: %s", wm.baseRepo)

	// Delegate to GitService method
	if err := wm.gitService.PruneWorktrees(ctx); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}

	getWorktreeLog().Info().Msgf("Successfully pruned worktrees for repository: %s", wm.baseRepo)
	return nil
}

// CleanupAgentWorktrees removes all worktrees and associated branches for a specific agent
func (wm *WorktreeManager) CleanupAgentWorktrees(ctx context.Context, agentID string) error {
	getWorktreeLog().Debug().Msgf("Cleaning up worktrees for agent: %s", agentID)

	// Validate agentID using GitService validation
	if err := validateAgentID(agentID); err != nil {
		return fmt.Errorf("invalid agent ID: %w", err)
	}

	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	var errors []string
	var branchesToDelete []string

	for _, worktree := range worktrees {
		// Check if worktree belongs to the agent
		if strings.Contains(filepath.Base(worktree.Path), agentID) {
			// Capture branch name before removing worktree
			if worktree.Branch != "" {
				branchesToDelete = append(branchesToDelete, worktree.Branch)
			}

			if err := wm.RemoveWorktree(ctx, worktree.Path); err != nil {
				errors = append(errors, fmt.Sprintf("failed to remove worktree %s: %v", worktree.Path, err))
			}
		}
	}

	// Delete associated branches after worktrees are removed
	// (branches can only be deleted after their worktrees are gone)
	for _, branch := range branchesToDelete {
		if err := wm.gitService.DeleteBranch(ctx, wm.baseRepo, branch); err != nil {
			// Log warning but don't fail - branch deletion is best-effort cleanup
			getWorktreeLog().Warn().Msgf("Failed to delete branch %s: %v", branch, err)
			errors = append(errors, fmt.Sprintf("failed to delete branch %s: %v", branch, err))
		} else {
			getWorktreeLog().Info().Msgf("Successfully deleted branch: %s", branch)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	getWorktreeLog().Info().Msgf("Successfully cleaned up worktrees and branches for agent: %s", agentID)
	return nil
}

// GetWorktreeInfo gets information about a specific worktree
func (wm *WorktreeManager) GetWorktreeInfo(ctx context.Context, worktreePath string) (*WorktreeInfo, error) {
	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Resolve symlinks for comparison
	resolvedWorktreePath, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		resolvedWorktreePath = worktreePath // fallback to original path
	}

	for _, worktree := range worktrees {
		resolvedPath, err := filepath.EvalSymlinks(worktree.Path)
		if err != nil {
			resolvedPath = worktree.Path // fallback to original path
		}

		if resolvedPath == resolvedWorktreePath {
			return &worktree, nil
		}
	}

	return nil, fmt.Errorf("worktree not found: %s", worktreePath)
}

// EnsureWorktreeClean ensures the worktree is in a clean state
func (wm *WorktreeManager) EnsureWorktreeClean(ctx context.Context, worktreePath string) error {
	getWorktreeLog().Debug().Msgf("Ensuring worktree is clean: %s", worktreePath)

	// Validate path using GitService
	validatedPath, err := wm.gitService.validateRepoPath(worktreePath)
	if err != nil {
		return fmt.Errorf("invalid worktree path: %w", err)
	}

	// Check if worktree exists using GitService method
	if !wm.gitService.WorktreeExists(validatedPath) {
		return fmt.Errorf("worktree does not exist: %s", validatedPath)
	}

	// Check working directory status using GitService method
	isClean, err := wm.gitService.IsWorkingDirectoryClean(ctx, validatedPath)
	if err != nil {
		return fmt.Errorf("failed to check working directory status: %w", err)
	}

	if !isClean {
		// Reset to clean state using GitService method
		if err := wm.gitService.runSafeGitCommand(ctx, validatedPath, "reset", "--hard", "HEAD"); err != nil {
			return fmt.Errorf("failed to reset worktree to clean state: %w", err)
		}

		// Clean untracked files using GitService method
		if err := wm.gitService.runSafeGitCommand(ctx, validatedPath, "clean", "-fd"); err != nil {
			return fmt.Errorf("failed to clean untracked files: %w", err)
		}
	}

	getWorktreeLog().Info().Msgf("Worktree is clean: %s", validatedPath)
	return nil
}

// parseWorktreeList parses the output of git worktree list --porcelain
func (wm *WorktreeManager) parseWorktreeList(output string) ([]WorktreeInfo, error) {
	var worktrees []WorktreeInfo
	var current WorktreeInfo

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch ")
			// Strip refs/heads/ prefix if present
			branch = strings.TrimPrefix(branch, "refs/heads/")
			current.Branch = branch
		} else if strings.HasPrefix(line, "HEAD ") {
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		} else if line == "locked" {
			current.Locked = true
		} else if line == "prunable" {
			current.Prunable = true
		}
	}

	// Add the last worktree if exists
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// IsWorktreeValid checks if a worktree is valid and accessible
func (wm *WorktreeManager) IsWorktreeValid(ctx context.Context, worktreePath string) bool {
	// Validate path using GitService first
	validatedPath, err := wm.gitService.validateRepoPath(worktreePath)
	if err != nil {
		return false
	}

	// Check if worktree exists using GitService method
	if !wm.gitService.WorktreeExists(validatedPath) {
		return false
	}

	// Check if it's a valid git repository using GitService method
	if !wm.gitService.isGitRepository(validatedPath) {
		return false
	}

	// Try to get git status using GitService method
	if _, err := wm.gitService.IsWorkingDirectoryClean(ctx, validatedPath); err != nil {
		return false
	}

	return true
}

// CreateWorktreeFromCommit creates a worktree from a specific commit
func (wm *WorktreeManager) CreateWorktreeFromCommit(ctx context.Context, agentID, commitID string) (string, error) {
	getWorktreeLog().Debug().Msgf("Creating worktree for agent %s from commit %s", agentID, commitID)

	// Validate inputs using GitService validation
	if err := validateAgentID(agentID); err != nil {
		return "", fmt.Errorf("invalid agent ID: %w", err)
	}
	if err := validateCommitHash(commitID); err != nil {
		return "", fmt.Errorf("invalid commit ID: %w", err)
	}

	// Generate unique worktree path using GitService method
	worktreeName := GenerateTaskWorktreeName(agentID)
	worktreePath := filepath.Join(wm.baseRepo, ".worktrees", worktreeName)

	// Validate the path using GitService
	validatedPath, err := wm.gitService.validateRepoPath(worktreePath)
	if err != nil {
		return "", fmt.Errorf("invalid worktree path: %w", err)
	}
	worktreePath = validatedPath

	// Ensure worktrees directory exists
	worktreesDir := filepath.Dir(worktreePath)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Check if worktree already exists using GitService method
	if wm.gitService.WorktreeExists(worktreePath) {
		getWorktreeLog().Debug().Msgf("Worktree already exists at: %s", worktreePath)
		return worktreePath, nil
	}

	// Generate branch name using GitService method
	branchName := GenerateTaskBranchName(agentID)

	// Use GitService AddWorktree method for consistency
	if err := wm.gitService.AddWorktree(ctx, worktreePath, branchName, commitID); err != nil {
		return "", fmt.Errorf("failed to create worktree from commit: %w", err)
	}

	getWorktreeLog().Info().Msgf("Successfully created worktree for agent %s from commit %s at: %s", agentID, commitID, worktreePath)
	return worktreePath, nil
}

// FindWorktreeByAgent finds an existing worktree for a specific agent
func (wm *WorktreeManager) FindWorktreeByAgent(ctx context.Context, agentID string) (string, error) {
	getWorktreeLog().Debug().Msgf("Finding worktree for agent: %s", agentID)

	// Validate agentID using GitService validation
	if err := validateAgentID(agentID); err != nil {
		return "", fmt.Errorf("invalid agent ID: %w", err)
	}

	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Look for worktree containing agent ID
	for _, worktree := range worktrees {
		if strings.Contains(filepath.Base(worktree.Path), agentID) {
			// Validate worktree is still valid
			if wm.IsWorktreeValid(ctx, worktree.Path) {
				getWorktreeLog().Debug().Msgf("Found valid worktree for agent %s at: %s", agentID, worktree.Path)
				return worktree.Path, nil
			}
		}
	}

	return "", fmt.Errorf("no valid worktree found for agent: %s", agentID)
}

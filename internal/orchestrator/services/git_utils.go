// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/logger"
)

var (
	utilsLog     *zerolog.Logger
	utilsLogOnce sync.Once
)

func getUtilsLog() *zerolog.Logger {
	utilsLogOnce.Do(func() {
		l := logger.GetGitLogger().With().Str("component", "utils").Logger()
		utilsLog = &l
	})
	return utilsLog
}

// ParseWorktreeGitFile extracts parent repository path from worktree .git file
// Git worktrees contain a .git file (not directory) that points to the parent repo's .git/worktrees/name
// Format: "gitdir: /path/to/parent/.git/worktrees/name"
func ParseWorktreeGitFile(worktreePath string) (string, error) {
	if worktreePath == "" {
		return "", fmt.Errorf("worktree path cannot be empty")
	}

	// Clean and validate the worktree path
	worktreePath = filepath.Clean(worktreePath)

	// Path to the .git file in the worktree
	gitFilePath := filepath.Join(worktreePath, ".git")

	// Check if .git file exists
	if _, err := os.Stat(gitFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("worktree .git file not found at %s", gitFilePath)
	}

	// Read the .git file content
	content, err := os.ReadFile(gitFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read .git file: %w", err)
	}

	// Parse the gitdir line
	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", fmt.Errorf("invalid .git file format: %s", line)
	}

	// Extract the path after "gitdir: "
	gitdirPath := strings.TrimPrefix(line, "gitdir: ")
	gitdirPath = strings.TrimSpace(gitdirPath)

	// The gitdir path points to .git/worktrees/name in the parent repository
	// We need to extract the parent repository path by removing /.git/worktrees/name
	if !strings.Contains(gitdirPath, ".git/worktrees/") {
		return "", fmt.Errorf("unexpected gitdir path format: %s", gitdirPath)
	}

	// Find the position of "/.git/worktrees/" and extract everything before it
	parts := strings.Split(gitdirPath, ".git/worktrees/")
	if len(parts) != 2 {
		return "", fmt.Errorf("could not parse gitdir path: %s", gitdirPath)
	}

	// The parent repository path is everything before /.git/worktrees/
	parentRepoPath := strings.TrimSuffix(parts[0], "/")
	if parentRepoPath == "" {
		return "", fmt.Errorf("could not determine parent repository path from: %s", gitdirPath)
	}

	// Convert to absolute path and validate
	absParentPath, err := filepath.Abs(parentRepoPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for parent repo: %w", err)
	}

	// Verify the parent repository exists and has a .git directory
	parentGitDir := filepath.Join(absParentPath, ".git")
	if _, err := os.Stat(parentGitDir); os.IsNotExist(err) {
		return "", fmt.Errorf("parent repository .git directory not found at %s", parentGitDir)
	}

	getUtilsLog().Debug().
		Str("worktreePath", worktreePath).
		Str("parentRepoPath", absParentPath).
		Msg("Successfully parsed worktree parent repository")

	return absParentPath, nil
}

// ValidateWorktreePath performs basic validation on a worktree path
func ValidateWorktreePath(worktreePath string) error {
	if worktreePath == "" {
		return fmt.Errorf("worktree path cannot be empty")
	}

	// Check for path traversal attempts
	cleanPath := filepath.Clean(worktreePath)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal not allowed: %s", worktreePath)
	}

	// Convert to absolute path for consistency
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Basic existence check
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("worktree path does not exist: %s", absPath)
	}

	return nil
}

// ExtractTaskIDFromWorktreePath attempts to extract a task ID from a worktree path
// This assumes worktree paths follow a specific naming convention
func ExtractTaskIDFromWorktreePath(worktreePath string) string {
	if worktreePath == "" {
		return ""
	}

	// Get the base name of the worktree path
	baseName := filepath.Base(worktreePath)

	// Common patterns for task ID extraction
	// Pattern 1: task-{taskID}
	if strings.HasPrefix(baseName, "task-") {
		return strings.TrimPrefix(baseName, "task-")
	}

	// Pattern 2: worktree-{taskID}
	if strings.HasPrefix(baseName, "worktree-") {
		return strings.TrimPrefix(baseName, "worktree-")
	}

	// Pattern 3: agent-{taskID}
	if strings.HasPrefix(baseName, "agent-") {
		return strings.TrimPrefix(baseName, "agent-")
	}

	// Pattern 4: Just the task ID itself (UUID-like)
	// This is a fallback - assume the basename is the task ID if it looks like a UUID
	if len(baseName) >= 8 && !strings.Contains(baseName, " ") {
		return baseName
	}

	getUtilsLog().Debug().
		Str("worktreePath", worktreePath).
		Str("baseName", baseName).
		Msg("Could not extract task ID from worktree path")

	return ""
}

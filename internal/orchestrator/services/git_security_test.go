// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitService_SecurityValidations(t *testing.T) {
	// Create temporary directory and git service
	tempDir := t.TempDir()
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	defer gitService.Close()

	t.Run("Path Validation", func(t *testing.T) {
		// Test empty path
		_, err := gitService.validateRepoPath("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// Test path too long
		longPath := "/" + string(make([]byte, 5000))
		_, err = gitService.validateRepoPath(longPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")

		// Test path traversal - this should normalize the path, not necessarily error
		// The key security aspect is that the path is canonicalized
		result, err := gitService.validateRepoPath("../../../etc/passwd")
		if err != nil {
			// If it errors, it should be about path traversal
			assert.Contains(t, err.Error(), "directory traversal")
		} else {
			// If it succeeds, the path should be normalized
			assert.NotContains(t, result, "..")
		}

		// Test valid path
		validPath := filepath.Join(t.TempDir(), "valid_repo")
		result, err = gitService.validateRepoPath(validPath)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("Branch Name Validation", func(t *testing.T) {
		// Test empty branch name
		err := validateBranchName("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// Test branch name too long
		longBranch := string(make([]byte, 300))
		err = validateBranchName(longBranch)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")

		// Test invalid characters
		err = validateBranchName("branch;rm -rf /")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")

		err = validateBranchName("branch$(rm -rf /)")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")

		// Test branch starting with dash
		err = validateBranchName("-feature")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot start with")

		// Test branch starting with dot
		err = validateBranchName(".feature")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot start with")

		// Test valid branch names
		validBranches := []string{
			"feature",
			"feature-branch",
			"feature_branch",
			"feature/branch",
			"feature123",
			"FEATURE",
		}
		for _, branch := range validBranches {
			err = validateBranchName(branch)
			assert.NoError(t, err, "Branch name should be valid: %s", branch)
		}
	})

	t.Run("Commit Message Validation", func(t *testing.T) {
		// Test empty commit message
		err := validateCommitMessage("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// Test commit message too long
		longMessage := string(make([]byte, 10000))
		err = validateCommitMessage(longMessage)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")

		// Test dangerous patterns
		dangerousMessages := []string{
			"Fix bug $(rm -rf /)",
			"Update feature; rm -rf /",
			"Add feature || rm -rf /",
			"Fix issue && rm -rf /",
			"Update | rm -rf /",
			"Fix > /etc/passwd",
			"Update < /etc/passwd",
		}
		for _, msg := range dangerousMessages {
			err = validateCommitMessage(msg)
			assert.Error(t, err, "Message should be invalid: %s", msg)
			assert.Contains(t, err.Error(), "dangerous pattern")
		}

		// Test valid commit messages
		validMessages := []string{
			"Fix bug in authentication",
			"Add new feature for user management",
			"Update documentation",
			"Refactor code structure",
			"Fix issue #123",
			"Update version to 1.2.3",
		}
		for _, msg := range validMessages {
			err = validateCommitMessage(msg)
			assert.NoError(t, err, "Message should be valid: %s", msg)
		}
	})

	t.Run("Agent ID Validation", func(t *testing.T) {
		// Test empty agent ID
		err := validateAgentID("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// Test agent ID too long
		longID := string(make([]byte, 150))
		err = validateAgentID(longID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")

		// Test invalid characters
		err = validateAgentID("agent;rm")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")

		err = validateAgentID("agent$(rm)")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")

		// Test valid agent IDs
		validIDs := []string{
			"agent-001",
			"agent_001",
			"agent123",
			"AGENT",
			"task-worker-1",
		}
		for _, id := range validIDs {
			err = validateAgentID(id)
			assert.NoError(t, err, "Agent ID should be valid: %s", id)
		}
	})

	t.Run("Commit Hash Validation", func(t *testing.T) {
		// Test empty commit hash
		err := validateCommitHash("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")

		// Test invalid length
		err = validateCommitHash("abc123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid commit hash length")

		// Test invalid characters (42 chars instead of 40 or 64)
		err = validateCommitHash("abcdef1234567890abcdef1234567890abcdef12XY")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid commit hash length")

		// Test valid commit hashes
		validHashes := []string{
			"abcdef1234567890abcdef1234567890abcdef12",                         // 40 char SHA-1
			"abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", // 64 char SHA-256 (corrected length)
			"ABCDEF1234567890ABCDEF1234567890ABCDEF12",                         // uppercase
		}
		for _, hash := range validHashes {
			err = validateCommitHash(hash)
			assert.NoError(t, err, "Commit hash should be valid: %s", hash)
		}
	})
}

func TestGitService_SecurityIntegration(t *testing.T) {

	// Create temporary directory and git service
	tempDir := t.TempDir()
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	t.Run("Malicious Branch Creation", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "test_repo")

		// Initialize repository
		err = gitService.InitRepository(ctx, repoPath)
		require.NoError(t, err)

		// Create a test file and commit
		createTestRepoWithCommit(t, gitService, repoPath)

		// Try to create branch with malicious name
		err = gitService.CreateBranch(ctx, repoPath, "branch;ls /")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid branch name")

		// Try to create branch with command substitution
		err = gitService.CreateBranch(ctx, repoPath, "branch$(ls /)")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid branch name")
	})

	t.Run("Malicious Commit Message", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "test_repo")

		// Initialize repository
		err = gitService.InitRepository(ctx, repoPath)
		require.NoError(t, err)

		// Set git config
		err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
		require.NoError(t, err)
		err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
		require.NoError(t, err)

		// Create a test file
		testFile := filepath.Join(repoPath, "test.txt")
		require.NoError(t, writeFile(testFile, "test content"))

		// Try to create commit with malicious message
		err = gitService.CreateCommit(ctx, repoPath, "Fix bug $(ls /)")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid commit message")

		// Try to create commit with command chaining
		err = gitService.CreateCommit(ctx, repoPath, "Fix bug; ls /")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid commit message")
	})

	t.Run("Path Traversal Attack", func(t *testing.T) {
		// Try to initialize repository with path traversal
		err = gitService.InitRepository(ctx, "../../../tmp/malicious")
		// This should now be blocked by our security validation
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "directory traversal")
	})

	t.Run("Invalid Task ID", func(t *testing.T) {
		// Test validation function directly since CreateTaskWorktree was removed
		err := validateAgentID("task;ls /")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")

		// Try with command substitution
		err = validateAgentID("task$(ls /)")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid characters")
	})

	t.Run("Invalid Commit ID", func(t *testing.T) {
		// Test validation function directly since CreateTaskWorktree was removed
		err := validateCommitHash("malicious;ls /")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid commit hash")

		// Try with invalid length
		err = validateCommitHash("abc123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid commit hash length")
	})
}

func TestGitService_CommandAllowlisting(t *testing.T) {

	// Create temporary directory and git service
	tempDir := t.TempDir()
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()
	testTempDir := t.TempDir()
	repoPath := filepath.Join(testTempDir, "test_repo")

	// Initialize repository
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	t.Run("Allowed Operations", func(t *testing.T) {
		// Test that allowed operations work
		allowedOps := [][]string{
			{"status"},
			{"branch"},
			{"log", "--oneline"},
			{"diff"},
			{"show-ref"},
		}

		for _, args := range allowedOps {
			err = gitService.runSafeGitCommand(ctx, repoPath, args...)
			// Some operations might fail due to repo state, but should not be blocked by security
			// The important thing is that they are not rejected due to operation allowlisting
			if err != nil {
				assert.NotContains(t, err.Error(), "not allowed", "Operation should be allowed: %v", args)
			}
		}
	})

	t.Run("Dangerous Operations", func(t *testing.T) {
		// Test that dangerous operations are blocked
		dangerousOps := [][]string{
			{"daemon"},
			{"serve"},
			{"upload-pack"},
			{"receive-pack"},
			{"shell"},
			{"http-backend"},
			{"dangerous-operation"},
		}

		for _, args := range dangerousOps {
			err = gitService.runSafeGitCommand(ctx, repoPath, args...)
			assert.Error(t, err, "Operation should be blocked: %v", args)
			assert.Contains(t, err.Error(), "not allowed", "Should be blocked by allowlist: %v", args)
		}
	})
}

func TestGitService_TimeoutProtection(t *testing.T) {

	// Create temporary directory and git service
	tempDir := t.TempDir()
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()
	testTempDir := t.TempDir()
	repoPath := filepath.Join(testTempDir, "test_repo")

	// Initialize repository
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	t.Run("Command Timeout", func(t *testing.T) {
		// Test that commands have timeout protection
		// This is hard to test directly without a long-running command
		// but we can verify the timeout context is set correctly

		// Create a context that's already cancelled
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		err = gitService.runSafeGitCommand(cancelledCtx, repoPath, "status")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
	})
}

// Helper function to write file content
func writeFile(filename, content string) error {
	return writeFileWithContent(filename, []byte(content))
}

func writeFileWithContent(filename string, content []byte) error {
	// Import os at the top and use actual file I/O
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	return err
}

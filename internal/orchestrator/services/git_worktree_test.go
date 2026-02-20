// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorktreeManager_CreateWorktree(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Test creating worktree
	agentID := "agent-001"
	branchName := "feature-test"
	worktreePath, err := worktreeManager.CreateWorktree(ctx, agentID, branchName)
	assert.NoError(t, err)
	assert.NotEmpty(t, worktreePath)

	// Verify worktree exists
	assert.DirExists(t, worktreePath)

	// Verify worktree is valid
	isValid := worktreeManager.IsWorktreeValid(ctx, worktreePath)
	assert.True(t, isValid)

	// Test creating worktree with different branch name to avoid collision
	agentID2 := "agent-002"
	branchName2 := "feature-test-2"
	worktreePath2, err := worktreeManager.CreateWorktree(ctx, agentID2, branchName2)
	assert.NoError(t, err)
	assert.NotEmpty(t, worktreePath2)
	assert.NotEqual(t, worktreePath, worktreePath2)
}

func TestWorktreeManager_RemoveWorktree(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create worktree
	agentID := "agent-001"
	branchName := "feature-test"
	worktreePath, err := worktreeManager.CreateWorktree(ctx, agentID, branchName)
	require.NoError(t, err)

	// Verify worktree exists
	assert.DirExists(t, worktreePath)

	// Test removing worktree
	err = worktreeManager.RemoveWorktree(ctx, worktreePath)
	assert.NoError(t, err)

	// Verify worktree is removed
	assert.NoDirExists(t, worktreePath)

	// Test removing non-existent worktree (should not error)
	err = worktreeManager.RemoveWorktree(ctx, worktreePath)
	assert.NoError(t, err)
}

func TestWorktreeManager_ListWorktrees(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create multiple worktrees
	agentIDs := []string{"agent-001", "agent-002", "agent-003"}
	createdWorktrees := make([]string, 0, len(agentIDs))

	for i, agentID := range agentIDs {
		branchName := fmt.Sprintf("feature-%d", i+1)
		worktreePath, err := worktreeManager.CreateWorktree(ctx, agentID, branchName)
		require.NoError(t, err)
		createdWorktrees = append(createdWorktrees, worktreePath)
	}

	// Test listing worktrees
	worktrees, err := worktreeManager.ListWorktrees(ctx)
	assert.NoError(t, err)
	assert.Len(t, worktrees, len(agentIDs)+1) // +1 for main worktree

	// Verify all created worktrees are listed
	worktreePaths := make([]string, 0, len(worktrees))
	for _, worktree := range worktrees {
		// Use filepath.EvalSymlinks to handle /private vs /var symlinks on macOS
		resolvedPath, _ := filepath.EvalSymlinks(worktree.Path)
		worktreePaths = append(worktreePaths, resolvedPath)
	}

	for _, createdWorktree := range createdWorktrees {
		// Use filepath.EvalSymlinks to handle /private vs /var symlinks on macOS
		resolvedCreatedWorktree, _ := filepath.EvalSymlinks(createdWorktree)
		assert.Contains(t, worktreePaths, resolvedCreatedWorktree)
	}
}

func TestWorktreeManager_PruneWorktrees(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create worktree
	agentID := "agent-001"
	branchName := "feature-test"
	worktreePath, err := worktreeManager.CreateWorktree(ctx, agentID, branchName)
	require.NoError(t, err)

	// Manually remove worktree directory to simulate stale reference
	err = os.RemoveAll(worktreePath)
	require.NoError(t, err)

	// Test pruning worktrees
	err = worktreeManager.PruneWorktrees(ctx)
	assert.NoError(t, err)

	// Verify worktree is no longer listed
	worktrees, err := worktreeManager.ListWorktrees(ctx)
	assert.NoError(t, err)

	for _, worktree := range worktrees {
		assert.NotEqual(t, worktreePath, worktree.Path)
	}
}

func TestWorktreeManager_CleanupAgentWorktrees(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create worktrees for multiple agents
	// Note: Each agent ID produces one worktree with branch name = branchName parameter
	targetAgentID := "agent-001"
	otherAgentID := "agent-002"

	targetWorktree, err := worktreeManager.CreateWorktree(ctx, targetAgentID, "feature-1")
	require.NoError(t, err)

	otherWorktree, err := worktreeManager.CreateWorktree(ctx, otherAgentID, "feature-2")
	require.NoError(t, err)

	// Verify branches exist before cleanup
	branches, err := gitService.ListBranches(ctx, repoPath)
	require.NoError(t, err)
	assert.Contains(t, branches, "feature-1", "feature-1 branch should exist before cleanup")
	assert.Contains(t, branches, "feature-2", "feature-2 branch should exist before cleanup")

	// Test cleanup agent worktrees
	err = worktreeManager.CleanupAgentWorktrees(ctx, targetAgentID)
	assert.NoError(t, err)

	// Verify target agent's worktree is removed
	assert.NoDirExists(t, targetWorktree)

	// Verify other agent's worktree still exists
	assert.DirExists(t, otherWorktree)

	// Verify target agent's branch is deleted
	branches, err = gitService.ListBranches(ctx, repoPath)
	require.NoError(t, err)
	assert.NotContains(t, branches, "feature-1", "feature-1 branch should be deleted after cleanup")

	// Verify other agent's branch still exists
	assert.Contains(t, branches, "feature-2", "feature-2 branch should still exist after cleanup")
}

func TestWorktreeManager_EnsureWorktreeClean(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create worktree
	agentID := "agent-001"
	branchName := "feature-test"
	worktreePath, err := worktreeManager.CreateWorktree(ctx, agentID, branchName)
	require.NoError(t, err)

	// Make changes in worktree
	worktreeTestFile := filepath.Join(worktreePath, "worktree_test.txt")
	err = os.WriteFile(worktreeTestFile, []byte("worktree content"), 0644)
	require.NoError(t, err)

	// Verify worktree is not clean
	isClean, err := gitService.IsWorkingDirectoryClean(ctx, worktreePath)
	require.NoError(t, err)
	assert.False(t, isClean)

	// Test ensuring worktree is clean
	err = worktreeManager.EnsureWorktreeClean(ctx, worktreePath)
	assert.NoError(t, err)

	// Verify worktree is now clean
	isClean, err = gitService.IsWorkingDirectoryClean(ctx, worktreePath)
	assert.NoError(t, err)
	assert.True(t, isClean)

	// Verify untracked file is removed
	assert.NoFileExists(t, worktreeTestFile)
}

func TestWorktreeManager_GetWorktreeInfo(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create worktree
	agentID := "agent-001"
	branchName := "feature-test"
	worktreePath, err := worktreeManager.CreateWorktree(ctx, agentID, branchName)
	require.NoError(t, err)

	// Test getting worktree info
	worktreeInfo, err := worktreeManager.GetWorktreeInfo(ctx, worktreePath)
	assert.NoError(t, err)
	assert.NotNil(t, worktreeInfo)
	// Use filepath.EvalSymlinks to handle /private vs /var symlinks on macOS
	expectedPathResolved, _ := filepath.EvalSymlinks(worktreePath)
	actualPathResolved, _ := filepath.EvalSymlinks(worktreeInfo.Path)
	assert.Equal(t, expectedPathResolved, actualPathResolved)
	assert.Equal(t, branchName, worktreeInfo.Branch)

	// Test getting info for non-existent worktree
	nonExistentPath := filepath.Join(tempDir, "non_existent")
	_, err = worktreeManager.GetWorktreeInfo(ctx, nonExistentPath)
	assert.Error(t, err)
}

func TestWorktreeManager_Integration(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service and worktree manager
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	worktreeManager := NewWorktreeManager(gitService, repoPath)

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Test complete workflow
	agentID := "agent-001"
	branchName := "feature-integration"

	// Create worktree
	worktreePath, err := worktreeManager.CreateWorktree(ctx, agentID, branchName)
	require.NoError(t, err)

	// Verify worktree is valid
	isValid := worktreeManager.IsWorktreeValid(ctx, worktreePath)
	assert.True(t, isValid)

	// Make changes in worktree
	worktreeFile := filepath.Join(worktreePath, "feature.txt")
	err = os.WriteFile(worktreeFile, []byte("feature content"), 0644)
	require.NoError(t, err)

	// Commit changes in worktree
	err = gitService.CreateCommit(ctx, worktreePath, "Add feature")
	require.NoError(t, err)

	// Verify commit was created
	commitHash, err := gitService.getCurrentCommit(ctx, worktreePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, commitHash)

	// Switch to main branch in main repo
	err = gitService.SwitchBranch(ctx, repoPath, "main")
	require.NoError(t, err)

	// Verify feature file doesn't exist in main
	mainFeatureFile := filepath.Join(repoPath, "feature.txt")
	assert.NoFileExists(t, mainFeatureFile)

	// Clean up worktree
	err = worktreeManager.RemoveWorktree(ctx, worktreePath)
	assert.NoError(t, err)

	// Verify worktree is removed
	assert.NoDirExists(t, worktreePath)
}

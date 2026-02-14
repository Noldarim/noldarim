// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/noldarim/noldarim/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitService_InitRepository(t *testing.T) {
	// Use fixture for git service setup
	fixture := WithGitService(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Test initializing new repository
	err := fixture.Service.InitRepository(ctx, fixture.RepoPath)
	assert.NoError(t, err)

	// Check if .git directory exists
	gitDir := filepath.Join(fixture.RepoPath, ".git")
	assert.DirExists(t, gitDir)

	// Test initializing existing repository (should be idempotent)
	err = fixture.Service.InitRepository(ctx, fixture.RepoPath)
	assert.NoError(t, err)
}

func TestGitService_ValidateRepository(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Test validating repository
	state, err := gitService.ValidateRepository(ctx, repoPath)
	assert.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, repoPath, state.RepoPath)

	// Test validating non-existent repository
	nonExistentPath := filepath.Join(tempDir, "non_existent")
	_, err = gitService.ValidateRepository(ctx, nonExistentPath)
	assert.Error(t, err)
}

func TestGitService_CreateCommit(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Test creating commit
	commitMessage := "Initial commit"
	err = gitService.CreateCommit(ctx, repoPath, commitMessage)
	assert.NoError(t, err)

	// Verify commit was created
	commitHash, err := gitService.getCurrentCommit(ctx, repoPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, commitHash)

	// Test creating commit with no changes
	err = gitService.CreateCommit(ctx, repoPath, "No changes")
	assert.NoError(t, err) // Should not error, just no-op
}

func TestGitService_CommitSpecificFiles(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create multiple test files
	testFile1 := filepath.Join(repoPath, "file1.txt")
	testFile2 := filepath.Join(repoPath, "file2.txt")
	testFile3 := filepath.Join(repoPath, "file3.txt")

	err = os.WriteFile(testFile1, []byte("content 1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("content 2"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(testFile3, []byte("content 3"), 0644)
	require.NoError(t, err)

	// Test committing specific files only
	filesToCommit := []string{"file1.txt", "file2.txt"}
	commitMessage := "Add specific files"
	err = gitService.CommitSpecificFiles(ctx, repoPath, filesToCommit, commitMessage)
	assert.NoError(t, err)

	// Verify commit was created
	commitHash, err := gitService.getCurrentCommit(ctx, repoPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, commitHash)

	// Verify that file3.txt is still untracked (not committed)
	// We can check this by trying to commit it specifically
	err = gitService.CommitSpecificFiles(ctx, repoPath, []string{"file3.txt"}, "Add file3")
	assert.NoError(t, err)

	// Test committing with no files specified
	err = gitService.CommitSpecificFiles(ctx, repoPath, []string{}, "Empty commit")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no files specified to commit")

	// Test committing non-existent file
	err = gitService.CommitSpecificFiles(ctx, repoPath, []string{"non-existent.txt"}, "Non-existent file")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file does not exist")

	// Test committing with no changes (all files already committed)
	err = gitService.CommitSpecificFiles(ctx, repoPath, []string{"file1.txt"}, "No changes")
	assert.NoError(t, err) // Should not error, just no-op
}

func TestGitService_CreateBranch(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Test creating new branch
	branchName := "feature-test"
	err = gitService.CreateBranch(ctx, repoPath, branchName)
	assert.NoError(t, err)

	// Verify branch was created and is current
	currentBranch, err := gitService.getCurrentBranch(ctx, repoPath)
	assert.NoError(t, err)
	assert.Equal(t, branchName, currentBranch)

	// Test creating branch that already exists
	err = gitService.CreateBranch(ctx, repoPath, branchName)
	assert.NoError(t, err) // Should not error, just no-op
}

func TestGitService_SwitchBranch(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create a new branch
	branchName := "feature-test"
	err = gitService.CreateBranch(ctx, repoPath, branchName)
	require.NoError(t, err)

	// Switch back to main
	err = gitService.SwitchBranch(ctx, repoPath, "main")
	assert.NoError(t, err)

	// Verify current branch
	currentBranch, err := gitService.getCurrentBranch(ctx, repoPath)
	assert.NoError(t, err)
	assert.Equal(t, "main", currentBranch)

	// Test switching to non-existent branch
	err = gitService.SwitchBranch(ctx, repoPath, "non-existent")
	assert.Error(t, err)
}

func TestGitService_ListBranches(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create multiple branches
	branches := []string{"feature-1", "feature-2", "bugfix-1"}
	for _, branch := range branches {
		err = gitService.CreateBranch(ctx, repoPath, branch)
		require.NoError(t, err)
		err = gitService.SwitchBranch(ctx, repoPath, "main")
		require.NoError(t, err)
	}

	// Test listing branches
	listedBranches, err := gitService.ListBranches(ctx, repoPath)
	assert.NoError(t, err)
	assert.Contains(t, listedBranches, "main")
	for _, branch := range branches {
		assert.Contains(t, listedBranches, branch)
	}
}

func TestGitService_StashAndPop(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Make changes to the file
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	// Test stashing changes
	err = gitService.StashChanges(ctx, repoPath, "Test stash")
	assert.NoError(t, err)

	// Verify working directory is clean
	isClean, err := gitService.IsWorkingDirectoryClean(ctx, repoPath)
	assert.NoError(t, err)
	assert.True(t, isClean)

	// Test popping stash
	err = gitService.PopStash(ctx, repoPath)
	assert.NoError(t, err)

	// Verify changes are restored
	isClean, err = gitService.IsWorkingDirectoryClean(ctx, repoPath)
	assert.NoError(t, err)
	assert.False(t, isClean)
}

func TestGitService_ResetToCommit(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Get initial commit hash
	initialCommit, err := gitService.getCurrentCommit(ctx, repoPath)
	require.NoError(t, err)

	// Make another commit
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Second commit")
	require.NoError(t, err)

	// Test resetting to initial commit
	err = gitService.ResetToCommit(ctx, repoPath, initialCommit, true)
	assert.NoError(t, err)

	// Verify reset worked
	currentCommit, err := gitService.getCurrentCommit(ctx, repoPath)
	assert.NoError(t, err)
	assert.Equal(t, initialCommit, currentCommit)
}

func TestGitService_GetCommitInfo(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	commitMessage := "Initial commit"
	err = gitService.CreateCommit(ctx, repoPath, commitMessage)
	require.NoError(t, err)

	// Get commit hash
	commitHash, err := gitService.getCurrentCommit(ctx, repoPath)
	require.NoError(t, err)

	// Test getting commit message
	retrievedMessage, err := gitService.GetCommitMessage(ctx, repoPath, commitHash)
	assert.NoError(t, err)
	assert.Equal(t, commitMessage, retrievedMessage)

	// Test getting commit author
	author, err := gitService.GetCommitAuthor(ctx, repoPath, commitHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, author)

	// Test getting commit timestamp
	timestamp, err := gitService.GetCommitTimestamp(ctx, repoPath, commitHash)
	assert.NoError(t, err)
	assert.True(t, timestamp.Before(time.Now()))
	assert.True(t, timestamp.After(time.Now().Add(-1*time.Minute)))
}

func TestGitService_CleanWorkingDirectory(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	// Create git service
	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err)
	defer gitService.Close()

	ctx := context.Background()

	// Initialize repository and create initial commit
	err = gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Create a test file and commit
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)

	// Create untracked file
	untrackedFile := filepath.Join(repoPath, "untracked.txt")
	err = os.WriteFile(untrackedFile, []byte("untracked content"), 0644)
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, untrackedFile)

	// Test cleaning working directory
	err = gitService.CleanWorkingDirectory(ctx, repoPath)
	assert.NoError(t, err)

	// Verify untracked file is removed
	assert.NoFileExists(t, untrackedFile)
}

// Helper function to create a git service for testing
// DEPRECATED: Use testutil.WithGitService instead
func createTestGitService(t *testing.T) (*GitService, string, func()) {
	fixture := WithGitService(t)
	return fixture.Service, fixture.RepoPath, fixture.Cleanup
}

// Helper function to create a repository with initial commit
func createTestRepoWithCommit(t *testing.T, gitService *GitService, repoPath string) {
	ctx := context.Background()

	err := gitService.InitRepository(ctx, repoPath)
	require.NoError(t, err)

	// Set git config for the test repository
	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.runSafeGitCommand(ctx, repoPath, "config", "user.email", "test@example.com")
	require.NoError(t, err)

	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Initial commit")
	require.NoError(t, err)
}

func TestGitService_IntegrationWorkflow(t *testing.T) {
	gitService, repoPath, cleanup := createTestGitService(t)
	defer cleanup()

	ctx := context.Background()

	// Create repository and initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Create feature branch
	featureBranch := "feature-test"
	err := gitService.CreateBranch(ctx, repoPath, featureBranch)
	assert.NoError(t, err)

	// Make changes and commit
	testFile := filepath.Join(repoPath, "feature.txt")
	err = os.WriteFile(testFile, []byte("feature content"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, repoPath, "Add feature")
	assert.NoError(t, err)

	// Switch back to main
	err = gitService.SwitchBranch(ctx, repoPath, "main")
	assert.NoError(t, err)

	// Verify feature file doesn't exist in main
	assert.NoFileExists(t, testFile)

	// Switch back to feature branch
	err = gitService.SwitchBranch(ctx, repoPath, featureBranch)
	assert.NoError(t, err)

	// Verify feature file exists
	assert.FileExists(t, testFile)

	// Validate repository state
	state, err := gitService.ValidateRepository(ctx, repoPath)
	assert.NoError(t, err)
	assert.Equal(t, featureBranch, state.Branch)
	assert.True(t, state.IsClean)
}

func TestGitService_CreateBranchIfNotExists(t *testing.T) {
	gitService, repoPath, cleanup := createTestGitService(t)
	defer cleanup()

	ctx := context.Background()

	// Create repository and initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Test creating new branch
	branchName := "new-feature"
	err := gitService.CreateBranchIfNotExists(ctx, repoPath, branchName)
	assert.NoError(t, err)

	// Verify branch was created
	branches, err := gitService.ListBranches(ctx, repoPath)
	assert.NoError(t, err)
	assert.Contains(t, branches, branchName)

	// Test creating branch that already exists (should be idempotent)
	err = gitService.CreateBranchIfNotExists(ctx, repoPath, branchName)
	assert.NoError(t, err)

	// Verify branch still exists
	branches, err = gitService.ListBranches(ctx, repoPath)
	assert.NoError(t, err)
	assert.Contains(t, branches, branchName)
}

func TestGitService_SwitchBranchIfNotCurrent(t *testing.T) {
	gitService, repoPath, cleanup := createTestGitService(t)
	defer cleanup()

	ctx := context.Background()

	// Create repository and initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Create feature branch
	featureBranch := "feature-test"
	err := gitService.CreateBranch(ctx, repoPath, featureBranch)
	require.NoError(t, err)

	// Switch to main
	err = gitService.SwitchBranch(ctx, repoPath, "main")
	require.NoError(t, err)

	// Test switching to feature branch
	err = gitService.SwitchBranchIfNotCurrent(ctx, repoPath, featureBranch)
	assert.NoError(t, err)

	// Verify current branch
	currentBranch, err := gitService.getCurrentBranch(ctx, repoPath)
	assert.NoError(t, err)
	assert.Equal(t, featureBranch, currentBranch)

	// Test switching to same branch (should be idempotent)
	err = gitService.SwitchBranchIfNotCurrent(ctx, repoPath, featureBranch)
	assert.NoError(t, err)

	// Verify still on the same branch
	currentBranch, err = gitService.getCurrentBranch(ctx, repoPath)
	assert.NoError(t, err)
	assert.Equal(t, featureBranch, currentBranch)
}

func TestGitService_CreateTaskWorktree(t *testing.T) {
	gitService, repoPath, cleanup := createTestGitService(t)
	defer cleanup()

	ctx := context.Background()

	// Create repository and initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Get initial commit hash
	initialCommit, err := gitService.getCurrentCommit(ctx, repoPath)
	require.NoError(t, err)

	// Set the workDir to the repo path for testing
	gitService.workDir = repoPath

	// Test creating task worktree using WorktreeManager (since deprecated method was removed)
	taskID := "task-001"
	worktreeManager := NewWorktreeManager(gitService, repoPath)
	worktreePath, err := worktreeManager.CreateWorktreeFromCommit(ctx, taskID, initialCommit)
	assert.NoError(t, err)
	assert.NotEmpty(t, worktreePath)

	// Verify worktree exists
	assert.DirExists(t, worktreePath)

	// Verify branch name format (should be just task-{taskID} without timestamp)
	expectedBranchName := fmt.Sprintf("task-%s", taskID)
	branchName := GenerateTaskBranchName(taskID)
	assert.Equal(t, expectedBranchName, branchName)

	// Test with empty taskID
	_, err = worktreeManager.CreateWorktreeFromCommit(ctx, "", initialCommit)
	assert.Error(t, err)

	// Test with empty commit
	_, err = worktreeManager.CreateWorktreeFromCommit(ctx, taskID, "")
	assert.Error(t, err)
}

func TestGitService_CleanupTaskWorktree(t *testing.T) {
	gitService, repoPath, cleanup := createTestGitService(t)
	defer cleanup()

	ctx := context.Background()

	// Create repository and initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Get initial commit hash
	initialCommit, err := gitService.getCurrentCommit(ctx, repoPath)
	require.NoError(t, err)

	// Set the workDir to the repo path for testing
	gitService.workDir = repoPath

	// Create task worktree using WorktreeManager
	taskID := "task-001"
	worktreeManager := NewWorktreeManager(gitService, repoPath)
	worktreePath, err := worktreeManager.CreateWorktreeFromCommit(ctx, taskID, initialCommit)
	require.NoError(t, err)

	// Verify worktree exists
	assert.DirExists(t, worktreePath)

	// Test cleanup using WorktreeManager
	err = worktreeManager.CleanupAgentWorktrees(ctx, taskID)
	assert.NoError(t, err)

	// Test cleanup with empty taskID
	err = worktreeManager.CleanupAgentWorktrees(ctx, "")
	assert.Error(t, err)
}

func TestGitService_GetTaskWorktreePath(t *testing.T) {
	gitService, repoPath, cleanup := createTestGitService(t)
	defer cleanup()

	ctx := context.Background()

	// Create repository and initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Get initial commit hash
	initialCommit, err := gitService.getCurrentCommit(ctx, repoPath)
	require.NoError(t, err)

	// Set the workDir to the repo path for testing
	gitService.workDir = repoPath

	// Create task worktree using WorktreeManager
	taskID := "task-001"
	worktreeManager := NewWorktreeManager(gitService, repoPath)
	expectedPath, err := worktreeManager.CreateWorktreeFromCommit(ctx, taskID, initialCommit)
	require.NoError(t, err)

	// Test finding worktree path using WorktreeManager
	actualPath, err := worktreeManager.FindWorktreeByAgent(ctx, taskID)
	assert.NoError(t, err)
	// Use filepath.EvalSymlinks to handle /private vs /var symlinks on macOS
	expectedPathResolved, _ := filepath.EvalSymlinks(expectedPath)
	actualPathResolved, _ := filepath.EvalSymlinks(actualPath)
	assert.Equal(t, expectedPathResolved, actualPathResolved)

	// Test with empty taskID
	_, err = worktreeManager.FindWorktreeByAgent(ctx, "")
	assert.Error(t, err)

	// Test with non-existent taskID
	_, err = worktreeManager.FindWorktreeByAgent(ctx, "non-existent-task")
	assert.Error(t, err)
}

func TestGitService_TaskWorkflowIntegration(t *testing.T) {
	gitService, repoPath, cleanup := createTestGitService(t)
	defer cleanup()

	ctx := context.Background()

	// Create repository and initial commit
	createTestRepoWithCommit(t, gitService, repoPath)

	// Get initial commit hash
	initialCommit, err := gitService.getCurrentCommit(ctx, repoPath)
	require.NoError(t, err)

	// Set the workDir to the repo path for testing
	gitService.workDir = repoPath

	// Test complete task workflow
	taskID := "integration-task"

	// 1. Create task worktree using WorktreeManager
	worktreeManager := NewWorktreeManager(gitService, repoPath)
	worktreePath, err := worktreeManager.CreateWorktreeFromCommit(ctx, taskID, initialCommit)
	require.NoError(t, err)
	assert.DirExists(t, worktreePath)
	branchName := GenerateTaskBranchName(taskID)

	// 2. Make changes in worktree
	taskFile := filepath.Join(worktreePath, "task-work.txt")
	err = os.WriteFile(taskFile, []byte("task work content"), 0644)
	require.NoError(t, err)

	// 3. Commit changes in worktree
	err = gitService.CreateCommit(ctx, worktreePath, "Task work commit")
	require.NoError(t, err)

	// 4. Verify task worktree path can be retrieved
	retrievedPath, err := worktreeManager.FindWorktreeByAgent(ctx, taskID)
	require.NoError(t, err)
	// Use filepath.EvalSymlinks to handle /private vs /var symlinks on macOS
	worktreePathResolved, _ := filepath.EvalSymlinks(worktreePath)
	retrievedPathResolved, _ := filepath.EvalSymlinks(retrievedPath)
	assert.Equal(t, worktreePathResolved, retrievedPathResolved)

	// 5. Verify main repo is not affected
	mainTaskFile := filepath.Join(repoPath, "task-work.txt")
	assert.NoFileExists(t, mainTaskFile)

	// 6. Cleanup task worktree
	err = worktreeManager.CleanupAgentWorktrees(ctx, taskID)
	require.NoError(t, err)

	// 7. Verify worktree is removed
	assert.NoDirExists(t, worktreePath)

	// 8. Verify getting worktree path fails after cleanup
	_, err = worktreeManager.FindWorktreeByAgent(ctx, taskID)
	assert.Error(t, err)

	// 9. Verify repository is still valid
	state, err := gitService.ValidateRepository(ctx, repoPath)
	require.NoError(t, err)
	assert.True(t, state.IsClean)

	t.Logf("Task workflow integration test completed successfully")
	t.Logf("Task ID: %s", taskID)
	t.Logf("Branch: %s", branchName)
	t.Logf("Initial commit: %s", initialCommit)
}

func TestGitService_SetConfig(t *testing.T) {
	// Use fixture for git service setup
	fixture := WithGitService(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Initialize repository
	err := fixture.Service.InitRepository(ctx, fixture.RepoPath)
	require.NoError(t, err)

	tests := []struct {
		name        string
		key         string
		value       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid user.name config",
			key:         "user.name",
			value:       "Test User",
			expectError: false,
		},
		{
			name:        "Valid user.email config",
			key:         "user.email",
			value:       "test@example.com",
			expectError: false,
		},
		{
			name:        "Valid core.editor config",
			key:         "core.editor",
			value:       "vim",
			expectError: false,
		},
		{
			name:        "Valid custom.section.key config",
			key:         "custom.section.key",
			value:       "custom value",
			expectError: false,
		},
		{
			name:        "Empty key",
			key:         "",
			value:       "value",
			expectError: true,
			errorMsg:    "config key cannot be empty",
		},
		{
			name:        "Invalid key format with special chars",
			key:         "user@name",
			value:       "Test User",
			expectError: true,
			errorMsg:    "invalid config key format",
		},
		{
			name:        "Key starting with number",
			key:         "1user.name",
			value:       "Test User",
			expectError: true,
			errorMsg:    "invalid config key format",
		},
		{
			name:        "Key too long",
			key:         "a" + string(make([]byte, 250)),
			value:       "value",
			expectError: true,
			errorMsg:    "config key too long",
		},
		{
			name:        "Value too long",
			key:         "test.key",
			value:       string(make([]byte, 1001)),
			expectError: true,
			errorMsg:    "config value too long",
		},
		{
			name:        "Value with dangerous pattern - command substitution",
			key:         "test.key",
			value:       "$(malicious command)",
			expectError: true,
			errorMsg:    "config value contains dangerous pattern",
		},
		{
			name:        "Value with dangerous pattern - semicolon",
			key:         "test.key",
			value:       "normal; rm -rf /",
			expectError: true,
			errorMsg:    "config value contains dangerous pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fixture.Service.SetConfig(ctx, fixture.RepoPath, tt.key, tt.value)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)

				// Verify config was actually set by reading it back
				gitConfigCmd := exec.Command("git", "-C", fixture.RepoPath, "config", "--get", tt.key)
				output, err := gitConfigCmd.Output()
				require.NoError(t, err)
				assert.Equal(t, tt.value, strings.TrimSpace(string(output)))
			}
		})
	}
}

func TestGitService_SetConfig_InvalidPath(t *testing.T) {
	// Use fixture for git service setup
	fixture := WithGitService(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	tests := []struct {
		name     string
		repoPath string
		errorMsg string
	}{
		{
			name:     "Non-existent path",
			repoPath: "/non/existent/path",
			errorMsg: "failed to set git config",
		},
		{
			name:     "Empty path",
			repoPath: "",
			errorMsg: "invalid repository path",
		},
		{
			name:     "Path too long",
			repoPath: "/" + string(make([]byte, 5000)),
			errorMsg: "invalid repository path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fixture.Service.SetConfig(ctx, tt.repoPath, "user.name", "Test User")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

// Tests for new helper methods

func TestGitService_GetWorktreePath(t *testing.T) {
	// Create temp directories for testing
	tempDir := t.TempDir()
	customWorktreeDir := t.TempDir()

	tests := []struct {
		name           string
		taskID         string
		workDir        string
		configBasePath string
		wantContains   string
		wantEmpty      bool
	}{
		{
			name:         "Valid task ID with default config",
			taskID:       "task-123",
			workDir:      tempDir,
			wantContains: filepath.Join(tempDir, ".worktrees", "task-task-123"),
		},
		{
			name:           "Valid task ID with config base path",
			taskID:         "task-456",
			workDir:        tempDir,
			configBasePath: customWorktreeDir,
			wantContains:   filepath.Join(customWorktreeDir, ".worktrees", "task-task-456"),
		},
		{
			name:      "Empty task ID",
			taskID:    "",
			workDir:   tempDir,
			wantEmpty: true,
		},
		{
			name:      "Invalid task ID with special chars",
			taskID:    "task@123",
			workDir:   tempDir,
			wantEmpty: true,
		},
		{
			name:      "Task ID too long",
			taskID:    string(make([]byte, 101)),
			workDir:   tempDir,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create service with optional config
			gitService, err := NewGitService(tt.workDir, true)
			require.NoError(t, err)
			defer gitService.Close()

			gitService.workDir = tt.workDir

			if tt.configBasePath != "" {
				cfg := &config.AppConfig{
					Git: config.GitConfig{
						WorktreeBasePath: tt.configBasePath,
					},
				}
				gitService.config = cfg
			}

			result := gitService.GetWorktreePath(tt.taskID)

			if tt.wantEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				assert.Contains(t, result, tt.wantContains)
				// Verify it's an absolute path
				assert.True(t, filepath.IsAbs(result))
			}
		})
	}
}

func TestGitService_WorktreeExists(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		setup      func(path string)
		path       string
		wantExists bool
	}{
		{
			name: "Valid worktree exists",
			setup: func(path string) {
				// Create directory structure for worktree
				os.MkdirAll(path, 0755)
				// Create .git file (not directory) which indicates a worktree
				gitFile := filepath.Join(path, ".git")
				os.WriteFile(gitFile, []byte("gitdir: /path/to/main/.git/worktrees/test"), 0644)
			},
			path:       filepath.Join(tempDir, "valid-worktree"),
			wantExists: true,
		},
		{
			name: "Regular git repository (not worktree)",
			setup: func(path string) {
				// Create directory with .git directory (not file)
				os.MkdirAll(filepath.Join(path, ".git"), 0755)
			},
			path:       filepath.Join(tempDir, "regular-repo"),
			wantExists: false,
		},
		{
			name:       "Non-existent path",
			setup:      func(path string) {},
			path:       filepath.Join(tempDir, "non-existent"),
			wantExists: false,
		},
		{
			name:       "Empty path",
			setup:      func(path string) {},
			path:       "",
			wantExists: false,
		},
		{
			name:       "Path with traversal",
			setup:      func(path string) {},
			path:       filepath.Join(tempDir, "..", "invalid"),
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			if tt.setup != nil {
				tt.setup(tt.path)
			}

			gitService, err := NewGitService(tempDir, true)
			require.NoError(t, err)
			defer gitService.Close()

			result := gitService.WorktreeExists(tt.path)
			assert.Equal(t, tt.wantExists, result)
		})
	}
}

func TestGitService_GetWorktreeBranch(t *testing.T) {
	// Create git service and test repository
	fixture := WithGitService(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Initialize main repository
	createTestRepoWithCommit(t, fixture.Service, fixture.RepoPath)

	// Get initial commit for worktree creation
	initialCommit, err := fixture.Service.getCurrentCommit(ctx, fixture.RepoPath)
	require.NoError(t, err)

	// Set workDir for worktree creation
	fixture.Service.workDir = fixture.RepoPath

	// Create a worktree using WorktreeManager
	taskID := "test-task"
	worktreeManager := NewWorktreeManager(fixture.Service, fixture.RepoPath)
	worktreePath, err := worktreeManager.CreateWorktreeFromCommit(ctx, taskID, initialCommit)
	require.NoError(t, err)
	defer worktreeManager.CleanupAgentWorktrees(ctx, taskID)
	branchName := GenerateTaskBranchName(taskID)

	tests := []struct {
		name       string
		path       string
		wantBranch string
		wantError  bool
		errorMsg   string
		setupFunc  func()
	}{
		{
			name:       "Valid worktree path",
			path:       worktreePath,
			wantBranch: branchName,
		},
		{
			name:      "Empty path",
			path:      "",
			wantError: true,
			errorMsg:  "invalid worktree path",
		},
		{
			name:      "Path with traversal",
			path:      "../../../etc/passwd",
			wantError: true,
			errorMsg:  "path contains invalid directory traversal",
		},
		{
			name:      "Non-existent path",
			path:      filepath.Join(fixture.RepoPath, "non-existent"),
			wantError: true,
			errorMsg:  "failed to get worktree branch",
		},
		{
			name:       "Detached HEAD state",
			path:       worktreePath,
			wantBranch: fmt.Sprintf("(detached HEAD at %s)", initialCommit[:8]),
			setupFunc: func() {
				// Checkout specific commit to create detached HEAD
				cmd := exec.Command("git", "-C", worktreePath, "checkout", initialCommit)
				err := cmd.Run()
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			branch, err := fixture.Service.GetWorktreeBranch(tt.path)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantBranch, branch)
			}
		})
	}
}

func TestGitService_ExtractTaskIDFromPath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantTaskID string
	}{
		{
			name:       "Standard task worktree path",
			path:       "/workspace/.worktrees/task-ABC123",
			wantTaskID: "ABC123",
		},
		{
			name:       "Task ID with hyphens",
			path:       "/workspace/.worktrees/task-my-task-id",
			wantTaskID: "my-task-id",
		},
		{
			name:       "Task ID with underscores",
			path:       "/workspace/.worktrees/task-my_task_123",
			wantTaskID: "my_task_123",
		},
		{
			name:       "Path without task prefix",
			path:       "/workspace/.worktrees/ABC123",
			wantTaskID: "",
		},
		{
			name:       "Empty path",
			path:       "",
			wantTaskID: "",
		},
		{
			name:       "Path with timestamp pattern (future use)",
			path:       "/workspace/.worktrees/ABC123-1234567890",
			wantTaskID: "ABC123",
		},
		{
			name:       "Invalid task ID in timestamp pattern",
			path:       "/workspace/.worktrees/@invalid-1234567890",
			wantTaskID: "",
		},
		{
			name:       "Root path",
			path:       "/",
			wantTaskID: "",
		},
		{
			name:       "Just task prefix",
			path:       "task-",
			wantTaskID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTaskIDFromPath(tt.path)
			assert.Equal(t, tt.wantTaskID, result)
		})
	}
}

func TestGitService_HelperMethodsIntegration(t *testing.T) {
	// Create git service and test repository
	fixture := WithGitService(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Initialize main repository
	createTestRepoWithCommit(t, fixture.Service, fixture.RepoPath)

	// Get initial commit
	initialCommit, err := fixture.Service.getCurrentCommit(ctx, fixture.RepoPath)
	require.NoError(t, err)

	// Set workDir for worktree creation
	fixture.Service.workDir = fixture.RepoPath

	// Test complete flow
	taskID := "integration-test-123"

	// 1. Get expected worktree path
	expectedPath := fixture.Service.GetWorktreePath(taskID)
	assert.NotEmpty(t, expectedPath)
	assert.Contains(t, expectedPath, "task-"+taskID)

	// 2. Verify worktree doesn't exist yet
	assert.False(t, fixture.Service.WorktreeExists(expectedPath))

	// 3. Create worktree using WorktreeManager
	worktreeManager := NewWorktreeManager(fixture.Service, fixture.RepoPath)
	actualPath, err := worktreeManager.CreateWorktreeFromCommit(ctx, taskID, initialCommit)
	require.NoError(t, err)
	branchName := GenerateTaskBranchName(taskID)

	// 4. Verify paths are in the same directory (worktree manager adds timestamp)
	expectedDir := filepath.Dir(expectedPath)
	actualDir := filepath.Dir(actualPath)
	expectedDirResolved, _ := filepath.EvalSymlinks(expectedDir)
	actualDirResolved, _ := filepath.EvalSymlinks(actualDir)
	assert.Equal(t, expectedDirResolved, actualDirResolved)

	// 5. Verify worktree now exists
	assert.True(t, fixture.Service.WorktreeExists(actualPath))

	// 6. Get branch of worktree
	branch, err := fixture.Service.GetWorktreeBranch(actualPath)
	assert.NoError(t, err)
	assert.Equal(t, branchName, branch)

	// 7. Extract task ID from path (worktree manager creates paths with format "{taskID}-{timestamp}")
	extractedID := ExtractTaskIDFromPath(actualPath)
	// The fixed logic should now correctly extract the full task ID
	assert.Equal(t, taskID, extractedID)

	// 8. Cleanup
	err = worktreeManager.CleanupAgentWorktrees(ctx, taskID)
	assert.NoError(t, err)

	// 9. Verify worktree no longer exists
	assert.False(t, fixture.Service.WorktreeExists(actualPath))
}

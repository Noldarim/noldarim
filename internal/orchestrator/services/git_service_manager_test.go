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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/noldarim/noldarim/internal/orchestrator/database"
)

// TestGitServiceManager_BasicFunctionality tests basic operations
func TestGitServiceManager_BasicFunctionality(t *testing.T) {
	manager := NewGitServiceManager(nil)
	defer manager.Close()

	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "test-repo-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	ctx := context.Background()
	err = gitService.InitRepository(ctx, tempDir)
	require.NoError(t, err)
	gitService.Close()

	// Test getting a service
	handle1, err := manager.GetService(tempDir)
	require.NoError(t, err)
	require.NotNil(t, handle1)
	defer handle1.Release()

	// Test getting the same service returns the same instance
	handle2, err := manager.GetService(tempDir)
	require.NoError(t, err)
	require.NotNil(t, handle2)
	defer handle2.Release()

	// Verify it's the same underlying repository
	assert.Equal(t, handle1.repo, handle2.repo)

	// Verify reference count increased
	assert.Equal(t, int32(2), atomic.LoadInt32(&handle1.repo.refCount))
}

// TestRaceCondition_WithoutManager demonstrates race conditions without GitServiceManager
func TestRaceCondition_WithoutManager(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "test-race-without-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repo
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)

	ctx := context.Background()
	err = gitService.InitRepository(ctx, tempDir)
	require.NoError(t, err)

	// Create initial file and commit
	testFile := filepath.Join(tempDir, "counter.txt")
	err = os.WriteFile(testFile, []byte("0"), 0644)
	require.NoError(t, err)

	err = gitService.CreateCommit(ctx, tempDir, "Initial commit")
	require.NoError(t, err)

	// Track branches created successfully
	var successCount int32
	var errorCount int32

	// Run concurrent operations WITHOUT protection
	numGoroutines := 50
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			branchName := fmt.Sprintf("branch-%d", id)

			// Try to create and switch to a branch
			err := gitService.CreateBranch(ctx, tempDir, branchName)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			// Try to switch to the branch
			err = gitService.SwitchBranch(ctx, tempDir, branchName)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			// Try to modify the file and commit
			content := fmt.Sprintf("%d", id)
			err = os.WriteFile(testFile, []byte(content), 0644)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			err = gitService.CreateCommit(ctx, tempDir, fmt.Sprintf("Update from goroutine %d", id))
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	// We expect to see errors due to race conditions
	t.Logf("Without manager - Success: %d, Errors: %d", successCount, errorCount)

	// In a race condition scenario, we expect some operations to fail
	// due to concurrent modifications
	assert.Greater(t, int(errorCount), 0, "Expected errors due to race conditions")
	assert.Less(t, int(successCount), numGoroutines, "Expected not all operations to succeed")
}

// TestRaceCondition_WithManager demonstrates race condition prevention with GitServiceManager
func TestRaceCondition_WithManager(t *testing.T) {
	manager := NewGitServiceManager(nil)
	defer manager.Close()

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "test-race-with-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	ctx := context.Background()
	err = gitService.InitRepository(ctx, tempDir)
	require.NoError(t, err)
	gitService.Close()

	// Initialize git repo using manager
	handle, err := manager.GetService(tempDir)
	require.NoError(t, err)
	handle.Release()

	// Create initial file and commit
	testFile := filepath.Join(tempDir, "counter.txt")
	err = os.WriteFile(testFile, []byte("0"), 0644)
	require.NoError(t, err)

	handle, err = manager.GetService(tempDir)
	require.NoError(t, err)
	err = handle.WithWriteLock(ctx, func(gs *GitService) error {
		return gs.CreateCommit(ctx, tempDir, "Initial commit")
	})
	require.NoError(t, err)
	handle.Release()

	// Track operations
	var successCount int32
	var errorCount int32

	// Run concurrent operations WITH protection
	numGoroutines := 50
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Get handle for this goroutine
			handle, err := manager.GetService(tempDir)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}
			defer handle.Release()

			branchName := fmt.Sprintf("branch-%d", id)

			// Create branch with write lock
			err = handle.WithWriteLock(ctx, func(gs *GitService) error {
				return gs.CreateBranch(ctx, tempDir, branchName)
			})
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			// Switch to branch with write lock
			err = handle.WithWriteLock(ctx, func(gs *GitService) error {
				return gs.SwitchBranch(ctx, tempDir, branchName)
			})
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			// Modify file and commit with write lock
			err = handle.WithWriteLock(ctx, func(gs *GitService) error {
				// Write file
				content := fmt.Sprintf("%d", id)
				if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
					return err
				}

				// Commit changes
				return gs.CreateCommit(ctx, tempDir, fmt.Sprintf("Update from goroutine %d", id))
			})
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	// With proper locking, all operations should succeed
	t.Logf("With manager - Success: %d, Errors: %d", successCount, errorCount)

	assert.Equal(t, int32(numGoroutines), successCount, "All operations should succeed with proper locking")
	assert.Equal(t, int32(0), errorCount, "No errors should occur with proper locking")

	// Verify all branches were created
	handle, err = manager.GetService(tempDir)
	require.NoError(t, err)
	defer handle.Release()

	var branches []string
	err = handle.WithReadLock(ctx, func(gs *GitService) error {
		branches, err = gs.ListBranches(ctx, tempDir)
		return err
	})
	require.NoError(t, err)

	// Should have main/master + all created branches
	assert.GreaterOrEqual(t, len(branches), numGoroutines, "All branches should be created")
}

// TestGitServiceManager_ConcurrentReadOperations tests that multiple reads can happen simultaneously
func TestGitServiceManager_ConcurrentReadOperations(t *testing.T) {
	manager := NewGitServiceManager(nil)
	defer manager.Close()

	tempDir, err := os.MkdirTemp("", "test-concurrent-reads-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	ctx := context.Background()
	err = gitService.InitRepository(ctx, tempDir)
	require.NoError(t, err)
	gitService.Close()

	// Initialize repo
	handle, err := manager.GetService(tempDir)
	require.NoError(t, err)

	err = handle.WithWriteLock(ctx, func(gs *GitService) error {
		// Create an initial commit so the repository has a HEAD
		testFile := filepath.Join(tempDir, "README.md")
		if err := os.WriteFile(testFile, []byte("# Test Repository"), 0644); err != nil {
			return err
		}
		if err := gs.runSafeGitCommand(ctx, tempDir, "add", "README.md"); err != nil {
			return err
		}
		return gs.runSafeGitCommand(ctx, tempDir, "commit", "-m", "Initial commit")
	})
	require.NoError(t, err)
	handle.Release()

	// Measure concurrent read performance
	numReaders := 20
	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			handle, err := manager.GetService(tempDir)
			if err != nil {
				t.Errorf("Reader %d: failed to get handle: %v", id, err)
				return
			}
			defer handle.Release()

			// Perform read operation that takes some time
			err = handle.WithReadLock(ctx, func(gs *GitService) error {
				// Simulate some work
				time.Sleep(100 * time.Millisecond)

				// Do actual read
				_, err := gs.getCurrentBranch(ctx, tempDir)
				return err
			})

			if err != nil {
				t.Errorf("Reader %d: read failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// If reads were serialized, it would take at least numReaders * 100ms
	// With concurrent reads, it should be close to just 100ms
	expectedSerialTime := time.Duration(numReaders) * 100 * time.Millisecond

	t.Logf("Concurrent read time: %v (serial would be: %v)", duration, expectedSerialTime)
	assert.Less(t, duration.Milliseconds(), expectedSerialTime.Milliseconds()/2,
		"Concurrent reads should be much faster than serial reads")
}

// TestGitServiceManager_WriteOperationsAreSerialized tests that writes are properly serialized
func TestGitServiceManager_WriteOperationsAreSerialized(t *testing.T) {
	manager := NewGitServiceManager(nil)
	defer manager.Close()

	tempDir, err := os.MkdirTemp("", "test-serial-writes-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	ctx := context.Background()
	err = gitService.InitRepository(ctx, tempDir)
	require.NoError(t, err)
	gitService.Close()

	// Initialize repo
	handle, err := manager.GetService(tempDir)
	require.NoError(t, err)

	err = handle.WithWriteLock(ctx, func(gs *GitService) error {
		if err := gs.InitRepository(ctx, tempDir); err != nil {
			return err
		}
		// Create initial commit
		testFile := filepath.Join(tempDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
			return err
		}
		return gs.CreateCommit(ctx, tempDir, "Initial commit")
	})
	require.NoError(t, err)
	handle.Release()

	// Track order of operations
	var operationOrder []int
	var orderMutex sync.Mutex

	numWriters := 5
	var wg sync.WaitGroup

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			handle, err := manager.GetService(tempDir)
			if err != nil {
				t.Errorf("Writer %d: failed to get handle: %v", id, err)
				return
			}
			defer handle.Release()

			// Perform write operation
			err = handle.WithWriteLock(ctx, func(gs *GitService) error {
				// Record when this operation starts
				orderMutex.Lock()
				operationOrder = append(operationOrder, id)
				orderMutex.Unlock()

				// Create a branch (write operation)
				branchName := fmt.Sprintf("feature-%d", id)
				return gs.CreateBranch(ctx, tempDir, branchName)
			})

			if err != nil {
				t.Errorf("Writer %d: write failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all operations completed
	assert.Equal(t, numWriters, len(operationOrder), "All write operations should complete")

	// Verify branches were created
	handle, err = manager.GetService(tempDir)
	require.NoError(t, err)
	defer handle.Release()

	var branches []string
	err = handle.WithReadLock(ctx, func(gs *GitService) error {
		branches, err = gs.ListBranches(ctx, tempDir)
		return err
	})
	require.NoError(t, err)

	// Should have main/master + all created branches
	assert.GreaterOrEqual(t, len(branches), numWriters, "All branches should be created")
}

// TestGitServiceManager_TimeoutHandling tests timeout behavior
func TestGitServiceManager_TimeoutHandling(t *testing.T) {
	manager := NewGitServiceManager(nil)
	defer manager.Close()

	tempDir, err := os.MkdirTemp("", "test-timeout-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitService, err := NewGitService(tempDir, true)
	require.NoError(t, err)
	ctx := context.Background()
	err = gitService.InitRepository(ctx, tempDir)
	require.NoError(t, err)
	gitService.Close()

	handle, err := manager.GetService(tempDir)
	require.NoError(t, err)
	defer handle.Release()

	// Test timeout on read operation
	shortCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = handle.WithReadLock(shortCtx, func(gs *GitService) error {
		// Simulate long operation
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	assert.Error(t, err, "Should timeout")
	assert.Contains(t, err.Error(), "timed out", "Error should indicate timeout")

	// Test timeout on write operation
	shortCtx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	err = handle.WithWriteLock(shortCtx2, func(gs *GitService) error {
		// Simulate long operation
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	assert.Error(t, err, "Should timeout")
	assert.Contains(t, err.Error(), "timed out", "Error should indicate timeout")
}

// TestGitServiceManager_NewDirectoryInitialization tests that GitServiceManager
// properly initializes a new directory as a git repository with initial commit
func TestGitServiceManager_NewDirectoryInitialization(t *testing.T) {
	// Load test config
	cfg := database.WithInMemoryConfig()
	manager := NewGitServiceManager(cfg)
	defer manager.Close()

	// Create temp directory for testing (no git repo initially)
	tempDir, err := os.MkdirTemp("", "test-new-repo-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Verify .git does not exist initially
	gitPath := filepath.Join(tempDir, ".git")
	_, err = os.Stat(gitPath)
	assert.True(t, os.IsNotExist(err), ".git directory should not exist initially")

	// Get service - this should create the git repo and initial commit
	handle, err := manager.GetService(tempDir)
	require.NoError(t, err)
	defer handle.Release()

	// Verify .git directory was created
	_, err = os.Stat(gitPath)
	assert.NoError(t, err, ".git directory should exist after GitService creation")

	// Verify initial commit was created
	ctx := context.Background()
	err = handle.WithReadLock(ctx, func(gs *GitService) error {
		// Check for HEAD commit
		commit, err := gs.getCurrentCommit(ctx, tempDir)
		if err != nil {
			t.Errorf("Should have initial commit, but got error: %v", err)
			return nil
		}

		assert.NotEmpty(t, commit, "Should have a commit hash")

		// Verify noldarim.md file was created
		noldarimFile := filepath.Join(tempDir, "noldarim.md")
		_, err = os.Stat(noldarimFile)
		assert.NoError(t, err, "noldarim.md should exist after initialization")

		return nil
	})
	require.NoError(t, err)
}

// TestGitServiceManager_EmptyGitRepositoryInitialization tests the bug fix:
// when a directory already contains a .git directory but has no commits,
// GitServiceManager should create an initial commit
func TestGitServiceManager_EmptyGitRepositoryInitialization(t *testing.T) {
	// Load test config
	cfg := database.WithInMemoryConfig()
	manager := NewGitServiceManager(cfg)
	defer manager.Close()

	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "test-empty-git-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Manually initialize git repository (but don't create any commits)
	ctx := context.Background()
	err = exec.Command("git", "init", tempDir).Run()
	require.NoError(t, err)

	// Verify .git exists but no commits
	gitPath := filepath.Join(tempDir, ".git")
	_, err = os.Stat(gitPath)
	assert.NoError(t, err, ".git directory should exist")

	// Verify no commits initially - git rev-parse HEAD should fail in an empty repo
	err = exec.Command("git", "-C", tempDir, "rev-parse", "HEAD").Run()
	assert.Error(t, err, "Should have no commits initially")

	// Get service - this should detect empty repo and create initial commit
	handle, err := manager.GetService(tempDir)
	require.NoError(t, err)
	defer handle.Release()

	// Verify initial commit was created (this is the bug fix verification)
	err = handle.WithReadLock(ctx, func(gs *GitService) error {
		// Check for HEAD commit
		commit, err := gs.getCurrentCommit(ctx, tempDir)
		if err != nil {
			t.Errorf("Should have initial commit after GitService creation, but got error: %v", err)
			return nil
		}

		assert.NotEmpty(t, commit, "Should have a commit hash")

		// Verify noldarim.md file was created
		noldarimFile := filepath.Join(tempDir, "noldarim.md")
		_, err = os.Stat(noldarimFile)
		assert.NoError(t, err, "noldarim.md should exist after initialization")

		// Verify commit message
		out, err := exec.Command("git", "-C", tempDir, "log", "-1", "--pretty=format:%s").Output()
		assert.NoError(t, err)
		assert.Equal(t, "noldarim project initialized", strings.TrimSpace(string(out)))

		return nil
	})
	require.NoError(t, err)
}

// TestGitServiceManager_ExistingRepositoryWithCommits tests that GitServiceManager
// does not modify repositories that already have commits
func TestGitServiceManager_ExistingRepositoryWithCommits(t *testing.T) {
	// Load test config
	cfg := database.WithInMemoryConfig()
	manager := NewGitServiceManager(cfg)
	defer manager.Close()

	// Create temp directory and initialize with existing commits
	tempDir, err := os.MkdirTemp("", "test-existing-repo-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create git repo with existing commit
	ctx := context.Background()
	err = exec.Command("git", "init", tempDir).Run()
	require.NoError(t, err)

	// Create a file and commit it
	testFile := filepath.Join(tempDir, "existing.txt")
	err = os.WriteFile(testFile, []byte("existing content"), 0644)
	require.NoError(t, err)

	err = exec.Command("git", "-C", tempDir, "add", ".").Run()
	require.NoError(t, err)

	err = exec.Command("git", "-C", tempDir, "commit", "-m", "Existing commit").Run()
	require.NoError(t, err)

	// Get original commit hash
	out, err := exec.Command("git", "-C", tempDir, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	originalCommitHash := strings.TrimSpace(string(out))

	// Get service - should NOT modify existing repository
	handle, err := manager.GetService(tempDir)
	require.NoError(t, err)
	defer handle.Release()

	// Verify the commit hash is unchanged
	err = handle.WithReadLock(ctx, func(gs *GitService) error {
		currentCommit, err := gs.getCurrentCommit(ctx, tempDir)
		assert.NoError(t, err)
		assert.Equal(t, originalCommitHash, currentCommit, "Original commit should be preserved")

		// Verify noldarim.md was NOT created (should not modify existing repos)
		noldarimFile := filepath.Join(tempDir, "noldarim.md")
		_, err = os.Stat(noldarimFile)
		assert.True(t, os.IsNotExist(err), "noldarim.md should not be created in existing repos")

		// Verify original file still exists
		_, err = os.Stat(testFile)
		assert.NoError(t, err, "Original file should still exist")

		return nil
	})
	require.NoError(t, err)
}

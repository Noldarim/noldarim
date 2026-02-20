// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"

	"github.com/rs/zerolog"
)

var (
	managerLog     *zerolog.Logger
	managerLogOnce sync.Once
)

func getManagerLog() *zerolog.Logger {
	managerLogOnce.Do(func() {
		l := logger.GetGitLogger().With().Str("component", "manager").Logger()
		managerLog = &l
	})
	return managerLog
}

// GitServiceManager manages GitService instances per repository with concurrency control
type GitServiceManager struct {
	mu            sync.RWMutex
	repositories  map[string]*ManagedRepository // key: canonical repo path
	worktreeCache map[string]string             // key: worktree path, value: repo path
	config        *config.AppConfig
	cleanupTick   *time.Ticker
	stopCleanup   chan struct{}
}

// ManagedRepository wraps a GitService with enhanced state management
type ManagedRepository struct {
	mu              sync.RWMutex
	repoPath        string
	gitService      *GitService
	worktreeManager *WorktreeManager         // Persistent instance
	activeWorktrees map[string]*WorktreeInfo // key: task ID
	lastAccess      time.Time
	refCount        int32
}

// GitServiceHandle provides locked access to GitService methods
type GitServiceHandle struct {
	repo     *ManagedRepository
	repoPath string
	manager  *GitServiceManager
}

// NewGitServiceManager creates a new GitServiceManager
func NewGitServiceManager(cfg *config.AppConfig) *GitServiceManager {
	gsm := &GitServiceManager{
		repositories:  make(map[string]*ManagedRepository),
		worktreeCache: make(map[string]string),
		config:        cfg,
		stopCleanup:   make(chan struct{}),
	}

	// Start cleanup routine
	gsm.startCleanupRoutine()

	return gsm
}

// GetService gets or creates a GitService for the specified repository
func (gsm *GitServiceManager) GetService(repoPath string) (*GitServiceHandle, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}

	// First check if we should create the repository if it doesn't exist
	shouldCreate := false
	if gsm.config != nil {
		shouldCreate = gsm.config.Git.CreateGitRepoForProjectIfNotExist
	}

	// Try to resolve the provided path to the actual repository root
	// This handles cases where repoPath is a worktree or a subdirectory
	canonicalPath, err := gsm.resolveRepoPath(repoPath)
	if err != nil {
		// If resolving fails and we should create, use the provided path as canonical
		if shouldCreate {
			canonicalPath = filepath.Clean(repoPath)
		} else {
			return nil, fmt.Errorf("failed to resolve repository path: %w", err)
		}
	}

	// First try with read lock (fast path for existing repositories)
	gsm.mu.RLock()
	if repo, exists := gsm.repositories[canonicalPath]; exists {
		// Increment reference count
		atomic.AddInt32(&repo.refCount, 1)
		repo.lastAccess = time.Now()
		gsm.mu.RUnlock()

		getManagerLog().Debug().
			Str("repo", canonicalPath).
			Int32("refCount", atomic.LoadInt32(&repo.refCount)).
			Msg("Reusing existing GitService")

		return &GitServiceHandle{
			repo:     repo,
			repoPath: canonicalPath,
			manager:  gsm,
		}, nil
	}
	gsm.mu.RUnlock()

	// Need write lock to create new service
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	// Double-check after acquiring write lock
	if repo, exists := gsm.repositories[canonicalPath]; exists {
		atomic.AddInt32(&repo.refCount, 1)
		repo.lastAccess = time.Now()

		return &GitServiceHandle{
			repo:     repo,
			repoPath: canonicalPath,
			manager:  gsm,
		}, nil
	}

	// Create new GitService
	var gitService *GitService
	var createIfNotExist bool
	if gsm.config != nil {
		createIfNotExist = gsm.config.Git.CreateGitRepoForProjectIfNotExist
		gitService, err = NewGitServiceWithConfig(canonicalPath, gsm.config, createIfNotExist)
	} else {
		createIfNotExist = false // Default to false if no config
		gitService, err = NewGitService(canonicalPath, createIfNotExist)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create GitService: %w", err)
	}

	// Create persistent WorktreeManager
	worktreeManager := NewWorktreeManager(gitService, canonicalPath)

	// Wrap in ManagedRepository
	repo := &ManagedRepository{
		repoPath:        canonicalPath,
		gitService:      gitService,
		worktreeManager: worktreeManager,
		activeWorktrees: make(map[string]*WorktreeInfo),
		lastAccess:      time.Now(),
		refCount:        1,
	}

	gsm.repositories[canonicalPath] = repo

	getManagerLog().Info().
		Str("repo", canonicalPath).
		Int("totalRepositories", len(gsm.repositories)).
		Msg("Created new ManagedRepository")

	return &GitServiceHandle{
		repo:     repo,
		repoPath: canonicalPath,
		manager:  gsm,
	}, nil
}

// WithReadLock executes a function with read lock on the repository
func (h *GitServiceHandle) WithReadLock(ctx context.Context, fn func(*GitService) error) error {
	// Use context for timeout
	done := make(chan error, 1)

	go func() {
		// Acquire read lock with logging
		startTime := time.Now()
		h.repo.mu.RLock()
		defer h.repo.mu.RUnlock()

		lockTime := time.Since(startTime)
		if lockTime > 100*time.Millisecond {
			getManagerLog().Warn().
				Str("repo", h.repoPath).
				Dur("lockTime", lockTime).
				Msg("Slow read lock acquisition")
		}

		// Execute function
		done <- fn(h.repo.gitService)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("read operation timed out: %w", ctx.Err())
	}
}

// WithWriteLock executes a function with write lock on the repository
func (h *GitServiceHandle) WithWriteLock(ctx context.Context, fn func(*GitService) error) error {
	// Use context for timeout
	done := make(chan error, 1)

	go func() {
		// Acquire write lock with logging
		startTime := time.Now()
		h.repo.mu.Lock()
		defer h.repo.mu.Unlock()

		lockTime := time.Since(startTime)
		if lockTime > 100*time.Millisecond {
			getManagerLog().Warn().
				Str("repo", h.repoPath).
				Dur("lockTime", lockTime).
				Msg("Slow write lock acquisition")
		}

		// Execute function
		done <- fn(h.repo.gitService)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("write operation timed out: %w", ctx.Err())
	}
}

// GetGitService returns the underlying GitService for direct access
// This should only be used in legacy code during migration
func (h *GitServiceHandle) GetGitService() *GitService {
	return h.repo.gitService
}

// Release decrements the reference count for this handle
func (h *GitServiceHandle) Release() {
	newCount := atomic.AddInt32(&h.repo.refCount, -1)

	getManagerLog().Debug().
		Str("repo", h.repoPath).
		Int32("refCount", newCount).
		Msg("Released GitService handle")

	if newCount < 0 {
		getManagerLog().Error().
			Str("repo", h.repoPath).
			Int32("refCount", newCount).
			Msg("Reference count went negative!")
	}
}

// startCleanupRoutine starts a background routine to clean up unused services
func (gsm *GitServiceManager) startCleanupRoutine() {
	gsm.cleanupTick = time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-gsm.cleanupTick.C:
				gsm.cleanupUnusedServices()
			case <-gsm.stopCleanup:
				gsm.cleanupTick.Stop()
				return
			}
		}
	}()
}

// cleanupUnusedServices removes repositories that haven't been used recently
func (gsm *GitServiceManager) cleanupUnusedServices() {
	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	threshold := time.Now().Add(-10 * time.Minute)
	toRemove := []string{}

	for path, repo := range gsm.repositories {
		refCount := atomic.LoadInt32(&repo.refCount)
		if refCount == 0 && repo.lastAccess.Before(threshold) {
			toRemove = append(toRemove, path)
		}
	}

	for _, path := range toRemove {
		repo := gsm.repositories[path]
		if err := repo.gitService.Close(); err != nil {
			getManagerLog().Error().
				Str("repo", path).
				Err(err).
				Msg("Failed to close GitService during cleanup")
		}

		// Clean up worktree cache entries for this repository
		for worktreePath, repoPath := range gsm.worktreeCache {
			if repoPath == path {
				delete(gsm.worktreeCache, worktreePath)
			}
		}

		delete(gsm.repositories, path)

		getManagerLog().Info().
			Str("repo", path).
			Msg("Cleaned up unused ManagedRepository")
	}

	if len(toRemove) > 0 {
		getManagerLog().Info().
			Int("cleaned", len(toRemove)).
			Int("remaining", len(gsm.repositories)).
			Msg("Cleanup completed")
	}
}

// Close shuts down the manager and all managed services
func (gsm *GitServiceManager) Close() error {
	// Stop cleanup routine
	close(gsm.stopCleanup)

	gsm.mu.Lock()
	defer gsm.mu.Unlock()

	var errors []error

	// Close all repositories
	for path, repo := range gsm.repositories {
		if err := repo.gitService.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close repository for %s: %w", path, err))
		}
	}

	// Clear the maps
	gsm.repositories = make(map[string]*ManagedRepository)
	gsm.worktreeCache = make(map[string]string)

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	getManagerLog().Info().Msg("GitServiceManager shut down successfully")
	return nil
}

// Stats returns statistics about the manager
func (gsm *GitServiceManager) Stats() map[string]interface{} {
	gsm.mu.RLock()
	defer gsm.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_repositories"] = len(gsm.repositories)
	stats["cached_worktrees"] = len(gsm.worktreeCache)

	activeCount := 0
	totalWorktrees := 0
	for _, repo := range gsm.repositories {
		if atomic.LoadInt32(&repo.refCount) > 0 {
			activeCount++
		}
		totalWorktrees += len(repo.activeWorktrees)
	}
	stats["active_repositories"] = activeCount
	stats["total_active_worktrees"] = totalWorktrees

	return stats
}

// GetRepositoryForWorktree resolves the parent repository path from a worktree path
// Uses a hybrid approach: cache first (fast), then .git file parsing (accurate)
// resolveRepoPath finds the root of the git repository for any given path inside it.
// It handles both worktrees and standard directories.
func (gsm *GitServiceManager) resolveRepoPath(path string) (string, error) {
	// First, try to resolve it as a worktree, as this is more specific.
	repoPath, err := gsm.GetRepositoryForWorktree(path)
	if err == nil {
		return repoPath, nil
	}

	// If it's not a worktree (or resolution failed), find the git dir for a normal path.
	gitService, err := NewGitService(path, false) // Don't create if not exists
	if err != nil {
		return "", fmt.Errorf("path is not a valid git repository or worktree: %w", err)
	}
	defer gitService.Close()

	return gitService.GetWorkDir(), nil
}

// GetRepositoryForWorktree resolves the parent repository path from a worktree path
// Uses a hybrid approach: cache first (fast), then .git file parsing (accurate)
func (gsm *GitServiceManager) GetRepositoryForWorktree(worktreePath string) (string, error) {
	if worktreePath == "" {
		return "", fmt.Errorf("worktree path cannot be empty")
	}

	// Clean the worktree path for consistent cache keys
	cleanWorktreePath := filepath.Clean(worktreePath)

	// 1. Check cache first (fast path)
	gsm.mu.RLock()
	if repoPath, exists := gsm.worktreeCache[cleanWorktreePath]; exists {
		gsm.mu.RUnlock()
		getManagerLog().Debug().
			Str("worktreePath", cleanWorktreePath).
			Str("cachedRepoPath", repoPath).
			Msg("Found repository path in cache")
		return repoPath, nil
	}
	gsm.mu.RUnlock()

	// 2. Parse .git file if not cached (accurate fallback)
	repoPath, err := ParseWorktreeGitFile(cleanWorktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse worktree git file: %w", err)
	}

	// 3. Update cache for future lookups
	gsm.mu.Lock()
	gsm.worktreeCache[cleanWorktreePath] = repoPath
	gsm.mu.Unlock()

	getManagerLog().Debug().
		Str("worktreePath", cleanWorktreePath).
		Str("resolvedRepoPath", repoPath).
		Msg("Resolved repository path and updated cache")

	return repoPath, nil
}

// RegisterWorktree registers a new worktree in the active worktrees map
func (h *GitServiceHandle) RegisterWorktree(taskID, worktreePath, branchName string) {
	h.repo.mu.Lock()
	defer h.repo.mu.Unlock()

	worktreeInfo := &WorktreeInfo{
		Path:         worktreePath,
		Branch:       branchName,
		TaskID:       taskID,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}

	h.repo.activeWorktrees[taskID] = worktreeInfo

	// Update the global worktree cache
	h.manager.mu.Lock()
	h.manager.worktreeCache[worktreePath] = h.repoPath
	h.manager.mu.Unlock()

	getManagerLog().Info().
		Str("taskID", taskID).
		Str("worktreePath", worktreePath).
		Str("branchName", branchName).
		Str("repo", h.repoPath).
		Msg("Registered active worktree")
}

// UnregisterWorktree removes a worktree from the active worktrees map
func (h *GitServiceHandle) UnregisterWorktree(taskID string) {
	h.repo.mu.Lock()
	defer h.repo.mu.Unlock()

	if worktreeInfo, exists := h.repo.activeWorktrees[taskID]; exists {
		// Remove from global worktree cache
		h.manager.mu.Lock()
		delete(h.manager.worktreeCache, worktreeInfo.Path)
		h.manager.mu.Unlock()

		// Remove from active worktrees
		delete(h.repo.activeWorktrees, taskID)

		getManagerLog().Info().
			Str("taskID", taskID).
			Str("worktreePath", worktreeInfo.Path).
			Str("repo", h.repoPath).
			Msg("Unregistered active worktree")
	} else {
		getManagerLog().Warn().
			Str("taskID", taskID).
			Str("repo", h.repoPath).
			Msg("Attempted to unregister non-existent worktree")
	}
}

// GetActiveWorktrees returns a copy of the active worktrees map
func (h *GitServiceHandle) GetActiveWorktrees() map[string]*WorktreeInfo {
	h.repo.mu.RLock()
	defer h.repo.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*WorktreeInfo)
	for taskID, worktreeInfo := range h.repo.activeWorktrees {
		// Create a copy of the WorktreeInfo
		result[taskID] = &WorktreeInfo{
			Path:         worktreeInfo.Path,
			Branch:       worktreeInfo.Branch,
			Commit:       worktreeInfo.Commit,
			Locked:       worktreeInfo.Locked,
			Prunable:     worktreeInfo.Prunable,
			TaskID:       worktreeInfo.TaskID,
			CreatedAt:    worktreeInfo.CreatedAt,
			LastAccessed: worktreeInfo.LastAccessed,
		}
	}

	return result
}

// GetWorktreeManager returns the persistent WorktreeManager instance
func (h *GitServiceHandle) GetWorktreeManager() *WorktreeManager {
	return h.repo.worktreeManager
}

// UpdateWorktreeAccess updates the last accessed time for a worktree
func (h *GitServiceHandle) UpdateWorktreeAccess(taskID string) {
	h.repo.mu.Lock()
	defer h.repo.mu.Unlock()

	if worktreeInfo, exists := h.repo.activeWorktrees[taskID]; exists {
		worktreeInfo.LastAccessed = time.Now()
	}
}

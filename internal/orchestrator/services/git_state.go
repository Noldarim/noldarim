// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/logger"
)

var (
	gitStateLog     *zerolog.Logger
	gitStateLogOnce sync.Once
)

func getGitStateLog() *zerolog.Logger {
	gitStateLogOnce.Do(func() {
		l := logger.GetGitLogger().With().Str("component", "state").Logger()
		gitStateLog = &l
	})
	return gitStateLog
}

// GitStateManager manages git state tracking and validation
type GitStateManager struct {
	gitService *GitService
}

// NewGitStateManager creates a new git state manager
func NewGitStateManager(gitService *GitService) *GitStateManager {
	return &GitStateManager{
		gitService: gitService,
	}
}

// GitRepositoryState represents the complete state of a git repository
type GitRepositoryState struct {
	RepoPath         string
	IsInitialized    bool
	IsValid          bool
	State            *GitState
	LastValidation   time.Time
	ValidationErrors []string
}

// ValidateRepositoryState validates the complete state of a repository
func (gsm *GitStateManager) ValidateRepositoryState(ctx context.Context, repoPath string) (*GitRepositoryState, error) {
	getGitStateLog().Debug().Str("repo_path", repoPath).Msg("Validating repository state")

	// Validate path using GitService first
	validatedPath, err := gsm.gitService.validateRepoPath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid repository path: %w", err)
	}

	state := &GitRepositoryState{
		RepoPath:         validatedPath,
		LastValidation:   time.Now(),
		ValidationErrors: []string{},
	}

	// Check if repository is initialized using GitService method
	if !gsm.gitService.isGitRepository(validatedPath) {
		state.IsInitialized = false
		state.IsValid = false
		state.ValidationErrors = append(state.ValidationErrors, "repository is not initialized")
		return state, nil
	}

	state.IsInitialized = true

	// Get git state using GitService ValidateRepository method
	gitState, err := gsm.gitService.ValidateRepository(ctx, validatedPath)
	if err != nil {
		state.IsValid = false
		state.ValidationErrors = append(state.ValidationErrors, fmt.Sprintf("failed to validate git state: %v", err))
		return state, nil
	}

	state.State = gitState

	// Validate git state using internal validation
	if err := gsm.validateGitState(gitState); err != nil {
		state.IsValid = false
		state.ValidationErrors = append(state.ValidationErrors, err.Error())
		return state, nil
	}

	state.IsValid = true
	log.Info().Str("repo_path", validatedPath).Msg("Repository state validation completed successfully")
	return state, nil
}

// Removed CreateOperationState and UpdateOperationState - not needed for simplified workflow

// Removed task-related methods - task state tracking is handled via NATS events

// Removed ValidateGitOperation - not needed for simplified workflow

// Helper methods

// validateGitState validates the git state
func (gsm *GitStateManager) validateGitState(state *GitState) error {
	var errors []string

	// Check if branch is valid
	if state.Branch == "" {
		errors = append(errors, "branch is empty")
	}

	// Check if commit hash is valid
	if state.CommitHash == "" {
		errors = append(errors, "commit hash is empty")
	}

	// Check if commit hash is valid format (40 character hex)
	if len(state.CommitHash) != 40 {
		errors = append(errors, "commit hash is not valid format")
	}

	// Validate remote URL if present
	if state.RemoteURL != "" {
		if !strings.HasPrefix(state.RemoteURL, "http") && !strings.HasPrefix(state.RemoteURL, "git@") {
			errors = append(errors, "remote URL is not valid format")
		}
	}

	// Validate worktree paths
	for _, worktree := range state.WorktreePaths {
		if !filepath.IsAbs(worktree) {
			errors = append(errors, fmt.Sprintf("worktree path is not absolute: %s", worktree))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("git state validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// Removed IsOperationIdempotent and CreateRollbackOperation - not needed for simplified workflow

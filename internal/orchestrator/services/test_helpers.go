// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"path/filepath"
	"testing"

	"github.com/noldarim/noldarim/internal/orchestrator/database"

	"github.com/stretchr/testify/require"
)

// GitServiceFixture represents a git service setup with repository path and cleanup
type GitServiceFixture struct {
	Service  *GitService
	RepoPath string
	Cleanup  func()
}

// DataServiceFixture represents a data service setup.
// Cleanup is automatic via t.Cleanup — no manual cleanup needed.
type DataServiceFixture struct {
	Service *DataService
}

// WithGitService creates a git service with temporary directory
func WithGitService(t *testing.T) *GitServiceFixture {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test_repo")

	gitService, err := NewGitService(repoPath, true)
	require.NoError(t, err, "Failed to create git service")

	cleanup := func() {
		gitService.Close()
	}

	return &GitServiceFixture{
		Service:  gitService,
		RepoPath: repoPath,
		Cleanup:  cleanup,
	}
}

// WithDataService creates a data service with a fresh Postgres test database
func WithDataService(t *testing.T) *DataServiceFixture {
	fixture := database.UseFreshTestDatabase(t)

	// Create the data service directly with the migrated database
	ds := &DataService{db: fixture.DB}

	return &DataServiceFixture{
		Service: ds,
	}
}

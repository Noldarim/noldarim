// Copyright (C) 2026 Noldarim
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

// DataServiceFixture represents a data service setup with cleanup
type DataServiceFixture struct {
	Service *DataService
	Cleanup func()
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

// WithDataService creates a data service with in-memory database
func WithDataService(t *testing.T) *DataServiceFixture {
	cfg := database.WithInMemoryConfig()
	db, err := database.NewGormDB(&cfg.Database)
	require.NoError(t, err, "Failed to create database")

	err = db.AutoMigrate()
	require.NoError(t, err, "Failed to run migrations")

	// Create the data service directly with the migrated database
	ds := &DataService{db: db}

	cleanup := func() {
		ds.Close()
	}

	return &DataServiceFixture{
		Service: ds,
		Cleanup: cleanup,
	}
}

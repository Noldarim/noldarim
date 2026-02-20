// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"testing"

	"github.com/noldarim/noldarim/internal/config"

	"github.com/stretchr/testify/require"
)

// DatabaseFixture represents a database setup with cleanup
type DatabaseFixture struct {
	DB      *GormDB
	Cleanup func()
}

// UseFreshInMemoryDatabase creates an in-memory SQLite database with GORM AutoMigrate applied
func UseFreshInMemoryDatabase(t *testing.T) *DatabaseFixture {
	cfg := &config.DatabaseConfig{
		Driver:   "sqlite",
		Database: ":memory:",
	}

	db, err := NewGormDB(cfg)
	require.NoError(t, err, "Failed to create in-memory database")

	err = db.AutoMigrate()
	require.NoError(t, err, "Failed to run migrations on in-memory database")

	cleanup := func() {
		db.Close()
	}

	return &DatabaseFixture{
		DB:      db,
		Cleanup: cleanup,
	}
}

// UseExistingDatabase connects to an existing database file
func UseExistingDatabase(t *testing.T, dbPath string) *DatabaseFixture {
	cfg := &config.DatabaseConfig{
		Driver:   "sqlite",
		Database: dbPath,
	}

	db, err := NewGormDB(cfg)
	require.NoError(t, err, "Failed to connect to existing database at %s", dbPath)

	cleanup := func() {
		db.Close()
	}

	return &DatabaseFixture{
		DB:      db,
		Cleanup: cleanup,
	}
}

// WithInMemoryConfig creates a config with in-memory database
func WithInMemoryConfig() *config.AppConfig {
	return &config.AppConfig{
		Database: config.DatabaseConfig{
			Driver:   "sqlite",
			Database: ":memory:",
		},
		Git: config.GitConfig{
			WorktreeBasePath:                  "./worktrees",
			DefaultBranch:                     "main",
			CreateGitRepoForProjectIfNotExist: true,
		},
	}
}

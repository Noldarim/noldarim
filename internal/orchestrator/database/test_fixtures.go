// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/test/testutil"

	"github.com/stretchr/testify/require"
)

var testDBCounter atomic.Int64

// DatabaseFixture represents an isolated test database.
// Cleanup is automatic via t.Cleanup — no manual cleanup needed.
type DatabaseFixture struct {
	DB *GormDB
}

// UseFreshTestDatabase creates a fresh Postgres database for test isolation.
// Each call creates a uniquely-named database, runs migrations, and returns
// a fixture whose Cleanup drops the database.
func UseFreshTestDatabase(t *testing.T) *DatabaseFixture {
	adminCfg := testutil.TestPostgresConfig()

	// Connect to the admin database to create a fresh test DB
	adminDB, err := NewGormDB(adminCfg)
	if err != nil {
		t.Skipf("Test Postgres not available (run 'make test-postgres-start'): %v", err)
	}

	dbName := fmt.Sprintf("noldarim_test_%d_%d", os.Getpid(), testDBCounter.Add(1))

	err = adminDB.db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)).Error
	require.NoError(t, err, "Failed to create test database %s", dbName)
	adminDB.Close()

	// Connect to the fresh database
	testCfg := *adminCfg
	testCfg.Database = dbName

	db, err := NewGormDB(&testCfg)
	require.NoError(t, err, "Failed to connect to test database %s", dbName)

	err = db.AutoMigrate()
	require.NoError(t, err, "Failed to run migrations on test database %s", dbName)

	cleanup := func() {
		db.Close()
		// Reconnect to admin DB to drop the test database
		adminDB2, err := NewGormDB(adminCfg)
		if err == nil {
			adminDB2.db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", dbName))
			adminDB2.Close()
		}
	}
	t.Cleanup(cleanup)

	return &DatabaseFixture{
		DB: db,
	}
}

// WithTestConfig returns an AppConfig suitable for tests, pointing at the test Postgres.
func WithTestConfig() *config.AppConfig {
	return &config.AppConfig{
		Database: *testutil.TestPostgresConfig(),
		Git: config.GitConfig{
			WorktreeBasePath:                  "./worktrees",
			DefaultBranch:                     "main",
			CreateGitRepoForProjectIfNotExist: true,
		},
	}
}

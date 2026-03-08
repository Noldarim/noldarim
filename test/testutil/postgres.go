// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package testutil

import (
	"os"
	"strconv"

	"github.com/noldarim/noldarim/internal/config"
)

// EnvOr returns the value of the environment variable key, or fallback if unset.
func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestPostgresConfig returns a DatabaseConfig for the test Postgres instance.
// Connection details come from TEST_POSTGRES_* env vars, defaulting to the
// container started by `make test-postgres-start` (localhost:5433).
func TestPostgresConfig() *config.DatabaseConfig {
	port, _ := strconv.Atoi(EnvOr("TEST_POSTGRES_PORT", "5433"))
	return &config.DatabaseConfig{
		Host:     EnvOr("TEST_POSTGRES_HOST", "localhost"),
		Port:     port,
		Username: EnvOr("TEST_POSTGRES_USER", "noldarim_test"),
		Password: EnvOr("TEST_POSTGRES_PASSWORD", "noldarim_test"),
		Database: EnvOr("TEST_POSTGRES_DB", "noldarim_test"),
		SSLMode:  "disable",
	}
}

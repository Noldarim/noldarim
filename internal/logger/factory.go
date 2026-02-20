// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger

import (
	"github.com/rs/zerolog"
)

// Static logger getters that map directly to config.yaml log.levels
// These ensure consistent logger names across the codebase

// GetOrchestratorLogger returns a logger for the orchestrator
func GetOrchestratorLogger() zerolog.Logger {
	return GetLogger("orchestrator")
}

// GetTemporalLogger returns a logger for Temporal components
func GetTemporalLogger() zerolog.Logger {
	return GetLogger("temporal")
}

// GetTUILogger returns a logger for TUI components
func GetTUILogger() zerolog.Logger {
	return GetLogger("tui")
}

// GetDatabaseLogger returns a logger for database operations
func GetDatabaseLogger() zerolog.Logger {
	return GetLogger("database")
}

// GetGitLogger returns a logger for git operations
func GetGitLogger() zerolog.Logger {
	return GetLogger("git")
}

// GetContainerLogger returns a logger for container operations
func GetContainerLogger() zerolog.Logger {
	return GetLogger("container")
}

// GetAPILogger returns a logger for API operations
func GetAPILogger() zerolog.Logger {
	return GetLogger("api")
}

// GetAIObsLogger returns a logger for AI observability/adapter operations
func GetAIObsLogger() zerolog.Logger {
	return GetLogger("aiobs")
}

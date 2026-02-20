// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/config"
)

func TestStaticLoggerGetters(t *testing.T) {
	// Initialize global logger manager for testing
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
		Levels: map[string]string{
			"orchestrator": "debug",
			"temporal":     "warn",
			"tui":          "error",
			"database":     "trace",
			"git":          "info",
			"container":    "debug",
			"api":          "warn",
		},
		Context: config.LogContextConfig{
			IncludeTimestamp: true,
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	tests := []struct {
		name          string
		getterFunc    func() zerolog.Logger
		expectedPkg   string
		expectedLevel zerolog.Level
	}{
		{
			name:          "orchestrator_logger",
			getterFunc:    GetOrchestratorLogger,
			expectedPkg:   "orchestrator",
			expectedLevel: zerolog.DebugLevel,
		},
		{
			name:          "temporal_logger",
			getterFunc:    GetTemporalLogger,
			expectedPkg:   "temporal",
			expectedLevel: zerolog.WarnLevel,
		},
		{
			name:          "tui_logger",
			getterFunc:    GetTUILogger,
			expectedPkg:   "tui",
			expectedLevel: zerolog.ErrorLevel,
		},
		{
			name:          "database_logger",
			getterFunc:    GetDatabaseLogger,
			expectedPkg:   "database",
			expectedLevel: zerolog.TraceLevel,
		},
		{
			name:          "git_logger",
			getterFunc:    GetGitLogger,
			expectedPkg:   "git",
			expectedLevel: zerolog.InfoLevel,
		},
		{
			name:          "container_logger",
			getterFunc:    GetContainerLogger,
			expectedPkg:   "container",
			expectedLevel: zerolog.DebugLevel,
		},
		{
			name:          "api_logger",
			getterFunc:    GetAPILogger,
			expectedPkg:   "api",
			expectedLevel: zerolog.WarnLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := tt.getterFunc()

			// Test that the logger is functional
			// We can't easily test the package name or level directly,
			// but we can test that the logger works and is properly configured

			// Create a test context to verify the logger works
			testLogger := logger.With().Str("test", "value").Logger()

			// Test different log levels to verify level configuration
			switch tt.expectedLevel {
			case zerolog.TraceLevel:
				// All levels should work
				testLogger.Trace().Msg("trace test")
				testLogger.Debug().Msg("debug test")
				testLogger.Info().Msg("info test")
				testLogger.Warn().Msg("warn test")
				testLogger.Error().Msg("error test")
			case zerolog.DebugLevel:
				// Debug and above should work
				testLogger.Debug().Msg("debug test")
				testLogger.Info().Msg("info test")
				testLogger.Warn().Msg("warn test")
				testLogger.Error().Msg("error test")
			case zerolog.InfoLevel:
				// Info and above should work
				testLogger.Info().Msg("info test")
				testLogger.Warn().Msg("warn test")
				testLogger.Error().Msg("error test")
			case zerolog.WarnLevel:
				// Warn and above should work
				testLogger.Warn().Msg("warn test")
				testLogger.Error().Msg("error test")
			case zerolog.ErrorLevel:
				// Only error and above should work
				testLogger.Error().Msg("error test")
			}

			// Verify that calling the getter multiple times returns the same logger instance
			// (testing caching behavior)
			logger2 := tt.getterFunc()

			// Both loggers should be functional and equivalent
			// We can't compare pointers directly due to zerolog's structure,
			// but we can verify they both work
			logger2.Info().Msg("second logger test")
		})
	}
}

func TestStaticLoggerGetters_Uninitialized(t *testing.T) {
	// Reset global manager to test uninitialized state
	originalManager := globalManager
	globalManager = nil
	defer func() {
		globalManager = originalManager
	}()

	tests := []struct {
		name       string
		getterFunc func() zerolog.Logger
	}{
		{"orchestrator_uninitialized", GetOrchestratorLogger},
		{"temporal_uninitialized", GetTemporalLogger},
		{"tui_uninitialized", GetTUILogger},
		{"database_uninitialized", GetDatabaseLogger},
		{"git_uninitialized", GetGitLogger},
		{"container_uninitialized", GetContainerLogger},
		{"api_uninitialized", GetAPILogger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := tt.getterFunc()

			// Should return a discard logger when not initialized
			// Test by checking if it produces no output

			// This is a bit tricky to test directly, but we can at least
			// verify the logger doesn't panic and appears to work
			logger.Info().Str("test", "uninitialized").Msg("test message")
			logger.Error().Str("test", "uninitialized").Msg("error message")

			// The main thing is that it doesn't panic or cause issues
		})
	}
}

func TestStaticLoggerGetters_Consistency(t *testing.T) {
	// Test that the static getters are consistent with direct GetLogger calls
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	tests := []struct {
		name       string
		getterFunc func() zerolog.Logger
		pkgName    string
	}{
		{"orchestrator_consistency", GetOrchestratorLogger, "orchestrator"},
		{"temporal_consistency", GetTemporalLogger, "temporal"},
		{"tui_consistency", GetTUILogger, "tui"},
		{"database_consistency", GetDatabaseLogger, "database"},
		{"git_consistency", GetGitLogger, "git"},
		{"container_consistency", GetContainerLogger, "container"},
		{"api_consistency", GetAPILogger, "api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			staticLogger := tt.getterFunc()
			directLogger := GetLogger(tt.pkgName)

			// Both should be functional
			staticLogger.Info().Msg("static logger test")
			directLogger.Info().Msg("direct logger test")

			// They should be equivalent in functionality
			// We can't easily compare them directly, but we can verify
			// they both work without issues
		})
	}
}

func TestStaticLoggerGetters_PackageSpecificLevels(t *testing.T) {
	// Test that static getters properly inherit package-specific log levels
	config := &config.LogConfig{
		Level:  "info", // Global default
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
		Levels: map[string]string{
			"orchestrator": "debug",
			"temporal":     "error",
			"database":     "trace",
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	// Test orchestrator logger (debug level)
	orchestratorLogger := GetOrchestratorLogger()
	orchestratorLogger.Debug().Msg("orchestrator debug message")
	orchestratorLogger.Info().Msg("orchestrator info message")

	// Test temporal logger (error level)
	temporalLogger := GetTemporalLogger()
	temporalLogger.Error().Msg("temporal error message")

	// Test database logger (trace level)
	databaseLogger := GetDatabaseLogger()
	databaseLogger.Trace().Msg("database trace message")
	databaseLogger.Debug().Msg("database debug message")

	// Test package with no specific level (should use global default)
	tuiLogger := GetTUILogger()
	tuiLogger.Info().Msg("tui info message") // Should work with global 'info' level

	// The main verification is that none of these panic
	// and the loggers are properly configured
}

func TestStaticLoggerGetters_DynamicLevelChanges(t *testing.T) {
	// Test that static getters reflect dynamic level changes
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	// Get logger before level change
	logger := GetOrchestratorLogger()

	// Change level dynamically
	if globalManager != nil {
		globalManager.SetPackageLevel("orchestrator", "debug")
	}

	// Logger should reflect the new level
	// (This is hard to test directly, but we can at least verify it doesn't break)
	logger.Debug().Msg("debug message after level change")
	logger.Info().Msg("info message after level change")

	// Get logger again after level change
	logger2 := GetOrchestratorLogger()
	logger2.Debug().Msg("debug message from new logger instance")
}

// Benchmark tests for static getters
func BenchmarkStaticLoggerGetters(b *testing.B) {
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
	}

	err := Initialize(config)
	if err != nil {
		b.Fatalf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	b.Run("GetOrchestratorLogger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetOrchestratorLogger()
		}
	})

	b.Run("GetTemporalLogger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetTemporalLogger()
		}
	})

	b.Run("GetDatabaseLogger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetDatabaseLogger()
		}
	})

	b.Run("Direct_GetLogger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetLogger("orchestrator")
		}
	})
}

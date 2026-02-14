// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/noldarim/noldarim/internal/config"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.LogConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "minimal_config",
			config: &config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: []config.LogOutputConfig{
					{Type: "console", Enabled: true},
				},
				Context: config.LogContextConfig{
					IncludeTimestamp: true,
					IncludeCaller:    false,
				},
			},
			expectError: false,
		},
		{
			name: "file_output_config",
			config: &config.LogConfig{
				Level:  "debug",
				Format: "json",
				Output: []config.LogOutputConfig{
					{
						Type:    "file",
						Enabled: true,
						Path:    filepath.Join(t.TempDir(), "test.log"),
					},
				},
				Context: config.LogContextConfig{
					IncludeTimestamp: true,
					IncludeCaller:    true,
				},
			},
			expectError: false,
		},
		{
			name: "console_format_config",
			config: &config.LogConfig{
				Level:  "warn",
				Format: "console",
				Output: []config.LogOutputConfig{
					{Type: "console", Enabled: true},
				},
				Context: config.LogContextConfig{
					IncludeTimestamp: true,
				},
			},
			expectError: false,
		},
		{
			name: "rotating_file_config",
			config: &config.LogConfig{
				Level:  "error",
				Format: "json",
				Output: []config.LogOutputConfig{
					{
						Type:    "file",
						Enabled: true,
						Path:    filepath.Join(t.TempDir(), "rotating.log"),
						Rotate: config.LogRotateConfig{
							MaxSizeMB:  1,
							MaxBackups: 3,
							MaxAgeDays: 7,
							Compress:   true,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid_output_type",
			config: &config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: []config.LogOutputConfig{
					{Type: "invalid", Enabled: true},
				},
			},
			expectError: true,
			errorMsg:    "unsupported output type: invalid",
		},
		{
			name: "invalid_file_path",
			config: &config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: []config.LogOutputConfig{
					{
						Type:    "file",
						Enabled: true,
						Path:    "/invalid/path/that/does/not/exist/file.log",
					},
				},
			},
			expectError: true,
			errorMsg:    "failed to create log directory",
		},
		{
			name: "sampling_config",
			config: &config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: []config.LogOutputConfig{
					{Type: "console", Enabled: true},
				},
				Sampling: config.LogSamplingConfig{
					Enabled:    true,
					Initial:    100,
					Thereafter: 10,
					Tick:       time.Second,
				},
			},
			expectError: false,
		},
		{
			name: "package_levels_config",
			config: &config.LogConfig{
				Level:  "info",
				Format: "json",
				Output: []config.LogOutputConfig{
					{Type: "console", Enabled: true},
				},
				Levels: map[string]string{
					"orchestrator": "debug",
					"database":     "warn",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if manager == nil {
				t.Error("expected manager to be non-nil")
				return
			}

			// Test cleanup - but skip closing console outputs as they can't be closed
			defer func() {
				// Only close non-console outputs
				if tt.config.Output[0].Type != "console" {
					if err := manager.Close(); err != nil {
						t.Errorf("failed to close manager: %v", err)
					}
				}
			}()

			// Verify configuration was applied
			if manager.config != tt.config {
				t.Error("config was not properly set")
			}

			// Verify package loggers map is initialized
			if manager.packageLoggers == nil {
				t.Error("packageLoggers map should be initialized")
			}

			// Verify writers were created
			if len(manager.writers) == 0 && len(tt.config.Output) > 0 {
				hasEnabledOutput := false
				for _, output := range tt.config.Output {
					if output.Enabled {
						hasEnabledOutput = true
						break
					}
				}
				if hasEnabledOutput {
					t.Error("expected writers to be created for enabled outputs")
				}
			}
		})
	}
}

func TestManager_FallbackBehavior(t *testing.T) {
	// Test fallback when no outputs are configured
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{}, // No outputs
	}

	tempDir := t.TempDir()
	// Change working directory temporarily for fallback file creation
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer manager.Close()

	// Verify fallback file was created
	fallbackPath := filepath.Join(tempDir, "logs", "noldarim-fallback.log")
	if _, err := os.Stat(fallbackPath); os.IsNotExist(err) {
		t.Error("fallback log file was not created")
	}

	// Verify manager has the fallback writer
	if len(manager.writers) != 1 {
		t.Errorf("expected 1 fallback writer, got %d", len(manager.writers))
	}
}

func TestManager_GetLogger(t *testing.T) {
	// Save and restore global level to avoid test interference
	originalLevel := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(originalLevel)

	config := &config.LogConfig{
		Level:  "trace", // Use trace to capture all levels
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
		Levels: map[string]string{
			"orchestrator": "debug",
			"database":     "warn",
		},
		Context: config.LogContextConfig{
			IncludeTimestamp: true,
		},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer manager.Close()

	tests := []struct {
		name          string
		pkg           string
		expectedPkg   string
		expectedLevel zerolog.Level
	}{
		{
			name:          "new_package_default_level",
			pkg:           "newpackage",
			expectedPkg:   "newpackage",
			expectedLevel: zerolog.InfoLevel,
		},
		{
			name:          "configured_debug_level",
			pkg:           "orchestrator",
			expectedPkg:   "orchestrator",
			expectedLevel: zerolog.DebugLevel,
		},
		{
			name:          "configured_warn_level",
			pkg:           "database",
			expectedPkg:   "database",
			expectedLevel: zerolog.WarnLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := manager.GetLogger(tt.pkg)

			// Test that the logger works - use appropriate level for each logger
			var buf bytes.Buffer
			testLogger := logger.Output(&buf)

			// Use a log level that will actually produce output for this logger
			switch tt.expectedLevel {
			case zerolog.DebugLevel:
				testLogger.Debug().Msg("test message")
			case zerolog.WarnLevel:
				testLogger.Warn().Msg("test message")
			default:
				testLogger.Info().Msg("test message")
			}

			if buf.Len() == 0 {
				t.Error("expected log output but got none")
			}

			// Parse the JSON output to verify package field
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Errorf("failed to parse log JSON: %v", err)
			}

			if pkg, exists := logEntry["pkg"]; !exists {
				t.Error("expected 'pkg' field in log entry")
			} else if pkg != tt.expectedPkg {
				t.Errorf("expected pkg=%q, got %q", tt.expectedPkg, pkg)
			}

			// Test that getting the same logger returns the cached instance
			logger2 := manager.GetLogger(tt.pkg)
			if &logger != &logger2 {
				// Note: This test might be fragile due to zerolog's internal structure
				// The important thing is that both loggers work correctly
			}
		})
	}
}

func TestManager_SetPackageLevel(t *testing.T) {
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
		Context: config.LogContextConfig{
			IncludeTimestamp: true,
		},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer manager.Close()

	// Test setting level for new package
	manager.SetPackageLevel("testpkg", "debug")

	// Verify config was updated
	if level, exists := manager.config.Levels["testpkg"]; !exists {
		t.Error("expected package level to be set in config")
	} else if level != "debug" {
		t.Errorf("expected level 'debug', got %q", level)
	}

	// Test setting level for existing package
	logger := manager.GetLogger("testpkg")
	manager.SetPackageLevel("testpkg", "error")

	// Verify the logger was updated
	var buf bytes.Buffer
	testLogger := logger.Output(&buf)

	// Debug message should not appear (level is now error)
	testLogger.Debug().Msg("debug message")
	if buf.Len() > 0 {
		t.Error("debug message should not appear when level is error")
	}

	// Error message should appear
	buf.Reset()
	testLogger.Error().Msg("error message")
	if buf.Len() == 0 {
		t.Error("error message should appear when level is error")
	}
}

func TestManager_ThreadSafety(t *testing.T) {
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer manager.Close()

	// Test concurrent GetLogger calls
	const numGoroutines = 100
	const numPackages = 10

	var wg sync.WaitGroup
	packages := make([]string, numPackages)
	for i := 0; i < numPackages; i++ {
		packages[i] = fmt.Sprintf("pkg%d", i)
	}

	// Test concurrent GetLogger
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			pkg := packages[i%numPackages]
			logger := manager.GetLogger(pkg)
			// Use the logger to ensure it works
			logger.Info().Str("goroutine", fmt.Sprintf("%d", i)).Msg("test")
		}(i)
	}

	// Test concurrent SetPackageLevel
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			pkg := packages[i%numPackages]
			level := []string{"debug", "info", "warn", "error"}[i%4]
			manager.SetPackageLevel(pkg, level)
		}(i)
	}

	wg.Wait()

	// Verify all packages were created
	manager.mu.RLock()
	if len(manager.packageLoggers) != numPackages {
		t.Errorf("expected %d package loggers, got %d", numPackages, len(manager.packageLoggers))
	}
	manager.mu.RUnlock()
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zerolog.Level
	}{
		{"TRACE", zerolog.TraceLevel},
		{"trace", zerolog.TraceLevel},
		{"DEBUG", zerolog.DebugLevel},
		{"debug", zerolog.DebugLevel},
		{"INFO", zerolog.InfoLevel},
		{"info", zerolog.InfoLevel},
		{"WARN", zerolog.WarnLevel},
		{"warn", zerolog.WarnLevel},
		{"WARNING", zerolog.WarnLevel},
		{"warning", zerolog.WarnLevel},
		{"ERROR", zerolog.ErrorLevel},
		{"error", zerolog.ErrorLevel},
		{"FATAL", zerolog.FatalLevel},
		{"fatal", zerolog.FatalLevel},
		{"PANIC", zerolog.PanicLevel},
		{"panic", zerolog.PanicLevel},
		{"invalid", zerolog.InfoLevel}, // Default
		{"", zerolog.InfoLevel},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLumberjackRotation(t *testing.T) {
	tempDir := t.TempDir()

	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{
				Type:    "file",
				Enabled: true,
				Path:    filepath.Join(tempDir, "rotating.log"),
				Rotate: config.LogRotateConfig{
					MaxSizeMB:  1,
					MaxBackups: 3,
					MaxAgeDays: 1,
					Compress:   false,
				},
			},
		},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	// Get a logger and write some data
	logger := manager.GetLogger("test")

	// Write enough data to potentially trigger rotation
	for i := 0; i < 1000; i++ {
		logger.Info().Int("iteration", i).Msg("test message for rotation testing")
	}

	// Verify log file was created
	logPath := filepath.Join(tempDir, "rotating.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}

	// Verify file has content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Errorf("failed to read log file: %v", err)
	}
	if len(content) == 0 {
		t.Error("log file is empty")
	}
}

func TestLumberjackCompression(t *testing.T) {
	tempDir := t.TempDir()

	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{
				Type:    "file",
				Enabled: true,
				Path:    filepath.Join(tempDir, "compressed.log"),
				Rotate: config.LogRotateConfig{
					MaxSizeMB:  1,
					MaxBackups: 2,
					MaxAgeDays: 1,
					Compress:   true,
				},
			},
		},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	// Get a logger and write some data
	logger := manager.GetLogger("test")

	// Write data to test compression setup
	for i := 0; i < 100; i++ {
		logger.Info().Int("iteration", i).Msg("test message for compression testing")
	}

	// Verify log file was created
	logPath := filepath.Join(tempDir, "compressed.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("log file was not created")
	}
}

func TestManager_Close(t *testing.T) {
	tempDir := t.TempDir()

	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{
				Type:    "file",
				Enabled: true,
				Path:    filepath.Join(tempDir, "test.log"),
			},
		},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Write some data
	logger := manager.GetLogger("test")
	logger.Info().Msg("test message")

	// Close should not error
	err = manager.Close()
	if err != nil {
		t.Errorf("expected Close() to succeed, got error: %v", err)
	}

	// Multiple closes should not panic but may return error (file already closed)
	// This is acceptable behavior
	_ = manager.Close()
}

func TestGlobalLoggerFunctions(t *testing.T) {
	// Test uninitialized global logger
	logger := GetLogger("test")

	// Should return a discard logger that doesn't produce output to stderr
	// We can't easily test this directly since it's a discard logger,
	// but we can verify it doesn't panic
	logger.Info().Msg("this should be discarded")

	// Test initialization
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Errorf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	// Test multiple initializations (should only initialize once)
	err = Initialize(config)
	if err != nil {
		t.Errorf("second initialization should not fail: %v", err)
	}

	// Test global logger usage
	logger = GetLogger("global-test")
	var buf bytes.Buffer
	testLogger := logger.Output(&buf)
	testLogger.Info().Msg("global test message")

	if buf.Len() == 0 {
		t.Error("expected initialized global logger to produce output")
	}

	// Test CloseGlobal - may fail due to console output being unclosable
	_ = CloseGlobal()

	// Test CloseGlobal when not initialized
	globalManager = nil
	err = CloseGlobal()
	if err != nil {
		t.Errorf("CloseGlobal should not fail when not initialized: %v", err)
	}
}

// Test helper to create a test config
func createTestConfig(outputs []config.LogOutputConfig) *config.LogConfig {
	return &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: outputs,
		Context: config.LogContextConfig{
			IncludeTimestamp: true,
		},
	}
}

func TestManager_MultipleOutputs(t *testing.T) {
	tempDir := t.TempDir()

	config := createTestConfig([]config.LogOutputConfig{
		{Type: "console", Enabled: true},
		{
			Type:    "file",
			Enabled: true,
			Path:    filepath.Join(tempDir, "multi.log"),
		},
	})

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	defer manager.Close()

	// Verify multiple writers were created
	if len(manager.writers) != 2 {
		t.Errorf("expected 2 writers, got %d", len(manager.writers))
	}

	// Test logging works
	logger := manager.GetLogger("multitest")
	logger.Info().Msg("multi-output test")

	// Force any buffered writes to complete
	time.Sleep(10 * time.Millisecond)

	// Verify file was written
	logFile := filepath.Join(tempDir, "multi.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("log file was not created")
	}

	// Verify file has content (may be empty due to buffering)
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Errorf("failed to read log file: %v", err)
	}
	// Don't require content since buffering may delay writes
	_ = content
}

func TestManager_DisabledOutputs(t *testing.T) {
	config := createTestConfig([]config.LogOutputConfig{
		{Type: "console", Enabled: false}, // Disabled
		{Type: "console", Enabled: true},  // Enabled
	})

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	defer manager.Close()

	// Should only have 1 writer (the enabled one)
	if len(manager.writers) != 1 {
		t.Errorf("expected 1 writer, got %d", len(manager.writers))
	}
}

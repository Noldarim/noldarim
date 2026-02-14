// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/log"
	"github.com/noldarim/noldarim/internal/config"
)

// mockStringer implements fmt.Stringer for testing
type mockStringer struct {
	value string
}

func (m mockStringer) String() string {
	return m.value
}

func TestNewTemporalLogAdapter(t *testing.T) {
	// Create a test logger with output buffer
	var buf bytes.Buffer
	zerologLogger := zerolog.New(&buf).With().Timestamp().Logger()

	adapter := NewTemporalLogAdapter(zerologLogger)

	if adapter == nil {
		t.Error("expected adapter to be non-nil")
	}

	// Verify it implements the Temporal log.Logger interface
	var _ log.Logger = adapter

	// Test that it works
	adapter.Info("test message")

	if buf.Len() == 0 {
		t.Error("expected log output but got none")
	}

	// Parse the JSON to verify structure
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Errorf("failed to parse log JSON: %v", err)
	}

	if msg, exists := logEntry["message"]; !exists {
		t.Error("expected 'message' field in log entry")
	} else if msg != "test message" {
		t.Errorf("expected message 'test message', got %q", msg)
	}
}

func TestTemporalLogAdapter_LogLevels(t *testing.T) {
	tests := []struct {
		name    string
		logFunc func(adapter log.Logger, msg string)
		level   string
		message string
	}{
		{
			name: "debug_level",
			logFunc: func(adapter log.Logger, msg string) {
				adapter.Debug(msg)
			},
			level:   "debug",
			message: "debug test message",
		},
		{
			name: "info_level",
			logFunc: func(adapter log.Logger, msg string) {
				adapter.Info(msg)
			},
			level:   "info",
			message: "info test message",
		},
		{
			name: "warn_level",
			logFunc: func(adapter log.Logger, msg string) {
				adapter.Warn(msg)
			},
			level:   "warn",
			message: "warn test message",
		},
		{
			name: "error_level",
			logFunc: func(adapter log.Logger, msg string) {
				adapter.Error(msg)
			},
			level:   "error",
			message: "error test message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global level to avoid test interference
			originalLevel := zerolog.GlobalLevel()
			defer zerolog.SetGlobalLevel(originalLevel)
			zerolog.SetGlobalLevel(zerolog.TraceLevel)

			var buf bytes.Buffer
			// Set level to trace to capture all log levels
			zerologLogger := zerolog.New(&buf).Level(zerolog.TraceLevel).With().Timestamp().Logger()
			adapter := NewTemporalLogAdapter(zerologLogger)

			tt.logFunc(adapter, tt.message)

			if buf.Len() == 0 {
				t.Error("expected log output but got none")
			}

			// Parse and verify
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Errorf("failed to parse log JSON: %v", err)
			}

			if level, exists := logEntry["level"]; !exists {
				t.Error("expected 'level' field in log entry")
			} else if level != tt.level {
				t.Errorf("expected level %q, got %q", tt.level, level)
			}

			if msg, exists := logEntry["message"]; !exists {
				t.Error("expected 'message' field in log entry")
			} else if msg != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, msg)
			}
		})
	}
}

func TestTemporalLogAdapter_WithKeyValues(t *testing.T) {
	tests := []struct {
		name      string
		keyvals   []interface{}
		expectErr bool
	}{
		{
			name:    "string_values",
			keyvals: []interface{}{"key1", "value1", "key2", "value2"},
		},
		{
			name:    "mixed_types",
			keyvals: []interface{}{"str", "value", "int", 42, "float", 3.14, "bool", true},
		},
		{
			name:    "error_value",
			keyvals: []interface{}{"error", errors.New("test error")},
		},
		{
			name:    "stringer_value",
			keyvals: []interface{}{"stringer", mockStringer{value: "custom string"}},
		},
		{
			name:    "int64_value",
			keyvals: []interface{}{"int64", int64(1234567890)},
		},
		{
			name:    "float64_value",
			keyvals: []interface{}{"float64", float64(123.456)},
		},
		{
			name:    "complex_value",
			keyvals: []interface{}{"complex", map[string]interface{}{"nested": "value"}},
		},
		{
			name:    "odd_number_keyvals",
			keyvals: []interface{}{"key1", "value1", "key2"}, // Missing value for key2
		},
		{
			name:    "empty_keyvals",
			keyvals: []interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			zerologLogger := zerolog.New(&buf).With().Timestamp().Logger()
			adapter := NewTemporalLogAdapter(zerologLogger)

			// Test logging with keyvals directly
			adapter.Info("direct keyvals test", tt.keyvals...)

			if buf.Len() == 0 {
				t.Error("expected log output but got none")
			}

			// Parse the first log entry
			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(lines[0]), &logEntry); err != nil {
				t.Errorf("failed to parse log JSON: %v", err)
			}

			// Verify message
			if msg, exists := logEntry["message"]; !exists {
				t.Error("expected 'message' field in log entry")
			} else if msg != "direct keyvals test" {
				t.Errorf("expected message 'direct keyvals test', got %q", msg)
			}

			// Verify key-value pairs (for even number of keyvals)
			for i := 0; i < len(tt.keyvals)-1; i += 2 {
				key := fmt.Sprint(tt.keyvals[i])
				expectedValue := tt.keyvals[i+1]

				if actualValue, exists := logEntry[key]; !exists {
					t.Errorf("expected key %q in log entry", key)
				} else {
					// Special handling for different types
					switch expectedValue.(type) {
					case error:
						// Error values are logged under "error" key
						if errorVal, exists := logEntry["error"]; !exists {
							t.Error("expected 'error' field for error value")
						} else {
							expectedErr := expectedValue.(error)
							if !strings.Contains(fmt.Sprint(errorVal), expectedErr.Error()) {
								t.Errorf("expected error containing %q, got %q", expectedErr.Error(), errorVal)
							}
						}
					case mockStringer:
						// Stringer values should be converted to string
						expected := expectedValue.(mockStringer).String()
						if actualValue != expected {
							t.Errorf("expected stringer value %q, got %q", expected, actualValue)
						}
					case int64:
						// JSON unmarshaling converts int64 to float64
						if actualFloat, ok := actualValue.(float64); !ok {
							t.Errorf("expected int64 to be unmarshaled as float64, got %T", actualValue)
						} else if actualFloat != float64(expectedValue.(int64)) {
							t.Errorf("expected int64 value %d (as float64: %f), got %f", expectedValue, float64(expectedValue.(int64)), actualFloat)
						}
					default:
						// For complex types like maps, just verify they exist and are not nil
						// since direct comparison is not possible
						if actualValue == nil && expectedValue != nil {
							t.Errorf("expected non-nil value for key %q, got nil", key)
						} else if actualValue != nil && expectedValue == nil {
							t.Errorf("expected nil value for key %q, got %v", key, actualValue)
						}
						// For comparable types, try direct comparison
						if actualValue != nil && expectedValue != nil {
							defer func() {
								if r := recover(); r != nil {
									// If comparison panics (like with maps), just verify the key exists
									t.Logf("Key %q exists with value type %T (comparison not possible)", key, actualValue)
								}
							}()
							if actualValue != expectedValue {
								// Try to convert and compare as strings for complex types
								if fmt.Sprint(actualValue) != fmt.Sprint(expectedValue) {
									t.Errorf("expected value %v (%T), got %v (%T)", expectedValue, expectedValue, actualValue, actualValue)
								}
							}
						}
					}
				}
			}

			// Test using With method
			buf.Reset()
			concreteAdapter := adapter.(*TemporalLogAdapter)
			newAdapter := concreteAdapter.With(tt.keyvals...)
			newAdapter.Info("test message with With")

			if buf.Len() == 0 {
				t.Error("expected log output from With() adapter but got none")
			}
		})
	}
}

func TestTemporalLogAdapter_With(t *testing.T) {
	var buf bytes.Buffer
	zerologLogger := zerolog.New(&buf).With().Timestamp().Logger()
	adapter := NewTemporalLogAdapter(zerologLogger)

	// Test chaining With calls
	concreteAdapter := adapter.(*TemporalLogAdapter)
	tempAdapter := concreteAdapter.With("key1", "value1")
	newAdapter := tempAdapter.(*TemporalLogAdapter).With("key2", 42)
	newAdapter.Info("chained with test")

	if buf.Len() == 0 {
		t.Error("expected log output but got none")
	}

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Errorf("failed to parse log JSON: %v", err)
	}

	// Should have both keys
	if val, exists := logEntry["key1"]; !exists {
		t.Error("expected 'key1' in log entry")
	} else if val != "value1" {
		t.Errorf("expected key1='value1', got %q", val)
	}

	if val, exists := logEntry["key2"]; !exists {
		t.Error("expected 'key2' in log entry")
	} else if val != float64(42) { // JSON unmarshaling converts int to float64
		t.Errorf("expected key2=42, got %v", val)
	}
}

func TestTemporalLogAdapter_AddFields(t *testing.T) {
	var buf bytes.Buffer
	zerologLogger := zerolog.New(&buf).With().Timestamp().Logger()
	adapter := &TemporalLogAdapter{logger: zerologLogger}

	tests := []struct {
		name    string
		keyvals []interface{}
	}{
		{
			name: "all_types",
			keyvals: []interface{}{
				"string", "test",
				"int", 123,
				"int64", int64(456),
				"float64", 123.45,
				"bool", true,
				"error", errors.New("test error"),
				"stringer", mockStringer{value: "stringer test"},
				"interface", map[string]string{"key": "value"},
			},
		},
		{
			name:    "nil_values",
			keyvals: []interface{}{"key", nil},
		},
		{
			name:    "time_value",
			keyvals: []interface{}{"time", time.Now()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			event := adapter.logger.Info()
			finalEvent := adapter.addFields(event, tt.keyvals...)
			finalEvent.Msg("test message")

			if buf.Len() == 0 {
				t.Error("expected log output but got none")
			}

			// Verify it's valid JSON
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Errorf("failed to parse log JSON: %v", err)
			}

			if msg, exists := logEntry["message"]; !exists {
				t.Error("expected 'message' field")
			} else if msg != "test message" {
				t.Errorf("expected message 'test message', got %q", msg)
			}
		})
	}
}

func TestGetTemporalLogAdapter(t *testing.T) {
	// Test without global manager initialized
	adapter := GetTemporalLogAdapter("test-pkg")
	if adapter == nil {
		t.Error("expected adapter to be non-nil even without initialized manager")
	}

	// Test adapter works (should use discard logger)
	adapter.Info("test message")

	// Initialize global manager and test again
	config := &config.LogConfig{
		Level:  "debug",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
		Levels: map[string]string{
			"temporal-test": "warn",
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	adapter = GetTemporalLogAdapter("temporal-test")
	if adapter == nil {
		t.Error("expected adapter to be non-nil with initialized manager")
	}

	// Test adapter works with initialized manager
	adapter.Info("initialized test message")
	adapter.Warn("warn message")   // Should appear due to warn level
	adapter.Debug("debug message") // Should not appear due to warn level
}

func TestTemporalLogAdapter_Integration(t *testing.T) {
	// Integration test simulating real Temporal usage
	config := &config.LogConfig{
		Level:  "info",
		Format: "json",
		Output: []config.LogOutputConfig{
			{Type: "console", Enabled: true},
		},
		Context: config.LogContextConfig{
			IncludeTimestamp: true,
			IncludeCaller:    true,
		},
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("failed to initialize global logger: %v", err)
	}
	defer CloseGlobal()

	// Get Temporal adapter
	adapter := GetTemporalLogAdapter("temporal")

	// Simulate Temporal logging patterns
	concreteAdapter := adapter.(*TemporalLogAdapter)
	workflowAdapter := concreteAdapter.With(
		"WorkflowType", "TestWorkflow",
		"WorkflowID", "test-workflow-123",
		"RunID", "run-456",
	)

	activityAdapter := workflowAdapter.(*TemporalLogAdapter).With(
		"ActivityType", "TestActivity",
		"ActivityID", "activity-789",
	)

	// Test typical Temporal log messages
	workflowAdapter.Info("Workflow started")
	workflowAdapter.Debug("Workflow state", "state", "running")

	activityAdapter.Info("Activity started")
	activityAdapter.Warn("Activity warning", "warning", "temporary failure")
	activityAdapter.Error("Activity failed", "error", errors.New("activity execution failed"))

	workflowAdapter.Info("Workflow completed")

	// All of these should work without panicking
}

func TestTemporalLogAdapter_ErrorHandling(t *testing.T) {
	// Save and restore global level to avoid test interference
	originalLevel := zerolog.GlobalLevel()
	defer zerolog.SetGlobalLevel(originalLevel)
	zerolog.SetGlobalLevel(zerolog.TraceLevel)

	var buf bytes.Buffer
	zerologLogger := zerolog.New(&buf).Level(zerolog.TraceLevel).With().Timestamp().Logger()
	adapter := NewTemporalLogAdapter(zerologLogger)

	// Test with various problematic inputs
	tests := []struct {
		name    string
		logFunc func()
	}{
		{
			name: "nil_keyvals",
			logFunc: func() {
				adapter.Info("test", nil, "key", nil)
			},
		},
		{
			name: "empty_string_key",
			logFunc: func() {
				adapter.Info("test", "", "value") // Use Info level which should produce output
			},
		},
		{
			name: "very_long_message",
			logFunc: func() {
				longMsg := strings.Repeat("This is a very long message. ", 100)
				adapter.Info(longMsg)
			},
		},
		{
			name: "unicode_message",
			logFunc: func() {
				adapter.Warn("Unicode test: 你好世界 🌍 émojis")
			},
		},
		{
			name: "special_characters",
			logFunc: func() {
				adapter.Error("Special chars: \n\t\r\"'\\", "key", "value\nwith\nnewlines")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("logging should not panic, but got: %v", r)
				}
			}()

			tt.logFunc()

			// Should produce some output
			if buf.Len() == 0 {
				t.Error("expected some log output")
			}

			// Should be valid JSON
			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				var logEntry map[string]interface{}
				if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
					t.Errorf("log output should be valid JSON, got error: %v\nLine: %s", err, line)
				}
			}
		})
	}
}

// Benchmark tests for Temporal adapter
func BenchmarkTemporalLogAdapter(b *testing.B) {
	var buf bytes.Buffer
	zerologLogger := zerolog.New(&buf).With().Timestamp().Logger()
	adapter := NewTemporalLogAdapter(zerologLogger)

	b.Run("Info", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			adapter.Info("benchmark test message")
		}
	})

	b.Run("InfoWithKeyvals", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			adapter.Info("benchmark test", "key1", "value1", "key2", i)
		}
	})

	b.Run("With", func(b *testing.B) {
		concreteAdapter := adapter.(*TemporalLogAdapter)
		for i := 0; i < b.N; i++ {
			newAdapter := concreteAdapter.With("iteration", i)
			newAdapter.Info("benchmark with test")
		}
	})

	b.Run("ChainedWith", func(b *testing.B) {
		concreteAdapter := adapter.(*TemporalLogAdapter)
		for i := 0; i < b.N; i++ {
			tempAdapter := concreteAdapter.With("key1", "value1")
			tempAdapter.(*TemporalLogAdapter).With("key2", i).Info("chained test")
		}
	})
}

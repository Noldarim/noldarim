// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests for outputCollector
// =============================================================================

func TestOutputCollector_BasicWrite(t *testing.T) {
	c := newOutputCollector(false)

	n, err := c.Write([]byte("hello world\n"))
	require.NoError(t, err)
	assert.Equal(t, 12, n)

	lineCount, truncated := c.GetStats()
	assert.Equal(t, 1, lineCount)
	assert.False(t, truncated)
	assert.Equal(t, "hello world\n", c.GetOutput())
}

func TestOutputCollector_MultipleLines(t *testing.T) {
	c := newOutputCollector(false)

	c.Write([]byte("line1\nline2\nline3\n"))

	lineCount, _ := c.GetStats()
	assert.Equal(t, 3, lineCount)
	assert.Equal(t, "line1\nline2\nline3\n", c.GetOutput())
}

func TestOutputCollector_PartialLine(t *testing.T) {
	c := newOutputCollector(false)

	// Write partial line (no newline)
	c.Write([]byte("partial"))

	lineCount, _ := c.GetStats()
	assert.Equal(t, 0, lineCount) // No complete lines yet

	// GetOutput should still include the partial line
	assert.Equal(t, "partial", c.GetOutput())
}

func TestOutputCollector_PartialLineCompleted(t *testing.T) {
	c := newOutputCollector(false)

	// Write partial line
	c.Write([]byte("hello "))
	c.Write([]byte("world"))
	c.Write([]byte("\n"))

	lineCount, _ := c.GetStats()
	assert.Equal(t, 1, lineCount)
	assert.Equal(t, "hello world\n", c.GetOutput())
}

func TestOutputCollector_FlushPartial(t *testing.T) {
	// Use a shorter flush interval for testing
	originalInterval := PartialLineFlushInterval
	defer func() {
		// Note: Can't modify const, this is illustrative
		// In real test, you'd inject the interval or use a mock
		_ = originalInterval
	}()

	c := newOutputCollector(false)
	c.Write([]byte("waiting for newline"))

	// Simulate time passing by directly manipulating lastFlushTime
	c.mu.Lock()
	c.lastFlushTime = time.Now().Add(-3 * time.Second)
	c.mu.Unlock()

	c.FlushPartial()

	// Should have the partial line in recent output
	recent := c.GetRecentLines()
	require.Len(t, recent, 1)
	assert.Contains(t, recent[0], "[stdout...]") // Partial indicator
	assert.Contains(t, recent[0], "waiting for newline")
}

func TestOutputCollector_RecentLinesLimit(t *testing.T) {
	c := newOutputCollector(false)

	// Write more than MaxHeartbeatOutputLines
	for i := 0; i < MaxHeartbeatOutputLines+10; i++ {
		c.Write([]byte("line\n"))
	}

	recent := c.GetRecentLines()
	assert.Len(t, recent, MaxHeartbeatOutputLines)
}

func TestOutputCollector_SizeLimit(t *testing.T) {
	c := newOutputCollector(false)

	// Write more than MaxOutputSize
	bigLine := strings.Repeat("x", 1024) + "\n"
	bytesNeeded := MaxOutputSize + 1000

	for written := 0; written < bytesNeeded; written += len(bigLine) {
		c.Write([]byte(bigLine))
	}

	_, truncated := c.GetStats()
	assert.True(t, truncated)

	output := c.GetOutput()
	assert.Contains(t, output, "OUTPUT TRUNCATED")
	assert.LessOrEqual(t, len(output), MaxOutputSize+100) // Allow for truncation message
}

func TestOutputCollector_StderrPrefix(t *testing.T) {
	c := newOutputCollector(true) // stderr

	c.Write([]byte("error message\n"))

	recent := c.GetRecentLines()
	require.Len(t, recent, 1)
	assert.True(t, strings.HasPrefix(recent[0], "[stderr]"), "expected stderr prefix, got: %s", recent[0])
}

func TestOutputCollector_ConcurrentWrites(t *testing.T) {
	c := newOutputCollector(false)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Write([]byte("line\n"))
		}(i)
	}

	wg.Wait()

	lineCount, _ := c.GetStats()
	assert.Equal(t, 100, lineCount)
}

// =============================================================================
// Unit Tests for helper functions
// =============================================================================

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 3, "hel"},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCommandForLogging(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		contains []string
	}{
		{
			name:     "empty command",
			command:  []string{},
			contains: []string{"<empty>"},
		},
		{
			name:     "simple command",
			command:  []string{"echo", "hello"},
			contains: []string{"echo", "hello"},
		},
		{
			name:     "long argument truncated",
			command:  []string{"claude", strings.Repeat("x", 100)},
			contains: []string{"claude", "..."},
		},
		{
			name:     "many arguments",
			command:  []string{"cmd", "a", "b", "c", "d", "e"},
			contains: []string{"cmd", "+2 more args"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCommandForLogging(tt.command)
			for _, substr := range tt.contains {
				assert.Contains(t, result, substr)
			}
		})
	}
}

func TestMergeRecentLines(t *testing.T) {
	stdout := []string{"out1", "out2"}
	stderr := []string{"err1", "err2"}

	merged := mergeRecentLines(stdout, stderr)
	assert.Len(t, merged, 4)
	assert.Contains(t, merged, "out1")
	assert.Contains(t, merged, "err2")
}

func TestMergeRecentLines_Truncates(t *testing.T) {
	// Create more lines than MaxHeartbeatOutputLines
	stdout := make([]string, MaxHeartbeatOutputLines)
	stderr := make([]string, 5)

	for i := range stdout {
		stdout[i] = "stdout"
	}
	for i := range stderr {
		stderr[i] = "stderr"
	}

	merged := mergeRecentLines(stdout, stderr)
	assert.Len(t, merged, MaxHeartbeatOutputLines)
}

// =============================================================================
// Integration-style tests for LocalExecuteActivity
// These require mocking the Temporal activity context
// =============================================================================

// MockActivityLogger implements a minimal logger for testing
type MockActivityLogger struct {
	logs []string
	mu   sync.Mutex
}

func (m *MockActivityLogger) Debug(msg string, keyvals ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *MockActivityLogger) Info(msg string, keyvals ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *MockActivityLogger) Warn(msg string, keyvals ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *MockActivityLogger) Error(msg string, keyvals ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, "ERROR: "+msg)
}

// TestLocalExecuteActivity_SimpleCommand tests basic command execution
// Note: This test runs actual commands, so it's more of an integration test
func TestLocalExecuteActivity_SimpleCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would require setting up a proper Temporal test environment
	// For now, we can test the outputCollector directly
	t.Skip("Requires Temporal test environment setup - see integration tests")
}

// =============================================================================
// Benchmark tests
// =============================================================================

func BenchmarkOutputCollector_Write(b *testing.B) {
	c := newOutputCollector(false)
	data := []byte("This is a line of output that will be written repeatedly\n")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Write(data)
	}
}

func BenchmarkOutputCollector_ConcurrentWrite(b *testing.B) {
	c := newOutputCollector(false)
	data := []byte("This is a line of output\n")

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Write(data)
		}
	})
}

// =============================================================================
// Example usage (as documentation)
// =============================================================================

// TestOutputCollector_ExampleUsage demonstrates typical usage of outputCollector
func TestOutputCollector_ExampleUsage(t *testing.T) {
	c := newOutputCollector(false)

	// Write some output
	c.Write([]byte("Processing started\n"))
	c.Write([]byte("Step 1 complete\n"))
	c.Write([]byte("Step 2 in progress...")) // No newline

	// Get stats
	lines, truncated := c.GetStats()
	assert.Equal(t, 2, lines)    // 2 complete lines
	assert.False(t, truncated)

	// Get recent lines for heartbeat
	recent := c.GetRecentLines()
	assert.Len(t, recent, 2) // Only complete lines

	// After flush, partial line appears
	c.mu.Lock()
	c.lastFlushTime = time.Now().Add(-3 * time.Second)
	c.mu.Unlock()
	c.FlushPartial()

	recent = c.GetRecentLines()
	assert.Len(t, recent, 3) // Now includes partial line
	assert.Contains(t, recent[2], "[stdout...]") // Partial indicator
}

// Compile-time interface check
var _ = bytes.Buffer{} // Just to use bytes import

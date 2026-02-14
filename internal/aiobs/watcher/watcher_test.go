// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	"github.com/noldarim/noldarim/internal/aiobs/types"
)

func init() {
	// Register adapters for tests
	adapters.RegisterAll()
}

// generateClaudeTranscriptLine creates a valid Claude transcript JSONL line
func generateClaudeTranscriptLine(index int, entryType string) []byte {
	entry := map[string]interface{}{
		"type":      entryType,
		"uuid":      fmt.Sprintf("uuid-%d", index),
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"sessionId": "test-session-123",
		"cwd":       "/workspace",
		"version":   "1.0.0",
	}

	switch entryType {
	case "user":
		entry["message"] = map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("User message %d", index)},
			},
		}
	case "assistant":
		entry["message"] = map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Assistant response %d", index)},
			},
		}
	}

	line, _ := json.Marshal(entry)
	return append(line, '\n')
}

// generateToolUseTranscriptLine creates a tool_use transcript entry
func generateToolUseTranscriptLine(index int, toolName string) []byte {
	entry := map[string]interface{}{
		"type":      "assistant",
		"uuid":      fmt.Sprintf("uuid-tool-%d", index),
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"sessionId": "test-session-123",
		"message": map[string]interface{}{
			"role": "assistant",
			"content": []map[string]interface{}{
				{
					"type": "tool_use",
					"id":   fmt.Sprintf("tool-id-%d", index),
					"name": toolName,
					"input": map[string]interface{}{
						"command": fmt.Sprintf("echo 'test %d'", index),
					},
				},
			},
		},
	}

	line, _ := json.Marshal(entry)
	return append(line, '\n')
}

func TestTranscriptWatcher_BasicReading(t *testing.T) {
	// Create a temporary file for the transcript
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Write some initial content
	f, err := os.Create(transcriptPath)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err := f.Write(generateClaudeTranscriptLine(i, "user"))
		require.NoError(t, err)
	}
	f.Close()

	// Create and start watcher
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := Config{
		FilePath:        transcriptPath,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    10 * time.Millisecond,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Collect events
	var receivedEvents []types.ParsedEvent
	timeout := time.After(2 * time.Second)
	expectedCount := 5

	for len(receivedEvents) < expectedCount {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				break
			}
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			t.Fatalf("Timeout waiting for events, got %d, expected %d", len(receivedEvents), expectedCount)
		}
	}

	assert.Equal(t, expectedCount, len(receivedEvents))

	// Verify events are user input events
	for _, event := range receivedEvents {
		assert.Equal(t, types.EventTypeUserPrompt, event.EventType)
	}
}

func TestTranscriptWatcher_NonBlocking_HundredsOfLines(t *testing.T) {
	// This test verifies that writing hundreds of lines doesn't block the writer
	// and all events are read correctly

	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Create the file first (empty)
	f, err := os.Create(transcriptPath)
	require.NoError(t, err)
	f.Close()

	// Start watcher
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := Config{
		FilePath:        transcriptPath,
		Source:          "claude",
		EventBufferSize: 2000, // Large buffer for this test
		PollInterval:    10 * time.Millisecond,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Write 500 lines in a goroutine (simulating Claude writing transcript)
	numLines := 500
	writeDone := make(chan struct{})
	writeStartTime := time.Now()

	go func() {
		defer close(writeDone)

		f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Errorf("Failed to open file for writing: %v", err)
			return
		}
		defer f.Close()

		for i := 0; i < numLines; i++ {
			var line []byte
			if i%2 == 0 {
				line = generateClaudeTranscriptLine(i, "user")
			} else {
				line = generateToolUseTranscriptLine(i, "Bash")
			}

			_, err := f.Write(line)
			if err != nil {
				t.Errorf("Failed to write line %d: %v", i, err)
				return
			}

			// Simulate realistic writing pace (not too fast)
			if i%50 == 0 {
				f.Sync() // Flush periodically
			}
		}
		f.Sync()
	}()

	// Measure write time - it should not be blocked by reading
	<-writeDone
	writeDuration := time.Since(writeStartTime)
	t.Logf("Writing %d lines took %v", numLines, writeDuration)

	// The write should complete quickly (< 1 second for 500 lines)
	assert.Less(t, writeDuration, 1*time.Second,
		"Writing should not be blocked by reading - took %v", writeDuration)

	// Now collect all events
	var receivedEvents []types.ParsedEvent
	timeout := time.After(10 * time.Second)

	for len(receivedEvents) < numLines {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				t.Fatalf("Event channel closed prematurely after %d events", len(receivedEvents))
			}
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			t.Fatalf("Timeout waiting for events, got %d, expected %d", len(receivedEvents), numLines)
		}
	}

	assert.Equal(t, numLines, len(receivedEvents))
	t.Logf("Successfully read %d events", len(receivedEvents))

	// Verify stats
	stats := watcher.Stats()
	assert.Equal(t, int64(numLines), stats.LinesRead)
}

func TestTranscriptWatcher_ConcurrentWriteAndRead(t *testing.T) {
	// Test that writing and reading can happen concurrently without issues

	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := Config{
		FilePath:        transcriptPath,
		Source:          "claude",
		EventBufferSize: 1000,
		PollInterval:    5 * time.Millisecond, // Fast polling for this test
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Track events as they come in
	var receivedEvents []types.ParsedEvent
	var mu sync.Mutex
	readDone := make(chan struct{})

	numLines := 200

	// Reader goroutine
	go func() {
		defer close(readDone)
		for {
			select {
			case event, ok := <-watcher.Events():
				if !ok {
					return
				}
				mu.Lock()
				receivedEvents = append(receivedEvents, event)
				count := len(receivedEvents)
				mu.Unlock()

				if count >= numLines {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Writer - write lines with small delays to simulate real-time generation
	f, err = os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)

	for i := 0; i < numLines; i++ {
		line := generateClaudeTranscriptLine(i, "assistant")
		_, err := f.Write(line)
		require.NoError(t, err)

		// Small delay to simulate real-time writing
		if i%10 == 0 {
			f.Sync()
			time.Sleep(5 * time.Millisecond)
		}
	}
	f.Sync()
	f.Close()

	// Wait for reader to finish
	select {
	case <-readDone:
	case <-time.After(10 * time.Second):
		mu.Lock()
		count := len(receivedEvents)
		mu.Unlock()
		t.Fatalf("Timeout waiting for reader to finish, got %d events", count)
	}

	mu.Lock()
	finalCount := len(receivedEvents)
	mu.Unlock()

	assert.Equal(t, numLines, finalCount)
}

func TestTranscriptWatcher_FileCreatedAfterStart(t *testing.T) {
	// Test that watcher handles file being created after Start() is called

	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	// DON'T create the file yet
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := Config{
		FilePath:        transcriptPath,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    20 * time.Millisecond,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Wait a bit, then create and write to the file
	time.Sleep(100 * time.Millisecond)

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)

	numLines := 10
	for i := 0; i < numLines; i++ {
		_, err := f.Write(generateClaudeTranscriptLine(i, "user"))
		require.NoError(t, err)
	}
	f.Sync()
	f.Close()

	// Wait for events
	var receivedEvents []types.ParsedEvent
	timeout := time.After(5 * time.Second)

	for len(receivedEvents) < numLines {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				t.Fatal("Event channel closed")
			}
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			t.Fatalf("Timeout waiting for events, got %d, expected %d", len(receivedEvents), numLines)
		}
	}

	assert.Equal(t, numLines, len(receivedEvents))
}

func TestTranscriptWatcher_ToolUseEvents(t *testing.T) {
	// Test that tool_use events are parsed correctly

	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)

	toolNames := []string{"Bash", "Read", "Write", "Glob", "Grep"}
	for i, tool := range toolNames {
		_, err := f.Write(generateToolUseTranscriptLine(i, tool))
		require.NoError(t, err)
	}
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := DefaultConfig(transcriptPath, "claude")
	cfg.PollInterval = 10 * time.Millisecond

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Collect events
	var receivedEvents []types.ParsedEvent
	timeout := time.After(3 * time.Second)

	for len(receivedEvents) < len(toolNames) {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				break
			}
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			t.Fatalf("Timeout, got %d events", len(receivedEvents))
		}
	}

	assert.Equal(t, len(toolNames), len(receivedEvents))

	// Verify all are tool use events
	for i, event := range receivedEvents {
		assert.Equal(t, types.EventTypeToolUse, event.EventType,
			"Event %d should be tool use", i)
		assert.Equal(t, toolNames[i], event.ToolName,
			"Event %d should have tool name %s", i, toolNames[i])
	}
}

func TestTranscriptWatcher_StopDuringRead(t *testing.T) {
	// Test that stopping the watcher during reading doesn't cause panics or hangs

	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)
	f.Close()

	ctx := context.Background()

	cfg := Config{
		FilePath:        transcriptPath,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    10 * time.Millisecond,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)

	// Start writing lines
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		f, _ := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
		defer f.Close()

		for i := 0; i < 100; i++ {
			f.Write(generateClaudeTranscriptLine(i, "user"))
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Stop the watcher while it's reading
	time.Sleep(50 * time.Millisecond)

	// This should not panic or hang
	done := make(chan struct{})
	go func() {
		watcher.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good - Stop() completed
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() timed out - possible deadlock")
	}

	// Verify watcher is stopped
	stats := watcher.Stats()
	assert.True(t, stats.Closed)

	// Wait for writer to finish
	<-writerDone
}

func TestTranscriptWatcher_BufferFull(t *testing.T) {
	// Test behavior when event buffer is full

	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Small buffer to trigger overflow
	cfg := Config{
		FilePath:        transcriptPath,
		Source:          "claude",
		EventBufferSize: 5, // Very small buffer
		PollInterval:    10 * time.Millisecond,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Write more lines than buffer can hold
	f, _ = os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	for i := 0; i < 20; i++ {
		f.Write(generateClaudeTranscriptLine(i, "user"))
	}
	f.Sync()
	f.Close()

	// Give watcher time to process
	time.Sleep(200 * time.Millisecond)

	// Check for errors on error channel
	var errors []error
	for {
		select {
		case err := <-watcher.Errors():
			errors = append(errors, err)
		default:
			goto done
		}
	}
done:

	// Should have some buffer overflow errors
	// (not necessarily all, depends on timing)
	t.Logf("Got %d errors", len(errors))

	// Watcher should still be running
	stats := watcher.Stats()
	assert.False(t, stats.Closed)
	assert.Greater(t, stats.LinesRead, int64(0))
}

func TestTranscriptWatcher_UnknownAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	ctx := context.Background()

	cfg := Config{
		FilePath: transcriptPath,
		Source:   "unknown-ai-tool", // Non-existent adapter
	}

	_, err := NewTranscriptWatcher(ctx, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source")
}

func TestTranscriptWatcher_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)

	numLines := 25
	for i := 0; i < numLines; i++ {
		f.Write(generateClaudeTranscriptLine(i, "user"))
	}
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := DefaultConfig(transcriptPath, "claude")
	cfg.PollInterval = 10 * time.Millisecond

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	// Check stats before start
	stats := watcher.Stats()
	assert.False(t, stats.Initialized)
	assert.False(t, stats.Closed)
	assert.Equal(t, int64(0), stats.LinesRead)

	err = watcher.Start()
	require.NoError(t, err)

	// Wait for all events
	timeout := time.After(3 * time.Second)
	received := 0
	for received < numLines {
		select {
		case <-watcher.Events():
			received++
		case <-timeout:
			t.Fatal("Timeout")
		}
	}

	// Check stats after processing
	stats = watcher.Stats()
	assert.True(t, stats.Initialized)
	assert.False(t, stats.Closed)
	assert.Equal(t, int64(numLines), stats.LinesRead)
	assert.Equal(t, transcriptPath, stats.FilePath)
	assert.Equal(t, "claude", stats.Source)

	// Stop and check final stats
	watcher.Stop()

	stats = watcher.Stats()
	assert.True(t, stats.Closed)
}

func TestAdapterRegistry(t *testing.T) {
	// Verify Claude adapter is registered
	adapter, ok := adapters.Get("claude")
	require.True(t, ok, "Claude adapter should be registered")
	assert.Equal(t, "claude", adapter.Name())
}

func TestTranscriptWatcher_DiscoverUUID_SingleFile(t *testing.T) {
	// Test UUID discovery mode with a single UUID-named file
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher-discover-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a UUID-named transcript file
	sessionID := "88ad3a71-4c86-4b19-b41d-71a7b027ee63"
	transcriptPath := filepath.Join(tmpDir, sessionID+".jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)

	// Write a test entry
	entry := `{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Hello"}]}}`
	_, err = f.WriteString(entry + "\n")
	require.NoError(t, err)
	f.Close()

	// Create watcher in discovery mode (point to directory, not file)
	cfg := Config{
		FilePath:        tmpDir, // Directory path in discovery mode
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
		DiscoverUUID:    true,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Wait for event
	select {
	case event := <-watcher.Events():
		require.NotNil(t, event)
		assert.Equal(t, "Hello", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Check stats show discovered file (in discovery mode, FilePath is empty)
	stats := watcher.Stats()
	assert.Equal(t, tmpDir, stats.DiscoverDir)
	assert.Contains(t, stats.ActiveFiles, sessionID+".jsonl")
}

func TestTranscriptWatcher_DiscoverUUID_FileCreatedLater(t *testing.T) {
	// Test that watcher discovers file when it's created after watcher starts
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temp directory (empty)
	tmpDir, err := os.MkdirTemp("", "watcher-discover-later-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create watcher in discovery mode before file exists
	cfg := Config{
		FilePath:        tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
		DiscoverUUID:    true,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// File doesn't exist yet - watcher should be polling
	time.Sleep(100 * time.Millisecond)

	// Now create the UUID file
	sessionID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	transcriptPath := filepath.Join(tmpDir, sessionID+".jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)

	entry := `{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Delayed"}]}}`
	_, err = f.WriteString(entry + "\n")
	require.NoError(t, err)
	f.Close()

	// Wait for event
	select {
	case event := <-watcher.Events():
		require.NotNil(t, event)
		assert.Equal(t, "Delayed", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Verify discovery
	stats := watcher.Stats()
	assert.Contains(t, stats.ActiveFiles, sessionID+".jsonl")
}

func TestTranscriptWatcher_DiscoverUUID_IgnoresNonUUID(t *testing.T) {
	// Test that watcher ignores files that don't match UUID pattern
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "watcher-ignore-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some non-UUID files that should be ignored
	nonUUIDFiles := []string{
		"transcript.jsonl",           // No UUID
		"settings.json",              // Wrong extension
		"12345.jsonl",                // Not UUID format
		"not-a-uuid-at-all.jsonl",    // Not UUID format
	}
	for _, name := range nonUUIDFiles {
		f, _ := os.Create(filepath.Join(tmpDir, name))
		f.WriteString(`{"type":"user","message":{"content":[{"type":"text","text":"ignore me"}]}}` + "\n")
		f.Close()
	}

	// Create the actual UUID file
	sessionID := "12345678-1234-1234-1234-123456789abc"
	uuidPath := filepath.Join(tmpDir, sessionID+".jsonl")
	f, err := os.Create(uuidPath)
	require.NoError(t, err)
	f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Found me!"}]}}` + "\n")
	f.Close()

	cfg := Config{
		FilePath:        tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
		DiscoverUUID:    true,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Should only get event from UUID file
	select {
	case event := <-watcher.Events():
		require.NotNil(t, event)
		assert.Equal(t, "Found me!", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	stats := watcher.Stats()
	assert.Contains(t, stats.ActiveFiles, sessionID+".jsonl")
}

func TestTranscriptWatcher_DiscoverUUID_DirectoryNotExist(t *testing.T) {
	// Test that watcher handles non-existent directory gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temp dir path that doesn't exist
	tmpDir := filepath.Join(os.TempDir(), "watcher-nonexistent-"+time.Now().Format("20060102150405"))

	cfg := Config{
		FilePath:        tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
		DiscoverUUID:    true,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	// Start should succeed (directory may be created later)
	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Watcher should be polling, waiting for directory
	time.Sleep(100 * time.Millisecond)

	// Now create the directory and file
	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sessionID := "11111111-2222-3333-4444-555555555555"
	transcriptPath := filepath.Join(tmpDir, sessionID+".jsonl")
	f, err := os.Create(transcriptPath)
	require.NoError(t, err)
	f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Dir created"}]}}` + "\n")
	f.Close()

	// Wait for event
	select {
	case event := <-watcher.Events():
		require.NotNil(t, event)
		assert.Equal(t, "Dir created", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestTranscriptWatcher_DiscoverUUID_MultipleFiles(t *testing.T) {
	// Test behavior when multiple UUID files exist (should pick first one found)
	// This tests the current behavior - future DirectoryWatcher would handle multiple files
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "watcher-multi-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create two UUID files
	sessions := []string{
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"11111111-2222-3333-4444-555555555555",
	}

	for i, sessionID := range sessions {
		path := filepath.Join(tmpDir, sessionID+".jsonl")
		f, err := os.Create(path)
		require.NoError(t, err)
		content := fmt.Sprintf(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Session %d"}]}}`, i)
		f.WriteString(content + "\n")
		f.Close()
	}

	cfg := Config{
		FilePath:        tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
		DiscoverUUID:    true,
	}

	watcher, err := NewTranscriptWatcher(ctx, cfg)
	require.NoError(t, err)

	err = watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Should get event from one of the files (first one found by ReadDir)
	select {
	case event := <-watcher.Events():
		require.NotNil(t, event)
		// Content should be from one of the sessions
		assert.Contains(t, event.ContentPreview, "Session")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Verify a file was discovered
	stats := watcher.Stats()
	assert.NotEmpty(t, stats.ActiveFiles)
	assert.True(t, uuidFileRegex.MatchString(stats.ActiveFiles[0]))
}

// ============================================================================
// DirectoryWatcher Tests - Multi-session support
// ============================================================================

func TestDirectoryWatcher_SingleSession(t *testing.T) {
	// Test DirectoryWatcher with a single UUID file
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "dirwatcher-single-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a UUID-named transcript file
	sessionID := "88ad3a71-4c86-4b19-b41d-71a7b027ee63"
	transcriptPath := filepath.Join(tmpDir, sessionID+".jsonl")

	f, err := os.Create(transcriptPath)
	require.NoError(t, err)
	entry := `{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Hello from single session"}]}}`
	_, err = f.WriteString(entry + "\n")
	require.NoError(t, err)
	f.Close()

	// Create DirectoryWatcher
	cfg := DirectoryWatcherConfig{
		Directory:       tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
	}

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)
	defer dw.Stop()

	// Wait for event
	select {
	case event := <-dw.Events():
		require.NotNil(t, event)
		assert.Equal(t, "Hello from single session", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Check stats
	stats := dw.Stats()
	assert.Equal(t, 1, stats.WatcherCount)
	assert.Contains(t, stats.Watchers, sessionID+".jsonl")
}

func TestDirectoryWatcher_MultipleSessions(t *testing.T) {
	// Test DirectoryWatcher with multiple concurrent UUID files
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "dirwatcher-multi-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create multiple UUID files with different session content
	sessions := map[string]string{
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee": "Session A",
		"11111111-2222-3333-4444-555555555555": "Session B",
		"99999999-8888-7777-6666-555555555555": "Session C",
	}

	for sessionID, content := range sessions {
		path := filepath.Join(tmpDir, sessionID+".jsonl")
		f, err := os.Create(path)
		require.NoError(t, err)
		entry := fmt.Sprintf(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"%s"}]}}`, content)
		f.WriteString(entry + "\n")
		f.Close()
	}

	cfg := DirectoryWatcherConfig{
		Directory:       tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
	}

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)
	defer dw.Stop()

	// Collect events from all sessions
	receivedContent := make(map[string]bool)
	timeout := time.After(3 * time.Second)

	for len(receivedContent) < len(sessions) {
		select {
		case event := <-dw.Events():
			require.NotNil(t, event)
			receivedContent[event.ContentPreview] = true
		case <-timeout:
			t.Fatalf("Timeout waiting for events, got %d, expected %d", len(receivedContent), len(sessions))
		}
	}

	// Verify all sessions were captured
	for _, content := range sessions {
		assert.True(t, receivedContent[content], "Expected content %q not received", content)
	}

	// Check stats
	stats := dw.Stats()
	assert.Equal(t, len(sessions), stats.WatcherCount)
}

func TestDirectoryWatcher_SessionCreatedLater(t *testing.T) {
	// Test that DirectoryWatcher picks up new session files created after start
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "dirwatcher-later-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Start with one session
	session1 := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	path1 := filepath.Join(tmpDir, session1+".jsonl")
	f, err := os.Create(path1)
	require.NoError(t, err)
	f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"First session"}]}}` + "\n")
	f.Close()

	cfg := DirectoryWatcherConfig{
		Directory:       tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
	}

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)
	defer dw.Stop()

	// Get first event
	select {
	case event := <-dw.Events():
		assert.Equal(t, "First session", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for first event")
	}

	// Now create a second session file
	session2 := "11111111-2222-3333-4444-555555555555"
	path2 := filepath.Join(tmpDir, session2+".jsonl")
	f, err = os.Create(path2)
	require.NoError(t, err)
	f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:31:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Second session"}]}}` + "\n")
	f.Close()

	// Get second event from new session
	select {
	case event := <-dw.Events():
		assert.Equal(t, "Second session", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for second session event")
	}

	// Verify both sessions are tracked
	stats := dw.Stats()
	assert.Equal(t, 2, stats.WatcherCount)
	assert.Contains(t, stats.Watchers, session1+".jsonl")
	assert.Contains(t, stats.Watchers, session2+".jsonl")
}

func TestDirectoryWatcher_ConcurrentWrites(t *testing.T) {
	// Test that DirectoryWatcher handles concurrent writes to multiple session files
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "dirwatcher-concurrent-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sessions := []string{
		"aaaaaaaa-1111-2222-3333-444444444444",
		"bbbbbbbb-5555-6666-7777-888888888888",
	}

	// Create empty files first
	for _, session := range sessions {
		path := filepath.Join(tmpDir, session+".jsonl")
		f, _ := os.Create(path)
		f.Close()
	}

	cfg := DirectoryWatcherConfig{
		Directory:       tmpDir,
		Source:          "claude",
		EventBufferSize: 200,
		PollInterval:    20 * time.Millisecond,
	}

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)
	defer dw.Stop()

	// Write to both files concurrently
	linesPerSession := 20
	var wg sync.WaitGroup
	wg.Add(len(sessions))

	for i, session := range sessions {
		go func(idx int, sessionID string) {
			defer wg.Done()

			path := filepath.Join(tmpDir, sessionID+".jsonl")
			f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			defer f.Close()

			for j := 0; j < linesPerSession; j++ {
				entry := fmt.Sprintf(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"S%d-L%d"}]}}`, idx, j)
				f.WriteString(entry + "\n")
				time.Sleep(5 * time.Millisecond)
			}
			f.Sync()
		}(i, session)
	}

	wg.Wait()

	// Collect all events
	totalExpected := len(sessions) * linesPerSession
	var receivedEvents []types.ParsedEvent
	timeout := time.After(5 * time.Second)

	for len(receivedEvents) < totalExpected {
		select {
		case event := <-dw.Events():
			receivedEvents = append(receivedEvents, event)
		case <-timeout:
			t.Fatalf("Timeout, got %d events, expected %d", len(receivedEvents), totalExpected)
		}
	}

	assert.Equal(t, totalExpected, len(receivedEvents))

	// Verify we got events from both sessions
	sessionCounts := make(map[string]int)
	for _, event := range receivedEvents {
		if len(event.ContentPreview) >= 2 {
			sessionCounts[event.ContentPreview[:2]]++
		}
	}
	assert.Equal(t, linesPerSession, sessionCounts["S0"])
	assert.Equal(t, linesPerSession, sessionCounts["S1"])
}

func TestDirectoryWatcher_IgnoresNonUUID(t *testing.T) {
	// Test that DirectoryWatcher ignores non-UUID files
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "dirwatcher-ignore-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create non-UUID files
	nonUUIDFiles := []string{"transcript.jsonl", "settings.json", "notes.txt"}
	for _, name := range nonUUIDFiles {
		f, _ := os.Create(filepath.Join(tmpDir, name))
		f.WriteString(`{"type":"user","message":{"content":[{"type":"text","text":"ignore"}]}}` + "\n")
		f.Close()
	}

	// Create one valid UUID file
	validSession := "12345678-1234-1234-1234-123456789abc"
	f, err := os.Create(filepath.Join(tmpDir, validSession+".jsonl"))
	require.NoError(t, err)
	f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Valid UUID"}]}}` + "\n")
	f.Close()

	cfg := DirectoryWatcherConfig{
		Directory:       tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
	}

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)
	defer dw.Stop()

	// Should only get event from UUID file
	select {
	case event := <-dw.Events():
		assert.Equal(t, "Valid UUID", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Only one watcher should be active
	stats := dw.Stats()
	assert.Equal(t, 1, stats.WatcherCount)
}

func TestDirectoryWatcher_DirectoryNotExist(t *testing.T) {
	// Test that DirectoryWatcher handles non-existent directory gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := filepath.Join(os.TempDir(), "dirwatcher-nonexistent-"+time.Now().Format("20060102150405"))

	cfg := DirectoryWatcherConfig{
		Directory:       tmpDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    50 * time.Millisecond,
	}

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)
	defer dw.Stop()

	// Wait a bit, then create directory and file
	time.Sleep(100 * time.Millisecond)

	err = os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sessionID := "abcdefab-1234-5678-9abc-def012345678"
	f, err := os.Create(filepath.Join(tmpDir, sessionID+".jsonl"))
	require.NoError(t, err)
	f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"Dir created later"}]}}` + "\n")
	f.Close()

	// Should get the event
	select {
	case event := <-dw.Events():
		assert.Equal(t, "Dir created later", event.ContentPreview)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestDirectoryWatcher_ActiveSessions(t *testing.T) {
	// Test ActiveSessions() method
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "dirwatcher-active-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sessions := []string{
		"aaaaaaaa-1111-2222-3333-444444444444",
		"bbbbbbbb-5555-6666-7777-888888888888",
		"cccccccc-9999-aaaa-bbbb-cccccccccccc",
	}

	for _, session := range sessions {
		f, _ := os.Create(filepath.Join(tmpDir, session+".jsonl"))
		f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"test"}]}}` + "\n")
		f.Close()
	}

	cfg := DefaultDirectoryWatcherConfig(tmpDir, "claude")
	cfg.PollInterval = 50 * time.Millisecond

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)
	defer dw.Stop()

	// Wait for all watchers to be spawned
	time.Sleep(200 * time.Millisecond)

	activeSessions := dw.ActiveSessions()
	assert.Len(t, activeSessions, len(sessions))

	// Verify all sessions are in the list
	for _, session := range sessions {
		found := false
		for _, active := range activeSessions {
			if active == session+".jsonl" {
				found = true
				break
			}
		}
		assert.True(t, found, "Session %s should be active", session)
	}
}

func TestDirectoryWatcher_StopCleansUp(t *testing.T) {
	// Test that Stop() cleans up all child watchers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "dirwatcher-cleanup-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sessions := []string{
		"aaaaaaaa-1111-2222-3333-444444444444",
		"bbbbbbbb-5555-6666-7777-888888888888",
	}

	for _, session := range sessions {
		f, _ := os.Create(filepath.Join(tmpDir, session+".jsonl"))
		f.WriteString(`{"type":"user","timestamp":"2025-01-15T10:30:00.000Z","message":{"role":"user","content":[{"type":"text","text":"cleanup test"}]}}` + "\n")
		f.Close()
	}

	cfg := DefaultDirectoryWatcherConfig(tmpDir, "claude")
	cfg.PollInterval = 50 * time.Millisecond

	dw, err := NewDirectoryWatcher(ctx, cfg)
	require.NoError(t, err)

	err = dw.Start()
	require.NoError(t, err)

	// Wait for watchers to spawn
	time.Sleep(200 * time.Millisecond)

	stats := dw.Stats()
	assert.Equal(t, 2, stats.WatcherCount)

	// Stop should not hang
	done := make(chan struct{})
	go func() {
		dw.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() timed out")
	}

	// Verify watcher is closed
	finalStats := dw.Stats()
	assert.True(t, finalStats.Closed)
}

func TestDirectoryWatcher_ConfigValidation(t *testing.T) {
	ctx := context.Background()

	// Test missing directory
	_, err := NewDirectoryWatcher(ctx, DirectoryWatcherConfig{
		Source: "claude",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory path is required")

	// Test missing source
	_, err = NewDirectoryWatcher(ctx, DirectoryWatcherConfig{
		Directory: "/tmp/test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source is required")
}

func TestDirectoryWatcher_DefaultConfig(t *testing.T) {
	cfg := DefaultDirectoryWatcherConfig("/some/dir", "claude")

	assert.Equal(t, "/some/dir", cfg.Directory)
	assert.Equal(t, "claude", cfg.Source)
	assert.Equal(t, 1000, cfg.EventBufferSize)
	assert.Equal(t, 100*time.Millisecond, cfg.PollInterval)
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package watcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSEReader_BasicEvents(t *testing.T) {
	// Create a test SSE server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send 3 SSE events
		events := []string{
			`{"type":"session.created","properties":{"id":"sess-1"}}`,
			`{"type":"session.updated","properties":{"content":"hello"}}`,
			`{"type":"session.error","properties":{"message":"oops"}}`,
		}
		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reader := NewSSEReader(server.URL, "test-stream")
	lineChan := make(chan RawLine, 10)

	go func() {
		_ = reader.Start(ctx, lineChan)
	}()

	// Collect events
	var events []RawLine
	timeout := time.After(1 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case line := <-lineChan:
			events = append(events, line)
		case <-timeout:
			t.Fatal("Timed out waiting for events")
		}
	}

	require.Len(t, events, 3)
	assert.Contains(t, string(events[0].Line), "session.created")
	assert.Contains(t, string(events[1].Line), "session.updated")
	assert.Contains(t, string(events[2].Line), "session.error")
	assert.Equal(t, "test-stream", events[0].SourceFile)
}

func TestSSEReader_MultilineData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		// Send a multi-line data event
		fmt.Fprint(w, "data: {\"line1\": true,\n")
		fmt.Fprint(w, "data:  \"line2\": false}\n")
		fmt.Fprint(w, "\n")
		flusher.Flush()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reader := NewSSEReader(server.URL, "test-multiline")
	lineChan := make(chan RawLine, 10)

	go func() {
		_ = reader.Start(ctx, lineChan)
	}()

	select {
	case line := <-lineChan:
		// Multi-line data lines should be joined with \n
		assert.Equal(t, "{\"line1\": true,\n \"line2\": false}", string(line.Line))
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for event")
	}
}

func TestSSEReader_ReconnectsOnDisconnect(t *testing.T) {
	var mu sync.Mutex
	connectionCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectionCount++
		connNum := connectionCount
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send one event per connection, then close
		fmt.Fprintf(w, "data: {\"connection\":%d}\n\n", connNum)
		flusher.Flush()
		// Connection closes after handler returns
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	reader := NewSSEReader(server.URL, "test-reconnect")
	reader.initialBackoff = 100 * time.Millisecond // Fast backoff for testing
	reader.maxBackoff = 200 * time.Millisecond
	lineChan := make(chan RawLine, 10)

	go func() {
		_ = reader.Start(ctx, lineChan)
	}()

	// Should receive events from at least 2 connections
	var events []RawLine
	timeout := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case line := <-lineChan:
			events = append(events, line)
		case <-timeout:
			t.Fatalf("Timed out waiting for event %d", i)
		}
	}

	assert.GreaterOrEqual(t, len(events), 2, "Should have received events from multiple connections")

	mu.Lock()
	assert.GreaterOrEqual(t, connectionCount, 2, "Should have reconnected at least once")
	mu.Unlock()
}

func TestSSEReader_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Keep sending events until client disconnects
		for i := 0; ; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			fmt.Fprintf(w, "data: {\"i\":%d}\n\n", i)
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	reader := NewSSEReader(server.URL, "test-cancel")
	lineChan := make(chan RawLine, 100)

	done := make(chan error, 1)
	go func() {
		done <- reader.Start(ctx, lineChan)
	}()

	// Wait for at least one event
	select {
	case <-lineChan:
		// Got an event, now cancel
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for first event")
	}

	cancel()

	// Reader should stop
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Reader did not stop after context cancellation")
	}
}

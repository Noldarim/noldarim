// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package watcher

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

// SSEReader connects to a Server-Sent Events endpoint and emits events as RawLines.
type SSEReader struct {
	url        string
	streamName string
	client     *http.Client

	// Reconnection config
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

// NewSSEReader creates a new SSE reader for the given URL.
func NewSSEReader(url, streamName string) *SSEReader {
	return &SSEReader{
		url:            url,
		streamName:     streamName,
		client:         &http.Client{Timeout: 0}, // No timeout for streaming
		initialBackoff: time.Second,
		maxBackoff:     30 * time.Second,
	}
}

// Start begins reading SSE events and sending them to lineChan.
// Blocks until context is cancelled. Reconnects automatically on disconnect.
func (r *SSEReader) Start(ctx context.Context, lineChan chan<- RawLine) error {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := r.connect(ctx, lineChan)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Exponential backoff on disconnect
		attempt++
		backoff := time.Duration(float64(r.initialBackoff) * math.Pow(2, float64(attempt-1)))
		if backoff > r.maxBackoff {
			backoff = r.maxBackoff
		}

		log.Warn().Err(err).Str("url", r.url).Dur("backoff", backoff).
			Int("attempt", attempt).Msg("SSE disconnected, reconnecting")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}

// connect performs a single SSE connection and reads events until disconnect.
func (r *SSEReader) connect(ctx context.Context, lineChan chan<- RawLine) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return fmt.Errorf("sse: create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("sse: connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sse: unexpected status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024) // 1MB max line

	var dataLines []string

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Text()

		if line == "" {
			// Blank line = end of event, emit accumulated data
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				select {
				case lineChan <- RawLine{
					Line:       []byte(data),
					Timestamp:  time.Now(),
					SourceFile: r.streamName,
				}:
				case <-ctx.Done():
					return ctx.Err()
				}
				dataLines = dataLines[:0]
			}
			continue
		}

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimPrefix(data, " ") // Optional space after "data:"
			dataLines = append(dataLines, data)
		}
		// Ignore "event:", "id:", "retry:", and comment lines (starting with ":")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("sse: scan: %w", err)
	}

	return fmt.Errorf("sse: stream ended")
}

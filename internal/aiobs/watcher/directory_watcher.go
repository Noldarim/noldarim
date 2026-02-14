// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package watcher provides non-blocking file watching for AI transcript files.
package watcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

// ErrDirectoryWatcherClosed is returned when operations are attempted on a closed directory watcher.
var ErrDirectoryWatcherClosed = errors.New("directory watcher is closed")

// DirectoryWatcher watches a directory for UUID-named transcript files and manages
// individual TranscriptWatchers for each discovered file. It merges events from all
// active watchers into a single channel.
type DirectoryWatcher struct {
	dir           string
	source        string
	pollInterval  time.Duration
	bufferSize    int
	watchers      map[string]*TranscriptWatcher // UUID filename -> watcher
	eventChan     chan types.ParsedEvent
	errorChan     chan error
	doneChan      chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	initialized   bool
	closed        bool
	lastError     error
}

// DirectoryWatcherConfig holds configuration for a DirectoryWatcher.
type DirectoryWatcherConfig struct {
	// Directory is the path to watch for UUID-named .jsonl files.
	Directory string
	// Source identifies the AI tool (e.g., "claude", "gemini").
	Source string
	// EventBufferSize is the size of the merged event channel buffer.
	EventBufferSize int
	// PollInterval is how often to check for new files and content (default: 100ms).
	PollInterval time.Duration
}

// DefaultDirectoryWatcherConfig returns a DirectoryWatcherConfig with sensible defaults.
func DefaultDirectoryWatcherConfig(dir, source string) DirectoryWatcherConfig {
	return DirectoryWatcherConfig{
		Directory:       dir,
		Source:          source,
		EventBufferSize: 1000,
		PollInterval:    100 * time.Millisecond,
	}
}

// NewDirectoryWatcher creates a new directory watcher.
// The watcher must be started with Start() before events are emitted.
func NewDirectoryWatcher(ctx context.Context, cfg DirectoryWatcherConfig) (*DirectoryWatcher, error) {
	if cfg.Directory == "" {
		return nil, fmt.Errorf("%w: directory path is required", ErrInitFailed)
	}
	if cfg.Source == "" {
		return nil, fmt.Errorf("%w: source is required", ErrInitFailed)
	}

	if cfg.EventBufferSize == 0 {
		cfg.EventBufferSize = 1000
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}

	watchCtx, cancel := context.WithCancel(ctx)

	dw := &DirectoryWatcher{
		dir:          cfg.Directory,
		source:       cfg.Source,
		pollInterval: cfg.PollInterval,
		bufferSize:   cfg.EventBufferSize,
		watchers:     make(map[string]*TranscriptWatcher),
		eventChan:    make(chan types.ParsedEvent, cfg.EventBufferSize),
		errorChan:    make(chan error, 10),
		doneChan:     make(chan struct{}),
		ctx:          watchCtx,
		cancel:       cancel,
	}

	return dw, nil
}

// Start begins watching the directory for UUID transcript files.
// It returns immediately; events are emitted asynchronously on the Events() channel.
func (dw *DirectoryWatcher) Start() error {
	dw.mu.Lock()
	if dw.closed {
		dw.mu.Unlock()
		return ErrDirectoryWatcherClosed
	}
	if dw.initialized {
		dw.mu.Unlock()
		return nil // Already started
	}
	dw.mu.Unlock()

	// Verify directory is accessible (but don't require it to exist yet)
	if _, err := os.Stat(dw.dir); err != nil && !os.IsNotExist(err) {
		dw.lastError = fmt.Errorf("%w: cannot access directory %s: %v", ErrInitFailed, dw.dir, err)
		return dw.lastError
	}

	dw.mu.Lock()
	dw.initialized = true
	dw.mu.Unlock()

	log.Info().Str("dir", dw.dir).Str("source", dw.source).Msg("Directory watcher started")

	go dw.watch()

	return nil
}

// Events returns the channel on which merged events from all watchers are emitted.
func (dw *DirectoryWatcher) Events() <-chan types.ParsedEvent {
	return dw.eventChan
}

// Errors returns the channel on which non-fatal errors are reported.
func (dw *DirectoryWatcher) Errors() <-chan error {
	return dw.errorChan
}

// Done returns a channel that's closed when the directory watcher stops.
func (dw *DirectoryWatcher) Done() <-chan struct{} {
	return dw.doneChan
}

// Stop stops the directory watcher and all managed transcript watchers.
func (dw *DirectoryWatcher) Stop() {
	dw.mu.Lock()
	if dw.closed {
		dw.mu.Unlock()
		return
	}
	dw.closed = true
	dw.mu.Unlock()

	dw.cancel()
	// Wait for the watch goroutine to finish
	<-dw.doneChan
	log.Info().Str("dir", dw.dir).Int("watcherCount", len(dw.watchers)).Msg("Directory watcher stopped")
}

// Stats returns directory watcher statistics.
func (dw *DirectoryWatcher) Stats() DirectoryWatcherStats {
	dw.mu.RLock()
	defer dw.mu.RUnlock()

	watcherStats := make(map[string]WatcherStats)
	for uuid, w := range dw.watchers {
		watcherStats[uuid] = w.Stats()
	}

	return DirectoryWatcherStats{
		Directory:    dw.dir,
		Source:       dw.source,
		WatcherCount: len(dw.watchers),
		Watchers:     watcherStats,
		Initialized:  dw.initialized,
		Closed:       dw.closed,
		LastError:    dw.lastError,
	}
}

// ActiveSessions returns the UUIDs of all active sessions being watched.
func (dw *DirectoryWatcher) ActiveSessions() []string {
	dw.mu.RLock()
	defer dw.mu.RUnlock()

	sessions := make([]string, 0, len(dw.watchers))
	for uuid := range dw.watchers {
		sessions = append(sessions, uuid)
	}
	return sessions
}

// DirectoryWatcherStats contains directory watcher statistics.
type DirectoryWatcherStats struct {
	Directory    string
	Source       string
	WatcherCount int
	Watchers     map[string]WatcherStats // UUID -> stats
	Initialized  bool
	Closed       bool
	LastError    error
}

func (dw *DirectoryWatcher) watch() {
	defer close(dw.doneChan)
	defer close(dw.eventChan)
	defer close(dw.errorChan)
	defer dw.stopAllWatchers()

	ticker := time.NewTicker(dw.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-dw.ctx.Done():
			return
		case <-ticker.C:
			dw.scanForNewFiles()
		}
	}
}

func (dw *DirectoryWatcher) scanForNewFiles() {
	entries, err := os.ReadDir(dw.dir)
	if err != nil {
		if !os.IsNotExist(err) {
			dw.reportError(fmt.Errorf("failed to read directory: %w", err))
		}
		// Directory doesn't exist yet, keep polling
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !uuidFileRegex.MatchString(entry.Name()) {
			continue
		}

		// Check if we already have a watcher for this file
		dw.mu.RLock()
		_, exists := dw.watchers[entry.Name()]
		dw.mu.RUnlock()

		if !exists {
			dw.spawnWatcher(entry.Name())
		}
	}
}

func (dw *DirectoryWatcher) spawnWatcher(filename string) {
	filePath := filepath.Join(dw.dir, filename)

	cfg := Config{
		FilePath:        filePath,
		Source:          dw.source,
		EventBufferSize: dw.bufferSize / 10, // Smaller buffer per watcher
		PollInterval:    dw.pollInterval,
		DiscoverUUID:    false, // Direct file mode since we know the path
	}

	watcher, err := NewTranscriptWatcher(dw.ctx, cfg)
	if err != nil {
		dw.reportError(fmt.Errorf("failed to create watcher for %s: %w", filename, err))
		return
	}

	if err := watcher.Start(); err != nil {
		dw.reportError(fmt.Errorf("failed to start watcher for %s: %w", filename, err))
		return
	}

	dw.mu.Lock()
	dw.watchers[filename] = watcher
	dw.mu.Unlock()

	log.Info().Str("file", filename).Msg("Spawned new transcript watcher for session")

	// Start goroutine to forward events from this watcher
	go dw.forwardEvents(filename, watcher)
}

func (dw *DirectoryWatcher) forwardEvents(filename string, watcher *TranscriptWatcher) {
	for {
		select {
		case <-dw.ctx.Done():
			return
		case event, ok := <-watcher.Events():
			if !ok {
				// Watcher's event channel closed
				log.Info().Str("file", filename).Msg("Watcher event channel closed")
				return
			}
			// Forward event to merged channel
			select {
			case dw.eventChan <- event:
			default:
				dw.reportError(fmt.Errorf("event channel full, dropping event from %s", filename))
			}
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			// Forward error with context
			dw.reportError(fmt.Errorf("watcher %s: %w", filename, err))
		}
	}
}

func (dw *DirectoryWatcher) stopAllWatchers() {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	for filename, watcher := range dw.watchers {
		log.Info().Str("file", filename).Msg("Stopping transcript watcher")
		// Use a goroutine to avoid blocking if Stop() is slow
		go watcher.Stop()
	}
	// Clear the map
	dw.watchers = make(map[string]*TranscriptWatcher)
}

func (dw *DirectoryWatcher) reportError(err error) {
	dw.mu.Lock()
	dw.lastError = err
	dw.mu.Unlock()

	select {
	case dw.errorChan <- err:
	default:
		// Error channel full, log and continue
		log.Error().Err(err).Msg("Error channel full, dropping error")
	}
}

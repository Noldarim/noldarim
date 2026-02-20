// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package watcher provides non-blocking file watching for AI transcript files.
package watcher

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	"github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/logger"
)

// uuidFileRegex matches UUID-named .jsonl files (Claude session transcripts)
var uuidFileRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.jsonl$`)

var log = logger.GetLogger("aiobs.watcher")

// ErrWatcherClosed is returned when operations are attempted on a closed watcher.
var ErrWatcherClosed = errors.New("watcher is closed")

// ErrInitFailed is returned when the watcher fails to initialize.
var ErrInitFailed = errors.New("watcher initialization failed")

// RawLine represents an unparsed line from a transcript file.
// Used in raw mode where parsing is deferred to the orchestrator.
type RawLine struct {
	// Line is the raw bytes from the transcript file
	Line []byte `json:"line"`

	// Timestamp when the line was read
	Timestamp time.Time `json:"timestamp"`
}

// activeFile tracks the state of a single file being watched
type activeFile struct {
	path   string
	file   *os.File
	reader *bufio.Reader
	offset int64
}

// TranscriptWatcher watches transcript JSONL files and emits ParsedEvent.
// It uses non-blocking I/O to tail files without blocking other operations.
// When DiscoverUUID is enabled, it watches a directory and discovers UUID-named files.
// Multiple files can be watched simultaneously (for multi-step pipelines).
// When RawMode is enabled, it emits raw lines without parsing (for orchestrator-side parsing).
type TranscriptWatcher struct {
	filePath     string // Single file path (non-discovery mode)
	discoverDir  string // Directory to search for UUID files (if discovery mode)
	source       string
	pollInterval time.Duration
	adapter      types.Adapter
	eventChan    chan types.ParsedEvent
	rawEventChan chan RawLine // Raw line channel (used when RawMode is enabled)
	rawMode      bool         // When true, emit raw lines instead of parsed events
	errorChan    chan error
	doneChan     chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	initialized  bool
	closed       bool
	linesRead    int64
	lineNumber   int64                  // Current line number for RawEntry
	lastError    error
	activeFiles  map[string]*activeFile // All files currently being watched
}

// Config holds configuration for a TranscriptWatcher.
type Config struct {
	// FilePath is the path to the transcript JSONL file.
	// If DiscoverUUID is true, this should be a directory path instead.
	FilePath string
	// Source identifies the AI tool (e.g., "claude", "gemini").
	Source string
	// EventBufferSize is the size of the event channel buffer.
	EventBufferSize int
	// PollInterval is how often to check for new content (default: 100ms).
	PollInterval time.Duration
	// DiscoverUUID enables UUID file discovery mode.
	// When true, FilePath is treated as a directory and the watcher will
	// search for UUID-named .jsonl files (e.g., "88ad3a71-4c86-4b19-b41d-71a7b027ee63.jsonl").
	// This is useful when the exact session ID is not known ahead of time.
	DiscoverUUID bool
	// RawMode enables raw line emission mode.
	// When true, the watcher emits raw bytes via RawEvents() instead of parsed events via Events().
	// This is used when parsing should be done on the orchestrator side rather than in the agent.
	RawMode bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(filePath, source string) Config {
	return Config{
		FilePath:        filePath,
		Source:          source,
		EventBufferSize: 1000,
		PollInterval:    100 * time.Millisecond,
	}
}

// NewTranscriptWatcher creates a new transcript watcher.
// The watcher must be started with Start() before events are emitted.
func NewTranscriptWatcher(ctx context.Context, cfg Config) (*TranscriptWatcher, error) {
	// In raw mode, we don't need the adapter (parsing happens on orchestrator)
	var adapter types.Adapter
	if !cfg.RawMode {
		var ok bool
		adapter, ok = adapters.Get(cfg.Source)
		if !ok {
			return nil, fmt.Errorf("%w: unknown source %q", ErrInitFailed, cfg.Source)
		}
	}

	if cfg.EventBufferSize == 0 {
		cfg.EventBufferSize = 1000
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}

	watchCtx, cancel := context.WithCancel(ctx)

	w := &TranscriptWatcher{
		source:       cfg.Source,
		pollInterval: cfg.PollInterval,
		adapter:      adapter,
		rawMode:      cfg.RawMode,
		errorChan:    make(chan error, 10),
		doneChan:     make(chan struct{}),
		ctx:          watchCtx,
		cancel:       cancel,
		activeFiles:  make(map[string]*activeFile),
	}

	// Initialize the appropriate event channel based on mode
	if cfg.RawMode {
		w.rawEventChan = make(chan RawLine, cfg.EventBufferSize)
	} else {
		w.eventChan = make(chan types.ParsedEvent, cfg.EventBufferSize)
	}

	// Configure discovery mode or direct file mode
	if cfg.DiscoverUUID {
		w.discoverDir = cfg.FilePath // FilePath is treated as directory in discovery mode
	} else {
		w.filePath = cfg.FilePath
	}

	return w, nil
}

// Start begins watching the transcript file.
// It returns immediately; events are emitted asynchronously on the Events() channel.
// Returns an error if initialization fails (e.g., directory doesn't exist in discovery mode).
func (w *TranscriptWatcher) Start() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrWatcherClosed
	}
	if w.initialized {
		w.mu.Unlock()
		return nil // Already started
	}
	w.mu.Unlock()

	// Verify we can access the target path
	if w.discoverDir != "" {
		// Discovery mode: verify directory exists or can be accessed
		if _, err := os.Stat(w.discoverDir); err != nil && !os.IsNotExist(err) {
			w.lastError = fmt.Errorf("%w: cannot access discovery directory %s: %v", ErrInitFailed, w.discoverDir, err)
			return w.lastError
		}
		log.Info().Str("dir", w.discoverDir).Str("source", w.source).Msg("Transcript watcher started in multi-file discovery mode")
	} else if w.filePath != "" {
		// Direct file mode: verify parent directory exists
		dir := filepath.Dir(w.filePath)
		if _, err := os.Stat(dir); err != nil && !os.IsNotExist(err) {
			w.lastError = fmt.Errorf("%w: cannot access directory %s: %v", ErrInitFailed, dir, err)
			return w.lastError
		}
		log.Info().Str("file", w.filePath).Str("source", w.source).Msg("Transcript watcher started")
	}

	w.mu.Lock()
	w.initialized = true
	w.mu.Unlock()

	go w.watch()

	return nil
}

// Events returns the channel on which parsed events are emitted.
// The channel is buffered and will not block the watcher.
// Returns nil if the watcher is in RawMode - use RawEvents() instead.
func (w *TranscriptWatcher) Events() <-chan types.ParsedEvent {
	return w.eventChan
}

// RawEvents returns the channel on which raw lines are emitted.
// Only available when RawMode is enabled in Config.
// Returns nil if the watcher is not in RawMode - use Events() instead.
func (w *TranscriptWatcher) RawEvents() <-chan RawLine {
	return w.rawEventChan
}

// IsRawMode returns whether the watcher is in raw mode.
func (w *TranscriptWatcher) IsRawMode() bool {
	return w.rawMode
}

// Errors returns the channel on which non-fatal errors are reported.
func (w *TranscriptWatcher) Errors() <-chan error {
	return w.errorChan
}

// Done returns a channel that's closed when the watcher stops.
func (w *TranscriptWatcher) Done() <-chan struct{} {
	return w.doneChan
}

// Stop stops the watcher and closes all channels.
func (w *TranscriptWatcher) Stop() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.mu.Unlock()

	w.cancel()
	// Wait for the watch goroutine to finish
	<-w.doneChan
	log.Info().Int("activeFiles", len(w.activeFiles)).Int64("linesRead", w.linesRead).Msg("Transcript watcher stopped")
}

// Stats returns watcher statistics.
func (w *TranscriptWatcher) Stats() WatcherStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Collect active file names
	activeFileNames := make([]string, 0, len(w.activeFiles))
	for path := range w.activeFiles {
		activeFileNames = append(activeFileNames, filepath.Base(path))
	}

	return WatcherStats{
		FilePath:        w.filePath,
		DiscoverDir:     w.discoverDir,
		ActiveFiles:     activeFileNames,
		ActiveFileCount: len(w.activeFiles),
		Source:          w.source,
		LinesRead:       w.linesRead,
		Initialized:     w.initialized,
		Closed:          w.closed,
		LastError:       w.lastError,
	}
}

// WatcherStats contains watcher statistics.
type WatcherStats struct {
	FilePath        string   // Single file path (non-discovery mode)
	DiscoverDir     string   // Directory being watched (if discovery mode)
	ActiveFiles     []string // Names of files currently being watched
	ActiveFileCount int      // Number of files currently being watched
	Source          string
	LinesRead       int64
	Initialized     bool
	Closed          bool
	LastError       error
}

func (w *TranscriptWatcher) watch() {
	defer close(w.doneChan)
	defer func() {
		// Close all active files
		for _, af := range w.activeFiles {
			if af.file != nil {
				af.file.Close()
			}
		}
		// Close the appropriate event channel based on mode
		if w.rawMode {
			close(w.rawEventChan)
		} else {
			close(w.eventChan)
		}
	}()
	defer close(w.errorChan)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			// Discovery mode: scan for new UUID files
			if w.discoverDir != "" {
				w.discoverAndAddNewFiles()
			} else if w.filePath != "" && len(w.activeFiles) == 0 {
				// Single file mode: try to open the file if not yet tracked
				w.tryOpenSingleFile()
			}

			// Read from ALL active files
			for _, af := range w.activeFiles {
				w.readAvailableLines(af)
			}
		}
	}
}

// discoverAndAddNewFiles scans for new UUID files and adds them to activeFiles
func (w *TranscriptWatcher) discoverAndAddNewFiles() {
	entries, err := os.ReadDir(w.discoverDir)
	if err != nil {
		if !os.IsNotExist(err) {
			w.reportError(fmt.Errorf("failed to read discovery directory: %w", err))
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !uuidFileRegex.MatchString(entry.Name()) {
			continue
		}

		fullPath := filepath.Join(w.discoverDir, entry.Name())

		// Skip files we're already watching
		if _, exists := w.activeFiles[fullPath]; exists {
			continue
		}

		// Try to open the new file
		file, err := os.Open(fullPath)
		if err != nil {
			if !os.IsNotExist(err) {
				w.reportError(fmt.Errorf("failed to open transcript file %s: %w", fullPath, err))
			}
			continue
		}

		af := &activeFile{
			path:   fullPath,
			file:   file,
			reader: bufio.NewReader(file),
			offset: 0,
		}
		w.activeFiles[fullPath] = af
		log.Info().Str("file", entry.Name()).Int("totalFiles", len(w.activeFiles)).Msg("Now watching new transcript file")
	}
}

// tryOpenSingleFile attempts to open the single configured file
func (w *TranscriptWatcher) tryOpenSingleFile() {
	file, err := os.Open(w.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			w.reportError(fmt.Errorf("failed to open transcript file: %w", err))
		}
		return
	}

	af := &activeFile{
		path:   w.filePath,
		file:   file,
		reader: bufio.NewReader(file),
		offset: 0,
	}
	w.activeFiles[w.filePath] = af
	log.Info().Str("file", w.filePath).Msg("Now watching transcript file")
}

func (w *TranscriptWatcher) readAvailableLines(af *activeFile) {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		line, err := af.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// No more data available right now
				return
			}
			w.reportError(fmt.Errorf("error reading transcript %s: %w", af.path, err))
			return
		}

		af.offset += int64(len(line))
		w.mu.Lock()
		w.linesRead++
		w.mu.Unlock()

		// Skip empty lines
		if len(line) <= 1 {
			continue
		}

		// Parse and emit event
		w.processLine(line)
	}
}

func (w *TranscriptWatcher) processLine(line []byte) {
	if w.rawMode {
		// Raw mode: emit the line as-is without parsing
		rawLine := RawLine{
			Line:      line,
			Timestamp: time.Now(),
		}

		// Non-blocking send to raw event channel
		select {
		case w.rawEventChan <- rawLine:
		default:
			// Channel full, drop event and report
			w.reportError(fmt.Errorf("raw event channel full, dropping event"))
		}
		return
	}

	// Parsed mode: parse and emit structured events
	w.mu.Lock()
	w.lineNumber++
	lineNum := int(w.lineNumber)
	w.mu.Unlock()

	rawEntry := types.RawEntry{
		Line:      lineNum,
		Data:      json.RawMessage(line),
		SessionID: types.ExtractSessionID(json.RawMessage(line)),
	}

	events, err := w.adapter.ParseEntry(rawEntry)
	if err != nil {
		w.reportError(fmt.Errorf("failed to parse transcript line %d: %w", lineNum, err))
		return
	}

	// Emit all parsed events (one entry can produce multiple events)
	for _, event := range events {
		// Non-blocking send to event channel
		select {
		case w.eventChan <- event:
		default:
			// Channel full, drop event and report
			w.reportError(fmt.Errorf("event channel full, dropping event"))
		}
	}
}

func (w *TranscriptWatcher) reportError(err error) {
	w.mu.Lock()
	w.lastError = err
	w.mu.Unlock()

	select {
	case w.errorChan <- err:
	default:
		// Error channel full, log and continue
		log.Error().Err(err).Msg("Error channel full, dropping error")
	}
}

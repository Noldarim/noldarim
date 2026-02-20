// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command obsharness is a standalone observability harness for testing the
// AI event pipeline without running the full application.
//
// It watches a transcript directory, parses events through the adapter,
// saves them to the database, and shows how TUI would display each event.
//
// Usage:
//
//	go run cmd/dev/obsharness/main.go --watch /path/to/transcript/dir
//	go run cmd/dev/obsharness/main.go --watch /path/to/transcript/dir --task-id my-test-task
//	go run cmd/dev/obsharness/main.go --file /path/to/transcript.jsonl --no-save
//	go run cmd/dev/obsharness/main.go --file /path/to/transcript.jsonl --tui
//
// You can then write test events to the watched directory:
//
//	echo '{"type":"tool_use","tool":{"name":"Bash","input":{"command":"ls"}}}' >> /path/to/test.jsonl
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	"github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/aiobs/watcher"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/tui/components/hooksactivity"
)

func main() {
	watchDir := flag.String("watch", "", "Directory to watch for transcript files")
	inputFile := flag.String("file", "", "Single file to process (non-watching mode)")
	taskID := flag.String("task-id", "dev-harness", "Task ID to use for saved events")
	projectID := flag.String("project-id", "dev-project", "Project ID to use for saved events")
	noSave := flag.Bool("no-save", false, "Don't save events to database")
	showRaw := flag.Bool("raw", false, "Show raw JSON payload")
	configFile := flag.String("config", "test-config.yaml", "Config file path")
	verbose := flag.Bool("verbose", false, "Show verbose output including parse details")
	useTUI := flag.Bool("tui", false, "Use real TUI component for display")

	flag.Parse()

	if *watchDir == "" && *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: obsharness --watch <dir> or --file <file>\n")
		fmt.Fprintf(os.Stderr, "\nWatch mode: monitors directory for transcript files\n")
		fmt.Fprintf(os.Stderr, "File mode:  processes a single transcript file\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		fmt.Fprintf(os.Stderr, "  --tui     Use real Bubble Tea TUI component\n")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register adapters
	adapters.RegisterAll()

	// Setup data service if saving is enabled
	var ds *services.DataService
	if !*noSave {
		cfg, err := config.NewConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to load config, running without DB: %v\n", err)
			*noSave = true
		} else {
			ds, err = services.NewDataService(cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to create data service, running without DB: %v\n", err)
				*noSave = true
			} else {
				defer ds.Close()
			}
		}
	}

	// Get adapter
	adapter, ok := adapters.Get("claude")
	if !ok {
		fmt.Fprintf(os.Stderr, "Claude adapter not registered\n")
		os.Exit(1)
	}

	processor := &eventProcessor{
		adapter:   adapter,
		ds:        ds,
		taskID:    *taskID,
		projectID: *projectID,
		noSave:    *noSave,
		showRaw:   *showRaw,
		verbose:   *verbose,
	}

	if *inputFile != "" {
		if *useTUI {
			// TUI mode - process file with real Bubble Tea component
			processFileWithTUI(*inputFile, processor)
		} else {
			// File mode - process single file
			processFile(*inputFile, processor)
		}
		return
	}

	// Watch mode
	runWatchMode(ctx, *watchDir, processor)
}

type eventProcessor struct {
	adapter   adapters.Adapter
	ds        *services.DataService
	taskID    string
	projectID string
	noSave    bool
	showRaw   bool
	verbose   bool
	count     int
}

func (p *eventProcessor) process(ctx context.Context, rawLine []byte, timestamp time.Time) {
	p.count++

	fmt.Printf("\n%s EVENT #%d %s\n",
		strings.Repeat("=", 20),
		p.count,
		strings.Repeat("=", 20))
	fmt.Printf("Time: %s\n", timestamp.Format("15:04:05.000"))

	if p.showRaw {
		fmt.Printf("Raw:  %s\n", truncate(string(rawLine), 200))
	}

	rawEntry := types.RawEntry{
		Line:      p.count,
		Data:      json.RawMessage(rawLine),
		SessionID: types.ExtractSessionID(json.RawMessage(rawLine)),
	}

	// Parse the event using new adapter API
	events, err := p.adapter.ParseEntry(rawEntry)
	if err != nil {
		fmt.Printf("PARSE ERROR: %v\n", err)
		printTUIRepresentation(nil, rawLine)
		return
	}

	// Process each parsed event
	for i, event := range events {
		if len(events) > 1 {
			fmt.Printf("\n  [Sub-event %d/%d]\n", i+1, len(events))
		}

		if p.verbose {
			fmt.Printf("\nParsed:\n")
			fmt.Printf("  EventType:    %s\n", event.EventType)
			fmt.Printf("  IsHumanInput: %v\n", event.IsHumanInput)
			fmt.Printf("  SessionID:    %s\n", event.SessionID)
			if event.Model != "" {
				fmt.Printf("  Model:        %s\n", event.Model)
			}
			if event.InputTokens > 0 || event.OutputTokens > 0 {
				fmt.Printf("  Tokens:       in=%d out=%d\n", event.InputTokens, event.OutputTokens)
			}
		}

		// Save to database if enabled
		if !p.noSave && p.ds != nil {
			// Set EventID and RawPayload before conversion
			event.EventID = models.GenerateEventID()
			event.RawPayload = rawLine
			record := models.NewAIActivityRecordFromParsed(event, p.taskID, "", "") // Empty RunID/StepID for dev harness
			if err := p.ds.SaveAIActivityRecord(ctx, record); err != nil {
				fmt.Printf("  DB SAVE ERROR: %v\n", err)
			} else if p.verbose {
				fmt.Printf("  Saved: %s\n", record.EventID)
			}
		}

		// Show TUI representation
		fmt.Printf("\nTUI Display:\n")
		printTUIRepresentation(&event, rawLine)
	}
}

func processFile(filePath string, processor *eventProcessor) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	fmt.Printf("Processing file: %s\n", filePath)
	fmt.Println(strings.Repeat("=", 60))

	ctx := context.Background()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// Make a copy since scanner reuses buffer
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		processor.process(ctx, lineCopy, time.Now())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Printf("Processed %d events\n", processor.count)
}

// ============================================================================
// TUI Mode - Real Bubble Tea component
// ============================================================================

// tuiModel wraps the hooksactivity component for standalone use
type tuiModel struct {
	activity  hooksactivity.Model
	events    []*models.AIActivityRecord
	eventIdx  int
	width     int
	height    int
	autoPlay  bool
	done      bool
	processor *eventProcessor
}

// newActivityRecordMsg is sent when a new record should be displayed
type newActivityRecordMsg struct {
	record *models.AIActivityRecord
}

// tickMsg triggers the next event in autoplay mode
type tickMsg struct{}

func (m tuiModel) Init() tea.Cmd {
	// Start autoplay after a short delay
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case " ", "n", "enter":
			// Manual advance to next event
			if m.eventIdx < len(m.events) {
				record := m.events[m.eventIdx]
				m.activity.AddEvent(record)
				m.eventIdx++
			}
			if m.eventIdx >= len(m.events) {
				m.done = true
			}
		case "a":
			// Toggle autoplay
			m.autoPlay = !m.autoPlay
			if m.autoPlay {
				return m, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
					return tickMsg{}
				})
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.activity.SetSize(msg.Width-4, msg.Height-4)

	case tickMsg:
		if m.autoPlay && m.eventIdx < len(m.events) {
			record := m.events[m.eventIdx]
			m.activity.AddEvent(record)
			m.eventIdx++
			return m, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
				return tickMsg{}
			})
		}
		if m.eventIdx >= len(m.events) {
			m.done = true
		}
	}

	// Update the activity component
	var cmd tea.Cmd
	m.activity, cmd = m.activity.Update(msg)
	return m, cmd
}

func (m tuiModel) View() string {
	status := fmt.Sprintf(" Event %d/%d ", m.eventIdx, len(m.events))
	if m.autoPlay {
		status += "[AUTO] "
	}
	if m.done {
		status += "[DONE - press q to quit] "
	} else {
		status += "[space=next, a=autoplay, q=quit] "
	}

	return m.activity.View() + "\n" + status
}

// processFileWithTUI processes a transcript file using the real TUI component
func processFileWithTUI(filePath string, processor *eventProcessor) {
	// First, parse all events from the file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var records []*models.AIActivityRecord
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		rawEntry := types.RawEntry{
			Line:      len(records) + 1,
			Data:      json.RawMessage(line),
			SessionID: types.ExtractSessionID(json.RawMessage(line)),
		}

		// Parse the event
		parsedEvents, err := processor.adapter.ParseEntry(rawEntry)
		if err != nil {
			continue
		}

		// Convert each parsed event to AIActivityRecord
		for _, parsed := range parsedEvents {
			parsed.EventID = models.GenerateEventID()
			parsed.RawPayload = line
			record := models.NewAIActivityRecordFromParsed(parsed, processor.taskID, "", "") // Empty RunID/StepID for dev harness
			records = append(records, record)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
	}

	if len(records) == 0 {
		fmt.Printf("No events parsed from file: %s\n", filePath)
		fmt.Printf("This might mean the file is empty or contains only unsupported entry types.\n")
		return
	}

	// Create the TUI model with preloaded records
	activity := hooksactivity.New(processor.taskID, 80, 24)
	activity.SetFocus(true)
	activity.StartStream()

	m := tuiModel{
		activity:  activity,
		events:    records,
		eventIdx:  0,
		autoPlay:  true, // Start with autoplay
		processor: processor,
	}

	// Run Bubble Tea
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Processed %d events\n", len(records))
}

func runWatchMode(ctx context.Context, watchDir string, processor *eventProcessor) {
	fmt.Printf("Watching directory: %s\n", watchDir)
	fmt.Printf("Task ID: %s\n", processor.taskID)
	fmt.Printf("Save to DB: %v\n", !processor.noSave)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Waiting for events... (Ctrl+C to stop)")
	fmt.Println("Write test events to a .jsonl file in the watched directory")
	fmt.Println()

	// Create watcher
	cfg := watcher.Config{
		FilePath:        watchDir,
		Source:          "claude",
		EventBufferSize: 100,
		PollInterval:    100 * time.Millisecond,
		DiscoverUUID:    true,
		RawMode:         true,
	}

	w, err := watcher.NewTranscriptWatcher(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create watcher: %v\n", err)
		os.Exit(1)
	}

	if err := w.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start watcher: %v\n", err)
		os.Exit(1)
	}
	defer w.Stop()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	rawEvents := w.RawEvents()
	errors := w.Errors()
	done := w.Done()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nContext cancelled, stopping...")
			return

		case <-sigChan:
			fmt.Printf("\n\nStopping... Processed %d events total\n", processor.count)
			return

		case <-done:
			fmt.Printf("\nWatcher done. Processed %d events total\n", processor.count)
			return

		case rawLine, ok := <-rawEvents:
			if !ok {
				fmt.Printf("\nEvent channel closed. Processed %d events total\n", processor.count)
				return
			}
			processor.process(ctx, rawLine.Line, rawLine.Timestamp)

		case err := <-errors:
			fmt.Printf("Watcher error: %v\n", err)
		}
	}
}

func printTUIRepresentation(event *types.ParsedEvent, rawLine []byte) {
	if event == nil {
		// Parse failed - show what we can
		var raw map[string]interface{}
		if err := json.Unmarshal(rawLine, &raw); err == nil {
			if t, ok := raw["type"].(string); ok {
				fmt.Printf("  [unparsed: %s]\n", t)
			} else {
				fmt.Printf("  [unparsed event]\n")
			}
		} else {
			fmt.Printf("  [invalid JSON]\n")
		}
		return
	}

	switch event.EventType {
	case types.EventTypeToolUse:
		if event.ToolName != "" {
			if event.ToolInputSummary != "" {
				fmt.Printf("  > %s: %s\n", event.ToolName, truncate(event.ToolInputSummary, 80))
			} else {
				fmt.Printf("  > %s\n", event.ToolName)
			}
		} else {
			fmt.Printf("  > [tool_use]\n")
		}

	case types.EventTypeToolResult:
		status := "OK"
		if event.ToolSuccess != nil && !*event.ToolSuccess {
			status = "ERR"
		}
		toolName := event.ToolName
		if toolName == "" {
			toolName = "tool"
		}
		if event.ContentPreview != "" {
			fmt.Printf("  < %s [%s]: %s\n", toolName, status, truncate(event.ContentPreview, 60))
		} else {
			fmt.Printf("  < %s [%s]\n", toolName, status)
		}

	case types.EventTypeThinking:
		if event.ContentPreview != "" {
			fmt.Printf("  ~ %s\n", truncate(event.ContentPreview, 100))
		} else {
			fmt.Printf("  ~ [thinking]\n")
		}

	case types.EventTypeAIOutput:
		if event.ContentPreview != "" {
			fmt.Printf("    %s\n", truncate(event.ContentPreview, 120))
		} else {
			fmt.Printf("    [output]\n")
		}

	case types.EventTypeSessionEnd:
		fmt.Printf("  X Session ended\n")

	case types.EventTypeError:
		if event.ContentPreview != "" {
			fmt.Printf("  ! Error: %s\n", truncate(event.ContentPreview, 100))
		} else {
			fmt.Printf("  ! Error\n")
		}

	case types.EventTypeUserPrompt:
		if event.ContentPreview != "" {
			fmt.Printf("  @ User: %s\n", truncate(event.ContentPreview, 100))
		} else {
			fmt.Printf("  @ User prompt\n")
		}

	default:
		fmt.Printf("  [%s]\n", event.EventType)
	}
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)

	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return "..."
	}
	return s[:max-3] + "..."
}

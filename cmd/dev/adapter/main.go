// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command adapter parses Claude transcript.jsonl files through the adapter.
// Usage:
//
//	go run cmd/dev/adapter/main.go <transcript.jsonl>
//	go run cmd/dev/adapter/main.go --raw <transcript.jsonl>
//	go run cmd/dev/adapter/main.go --line 164 <transcript.jsonl>
//	go run cmd/dev/adapter/main.go --type tool_use <transcript.jsonl>
//	go run cmd/dev/adapter/main.go --stats <transcript.jsonl>
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	"github.com/noldarim/noldarim/internal/aiobs/types"
)

var (
	showRaw    bool
	lineFilter int
	typeFilter string
	startLine  int
	endLine    int
	showStats  bool
)

func init() {
	// Register adapters
	adapters.RegisterAll()
}

func main() {
	flag.BoolVar(&showRaw, "raw", false, "Show raw JSON payload")
	flag.IntVar(&lineFilter, "line", 0, "Show only specific line number")
	flag.StringVar(&typeFilter, "type", "", "Filter by event type (tool_use, tool_result, thinking, ai_output, user_prompt, etc.)")
	flag.IntVar(&startLine, "from", 0, "Start from line number")
	flag.IntVar(&endLine, "to", 0, "End at line number")
	flag.BoolVar(&showStats, "stats", false, "Show token usage statistics")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <transcript.jsonl>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s transcript.jsonl                    # Parse all lines\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --raw --line 164 transcript.jsonl   # Show raw JSON for line 164\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --type tool_use transcript.jsonl    # Show only tool_use events\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --from 100 --to 110 transcript.jsonl # Show lines 100-110\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --stats transcript.jsonl            # Show token statistics\n", os.Args[0])
		os.Exit(1)
	}

	filename := args[0]
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	adapter, ok := adapters.Get("claude")
	if !ok {
		fmt.Fprintf(os.Stderr, "Claude adapter not registered\n")
		os.Exit(1)
	}

	scanner := bufio.NewScanner(file)
	// Increase buffer for large JSON lines
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	lineNum := 0
	matchCount := 0

	// Stats tracking
	var stats Stats

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Line range filter
		if lineFilter > 0 && lineNum != lineFilter {
			continue
		}
		if startLine > 0 && lineNum < startLine {
			continue
		}
		if endLine > 0 && lineNum > endLine {
			continue
		}

		rawEntry := types.RawEntry{
			Line:      lineNum,
			Data:      json.RawMessage(line),
			SessionID: types.ExtractSessionID(json.RawMessage(line)),
		}

		events, err := adapter.ParseEntry(rawEntry)
		if err != nil {
			fmt.Printf("Line %d: ERROR: %v\n", lineNum, err)
			if showRaw {
				printRawJSON(line)
			}
			continue
		}

		for _, event := range events {
			// Type filter
			if typeFilter != "" && !strings.EqualFold(event.EventType, typeFilter) {
				continue
			}

			matchCount++
			stats.Update(event)

			if !showStats {
				printEvent(lineNum, event, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Print stats if requested
	if showStats {
		stats.Print()
	}

	// Print summary if filtering was applied
	if lineFilter > 0 || typeFilter != "" || startLine > 0 || endLine > 0 {
		fmt.Printf("\n--- Matched %d events ---\n", matchCount)
	}
}

// Stats tracks token usage and event counts
type Stats struct {
	TotalInputTokens       int
	TotalOutputTokens      int
	TotalCacheReadTokens   int
	TotalCacheCreateTokens int
	EventCounts            map[string]int
	ToolCounts             map[string]int
	Models                 map[string]int
}

func (s *Stats) Update(event types.ParsedEvent) {
	if s.EventCounts == nil {
		s.EventCounts = make(map[string]int)
		s.ToolCounts = make(map[string]int)
		s.Models = make(map[string]int)
	}

	s.EventCounts[event.EventType]++
	s.TotalInputTokens += event.InputTokens
	s.TotalOutputTokens += event.OutputTokens
	s.TotalCacheReadTokens += event.CacheReadTokens
	s.TotalCacheCreateTokens += event.CacheCreateTokens

	if event.ToolName != "" {
		s.ToolCounts[event.ToolName]++
	}
	if event.Model != "" {
		s.Models[event.Model]++
	}
}

func (s *Stats) Print() {
	fmt.Println("\n=== Token Statistics ===")
	fmt.Printf("  Input Tokens:        %d\n", s.TotalInputTokens)
	fmt.Printf("  Output Tokens:       %d\n", s.TotalOutputTokens)
	fmt.Printf("  Cache Read Tokens:   %d\n", s.TotalCacheReadTokens)
	fmt.Printf("  Cache Create Tokens: %d\n", s.TotalCacheCreateTokens)

	fmt.Println("\n=== Event Type Counts ===")
	for eventType, count := range s.EventCounts {
		fmt.Printf("  %-15s: %d\n", eventType, count)
	}

	if len(s.ToolCounts) > 0 {
		fmt.Println("\n=== Tool Usage Counts ===")
		for tool, count := range s.ToolCounts {
			fmt.Printf("  %-15s: %d\n", tool, count)
		}
	}

	if len(s.Models) > 0 {
		fmt.Println("\n=== Models ===")
		for model, count := range s.Models {
			fmt.Printf("  %s: %d\n", model, count)
		}
	}
}

func printEvent(lineNum int, event types.ParsedEvent, rawLine []byte) {
	fmt.Printf("--- Line %d ---\n", lineNum)
	fmt.Printf("  EventType:    %s\n", event.EventType)
	fmt.Printf("  IsHumanInput: %v\n", event.IsHumanInput)
	fmt.Printf("  Timestamp:    %s\n", event.Timestamp.Format("15:04:05.000"))
	fmt.Printf("  SessionID:    %s\n", event.SessionID)

	if event.MessageUUID != "" {
		fmt.Printf("  MessageUUID:  %s\n", event.MessageUUID)
	}
	if event.ParentUUID != "" {
		fmt.Printf("  ParentUUID:   %s\n", event.ParentUUID)
	}
	if event.RequestID != "" {
		fmt.Printf("  RequestID:    %s\n", event.RequestID)
	}
	if event.Model != "" {
		fmt.Printf("  Model:        %s\n", event.Model)
	}
	if event.StopReason != "" {
		fmt.Printf("  StopReason:   %s\n", event.StopReason)
	}

	// Token usage
	if event.InputTokens > 0 || event.OutputTokens > 0 {
		fmt.Printf("  Tokens:       in=%d out=%d", event.InputTokens, event.OutputTokens)
		if event.CacheReadTokens > 0 {
			fmt.Printf(" cache_read=%d", event.CacheReadTokens)
		}
		if event.CacheCreateTokens > 0 {
			fmt.Printf(" cache_create=%d", event.CacheCreateTokens)
		}
		fmt.Println()
	}

	// Tool info
	if event.ToolName != "" {
		fmt.Printf("  ToolName:     %s\n", event.ToolName)
	}
	if event.ToolInputSummary != "" {
		fmt.Printf("  ToolInput:    %s\n", event.ToolInputSummary)
	}
	if event.FilePath != "" {
		fmt.Printf("  FilePath:     %s\n", event.FilePath)
	}
	if event.ToolSuccess != nil {
		fmt.Printf("  ToolSuccess:  %v\n", *event.ToolSuccess)
	}
	if event.ToolError != "" {
		fmt.Printf("  ToolError:    %s\n", event.ToolError)
	}

	// Content preview
	if event.ContentPreview != "" {
		preview := event.ContentPreview
		if !showRaw && len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("  Content:      %s\n", preview)
		fmt.Printf("  ContentLen:   %d\n", event.ContentLength)
	}

	// Show raw payload if requested
	if showRaw {
		fmt.Printf("  RawPayload:\n")
		printRawJSON(rawLine)
	}

	fmt.Println()
}

func printRawJSON(data []byte) {
	var pretty map[string]interface{}
	if err := json.Unmarshal(data, &pretty); err == nil {
		formatted, _ := json.MarshalIndent(pretty, "    ", "  ")
		fmt.Printf("    %s\n", formatted)
	} else {
		// Fallback to raw string
		fmt.Printf("    %s\n", string(data))
	}
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command dbexplorer queries the database for AI activity events.
// Usage:
//
//	go run cmd/dev/dbexplorer/main.go --task-id <id>
//	go run cmd/dev/dbexplorer/main.go --latest
//	go run cmd/dev/dbexplorer/main.go --list-tasks
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
)

func main() {
	taskID := flag.String("task-id", "", "Task ID to explore")
	latest := flag.Bool("latest", false, "Use the most recent task")
	listTasks := flag.Bool("list-tasks", false, "List all tasks with event counts")
	showRaw := flag.Bool("raw", false, "Show full raw payload")
	limit := flag.Int("limit", 50, "Maximum number of events to show")
	eventType := flag.String("type", "", "Filter by event type (tool_use, tool_result, thinking, output, etc.)")
	configFile := flag.String("config", "test-config.yaml", "Config file path")

	flag.Parse()

	ctx := context.Background()

	// Load config and create data service
	cfg, err := config.NewConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	ds, err := services.NewDataService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create data service: %v\n", err)
		os.Exit(1)
	}
	defer ds.Close()

	if *listTasks {
		listAllTasks(ctx, ds)
		return
	}

	// Resolve task ID
	resolvedTaskID := *taskID
	if *latest || resolvedTaskID == "" {
		task, err := ds.GetLatestTask(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get latest task: %v\n", err)
			os.Exit(1)
		}
		if task == nil {
			fmt.Fprintf(os.Stderr, "No tasks found\n")
			os.Exit(1)
		}
		resolvedTaskID = task.ID
		fmt.Printf("Using latest task: %s (%s)\n", task.Title, resolvedTaskID)
	}

	// Load events for task
	events, err := ds.GetAIActivityByTask(ctx, resolvedTaskID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load events: %v\n", err)
		os.Exit(1)
	}

	// Filter by event type if specified
	if *eventType != "" {
		filtered := make([]*models.AIActivityRecord, 0)
		for _, e := range events {
			if strings.EqualFold(string(e.EventType), *eventType) {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	fmt.Printf("\nFound %d events for task %s\n", len(events), resolvedTaskID)
	fmt.Println(strings.Repeat("=", 60))

	// Apply limit
	if *limit > 0 && len(events) > *limit {
		fmt.Printf("(showing first %d of %d events)\n", *limit, len(events))
		events = events[:*limit]
	}

	for i, event := range events {
		printEvent(i+1, event, *showRaw)
	}

	// Print summary
	printSummary(events)
}

func listAllTasks(ctx context.Context, ds *services.DataService) {
	// Load all projects first
	projects, err := ds.LoadProjects(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load projects: %v\n", err)
		return
	}

	fmt.Println("Tasks with AI activity events:")
	fmt.Println(strings.Repeat("=", 80))

	for _, project := range projects {
		tasks, err := ds.LoadTasks(ctx, project.ID)
		if err != nil {
			continue
		}

		for _, task := range tasks {
			events, err := ds.GetAIActivityByTask(ctx, task.ID)
			if err != nil {
				continue
			}

			if len(events) > 0 {
				fmt.Printf("%-40s | %-20s | %4d events | %s\n",
					truncate(task.Title, 40),
					task.Status,
					len(events),
					task.ID)
			}
		}
	}
}

func printEvent(num int, record *models.AIActivityRecord, showRaw bool) {
	fmt.Printf("\n--- Event %d ---\n", num)
	fmt.Printf("  EventID:   %s\n", record.EventID)
	fmt.Printf("  Type:      %s\n", record.EventType)
	fmt.Printf("  Timestamp: %s\n", record.Timestamp.Format("2006-01-02 15:04:05.000"))
	fmt.Printf("  SessionID: %s\n", record.SessionID)

	// Print type-specific data based on flat fields
	switch record.EventType {
	case "tool_use":
		fmt.Printf("  ToolCall:\n")
		fmt.Printf("    Name:  %s\n", record.ToolName)
		fmt.Printf("    Input: %s\n", truncate(record.ToolInputSummary, 200))
		if record.FilePath != "" {
			fmt.Printf("    Path:  %s\n", record.FilePath)
		}
	case "tool_result":
		fmt.Printf("  ToolResult:\n")
		fmt.Printf("    Tool:    %s\n", record.ToolName)
		if record.ToolSuccess != nil {
			fmt.Printf("    Success: %v\n", *record.ToolSuccess)
		}
		fmt.Printf("    Output:  %s\n", truncate(record.ContentPreview, 200))
		if record.ToolError != "" {
			fmt.Printf("    Error:   %s\n", record.ToolError)
		}
	case "ai_output", "user_prompt":
		fmt.Printf("  Content: %s\n", truncate(record.ContentPreview, 300))
	case "thinking":
		fmt.Printf("  Thinking: %s\n", truncate(record.ContentPreview, 300))
	case "error":
		fmt.Printf("  Error: %s\n", record.ContentPreview)
	case "session_end":
		fmt.Printf("  Stop: %s\n", record.StopReason)
	}

	// Show token usage if available
	if record.InputTokens > 0 || record.OutputTokens > 0 {
		fmt.Printf("  Tokens: in=%d out=%d\n", record.InputTokens, record.OutputTokens)
	}

	// Show raw payload if requested
	if showRaw && record.RawPayload != "" {
		fmt.Printf("  RawPayload:\n")
		var prettyJSON map[string]interface{}
		if err := json.Unmarshal([]byte(record.RawPayload), &prettyJSON); err == nil {
			formatted, _ := json.MarshalIndent(prettyJSON, "    ", "  ")
			fmt.Printf("    %s\n", formatted)
		} else {
			fmt.Printf("    %s\n", truncate(record.RawPayload, 500))
		}
	}
}

func printSummary(events []*models.AIActivityRecord) {
	if len(events) == 0 {
		return
	}

	// Count by type
	typeCounts := make(map[models.AIEventType]int)
	toolCounts := make(map[string]int)
	totalTokens := 0

	for _, e := range events {
		typeCounts[e.EventType]++
		if e.EventType == models.AIEventToolUse && e.ToolName != "" {
			toolCounts[e.ToolName]++
		}
		totalTokens += e.InputTokens + e.OutputTokens
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Summary:")

	fmt.Println("\nBy Event Type:")
	for t, count := range typeCounts {
		fmt.Printf("  %-20s: %d\n", t, count)
	}

	if len(toolCounts) > 0 {
		fmt.Println("\nTool Calls:")
		for tool, count := range toolCounts {
			fmt.Printf("  %-20s: %d\n", tool, count)
		}
	}

	if totalTokens > 0 {
		fmt.Printf("\nTotal Tokens: %d\n", totalTokens)
	}
}

func truncate(s string, max int) string {
	// Replace newlines with spaces for cleaner display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return "..."
	}
	return s[:max-3] + "..."
}

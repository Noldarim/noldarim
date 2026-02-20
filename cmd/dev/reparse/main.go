// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command reparse re-parses stored AI activity records through the adapter.
// Useful for testing adapter changes against real data and benchmarking.
//
// Usage:
//
//	go run cmd/dev/reparse/main.go                    # Re-parse all tool_result events
//	go run cmd/dev/reparse/main.go --type tool_use   # Re-parse tool_use events
//	go run cmd/dev/reparse/main.go --limit 10        # Limit to 10 records
//	go run cmd/dev/reparse/main.go --diff            # Show only records where parsing changed
//	go run cmd/dev/reparse/main.go --bench           # Run parsing benchmark
//	go run cmd/dev/reparse/main.go --update          # Re-parse and update records in DB
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/noldarim/noldarim/internal/aiobs/adapters"
	"github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
)

var (
	eventType   string
	limit       int
	showDiff    bool
	runBench    bool
	updateDB    bool
	verbose     bool
)

func init() {
	adapters.RegisterAll()
}

func main() {
	flag.StringVar(&eventType, "type", "tool_result", "Event type to re-parse (tool_result, tool_use, etc.)")
	flag.IntVar(&limit, "limit", 0, "Limit number of records (0 = all)")
	flag.BoolVar(&showDiff, "diff", false, "Only show records where parsing result changed")
	flag.BoolVar(&runBench, "bench", false, "Run parsing benchmark")
	flag.BoolVar(&updateDB, "update", false, "Update records in database with new parsed values")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.Parse()

	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data service: %v\n", err)
		os.Exit(1)
	}
	defer dataService.Close()

	adapter, ok := adapters.Get("claude")
	if !ok {
		fmt.Fprintf(os.Stderr, "Claude adapter not registered\n")
		os.Exit(1)
	}

	ctx := context.Background()
	records, err := dataService.GetAIActivityByEventType(ctx, eventType, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying records: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d %s records\n\n", len(records), eventType)

	if runBench {
		runBenchmark(records, adapter)
		return
	}

	// Re-parse and compare
	changed := 0
	updated := 0
	for _, rec := range records {
		rawEntry := types.RawEntry{
			Data:      json.RawMessage(rec.RawPayload),
			SessionID: types.ExtractSessionID(json.RawMessage(rec.RawPayload)),
		}

		events, err := adapter.ParseEntry(rawEntry)
		if err != nil {
			fmt.Printf("❌ %s: parse error: %v\n", rec.EventID, err)
			continue
		}

		// Find the matching event type (there may be multiple events per entry)
		var newEvent *types.ParsedEvent
		for i := range events {
			if events[i].EventType == eventType {
				newEvent = &events[i]
				break
			}
		}

		if newEvent == nil {
			if verbose {
				fmt.Printf("⚠ %s: no %s event in parsed result\n", rec.EventID, eventType)
			}
			continue
		}

		// Save old values before any modification
		oldPreview := rec.ContentPreview
		oldToolName := rec.ToolName
		oldFilePath := rec.FilePath

		// Compare old vs new
		previewChanged := oldPreview != newEvent.ContentPreview
		toolChanged := oldToolName != newEvent.ToolName
		pathChanged := oldFilePath != newEvent.FilePath

		if previewChanged || toolChanged || pathChanged {
			changed++

			// Update DB if requested
			if updateDB {
				rec.ContentPreview = newEvent.ContentPreview
				rec.ToolName = newEvent.ToolName
				rec.FilePath = newEvent.FilePath
				rec.ContentLength = newEvent.ContentLength
				if err := dataService.UpdateAIActivityRecord(ctx, rec); err != nil {
					fmt.Printf("❌ %s: update error: %v\n", rec.EventID, err)
				} else {
					updated++
				}
			}
		}

		if showDiff && !previewChanged && !toolChanged && !pathChanged {
			continue
		}

		// Print result
		fmt.Printf("─── %s ───\n", rec.EventID)

		if toolChanged {
			fmt.Printf("  ToolName:  %q → %q\n", oldToolName, newEvent.ToolName)
		} else if verbose {
			fmt.Printf("  ToolName:  %q\n", newEvent.ToolName)
		}

		if pathChanged {
			fmt.Printf("  FilePath:  %q → %q\n", oldFilePath, newEvent.FilePath)
		} else if newEvent.FilePath != "" && verbose {
			fmt.Printf("  FilePath:  %q\n", newEvent.FilePath)
		}

		if previewChanged {
			fmt.Printf("  OLD:       %s\n", truncate(oldPreview, 60))
			fmt.Printf("  NEW:       %s\n", truncate(newEvent.ContentPreview, 60))
		} else if verbose {
			fmt.Printf("  Preview:   %s\n", truncate(newEvent.ContentPreview, 80))
		}

		fmt.Println()
	}

	fmt.Printf("─── Summary ───\n")
	fmt.Printf("  Total:   %d records\n", len(records))
	fmt.Printf("  Changed: %d records\n", changed)
	if updateDB {
		fmt.Printf("  Updated: %d records\n", updated)
	}
}

func runBenchmark(records []*models.AIActivityRecord, adapter types.Adapter) {
	if len(records) == 0 {
		fmt.Println("No records to benchmark")
		return
	}

	// Warm up
	for i := 0; i < 3 && i < len(records); i++ {
		rawEntry := types.RawEntry{
			Data: json.RawMessage(records[i].RawPayload),
		}
		adapter.ParseEntry(rawEntry)
	}

	// Benchmark
	iterations := 100
	if len(records) < iterations {
		iterations = len(records)
	}

	var totalDuration time.Duration
	var minDuration = time.Hour
	var maxDuration time.Duration
	var payloadSizes []int

	for i := 0; i < iterations; i++ {
		rec := records[i%len(records)]
		rawEntry := types.RawEntry{
			Data: json.RawMessage(rec.RawPayload),
		}
		payloadSizes = append(payloadSizes, len(rec.RawPayload))

		start := time.Now()
		adapter.ParseEntry(rawEntry)
		duration := time.Since(start)

		totalDuration += duration
		if duration < minDuration {
			minDuration = duration
		}
		if duration > maxDuration {
			maxDuration = duration
		}
	}

	avgDuration := totalDuration / time.Duration(iterations)

	// Calculate avg payload size
	var totalSize int
	for _, s := range payloadSizes {
		totalSize += s
	}
	avgSize := totalSize / len(payloadSizes)

	fmt.Println("─── Benchmark Results ───")
	fmt.Printf("  Iterations:     %d\n", iterations)
	fmt.Printf("  Avg payload:    %d bytes\n", avgSize)
	fmt.Printf("  Min parse time: %v\n", minDuration)
	fmt.Printf("  Max parse time: %v\n", maxDuration)
	fmt.Printf("  Avg parse time: %v\n", avgDuration)
	fmt.Printf("  Throughput:     %.0f records/sec\n", float64(time.Second)/float64(avgDuration))
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

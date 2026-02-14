// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
)

type taskShowOptions struct {
	configPath string
	showDiff   bool
	showRaw    bool
}

// taskCommand dispatches task subcommands
func taskCommand(args []string) error {
	if len(args) == 0 {
		return taskUsage()
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "show":
		return taskShowCommand(subargs)
	case "list":
		return taskListCommand(subargs)
	case "help", "-h", "--help":
		return taskUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown task subcommand: %s\n\n", subcommand)
		return taskUsage()
	}
}

func taskUsage() error {
	fmt.Printf(`Usage: %s task <subcommand> [arguments]

Subcommands:
  show <task-id>   Show detailed task information including tokens, commands, and diff
  list             List all tasks (use --project to filter)
  help             Show this help message

Examples:
  %s task show abc123
  %s task show abc123 --diff
  %s task list --project myproject

`, appName, appName, appName, appName)
	return nil
}

func taskShowCommand(args []string) error {
	opts := &taskShowOptions{}
	fs := flag.NewFlagSet("task show", flag.ExitOnError)
	fs.StringVar(&opts.configPath, "config", "config.yaml", "Path to config file")
	fs.BoolVar(&opts.showDiff, "diff", false, "Show full git diff")
	fs.BoolVar(&opts.showRaw, "raw", false, "Show raw activity records")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		// Try to get latest task
		return showLatestTask(opts)
	}
	taskID := remaining[0]

	return showTask(taskID, opts)
}

func showLatestTask(opts *taskShowOptions) error {
	cfg, err := config.NewConfig(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dataService.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	task, err := dataService.GetLatestTask(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("no tasks found")
	}

	return showTaskDetails(ctx, dataService, task, opts)
}

func showTask(taskID string, opts *taskShowOptions) error {
	cfg, err := config.NewConfig(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dataService.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	task, err := dataService.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return showTaskDetails(ctx, dataService, task, opts)
}

func showTaskDetails(ctx context.Context, dataService *services.DataService, task *models.Task, opts *taskShowOptions) error {
	// Get AI activity records for token aggregation
	records, err := dataService.GetAIActivityByTask(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("failed to get activity records: %w", err)
	}

	// Aggregate token usage
	stats := aggregateTokenStats(records)

	// Print task header
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("TASK: %s\n", task.Title)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Basic info
	fmt.Printf("ID:          %s\n", task.ID)
	fmt.Printf("Status:      %s\n", formatStatus(task.Status))
	fmt.Printf("Branch:      %s\n", task.BranchName)
	fmt.Printf("Created:     %s\n", task.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:     %s\n", task.LastUpdatedAt.Format(time.RFC3339))
	fmt.Println()

	if task.Description != "" {
		fmt.Println("DESCRIPTION:")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println(task.Description)
		fmt.Println()
	}

	// Token usage summary
	fmt.Println("TOKEN USAGE:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Input tokens:        %s\n", formatNumber(stats.InputTokens))
	fmt.Printf("  Output tokens:       %s\n", formatNumber(stats.OutputTokens))
	fmt.Printf("  Cache read tokens:   %s (saved)\n", formatNumber(stats.CacheReadTokens))
	fmt.Printf("  Cache create tokens: %s\n", formatNumber(stats.CacheCreateTokens))
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Total tokens:        %s\n", formatNumber(stats.TotalTokens()))
	if stats.Model != "" {
		fmt.Printf("  Model:               %s\n", stats.Model)
	}
	fmt.Println()

	// Commands executed
	if len(stats.ToolCalls) > 0 {
		fmt.Println("COMMANDS EXECUTED:")
		fmt.Println(strings.Repeat("-", 40))
		for i, tool := range stats.ToolCalls {
			status := "OK"
			if tool.Success != nil && !*tool.Success {
				status = "FAIL"
			}
			fmt.Printf("  %2d. [%s] %s", i+1, status, tool.ToolName)
			if tool.FilePath != "" {
				fmt.Printf(" - %s", tool.FilePath)
			} else if tool.Summary != "" {
				// For tools without file paths (like Bash), show the summary inline
				fmt.Printf(" - %s", truncate(tool.Summary, 50))
			}
			fmt.Println()
			if tool.Summary != "" && opts.showRaw && tool.FilePath != "" {
				// In raw mode, also show summary for file operations
				fmt.Printf("      %s\n", truncate(tool.Summary, 60))
			}
		}
		fmt.Printf("\n  Total: %d tool calls\n", len(stats.ToolCalls))
		fmt.Println()
	}

	// Files changed summary from git diff
	if task.GitDiff != "" {
		diffStats := parseDiffStats(task.GitDiff)
		fmt.Println("FILES CHANGED:")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Printf("  %d files changed, %d insertions(+), %d deletions(-)\n",
			diffStats.FilesChanged, diffStats.Insertions, diffStats.Deletions)
		fmt.Println()

		if len(diffStats.Files) > 0 {
			for _, f := range diffStats.Files {
				fmt.Printf("  %s %s\n", f.Status, f.Path)
			}
			fmt.Println()
		}

		// Show full diff if requested
		if opts.showDiff {
			fmt.Println("GIT DIFF:")
			fmt.Println(strings.Repeat("-", 40))
			fmt.Println(task.GitDiff)
		} else {
			fmt.Println("  (use --diff to show full diff)")
			fmt.Println()
		}
	} else {
		fmt.Println("FILES CHANGED: None")
		fmt.Println()
	}

	return nil
}

// TokenStats holds aggregated token statistics
type TokenStats struct {
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheCreateTokens int
	Model             string
	ToolCalls         []ToolCallInfo
}

type ToolCallInfo struct {
	ToolName  string
	FilePath  string
	Summary   string
	Success   *bool
	Timestamp time.Time
}

func (ts *TokenStats) TotalTokens() int {
	return ts.InputTokens + ts.OutputTokens
}

func aggregateTokenStats(records []*models.AIActivityRecord) *TokenStats {
	stats := &TokenStats{
		ToolCalls: make([]ToolCallInfo, 0),
	}

	// Sort records by timestamp
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	for _, r := range records {
		// Aggregate tokens (only count AI output messages to avoid double-counting)
		if r.EventType == models.AIEventAIOutput {
			stats.InputTokens += r.InputTokens
			stats.OutputTokens += r.OutputTokens
			stats.CacheReadTokens += r.CacheReadTokens
			stats.CacheCreateTokens += r.CacheCreateTokens

			// Capture model from first record that has it
			if stats.Model == "" && r.Model != "" {
				stats.Model = r.Model
			}
		}

		// Track tool calls
		if r.EventType == models.AIEventToolUse && r.ToolName != "" {
			stats.ToolCalls = append(stats.ToolCalls, ToolCallInfo{
				ToolName:  r.ToolName,
				FilePath:  r.FilePath,
				Summary:   r.ToolInputSummary,
				Success:   r.ToolSuccess,
				Timestamp: r.Timestamp,
			})
		}
	}

	return stats
}

// DiffStats holds parsed git diff statistics
type DiffStats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
	Files        []FileChange
}

type FileChange struct {
	Path   string
	Status string // M, A, D, R
}

func parseDiffStats(diff string) *DiffStats {
	stats := &DiffStats{
		Files: make([]FileChange, 0),
	}

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		// Parse diff --git a/path b/path
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				path := strings.TrimPrefix(parts[3], "b/")
				stats.Files = append(stats.Files, FileChange{
					Path:   path,
					Status: "M", // Default to modified
				})
				stats.FilesChanged++
			}
		}
		// Count insertions
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.Insertions++
		}
		// Count deletions
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.Deletions++
		}
		// Check for new file
		if strings.HasPrefix(line, "new file mode") {
			if len(stats.Files) > 0 {
				stats.Files[len(stats.Files)-1].Status = "A"
			}
		}
		// Check for deleted file
		if strings.HasPrefix(line, "deleted file mode") {
			if len(stats.Files) > 0 {
				stats.Files[len(stats.Files)-1].Status = "D"
			}
		}
	}

	return stats
}

func formatStatus(status models.TaskStatus) string {
	switch status {
	case models.TaskStatusPending:
		return "PENDING"
	case models.TaskStatusInProgress:
		return "IN PROGRESS"
	case models.TaskStatusCompleted:
		return "COMPLETED"
	case models.TaskStatusFailed:
		return "FAILED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", status)
	}
}

func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// taskListCommand lists tasks
func taskListCommand(args []string) error {
	var configPath, projectID string
	fs := flag.NewFlagSet("task list", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "config.yaml", "Path to config file")
	fs.StringVar(&projectID, "project", "", "Filter by project ID")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dataService.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load projects to iterate
	projects, err := dataService.LoadProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to load projects: %w", err)
	}

	fmt.Println("TASKS:")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("%-12s %-12s %-30s %s\n", "STATUS", "ID", "TITLE", "CREATED")
	fmt.Println(strings.Repeat("-", 80))

	count := 0
	for _, project := range projects {
		if projectID != "" && project.ID != projectID && project.Name != projectID {
			continue
		}

		tasks, err := dataService.LoadTasks(ctx, project.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load tasks for project %s: %v\n", project.ID, err)
			continue
		}

		for _, task := range tasks {
			fmt.Printf("%-12s %-12s %-30s %s\n",
				formatStatus(task.Status),
				truncate(task.ID, 12),
				truncate(task.Title, 30),
				task.CreatedAt.Format("2006-01-02 15:04"),
			)
			count++
		}
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total: %d tasks\n", count)

	return nil
}

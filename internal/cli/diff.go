// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
)

type diffOptions struct {
	configPath string
	noHeaders  bool // Skip step headers, output pure diff
	summary    bool // Show summary only, no diff content
}

// diffCommand handles the diff subcommand
func diffCommand(args []string) error {
	opts := &diffOptions{}
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	fs.StringVar(&opts.configPath, "config", "config.yaml", "Path to config file")
	fs.BoolVar(&opts.noHeaders, "no-headers", false, "Output pure diff without step headers (for piping)")
	fs.BoolVar(&opts.summary, "summary", false, "Show summary only, no diff content")

	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()

	// If no run_id provided, use latest
	if len(remaining) == 0 {
		return showLatestRunDiff(opts)
	}

	runID := remaining[0]
	return showRunDiff(runID, opts)
}

func showLatestRunDiff(opts *diffOptions) error {
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

	run, err := dataService.GetLatestPipelineRun(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest run: %w", err)
	}
	if run == nil {
		return fmt.Errorf("no pipeline runs found")
	}

	return displayRunDiff(ctx, dataService, run, opts)
}

func showRunDiff(runID string, opts *diffOptions) error {
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

	run, err := dataService.GetPipelineRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}
	if run == nil {
		return fmt.Errorf("run not found: %s", runID)
	}

	return displayRunDiff(ctx, dataService, run, opts)
}

func displayRunDiff(ctx context.Context, dataService *services.DataService, run *models.PipelineRun, opts *diffOptions) error {
	// ANSI color codes
	const (
		cyan   = "\033[36m"
		green  = "\033[32m"
		red    = "\033[31m"
		yellow = "\033[33m"
		bold   = "\033[1m"
		dim    = "\033[2m"
		reset  = "\033[0m"
	)

	// Get step results
	stepResults, err := dataService.GetStepResultsByRun(ctx, run.ID)
	if err != nil {
		return fmt.Errorf("failed to get step results: %w", err)
	}

	if len(stepResults) == 0 {
		if !opts.noHeaders {
			fmt.Printf("%s%s# No steps found for this run%s\n", dim, cyan, reset)
		}
		return nil
	}

	// Aggregate totals for summary
	totalFiles := 0
	totalInsertions := 0
	totalDeletions := 0
	for _, step := range stepResults {
		totalFiles += step.FilesChanged
		totalInsertions += step.Insertions
		totalDeletions += step.Deletions
	}

	// Print run header
	if !opts.noHeaders {
		fmt.Printf("%s%s# ══════════════════════════════════════════════════════════════%s\n", bold, cyan, reset)
		fmt.Printf("%s%s# Pipeline Run:%s %s\n", bold, cyan, reset, run.ID)
		fmt.Printf("%s# Status:%s %s %s│%s %s# Branch:%s %s\n",
			cyan, reset, run.Status.String(), dim, reset, cyan, reset, run.BranchName)
		if run.BaseCommitSHA != "" && run.HeadCommitSHA != "" {
			fmt.Printf("%s# %s%s..%s%s\n", cyan, yellow, truncateSHA(run.BaseCommitSHA), truncateSHA(run.HeadCommitSHA), reset)
		}
		fmt.Printf("%s# Total:%s %d files changed, %s+%d%s, %s-%d%s\n",
			cyan, reset, totalFiles, green, totalInsertions, reset, red, totalDeletions, reset)
		fmt.Printf("%s%s# ══════════════════════════════════════════════════════════════%s\n", bold, cyan, reset)
		fmt.Println()
	}

	// Summary mode - just show stats per step
	if opts.summary {
		for i, step := range stepResults {
			if step.FilesChanged > 0 {
				fmt.Printf("%s%s# Step %d%s %s(%s)%s: %d files, %s+%d%s/%s-%d%s\n",
					bold, yellow, i+1, reset, dim, step.StepID, reset,
					step.FilesChanged, green, step.Insertions, reset, red, step.Deletions, reset)
				// List files from diff
				files := extractFilePaths(step.GitDiff)
				for _, f := range files {
					fmt.Printf("%s#   %s%s\n", dim, f, reset)
				}
			}
		}
		return nil
	}

	// Output each step's diff
	for i, step := range stepResults {
		if step.GitDiff == "" {
			continue
		}

		// Step header
		if !opts.noHeaders {
			fmt.Printf("%s%s# ──────────────────────────────────────────────────────────────%s\n", dim, cyan, reset)
			fmt.Printf("%s%s# Step %d:%s %s %s(%d files, %s+%d%s/%s-%d%s)%s\n",
				bold, yellow, i+1, reset, step.StepID,
				dim, step.FilesChanged, green, step.Insertions, reset, red, step.Deletions, dim, reset)
			fmt.Printf("%s%s# ──────────────────────────────────────────────────────────────%s\n", dim, cyan, reset)
		}

		// Output raw diff - ensure it ends with newline for clean separation
		diff := strings.TrimRight(step.GitDiff, "\n")
		fmt.Println(diff)
		fmt.Println()
	}

	return nil
}

// extractFilePaths pulls file paths from a git diff
func extractFilePaths(diff string) []string {
	var files []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				path := strings.TrimPrefix(parts[3], "b/")
				files = append(files, path)
			}
		}
	}
	return files
}

func truncateSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

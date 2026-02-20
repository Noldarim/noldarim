// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/components/collapsiblefeed"
	"github.com/noldarim/noldarim/internal/tui/components/pipelinesummary"
	"github.com/noldarim/noldarim/internal/tui/components/pipelineview"
	"github.com/noldarim/noldarim/internal/tui/components/stepprogress"
	"github.com/noldarim/noldarim/internal/tui/components/tokendisplay"
)

type runOptions struct {
	project      string
	configPath   string
	noColor      bool
	pipelineFile string            // --pipeline or -p flag
	vars         map[string]string // --var key=value flags
	// Fork options for smart step reuse
	forkFrom   string // --fork-from RunID: explicitly fork from a previous run
	forkAfter  string // --fork-after StepID: fork after this step (requires --fork-from)
	noAutoFork bool   // --no-auto-fork: disable automatic fork detection
}

func runCommand(args []string) error {
	opts := &runOptions{vars: make(map[string]string)}
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fs.StringVar(&opts.project, "project", "", "Project ID or name (auto-detects from current directory if not specified)")
	fs.StringVar(&opts.configPath, "config", "config.yaml", "Path to config file")
	fs.BoolVar(&opts.noColor, "no-color", false, "Disable colored output")
	fs.StringVar(&opts.pipelineFile, "pipeline", "", "Path to pipeline YAML file")
	fs.StringVar(&opts.pipelineFile, "p", "", "Path to pipeline YAML file (shorthand)")

	// Custom flag for --var (can be repeated)
	fs.Func("var", "Set variable (key=value), can be repeated", func(s string) error {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid var format, use key=value")
		}
		opts.vars[parts[0]] = parts[1]
		return nil
	})

	// Fork flags for smart step reuse
	fs.StringVar(&opts.forkFrom, "fork-from", "", "Fork from a previous run ID (skip unchanged steps)")
	fs.StringVar(&opts.forkAfter, "fork-after", "", "Fork after this step ID (requires --fork-from)")
	fs.BoolVar(&opts.noAutoFork, "no-auto-fork", false, "Disable automatic fork detection")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate fork flags
	if opts.forkAfter != "" && opts.forkFrom == "" {
		return fmt.Errorf("--fork-after requires --fork-from to be specified")
	}

	// Get the task description from remaining args (only required if not using pipeline file)
	remaining := fs.Args()
	taskDescription := strings.Join(remaining, " ")

	// Validate: either pipeline file or task description required
	if opts.pipelineFile == "" && taskDescription == "" {
		return fmt.Errorf("task description or pipeline file required\n\nUsage:\n  noldarim run \"<task description>\"\n  noldarim run --pipeline <file.yaml>")
	}

	return executeRun(taskDescription, opts)
}

func executeRun(taskDescription string, opts *runOptions) error {
	// Load configuration
	cfg, err := config.NewConfig(opts.configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logging (to file only for CLI, keep terminal clean)
	if err := logger.Initialize(&cfg.Log); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.CloseGlobal()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create data service for DB polling
	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create data service: %w", err)
	}
	defer dataService.Close()

	// Resolve project using data service directly
	projectID, projectName, err := resolveProjectFromDB(ctx, opts.project, dataService)
	if err != nil {
		return fmt.Errorf("failed to resolve project: %w", err)
	}

	// Build steps from pipeline file or single task description
	var steps []protocol.StepInput
	var pipelineName string

	if opts.pipelineFile != "" {
		// Load multi-step pipeline from YAML
		pipelineCfg, err := LoadPipelineFile(opts.pipelineFile)
		if err != nil {
			return fmt.Errorf("failed to load pipeline: %w", err)
		}
		steps, err = pipelineCfg.ToStepInputs(cfg, opts.vars)
		if err != nil {
			return fmt.Errorf("failed to process pipeline: %w", err)
		}
		pipelineName = pipelineCfg.Name
	} else {
		// Single-step pipeline from task description
		agentConfig := buildAgentConfig(cfg, taskDescription)
		steps = []protocol.StepInput{{
			StepID:      "1",
			Name:        "Execute task",
			AgentConfig: agentConfig,
		}}
		pipelineName = truncateTitle(taskDescription)
	}

	// Print banner
	printRunBanner(pipelineName, projectName)

	// Create channels for communication with orchestrator
	cmdChan := make(chan protocol.Command, 100)
	eventChan := make(chan protocol.Event, 100)

	// Handle signals for the pre-TUI phase (before pipeline starts)
	// Once the TUI is running, Ctrl+C is handled as a key event inside the TUI
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n▸ Interrupted before pipeline started, exiting...\n")
		cancel()
	}()

	// Start orchestrator
	orch, err := orchestrator.New(cmdChan, eventChan, cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	// Start orchestrator in background
	go orch.Run(ctx)

	// Send start pipeline command
	cmdChan <- protocol.StartPipelineCommand{
		Metadata:        protocol.Metadata{},
		ProjectID:       projectID,
		Name:            pipelineName,
		Steps:           steps,
		ForkFromRunID:   opts.forkFrom,
		ForkAfterStepID: opts.forkAfter,
		NoAutoFork:      opts.noAutoFork,
	}

	// Wait for pipeline start event to get the run ID
	var runID string
	var isReplay bool
	fmt.Printf("▸ Starting pipeline workflow...\n")

	timeout := time.After(60 * time.Second)
waitForPipeline:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for pipeline to start")
		case event := <-eventChan:
			switch e := event.(type) {
			case protocol.PipelineRunStartedEvent:
				runID = e.RunID
				if e.AlreadyExists {
					isReplay = true
					fmt.Printf("▸ Same run detected, replaying from history: %s\n", runID)
				} else {
					fmt.Printf("▸ Pipeline started: %s\n", runID)
					// Show fork info if forking from a previous run
					if e.ForkFromRunID != "" {
						fmt.Printf("▸ Forking from run %s after step '%s' (skipping %d step(s))\n",
							truncateID(e.ForkFromRunID), e.ForkAfterStepID, e.SkippedSteps)
					}
					fmt.Printf("▸ Setting up container and worktree...\n")
				}
				break waitForPipeline
			case protocol.ErrorEvent:
				return fmt.Errorf("error: %s - %s", e.Message, e.Context)
			}
		}
	}

	if isReplay {
		fmt.Printf("▸ Loading cached results...\n\n")
	} else {
		fmt.Printf("▸ Agent starting...\n\n")
	}

	// Run the pipeline view TUI - pass channels for cancellation support
	return runPipelineView(ctx, dataService, runID, len(steps), cmdChan, eventChan)
}

// runPipelineView starts the Bubble Tea program for pipeline execution display
func runPipelineView(ctx context.Context, dataService *services.DataService, runID string, stepCount int, cmdChan chan<- protocol.Command, eventChan <-chan protocol.Event) error {
	// Use default size - Bubble Tea will send WindowSizeMsg with actual dimensions
	width, height := 80, 24

	// Track activity count for incremental updates
	lastActivityCount := 0

	// Create data fetcher that polls the database
	fetcher := func(fetchCtx context.Context) (*pipelineview.DataMsg, error) {
		run, err := dataService.GetPipelineRun(fetchCtx, runID)
		if err != nil || run == nil {
			return nil, err
		}

		data := &pipelineview.DataMsg{
			Status: pipelineview.StatusRunning,
		}

		// Build steps from run data
		steps := make([]stepprogress.Step, len(run.StepResults))
		for i, step := range run.StepResults {
			steps[i] = stepprogress.Step{
				Name:   step.StepID,
				Status: convertStepStatus(step.Status),
			}
		}
		if len(steps) == 0 {
			steps = []stepprogress.Step{{Name: "1", Status: stepprogress.StatusRunning}}
		}
		data.Steps = steps

		// Build tokens from step results
		var totalIn, totalOut, cacheRead, cacheCreate int
		for _, step := range run.StepResults {
			totalIn += step.InputTokens
			totalOut += step.OutputTokens
			cacheRead += step.CacheReadTokens
			cacheCreate += step.CacheCreateTokens
		}
		data.Tokens = tokendisplay.TokenData{
			InputTokens:       totalIn,
			OutputTokens:      totalOut,
			CacheReadTokens:   cacheRead,
			CacheCreateTokens: cacheCreate,
		}

		// Fetch activities for all steps in this run
		dbActivities, err := dataService.GetAIActivityByRunID(fetchCtx, runID)
		if err == nil && len(dbActivities) > lastActivityCount {
			// Parse records into collapsible activity groups
			data.Groups = collapsiblefeed.ParseRecords(dbActivities)
			lastActivityCount = len(dbActivities)
		}

		// Check final status
		switch run.Status {
		case models.PipelineRunStatusCompleted:
			data.Status = pipelineview.StatusCompleted
			// Ensure all steps show as completed (in case of race condition)
			for i := range steps {
				if steps[i].Status == stepprogress.StatusRunning || steps[i].Status == stepprogress.StatusPending {
					steps[i].Status = stepprogress.StatusCompleted
				}
			}
			data.Steps = steps
			data.Summary = buildPipelineSummary(run, steps, data.Tokens)
		case models.PipelineRunStatusFailed:
			data.Status = pipelineview.StatusFailed
			data.Summary = buildPipelineSummary(run, steps, data.Tokens)
		}

		return data, nil
	}

	// Create the pipeline view model
	model := pipelineview.New(width, height, fetcher)

	// Initialize with step count
	initialSteps := make([]stepprogress.Step, stepCount)
	for i := range initialSteps {
		initialSteps[i] = stepprogress.Step{
			Name:   fmt.Sprintf("%d", i+1),
			Status: stepprogress.StatusPending,
		}
	}
	if len(initialSteps) > 0 {
		initialSteps[0].Status = stepprogress.StatusRunning
	}
	model = model.SetSteps(initialSteps)

	// Set up cancellation request handler - called when user presses Ctrl+C in TUI
	model = model.SetCancelRequest(func() {
		fmt.Fprintf(os.Stderr, "\n▸ Cancelling pipeline %s...\n", truncateID(runID))
		fmt.Fprintf(os.Stderr, "▸ Waiting for workflow to stop (press Ctrl+C again to force quit)...\n")
		cmdChan <- protocol.CancelPipelineCommand{
			RunID:  runID,
			Reason: "User interrupted (Ctrl+C)",
		}
	})

	// Run the Bubble Tea program
	// Note: Bubble Tea receives Ctrl+C as a key event, which triggers our cancel handler
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Listen for cancellation confirmation from orchestrator
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.Quit()
				return
			case event, ok := <-eventChan:
				if !ok {
					return
				}
				switch e := event.(type) {
				case protocol.PipelineCancelledEvent:
					fmt.Fprintf(os.Stderr, "▸ Workflow stopped (status: %s)\n", e.WorkflowStatus)
					// Send confirmation to TUI so it can quit
					p.Send(pipelineview.CancelConfirmedMsg{Status: e.WorkflowStatus})
					return
				case protocol.ErrorEvent:
					fmt.Fprintf(os.Stderr, "▸ Error: %s\n", e.Message)
					p.Send(pipelineview.CancelConfirmedMsg{Status: "error"})
					return
				}
			}
		}
	}()

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Print final output after TUI exits
	if m, ok := finalModel.(pipelineview.Model); ok {
		// Print activity log so it persists in terminal
		fmt.Println()
		fmt.Println("─── Agent Output ───────────────────────────────────────────")
		fmt.Println()
		groups := m.Groups()
		if len(groups) > 0 {
			feed := collapsiblefeed.New(80, 1000).SetGroups(groups)
			fmt.Println(feed.RenderContent())
		}

		// Print summary
		if m.ShowSummary() {
			fmt.Println()
			fmt.Println("─────────────────────────────────────────────────────────────")
			fmt.Println()
			fmt.Println(m.GetSummary().View())
		}

		// Return error if pipeline failed
		if m.Status() == pipelineview.StatusFailed {
			return fmt.Errorf("pipeline failed")
		}
	}

	return nil
}

// convertStepStatus converts model status to stepprogress status
func convertStepStatus(s models.StepStatus) stepprogress.StepStatus {
	switch s {
	case models.StepStatusRunning:
		return stepprogress.StatusRunning
	case models.StepStatusCompleted:
		return stepprogress.StatusCompleted
	case models.StepStatusFailed:
		return stepprogress.StatusFailed
	case models.StepStatusSkipped:
		return stepprogress.StatusSkipped
	default:
		return stepprogress.StatusPending
	}
}


// buildPipelineSummary creates summary data from run results
func buildPipelineSummary(run *models.PipelineRun, steps []stepprogress.Step, tokens tokendisplay.TokenData) *pipelinesummary.SummaryData {
	data := &pipelinesummary.SummaryData{
		Status:         convertRunStatus(run.Status),
		TotalSteps:     len(steps),
		TotalTokens:    tokens.InputTokens + tokens.OutputTokens,
		CacheHitTokens: tokens.CacheReadTokens,
		BranchName:     run.BranchName,
		BaseCommitSHA:  run.BaseCommitSHA,
		HeadCommitSHA:  run.HeadCommitSHA,
		ErrorMessage:   run.ErrorMessage,
	}

	// Count completed/failed from steps
	for _, s := range steps {
		if s.Status == stepprogress.StatusCompleted {
			data.CompletedSteps++
		} else if s.Status == stepprogress.StatusFailed {
			data.FailedSteps++
		}
	}

	// Aggregate diff stats from step results
	for _, step := range run.StepResults {
		data.FilesChanged += step.FilesChanged
		data.Insertions += step.Insertions
		data.Deletions += step.Deletions
	}

	// Calculate duration from run timestamps
	if run.StartedAt != nil {
		endTime := time.Now()
		if run.CompletedAt != nil {
			endTime = *run.CompletedAt
		}
		data.Duration = endTime.Sub(*run.StartedAt)
	}

	return data
}

// convertRunStatus converts model status to component status
func convertRunStatus(s models.PipelineRunStatus) pipelinesummary.Status {
	switch s {
	case models.PipelineRunStatusPending:
		return pipelinesummary.StatusPending
	case models.PipelineRunStatusRunning:
		return pipelinesummary.StatusRunning
	case models.PipelineRunStatusCompleted:
		return pipelinesummary.StatusCompleted
	case models.PipelineRunStatusFailed:
		return pipelinesummary.StatusFailed
	default:
		return pipelinesummary.StatusPending
	}
}

func resolveProjectFromDB(ctx context.Context, projectFlag string, dataService *services.DataService) (string, string, error) {
	// Auto-detect git root for matching
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("failed to get current directory: %w", err)
	}
	gitDir := findGitRoot(cwd)

	// If no project flag and not in a git repo, error early
	if projectFlag == "" && gitDir == "" {
		return "", "", fmt.Errorf("not in a git repository. Use --project to specify a project, or run from a git repository")
	}

	// Query projects from database
	projects, err := dataService.LoadProjects(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to load projects: %w", err)
	}

	if len(projects) == 0 {
		return "", "", fmt.Errorf("no projects found. Create a project first with: make run")
	}

	// If project flag specified, look up by name or ID
	if projectFlag != "" {
		for _, p := range projects {
			if p.ID == projectFlag || p.Name == projectFlag {
				return p.ID, p.Name, nil
			}
		}
		// No match - show available ones
		var names []string
		for _, p := range projects {
			names = append(names, fmt.Sprintf("  - %s (ID: %s)", p.Name, truncateID(p.ID)))
		}
		return "", "", fmt.Errorf("project '%s' not found.\n\nAvailable projects:\n%s\n\nRun 'noldarim projects' for full list", projectFlag, strings.Join(names, "\n"))
	}

	// Auto-detect: find project matching current directory
	for _, p := range projects {
		if p.RepositoryPath == gitDir {
			return p.ID, p.Name, nil
		}
	}

	// No matching project - show available ones
	var names []string
	for _, p := range projects {
		names = append(names, fmt.Sprintf("  - %s (%s)", p.Name, p.RepositoryPath))
	}
	return "", "", fmt.Errorf("current directory (%s) doesn't match any project.\n\nAvailable projects:\n%s\n\nUse --project <name> to specify one", gitDir, strings.Join(names, "\n"))
}

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:12] + "..."
	}
	return id
}

func findGitRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func printRunBanner(task, project string) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  noldarim run\n")
	fmt.Printf("  Project: %s\n", project)
	fmt.Printf("  Task: %s\n", truncateForDisplay(task, 50))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

// buildAgentConfig creates an AgentConfigInput from application config
func buildAgentConfig(cfg *config.AppConfig, description string) *protocol.AgentConfigInput {
	// Populate variables with runtime values
	variables := make(map[string]string)
	for k, v := range cfg.Agent.Variables {
		if k == "description" {
			variables[k] = description
		} else {
			variables[k] = v
		}
	}

	return &protocol.AgentConfigInput{
		ToolName:       cfg.Agent.DefaultTool,
		ToolVersion:    cfg.Agent.DefaultVersion,
		PromptTemplate: cfg.Agent.PromptTemplate,
		Variables:      variables,
		ToolOptions:    cfg.Agent.ToolOptions,
		FlagFormat:     cfg.Agent.FlagFormat,
	}
}

func truncateTitle(s string) string {
	if len(s) > 100 {
		return s[:97] + "..."
	}
	return s
}

func truncateForDisplay(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}


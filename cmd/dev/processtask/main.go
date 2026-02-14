// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/utils"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workers"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workflows"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/pkg/containers/service"
)

// findWorktreePathByTaskID searches for a worktree directory containing the given task ID
func findWorktreePathByTaskID(repositoryPath, taskID string) (string, error) {
	worktreesDir := filepath.Join(repositoryPath, ".worktrees")

	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return "", fmt.Errorf("failed to read .worktrees directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), taskID) {
			return filepath.Join(worktreesDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no worktree directory found containing task ID: %s", taskID)
}


func main() {
	// Load test config for orchestrator
	cfg, err := config.NewConfig("test-config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Parse command line flags
	taskID := flag.String("task-id", "", "Task ID (leave empty to use latest task)")
	projectID := flag.String("project-id", "", "Project ID (leave empty to auto-detect)")
	workspaceDir := flag.String("workspace-dir", "/workspace", "Workspace directory")

	flag.Parse()

	// Create data service to look up task details
	dataService, err := services.NewDataService(cfg)
	if err != nil {
		log.Fatalf("Failed to create data service: %v", err)
	}
	defer dataService.Close()

	ctx := context.Background()

	// If no task ID provided, get the latest task
	if *taskID == "" {
		fmt.Println("No task ID provided, getting latest task...")
		latestTask, err := dataService.GetLatestTask(ctx)
		if err != nil {
			log.Fatalf("Failed to get latest task: %v", err)
		}
		*taskID = latestTask.ID
		*projectID = latestTask.ProjectID
		fmt.Printf("Using latest task: %s (project: %s)\n", *taskID, *projectID)
	}

	// Validate required fields
	if *taskID == "" {
		fmt.Println("Error: task-id is required")
		flag.Usage()
		os.Exit(1)
	}
	if *projectID == "" {
		fmt.Println("Error: project-id is required")
		flag.Usage()
		os.Exit(1)
	}

	// Look up task in database to get task file path and title
	tasks, err := dataService.LoadTasks(ctx, *projectID)
	if err != nil {
		log.Fatalf("Failed to load tasks: %v", err)
	}

	task, exists := tasks[*taskID]
	if !exists {
		log.Fatalf("Task %s not found in project %s", *taskID, *projectID)
	}

	if task.TaskFilePath == "" {
		log.Fatalf("Task %s has no file path set", *taskID)
	}

	// Generate task queue name using the shared utility (same logic as workflow)
	taskQueueName := utils.GenerateTaskQueueName(task.Title, *taskID)
	fmt.Printf("Generated task queue: %s\n", taskQueueName)

	// Create Temporal client with dynamic task queue
	client, err := temporal.NewClient(
		cfg.Temporal.HostPort,
		cfg.Temporal.Namespace,
		taskQueueName,
	)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer client.Close()

	// Initialize container service for the orchestrator worker
	containerService, err := service.NewService(nil)
	if err != nil {
		log.Fatalf("Failed to create container service: %v", err)
	}
	defer containerService.Close()

	// Initialize GitServiceManager for thread-safe git operations
	gitServiceManager := services.NewGitServiceManager(cfg)
	defer gitServiceManager.Close()

	// Create orchestrator Temporal worker that listens on the config's default task queue
	// This worker will handle GitCommitActivity on the host system
	orchestratorClient, err := temporal.NewClient(
		cfg.Temporal.HostPort,
		cfg.Temporal.Namespace,
		cfg.Temporal.TaskQueue, // Use default task queue from config
	)
	if err != nil {
		log.Fatalf("Failed to create orchestrator Temporal client: %v", err)
	}
	defer orchestratorClient.Close()

	// Create orchestrator worker with GitCommitActivity
	orchestratorWorker := workers.NewWorker(
		orchestratorClient.GetTemporalClient(),
		cfg,
		gitServiceManager,
		dataService,
		containerService,
		nil, // No event channel needed for this dev tool
	)

	// Start the orchestrator worker in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := orchestratorWorker.Start(context.Background()); err != nil {
			log.Printf("Failed to start orchestrator worker: %v", err)
			return
		}
		fmt.Println("Orchestrator worker started, listening for GitCommitActivity...")
	}()

	// Ensure orchestrator worker is always shutdown
	defer func() {
		fmt.Println("Stopping orchestrator worker...")
		orchestratorWorker.Stop()
		wg.Wait()
		fmt.Println("Orchestrator worker stopped.")
	}()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start shutdown handler in background
	shutdownDone := make(chan struct{})
	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal, will cleanup on exit...")
		close(shutdownDone)
	}()

	// Give the orchestrator worker a moment to start
	time.Sleep(2 * time.Second)

	// Build claude command to read the task file
	// Get project to determine repository path for worktree
	project, err := dataService.GetProject(ctx, *projectID)
	if err != nil {
		log.Fatalf("Failed to get project: %v", err)
	}

	// Find worktree path by searching for directory containing task ID
	// This is needed for the workflow to find and commit changes in the correct worktree
	worktreePath, err := findWorktreePathByTaskID(project.RepositoryPath, *taskID)
	if err != nil {
		log.Fatalf("Failed to find worktree for task %s: %v", *taskID, err)
	}
	fmt.Printf("Using worktree path: %s\n", worktreePath)

	// Build AgentConfig for claude
	agentConfig := &protocol.AgentConfigInput{
		ToolName:       "claude",
		PromptTemplate: fmt.Sprintf("Please read %s and implement it", task.TaskFilePath),
		Variables:      map[string]string{},
		ToolOptions: map[string]interface{}{
			"output-format":                "stream-json",
			"verbose":                      true,
			"dangerously-skip-permissions": true,
		},
		FlagFormat: "space",
	}

	input := types.ProcessTaskWorkflowInput{
		OrchestratorTaskQueue: cfg.Temporal.TaskQueue,
		TaskID:                *taskID,
		TaskFilePath:          task.TaskFilePath,
		ProjectID:             *projectID,
		WorkspaceDir:          *workspaceDir,
		AgentConfig:           agentConfig,
		WorktreePath:          worktreePath, // Critical for git commit activity
	}

	// Generate unique workflow ID for this dev run
	workflowID := fmt.Sprintf("dev-process-task-%d", time.Now().UnixNano())

	// Start the workflow
	execution, err := client.StartWorkflow(ctx, workflowID, workflows.ProcessTaskWorkflowName, input)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("Started ProcessTask workflow: %s\n", execution.GetID())

	// Monitor workflow status in background without blocking
	go func() {
		var result types.ProcessTaskWorkflowOutput
		err = execution.Get(ctx, &result)
		if err != nil {
			log.Printf("Workflow failed: %v", err)
		} else {
			fmt.Printf("Workflow completed successfully: %+v\n", result)
		}
	}()

	fmt.Println("Orchestrator worker is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-shutdownDone
	fmt.Println("Shutting down...")
}

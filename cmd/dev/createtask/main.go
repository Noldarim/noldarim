// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/protocol"
)

func main() {
	// Parse command line flags
	projectID := flag.String("project-id", "", "Project ID (required, or use --latest-project)")
	latestProject := flag.Bool("latest-project", false, "Use the most recently updated project")
	title := flag.String("title", "Dev Test Task", "Task title")
	description := flag.String("description", "Task created via dev_create_task tool", "Task description")
	prompt := flag.String("prompt", "", "Custom prompt template (default: read task file and implement)")
	toolName := flag.String("tool", "claude", "Agent tool to use (claude, test)")
	timeout := flag.Duration("timeout", 10*time.Minute, "Timeout for task completion")

	flag.Parse()

	// Load config
	cfg, err := config.NewConfig("test-config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create data service to look up project
	dataService, err := services.NewDataService(cfg)
	if err != nil {
		log.Fatalf("Failed to create data service: %v", err)
	}
	defer dataService.Close()

	// Resolve project ID
	resolvedProjectID := *projectID
	if *latestProject || resolvedProjectID == "" {
		fmt.Println("Finding latest project...")
		projects, err := dataService.LoadProjects(ctx)
		if err != nil {
			log.Fatalf("Failed to load projects: %v", err)
		}
		if len(projects) == 0 {
			log.Fatalf("No projects found. Create a project first via the TUI.")
		}

		// Find the most recently updated project
		var latestProj *models.Project
		for _, p := range projects {
			proj := p // Create a copy to avoid pointer issues
			if latestProj == nil || proj.LastUpdatedAt.After(latestProj.LastUpdatedAt) {
				latestProj = proj
			}
		}
		resolvedProjectID = latestProj.ID
		fmt.Printf("Using project: %s (%s)\n", latestProj.Name, resolvedProjectID)
	}

	// Validate project exists
	project, err := dataService.GetProject(ctx, resolvedProjectID)
	if err != nil {
		log.Fatalf("Failed to get project %s: %v", resolvedProjectID, err)
	}
	fmt.Printf("Project: %s\n", project.Name)
	fmt.Printf("Repository: %s\n", project.RepositoryPath)

	// Create command and event channels
	cmdChan := make(chan protocol.Command, 10)
	eventChan := make(chan protocol.Event, 100)

	// Create orchestrator
	orch, err := orchestrator.New(cmdChan, eventChan, cfg)
	if err != nil {
		log.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Start orchestrator in background
	go orch.Run(ctx)
	defer orch.Close()

	// Give Temporal worker time to start
	fmt.Println("Waiting for Temporal worker to start...")
	time.Sleep(2 * time.Second)

	// Build prompt template
	promptTemplate := *prompt
	if promptTemplate == "" {
		promptTemplate = "Please read the task file and implement it"
	}

	// Build AgentConfig
	agentConfig := &protocol.AgentConfigInput{
		ToolName:       *toolName,
		PromptTemplate: promptTemplate,
		Variables:      map[string]string{},
		FlagFormat:     "space",
	}

	// Add tool-specific options
	if *toolName == "claude" {
		agentConfig.ToolOptions = map[string]interface{}{
			"output-format":                "stream-json",
			"verbose":                      true,
			"dangerously-skip-permissions": true,
		}
	}

	// Generate task ID
	taskID := fmt.Sprintf("dev-%s", time.Now().Format("20060102-150405"))

	// Create the command
	cmd := protocol.CreateTaskCommand{
		Metadata: protocol.Metadata{
			TaskID:  taskID,
			Version: protocol.CurrentProtocolVersion,
		},
		ProjectID:   resolvedProjectID,
		Title:       *title,
		Description: *description,
		AgentConfig: agentConfig,
	}

	fmt.Println("\n========================================")
	fmt.Printf("Creating task: %s\n", *title)
	fmt.Printf("Task ID: %s\n", taskID)
	fmt.Printf("Tool: %s\n", *toolName)
	fmt.Printf("Timeout: %s\n", *timeout)
	fmt.Println("========================================")

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Send the command
	cmdChan <- cmd
	fmt.Println("Command sent, waiting for events...")

	// Event loop
	deadline := time.After(*timeout)
	startTime := time.Now()

	for {
		select {
		case event := <-eventChan:
			elapsed := time.Since(startTime).Round(time.Millisecond)
			printEvent(elapsed, event)

			// Check for terminal events
			switch evt := event.(type) {
			case protocol.TaskLifecycleEvent:
				if evt.Type == protocol.TaskFinished {
					fmt.Printf("\n[SUCCESS] Task completed in %s\n", time.Since(startTime).Round(time.Second))
					return
				}
			case protocol.ErrorEvent:
				fmt.Printf("\n[ERROR] Task failed: %s\n", evt.Message)
				if evt.Context != "" {
					fmt.Printf("Context: %s\n", evt.Context)
				}
				os.Exit(1)
			}

		case <-deadline:
			fmt.Printf("\n[TIMEOUT] Task did not complete within %s\n", *timeout)
			os.Exit(1)

		case sig := <-sigChan:
			fmt.Printf("\n[INTERRUPTED] Received %s, shutting down...\n", sig)
			return
		}
	}
}

func printEvent(elapsed time.Duration, event protocol.Event) {
	timestamp := fmt.Sprintf("[%s]", elapsed)

	switch evt := event.(type) {
	case protocol.TaskLifecycleEvent:
		icon := getLifecycleIcon(evt.Type)
		fmt.Printf("%s %s %s (task: %s)\n", timestamp, icon, evt.Type, evt.TaskID)

	case *models.AIActivityRecord:
		// Print AI record with meaningful content using flat fields
		switch evt.EventType {
		case models.AIEventToolUse:
			if evt.ToolInputSummary != "" {
				fmt.Printf("%s > %s: %s\n", timestamp, evt.ToolName, truncateStr(evt.ToolInputSummary, 80))
			} else {
				fmt.Printf("%s > %s\n", timestamp, evt.ToolName)
			}
		case models.AIEventToolResult:
			status := "OK"
			if evt.ToolSuccess != nil && !*evt.ToolSuccess {
				status = "ERR"
			}
			fmt.Printf("%s < %s [%s]\n", timestamp, evt.ToolName, status)
		case models.AIEventThinking:
			content := evt.ContentPreview
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("%s ~ %s\n", timestamp, content)
		case models.AIEventAIOutput:
			content := evt.ContentPreview
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			fmt.Printf("%s   %s\n", timestamp, content)
		case models.AIEventSessionEnd, models.AIEventStop:
			reason := "completed"
			if evt.StopReason != "" {
				reason = evt.StopReason
			}
			fmt.Printf("%s X Session ended: %s\n", timestamp, reason)
		case models.AIEventUserPrompt:
			prompt := extractUserPromptContent(evt.RawPayload)
			if prompt != "" {
				fmt.Printf("%s User: %s\n", timestamp, truncateStr(prompt, 80))
			} else {
				fmt.Printf("%s User prompt submitted\n", timestamp)
			}
		default:
			fmt.Printf("%s [%s]\n", timestamp, evt.EventType)
		}

	case protocol.AIActivityBatchEvent:
		fmt.Printf("%s AI Batch: %d events\n", timestamp, len(evt.Activities))

	case protocol.AIStreamStartEvent:
		fmt.Printf("%s AI Stream started (task: %s)\n", timestamp, evt.TaskID)

	case protocol.AIStreamEndEvent:
		fmt.Printf("%s AI Stream ended (task: %s, status: %s)\n", timestamp, evt.TaskID, evt.FinalStatus)

	case protocol.ErrorEvent:
		fmt.Printf("%s ERROR: %s\n", timestamp, evt.Message)

	case protocol.ProjectCreatedEvent:
		fmt.Printf("%s Project created: %s\n", timestamp, evt.Project.Name)

	case protocol.TaskCreationStartedEvent:
		fmt.Printf("%s Task creation started (workflow: %s)\n", timestamp, evt.WorkflowID)

	default:
		fmt.Printf("%s Event: %T\n", timestamp, event)
	}
}

func getLifecycleIcon(eventType protocol.TaskLifecycleType) string {
	switch eventType {
	case protocol.TaskCreated:
		return "+"
	case protocol.TaskInProgress:
		return ">"
	case protocol.TaskFinished:
		return "v"
	default:
		return "*"
	}
}

// extractUserPromptContent extracts the prompt from raw payload
func extractUserPromptContent(raw string) string {
	if raw == "" {
		return ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return ""
	}

	for _, key := range []string{"prompt", "message", "content", "user_message"} {
		if v, ok := data[key].(string); ok && v != "" {
			return v
		}
	}

	return ""
}

// truncateStr shortens a string to max length with ellipsis
func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return "..."
	}
	return s[:max-3] + "..."
}

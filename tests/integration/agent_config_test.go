// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/protocol"
)

// TestAgentConfigExecution tests the new AgentConfig feature
// This is a minimal test to verify the agent config preparation works
func TestAgentConfigExecution(t *testing.T) {
	// Skip if we don't want to run integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "noldarim-agent-config-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create sample test-claude-config.json
	testClaudeConfigPath := filepath.Join(tempDir, "test-claude-config.json")
	testClaudeConfig := `{
  "api_key": "test-api-key",
  "model": "claude-3-haiku-20240307"
}`
	err = os.WriteFile(testClaudeConfigPath, []byte(testClaudeConfig), 0o644)
	require.NoError(t, err)

	// Setup config
	cfg, err := config.NewConfig("../../test-config.yaml")
	require.NoError(t, err)
	cfg.Database.Driver = "sqlite"
	cfg.Database.Database = ":memory:"
	cfg.Claude.ClaudeJSONHostPath = testClaudeConfigPath

	ctx := context.Background()

	// Create test project directory with git repository
	testProjectDir := filepath.Join(tempDir, "test-project")

	// Initialize git repository
	gitService, err := services.NewGitService(testProjectDir, true)
	require.NoError(t, err)

	err = gitService.SetConfig(ctx, testProjectDir, "user.name", "Test User")
	require.NoError(t, err)

	err = gitService.SetConfig(ctx, testProjectDir, "user.email", "test@example.com")
	require.NoError(t, err)

	// Create a sample file to analyze
	sampleFile := filepath.Join(testProjectDir, "main.go")
	sampleCode := `package main

import "fmt"

func main() {
	// TODO: optimize this code
	for i := 0; i < 1000000; i++ {
		fmt.Println(i)
	}
}
`
	err = os.WriteFile(sampleFile, []byte(sampleCode), 0o644)
	require.NoError(t, err)

	// Create initial commit
	err = gitService.CreateCommit(ctx, testProjectDir, "Initial commit")
	require.NoError(t, err)

	// Setup orchestrator
	dataServiceFixture := services.WithDataService(t)
	cmdChan := make(chan protocol.Command, 10)
	eventChan := make(chan protocol.Event, 10)

	orch, err := orchestrator.New(cmdChan, eventChan, cfg)
	require.NoError(t, err)

	go orch.Run(ctx)
	defer orch.Close()

	// Give the Temporal worker time to start polling
	// The worker.Run() is started in a goroutine and needs time to connect
	time.Sleep(2 * time.Second)

	// Create test project
	project, err := dataServiceFixture.Service.CreateProject(ctx, "Agent Config Test", "Testing new agent config", testProjectDir)
	require.NoError(t, err)

	// Create task with AgentConfig
	taskID := "agent-test-" + time.Now().Format("20060102-150405")

	// Create an AgentConfig for testing
	agentConfig := &protocol.AgentConfigInput{
		ToolName:       "claude",
		ToolVersion:    "4.5",
		PromptTemplate: "Please analyze the file {{.file}} and suggest optimizations for {{.focus_area}}. Keep your response concise.",
		Variables: map[string]string{
			"file":       "main.go",
			"focus_area": "performance and memory usage",
		},
		ToolOptions: map[string]interface{}{
			"model":      "claude-sonnet-4-5",
			"max_tokens": 1000,
		},
		FlagFormat: "space",
	}

	// Use AgentConfig directly in the CreateTaskCommand
	cmd := protocol.CreateTaskCommand{
		Metadata: protocol.Metadata{
			TaskID:  taskID,
			Version: protocol.CurrentProtocolVersion,
		},
		ProjectID:   project.ID,
		Title:       "Test Agent Config",
		Description: "Testing the new AgentConfig feature",
		AgentConfig: agentConfig,
	}

	t.Logf("Created test command with task ID: %s", taskID)
	t.Logf("AgentConfig that would be used:")
	t.Logf("  Tool: %s v%s", agentConfig.ToolName, agentConfig.ToolVersion)
	t.Logf("  Prompt: %s", agentConfig.PromptTemplate)
	t.Logf("  Variables: %+v", agentConfig.Variables)
	t.Logf("  Options: %+v", agentConfig.ToolOptions)

	// Expected command that would be generated:
	t.Logf("\nExpected generated command:")
	t.Logf("  claude --prompt \"Please analyze the file main.go and suggest optimizations for performance and memory usage. Keep your response concise.\" --model claude-sonnet-4-5 --max-tokens 1000")

	cmdChan <- cmd

	// Collect events
	t.Log("\nWaiting for events...")
	events := collectEvents(t, eventChan, 30*time.Second)

	// Log received events
	t.Logf("\nReceived %d events:", len(events))
	for i, event := range events {
		switch evt := event.(type) {
		case protocol.TaskLifecycleEvent:
			if evt.Type == protocol.TaskCreated && evt.Task != nil {
				t.Logf("  %d. TaskLifecycleEvent(TaskCreated) - Task: %s, ProjectID: %s", i+1, evt.Task.ID, evt.ProjectID)
			} else {
				t.Logf("  %d. TaskLifecycleEvent(%s) - TaskID: %s, ProjectID: %s", i+1, evt.Type, evt.TaskID, evt.ProjectID)
			}
		case protocol.ErrorEvent:
			t.Logf("  %d. ErrorEvent - Message: %s, Context: %s", i+1, evt.Message, evt.Context)
		default:
			t.Logf("  %d. %T", i+1, event)
		}
	}

	// Note: This test demonstrates the structure but won't fully execute
	// because we need to modify the orchestrator to pass AgentConfig
	t.Log("\n=== MANUAL TEST INSTRUCTIONS ===")
	t.Log("To fully test AgentConfig:")
	t.Log("1. Check logs for 'Preparing agent command' with tool='claude'")
	t.Log("2. Check logs for 'Agent command prepared successfully' with the full command")
	t.Log("3. Look for 'Using structured agent config' in ProcessTaskWorkflow logs")
	t.Log("4. The command should be: claude --prompt \"...\" --model \"...\" --max-tokens N")
}

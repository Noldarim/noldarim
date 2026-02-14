// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestrator

import (
	"context"
	stdlog "log"
	"os"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/database"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"testing"
	"time"
)

var testFixture *OrchestratorFixture

// TestMain sets up the test suite with a shared orchestrator instance
func TestMain(m *testing.M) {
	// Setup: Create a single orchestrator instance for all tests
	cfg := database.WithInMemoryConfig()

	// Add Temporal config since WithInMemoryConfig doesn't include it
	cfg.Temporal = config.TemporalConfig{
		HostPort:  "localhost:7233",
		Namespace: "default",
		TaskQueue: "test-task-queue",
		Worker: config.WorkerConfig{
			MaxConcurrentActivityExecutions: 10,
			MaxConcurrentWorkflows:          10,
			ActivitiesPerSecond:             1000,
		},
	}

	// Add Container config
	cfg.Container = config.ContainerConfig{
		DefaultImage: "alpine:latest",
	}

	// Create database
	db, err := database.NewGormDB(&cfg.Database)
	if err != nil {
		stdlog.Fatalf("Failed to create in-memory database: %v", err)
	}

	// Run migrations
	if err := db.AutoMigrate(); err != nil {
		stdlog.Fatalf("Failed to run migrations: %v", err)
	}

	cmdChan := make(chan protocol.Command, 10)
	eventChan := make(chan protocol.Event, 10)

	orch, err := New(cmdChan, eventChan, cfg)
	if err != nil {
		stdlog.Fatalf("Failed to create orchestrator: %v", err)
	}

	testFixture = &OrchestratorFixture{
		Orchestrator: orch,
		CmdChan:      cmdChan,
		EventChan:    eventChan,
		Cleanup: func() {
			orch.Close()
			close(cmdChan)
			close(eventChan)
			db.Close()
		},
	}

	// Run tests
	code := m.Run()

	// Cleanup after all tests
	testFixture.Cleanup()

	os.Exit(code)
}

func TestHandleCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmd         protocol.Command
		expectLog   string
		expectEvent bool
	}{
		{
			name:        "LoadProjectsCommand",
			cmd:         protocol.LoadProjectsCommand{},
			expectLog:   "Processing command: protocol.LoadProjectsCommand",
			expectEvent: true,
		},
		{
			name:        "LoadTasksCommand",
			cmd:         protocol.LoadTasksCommand{ProjectID: "test-project"},
			expectLog:   "Processing command: protocol.LoadTasksCommand",
			expectEvent: true,
		},
		{
			name:        "ToggleTaskCommand",
			cmd:         protocol.ToggleTaskCommand{ProjectID: "test-project", TaskID: "test-task"},
			expectLog:   "Processing command: protocol.ToggleTaskCommand",
			expectEvent: true,
		},
		{
			name:        "DeleteTaskCommand",
			cmd:         protocol.DeleteTaskCommand{ProjectID: "test-project", TaskID: "test-task"},
			expectLog:   "Processing command: protocol.DeleteTaskCommand",
			expectEvent: true,
		},
		{
			name:        "CreateTaskCommand",
			cmd:         protocol.CreateTaskCommand{ProjectID: "test-project", Title: "Test Task", Description: "Test Description"},
			expectLog:   "Processing command: protocol.CreateTaskCommand",
			expectEvent: true,
		},
		{
			name:        "LoadAIActivityCommand",
			cmd:         protocol.LoadAIActivityCommand{ProjectID: "test-project", TaskID: "test-task"},
			expectLog:   "Processing command: protocol.LoadAIActivityCommand",
			expectEvent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the shared test fixture
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Clear event channel before test
			for len(testFixture.EventChan) > 0 {
				<-testFixture.EventChan
			}

			testFixture.Orchestrator.handleCommand(ctx, tt.cmd)

			if tt.expectEvent {
				select {
				case event := <-testFixture.EventChan:
					if event == nil {
						t.Error("Expected event but got nil")
					}
				case <-time.After(100 * time.Millisecond):
					t.Error("Expected event but none received")
				}
			}
		})
	}
}

func TestHandleCommandWithCancellation(t *testing.T) {
	// Clear event channel before test
	for len(testFixture.EventChan) > 0 {
		<-testFixture.EventChan
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	testFixture.Orchestrator.handleCommand(ctx, protocol.LoadProjectsCommand{})

	select {
	case event := <-testFixture.EventChan:
		if _, ok := event.(protocol.ErrorEvent); ok {
			t.Error("Should not receive error event when context is cancelled")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandleCommandWithTimeout(t *testing.T) {
	// Clear event channel before test
	for len(testFixture.EventChan) > 0 {
		<-testFixture.EventChan
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	testFixture.Orchestrator.handleCommand(ctx, protocol.LoadProjectsCommand{})

	select {
	case event := <-testFixture.EventChan:
		if _, ok := event.(protocol.ErrorEvent); ok {
			t.Error("Should not receive error event when context times out")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandleLoadAIActivity(t *testing.T) {
	// Clear event channel before test
	for len(testFixture.EventChan) > 0 {
		<-testFixture.EventChan
	}

	ctx := context.Background()
	taskID := "test-ai-activity-task"
	trueVal := true

	// Save some AI activity records directly via data service
	records := []*models.AIActivityRecord{
		{
			EventID:          "evt-test-001",
			TaskID:           taskID,
			SessionID:        "session-test",
			EventType:        models.AIEventToolUse,
			ToolName:         "Bash",
			ToolInputSummary: "ls",
		},
		{
			EventID:        "evt-test-002",
			TaskID:         taskID,
			SessionID:      "session-test",
			EventType:      models.AIEventToolResult,
			ToolName:       "Bash",
			ToolSuccess:    &trueVal,
			ContentPreview: "file.txt",
		},
	}

	for _, record := range records {
		err := testFixture.Orchestrator.dataService.SaveAIActivityRecord(ctx, record)
		if err != nil {
			t.Fatalf("Failed to save AI activity record: %v", err)
		}
	}

	// Execute the LoadAIActivityCommand
	testFixture.Orchestrator.handleCommand(ctx, protocol.LoadAIActivityCommand{
		ProjectID: "test-project",
		TaskID:    taskID,
	})

	// Verify we receive the AIActivityBatchEvent
	select {
	case event := <-testFixture.EventChan:
		loadedEvent, ok := event.(protocol.AIActivityBatchEvent)
		if !ok {
			t.Errorf("Expected AIActivityBatchEvent, got %T", event)
			return
		}
		if loadedEvent.TaskID != taskID {
			t.Errorf("Expected TaskID %s, got %s", taskID, loadedEvent.TaskID)
		}
		if len(loadedEvent.Activities) != 2 {
			t.Errorf("Expected 2 events, got %d", len(loadedEvent.Activities))
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected AIActivityBatchEvent but none received")
	}
}

func TestHandleLoadAIActivityEmpty(t *testing.T) {
	// Clear event channel before test
	for len(testFixture.EventChan) > 0 {
		<-testFixture.EventChan
	}

	ctx := context.Background()
	taskID := "nonexistent-task-id"

	// Execute the LoadAIActivityCommand for a task with no events
	testFixture.Orchestrator.handleCommand(ctx, protocol.LoadAIActivityCommand{
		ProjectID: "test-project",
		TaskID:    taskID,
	})

	// Verify we receive the AIActivityBatchEvent with empty activities
	select {
	case event := <-testFixture.EventChan:
		loadedEvent, ok := event.(protocol.AIActivityBatchEvent)
		if !ok {
			t.Errorf("Expected AIActivityBatchEvent, got %T", event)
			return
		}
		if loadedEvent.TaskID != taskID {
			t.Errorf("Expected TaskID %s, got %s", taskID, loadedEvent.TaskID)
		}
		if len(loadedEvent.Activities) != 0 {
			t.Errorf("Expected 0 events, got %d", len(loadedEvent.Activities))
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected AIActivityBatchEvent but none received")
	}
}

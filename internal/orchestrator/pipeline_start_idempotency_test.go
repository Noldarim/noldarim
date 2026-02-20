// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"github.com/noldarim/noldarim/internal/common"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/database"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal"
	"github.com/noldarim/noldarim/internal/protocol"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Note: MockTemporalClient and MockWorkflowRun are defined in mocks_test.go

// setupTestOrchestrator creates an orchestrator with mocked dependencies for testing
func setupTestOrchestrator(t *testing.T, mockTemporalClient *MockTemporalClient) (*Orchestrator, chan protocol.Event, *services.DataService) {
	t.Helper()

	cfg := database.WithInMemoryConfig()
	cfg.Temporal = config.TemporalConfig{
		HostPort:  "localhost:7233",
		Namespace: "default",
		TaskQueue: "test-task-queue",
	}
	cfg.Container = config.ContainerConfig{
		DefaultImage: "alpine:latest",
	}
	cfg.Git = config.GitConfig{
		WorktreeBasePath: t.TempDir(),
	}

	db, err := database.NewGormDB(&cfg.Database)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		t.Fatalf("Failed to create data service: %v", err)
	}
	t.Cleanup(func() { dataService.Close() })

	eventChan := make(chan protocol.Event, 10)

	pipelineSvc := services.NewPipelineService(dataService, nil, mockTemporalClient, cfg)

	// Create orchestrator with minimal dependencies for testing
	orch := &Orchestrator{
		cmdChan:           make(<-chan protocol.Command),
		eventChan:         eventChan,
		internalEventChan: make(chan protocol.Event, 100),
		dataService:       dataService,
		temporalClient:    mockTemporalClient,
		pipelineService:   pipelineSvc,
		config:            cfg,
	}

	return orch, eventChan, dataService
}

// TestHandleStartPipeline_IdempotentReturn verifies that running or completed workflows
// return AlreadyExists=true with the correct status instead of starting a new workflow.
func TestHandleStartPipeline_IdempotentReturn(t *testing.T) {
	cases := []struct {
		workflowStatus temporal.WorkflowStatus
		expectedStatus protocol.PipelineStatus
	}{
		{temporal.WorkflowStatusRunning, protocol.PipelineStatusRunning},
		{temporal.WorkflowStatusCompleted, protocol.PipelineStatusCompleted},
	}

	for _, tc := range cases {
		t.Run(tc.workflowStatus.String(), func(t *testing.T) {
			mockClient := new(MockTemporalClient)
			orch, eventChan, dataService := setupTestOrchestrator(t, mockClient)

			project, err := dataService.CreateProject(context.Background(), "test-project", "Test Project", t.TempDir())
			assert.NoError(t, err)

			mockClient.On("GetWorkflowStatus", mock.Anything, mock.Anything).Return(tc.workflowStatus, nil)

			cmd := protocol.StartPipelineCommand{
				Metadata:      common.Metadata{TaskID: "test-task", Version: common.CurrentProtocolVersion},
				ProjectID:     project.ID,
				Name:          "test-pipeline",
				Steps:         []protocol.StepInput{{StepID: "1", Name: "Step 1"}},
				BaseCommitSHA: "abc123def456",
			}

			go orch.handleStartPipeline(context.Background(), cmd)

			select {
			case event := <-eventChan:
				pipelineEvent, ok := event.(protocol.PipelineRunStartedEvent)
				assert.True(t, ok, "Expected PipelineRunStartedEvent, got %T", event)
				assert.True(t, pipelineEvent.AlreadyExists)
				assert.Equal(t, tc.expectedStatus, pipelineEvent.Status)
				assert.NotEmpty(t, pipelineEvent.RunID)
			case <-time.After(2 * time.Second):
				t.Fatal("Expected event but none received")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleStartPipeline_RetriableStatuses verifies that failed, canceled, terminated,
// and timed-out workflows are retried by starting a new workflow.
func TestHandleStartPipeline_RetriableStatuses(t *testing.T) {
	retriableStatuses := []temporal.WorkflowStatus{
		temporal.WorkflowStatusFailed,
		temporal.WorkflowStatusCanceled,
		temporal.WorkflowStatusTerminated,
		temporal.WorkflowStatusTimedOut,
	}

	for _, status := range retriableStatuses {
		t.Run(status.String(), func(t *testing.T) {
			mockClient := new(MockTemporalClient)
			orch, eventChan, dataService := setupTestOrchestrator(t, mockClient)

			project, err := dataService.CreateProject(context.Background(), "test-project", "Test Project", t.TempDir())
			assert.NoError(t, err)

			mockClient.On("GetWorkflowStatus", mock.Anything, mock.Anything).Return(status, nil)

			mockWorkflowRun := new(MockWorkflowRun)
			mockWorkflowRun.On("GetID").Return("retry12345678-pipeline")
			mockWorkflowRun.On("GetRunID").Return("retry12345678")
			mockClient.On("StartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockWorkflowRun, nil)

			cmd := protocol.StartPipelineCommand{
				Metadata:      common.Metadata{TaskID: "test-task", Version: common.CurrentProtocolVersion},
				ProjectID:     project.ID,
				Name:          "test-pipeline",
				Steps:         []protocol.StepInput{{StepID: "1", Name: "Step 1"}},
				BaseCommitSHA: "abc123def456",
			}

			go orch.handleStartPipeline(context.Background(), cmd)

			select {
			case event := <-eventChan:
				pipelineEvent, ok := event.(protocol.PipelineRunStartedEvent)
				assert.True(t, ok, "Expected PipelineRunStartedEvent, got %T", event)
				assert.False(t, pipelineEvent.AlreadyExists)
				assert.NotEmpty(t, pipelineEvent.RunID)
			case <-time.After(2 * time.Second):
				t.Fatal("Expected event but none received")
			}

			mockClient.AssertCalled(t, "StartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// TestHandleStartPipeline_NewWorkflow tests starting a completely new pipeline
func TestHandleStartPipeline_NewWorkflow(t *testing.T) {
	mockClient := new(MockTemporalClient)
	orch, eventChan, dataService := setupTestOrchestrator(t, mockClient)

	project, err := dataService.CreateProject(context.Background(), "test-project", "Test Project", t.TempDir())
	assert.NoError(t, err)

	// Mock GetWorkflowStatus to return error (workflow not found)
	mockClient.On("GetWorkflowStatus", mock.Anything, mock.Anything).Return(temporal.WorkflowStatusUnknown, errors.New("workflow not found"))

	// Mock StartWorkflow to succeed
	mockWorkflowRun := new(MockWorkflowRun)
	mockWorkflowRun.On("GetID").Return("newrun123456-pipeline")
	mockWorkflowRun.On("GetRunID").Return("newrun123456")
	mockClient.On("StartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockWorkflowRun, nil)

	cmd := protocol.StartPipelineCommand{
		Metadata:      common.Metadata{TaskID: "test-task", Version: common.CurrentProtocolVersion},
		ProjectID:     project.ID,
		Name:          "test-pipeline",
		Steps:         []protocol.StepInput{{StepID: "1", Name: "Step 1"}},
		BaseCommitSHA: "abc123def456",
	}

	go orch.handleStartPipeline(context.Background(), cmd)

	select {
	case event := <-eventChan:
		pipelineEvent, ok := event.(protocol.PipelineRunStartedEvent)
		assert.True(t, ok, "Expected PipelineRunStartedEvent, got %T", event)
		assert.False(t, pipelineEvent.AlreadyExists, "AlreadyExists should be false for new pipeline")
		assert.NotEmpty(t, pipelineEvent.RunID)
	case <-time.After(2 * time.Second):
		t.Fatal("Expected event but none received")
	}

	mockClient.AssertExpectations(t)
}

// TestHandleStartPipeline_StartWorkflowError tests error handling when StartWorkflow fails
func TestHandleStartPipeline_StartWorkflowError(t *testing.T) {
	mockClient := new(MockTemporalClient)
	orch, eventChan, dataService := setupTestOrchestrator(t, mockClient)

	project, err := dataService.CreateProject(context.Background(), "test-project", "Test Project", t.TempDir())
	assert.NoError(t, err)

	// Mock GetWorkflowStatus to return error (workflow not found)
	mockClient.On("GetWorkflowStatus", mock.Anything, mock.Anything).Return(temporal.WorkflowStatusUnknown, errors.New("workflow not found"))

	// Mock StartWorkflow to fail
	mockClient.On("StartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal unavailable"))

	cmd := protocol.StartPipelineCommand{
		Metadata:      common.Metadata{TaskID: "test-task", Version: common.CurrentProtocolVersion},
		ProjectID:     project.ID,
		Name:          "test-pipeline",
		Steps:         []protocol.StepInput{{StepID: "1", Name: "Step 1"}},
		BaseCommitSHA: "abc123def456",
	}

	go orch.handleStartPipeline(context.Background(), cmd)

	select {
	case event := <-eventChan:
		errorEvent, ok := event.(protocol.ErrorEvent)
		assert.True(t, ok, "Expected ErrorEvent, got %T", event)
		assert.Contains(t, errorEvent.Message, "Failed to start pipeline")
		assert.Contains(t, errorEvent.Context, "temporal unavailable")
	case <-time.After(2 * time.Second):
		t.Fatal("Expected event but none received")
	}

	mockClient.AssertExpectations(t)
}

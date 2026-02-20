// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package workers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/activities"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workflows"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/pkg/containers/service"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

var (
	log     *zerolog.Logger
	logOnce sync.Once
)

func getLog() *zerolog.Logger {
	logOnce.Do(func() {
		l := logger.GetTemporalLogger().With().Str("component", "worker").Logger()
		log = &l
	})
	return log
}

// Worker represents a Temporal worker
type Worker struct {
	temporalClient         client.Client
	taskQueue              string
	worker                 worker.Worker
	gitActivities          *activities.GitActivities
	dataActivities         *activities.DataActivities
	containerActivities    *activities.ContainerActivities
	agentSetupActivities   *activities.AgentSetupActivities
	eventActivities        *activities.EventActivities
	aiEventActivities      *activities.AIEventActivities // New: for raw event save/parse
	taskFileActivities     *activities.TaskFileActivities
	pipelineDataActivities *activities.PipelineDataActivities // Pipeline workflow activities
	stepDocActivities      *activities.StepDocumentationActivities
	config                 *config.AppConfig
	mu                     sync.Mutex
	stopped                bool
}

// NewWorker creates a new Temporal worker
func NewWorker(
	temporalClient client.Client,
	cfg *config.AppConfig,
	gitServiceManager *services.GitServiceManager,
	dataService *services.DataService,
	containerService *service.Service,
	eventChan chan<- protocol.Event,
) *Worker {
	// Create activity instances
	gitActivities := activities.NewGitActivities(gitServiceManager)
	dataActivities := activities.NewDataActivities(dataService, cfg)
	containerActivities := activities.NewContainerActivities(containerService, cfg)
	agentSetupActivities := activities.NewAgentSetupActivities(containerService, cfg)
	eventActivities := activities.NewEventActivities(eventChan)
	aiEventActivities := activities.NewAIEventActivities(dataService)
	taskFileActivities := activities.NewTaskFileActivities(cfg)
	pipelineDataActivities := activities.NewPipelineDataActivities(dataService)
	stepDocActivities := activities.NewStepDocumentationActivities()

	return &Worker{
		temporalClient:         temporalClient,
		taskQueue:              cfg.Temporal.TaskQueue,
		gitActivities:          gitActivities,
		dataActivities:         dataActivities,
		containerActivities:    containerActivities,
		agentSetupActivities:   agentSetupActivities,
		eventActivities:        eventActivities,
		aiEventActivities:      aiEventActivities,
		taskFileActivities:     taskFileActivities,
		pipelineDataActivities: pipelineDataActivities,
		stepDocActivities:      stepDocActivities,
		config:                 cfg,
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	getLog().Info().Str("task_queue", w.taskQueue).Msg("Starting Temporal worker")

	// Check if this worker instance has been stopped before
	if w.stopped {
		return fmt.Errorf("cannot restart a stopped worker - create a new worker instance")
	}

	// Check if worker already exists
	if w.worker != nil {
		getLog().Info().Msg("Worker already started")
		return nil
	}

	// Create worker options from config
	// Note: Worker inherits logger from the client, so no need to set it here
	workerOptions := worker.Options{
		MaxConcurrentActivityExecutionSize:      w.config.Temporal.Worker.MaxConcurrentActivityExecutions,
		MaxConcurrentWorkflowTaskExecutionSize:  w.config.Temporal.Worker.MaxConcurrentWorkflows,
		MaxConcurrentLocalActivityExecutionSize: w.config.Temporal.Worker.MaxConcurrentActivityExecutions,
		WorkerActivitiesPerSecond:               w.config.Temporal.Worker.ActivitiesPerSecond,
		WorkerLocalActivitiesPerSecond:          w.config.Temporal.Worker.ActivitiesPerSecond,
		TaskQueueActivitiesPerSecond:            w.config.Temporal.Worker.ActivitiesPerSecond,
	}

	// Create a fresh worker instance
	w.worker = worker.New(w.temporalClient, w.taskQueue, workerOptions)

	// Register workflows - unified pipeline system
	// Note: CreateTaskWorkflow and ProcessTaskWorkflow are deprecated and no longer registered.
	// All task creation now goes through PipelineWorkflow as single-step pipelines.
	w.worker.RegisterWorkflow(workflows.PipelineWorkflow)
	w.worker.RegisterWorkflow(workflows.SetupWorkflow)
	w.worker.RegisterWorkflow(workflows.ProcessingStepWorkflow)

	// Register activities
	w.registerActivities()

	// Capture worker reference to avoid race condition
	workerInstance := w.worker

	// Start the worker
	go func() {
		if err := workerInstance.Run(worker.InterruptCh()); err != nil {
			getLog().Error().Err(err).Msg("Worker stopped with error")
		}
	}()

	getLog().Info().Msg("Temporal worker started successfully")
	return nil
}

// registerActivities registers all activities with the worker
func (w *Worker) registerActivities() {
	// Register Git activities
	w.worker.RegisterActivity(w.gitActivities.CreateWorktreeActivity)
	w.worker.RegisterActivity(w.gitActivities.RemoveWorktreeActivity)
	w.worker.RegisterActivity(w.gitActivities.CommitChangesActivity)
	w.worker.RegisterActivity(w.gitActivities.GetWorktreeStatusActivity)
	w.worker.RegisterActivity(w.gitActivities.GitCommitActivity)
	w.worker.RegisterActivity(w.gitActivities.CaptureGitDiffActivity)

	// Register Data activities
	w.worker.RegisterActivity(w.dataActivities.CreateTaskActivity)
	w.worker.RegisterActivity(w.dataActivities.DeleteTaskActivity)
	w.worker.RegisterActivity(w.dataActivities.UpdateTaskStatusActivity)
	w.worker.RegisterActivity(w.dataActivities.UpdateTaskGitDiffActivity)
	w.worker.RegisterActivity(w.dataActivities.LoadProjectsActivity)
	w.worker.RegisterActivity(w.dataActivities.LoadTasksActivity)
	w.worker.RegisterActivity(w.dataActivities.SaveAIActivityRecordActivity)
	w.worker.RegisterActivity(w.dataActivities.LoadAIActivityByTaskActivity)

	// Register Container activities
	w.worker.RegisterActivity(w.containerActivities.CreateContainerActivity)
	w.worker.RegisterActivity(w.containerActivities.StopContainerActivity)
	w.worker.RegisterActivity(w.containerActivities.RemoveContainerActivity)
	w.worker.RegisterActivity(w.containerActivities.GetContainerStatusActivity)

	// Register Agent Setup activities
	w.worker.RegisterActivity(w.agentSetupActivities.CopyClaudeConfigActivity)
	w.worker.RegisterActivity(w.agentSetupActivities.CopyClaudeCredentialsActivity)

	// Register Event activities - strongly typed activities for TUI events
	w.worker.RegisterActivity(w.eventActivities.PublishTaskCreatedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishTaskDeletedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishTaskStatusUpdatedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishTaskInProgressEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishTaskFinishedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishTaskRequestedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishErrorEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishAIActivityEventActivity)

	// Register Pipeline Event activities - for pipeline lifecycle events to TUI
	w.worker.RegisterActivity(w.eventActivities.PublishPipelineCreatedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishPipelineStepStartedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishPipelineStepCompletedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishPipelineStepFailedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishPipelineFinishedEventActivity)
	w.worker.RegisterActivity(w.eventActivities.PublishPipelineFailedEventActivity)

	// Register AI Event activities - for raw event processing on orchestrator
	// These handle save/parse/update of AI activity events from the agent
	w.worker.RegisterActivity(w.aiEventActivities.SaveRawEventActivity)
	w.worker.RegisterActivity(w.aiEventActivities.ParseEventActivity)
	w.worker.RegisterActivity(w.aiEventActivities.UpdateParsedEventActivity)

	// Register Task File activities
	w.worker.RegisterActivity(w.taskFileActivities.WriteTaskFileActivity)

	// Register Pipeline Data activities
	w.worker.RegisterActivity(w.pipelineDataActivities.SavePipelineRunActivity)
	w.worker.RegisterActivity(w.pipelineDataActivities.SaveStepResultActivity)
	w.worker.RegisterActivity(w.pipelineDataActivities.SaveRunStepSnapshotsActivity)
	w.worker.RegisterActivity(w.pipelineDataActivities.GetPipelineRunActivity)
	w.worker.RegisterActivity(w.pipelineDataActivities.UpdatePipelineRunStatusActivity)
	w.worker.RegisterActivity(w.pipelineDataActivities.GetLatestPipelineRunActivity)
	w.worker.RegisterActivity(w.pipelineDataActivities.GetTokenTotalsActivity)

	// Register Step Documentation activities
	w.worker.RegisterActivity(w.stepDocActivities.GenerateStepDocumentationActivity)

	getLog().Info().Msg("All activities registered with worker")
}

// Stop stops the worker gracefully
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.worker != nil {
		getLog().Info().Msg("Stopping Temporal worker gracefully...")

		// Stop the worker and wait for it to finish current tasks
		w.worker.Stop()

		// Mark this worker instance as stopped to prevent reuse
		w.stopped = true

		// Clear the worker reference
		w.worker = nil

		// Give a moment for graceful shutdown to complete
		// This prevents database corruption during concurrent shutdowns
		time.Sleep(200 * time.Millisecond)

		getLog().Info().Msg("Temporal worker stopped")
	}
	return nil
}

// GetRegisteredActivities returns a list of registered activity names (for testing)
func (w *Worker) GetRegisteredActivities() []string {
	return []string{
		"CreateWorktreeActivity",
		"RemoveWorktreeActivity",
		"CommitChangesActivity",
		"GetWorktreeStatusActivity",
		"GitCommitActivity",
		"CaptureGitDiffActivity",
		"CreateTaskActivity",
		"DeleteTaskActivity",
		"UpdateTaskStatusActivity",
		"UpdateTaskGitDiffActivity",
		"LoadProjectsActivity",
		"LoadTasksActivity",
		"CreateContainerActivity",
		"StopContainerActivity",
		"RemoveContainerActivity",
		"GetContainerStatusActivity",
		"CopyClaudeConfigActivity",
		"CopyClaudeCredentialsActivity",
		"PublishTaskCreatedEventActivity",
		"PublishTaskDeletedEventActivity",
		"PublishTaskStatusUpdatedEventActivity",
		"PublishTaskInProgressEventActivity",
		"PublishTaskFinishedEventActivity",
		"PublishTaskRequestedEventActivity",
		"PublishErrorEventActivity",
		"PublishAIActivityEventActivity",
		"SaveRawEventActivity",
		"ParseEventActivity",
		"UpdateParsedEventActivity",
		"WriteTaskFileActivity",
		"SavePipelineRunActivity",
		"SaveStepResultActivity",
		"SaveRunStepSnapshotsActivity",
		"GetPipelineRunActivity",
		"UpdatePipelineRunStatusActivity",
		"GetLatestPipelineRunActivity",
		"GetTokenTotalsActivity",
		"GenerateStepDocumentationActivity",
	}
}

// GetRegisteredWorkflows returns a list of registered workflow names (for testing)
func (w *Worker) GetRegisteredWorkflows() []string {
	return []string{
		workflows.PipelineWorkflowName,
		workflows.SetupWorkflowName,
		workflows.ProcessingStepWorkflowName,
	}
}

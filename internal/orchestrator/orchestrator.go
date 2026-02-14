// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/workers"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/pkg/containers/service"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// TemporalClient is a type alias so that existing test code (mocks_test.go) keeps compiling.
type TemporalClient = services.TemporalClient

var (
	log     *zerolog.Logger
	logOnce sync.Once
)

func getLog() *zerolog.Logger {
	logOnce.Do(func() {
		l := logger.GetOrchestratorLogger()
		log = &l
	})
	return log
}

// Orchestrator handles business logic and data management
type Orchestrator struct {
	cmdChan           <-chan protocol.Command
	eventChan         chan<- protocol.Event
	internalEventChan chan protocol.Event
	dataService       *services.DataService
	containerService  *service.Service
	gitServiceManager *services.GitServiceManager
	temporalClient    TemporalClient
	temporalWorker    *workers.Worker
	pipelineService   *services.PipelineService
	config            *config.AppConfig
}

// New creates a new orchestrator instance
func New(cmdChan <-chan protocol.Command, eventChan chan<- protocol.Event, cfg *config.AppConfig) (*Orchestrator, error) {
	internalEventChan := make(chan protocol.Event, 100)

	dataService, err := services.NewDataService(cfg)
	if err != nil {
		return nil, err
	}

	containerService, err := service.NewServiceWithDockerHost(nil, cfg.Container.DockerHost)
	if err != nil {
		return nil, fmt.Errorf("failed to create container service: %w", err)
	}

	gitServiceManager := services.NewGitServiceManager(cfg)

	temporalClient, err := temporal.NewClient(
		cfg.Temporal.HostPort,
		cfg.Temporal.Namespace,
		cfg.Temporal.TaskQueue,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporal client: %w", err)
	}

	temporalWorker := workers.NewWorker(
		temporalClient.GetTemporalClient(),
		cfg,
		gitServiceManager,
		dataService,
		containerService,
		eventChan,
	)

	if err := temporalWorker.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start temporal worker: %w", err)
	}

	pipelineService := services.NewPipelineService(dataService, gitServiceManager, temporalClient, cfg)

	return &Orchestrator{
		cmdChan:           cmdChan,
		eventChan:         eventChan,
		internalEventChan: internalEventChan,
		dataService:       dataService,
		containerService:  containerService,
		gitServiceManager: gitServiceManager,
		temporalClient:    temporalClient,
		temporalWorker:    temporalWorker,
		pipelineService:   pipelineService,
		config:            cfg,
	}, nil
}

// PipelineService returns the pipeline service for direct access (e.g. by the API server).
func (o *Orchestrator) PipelineService() *services.PipelineService {
	return o.pipelineService
}

// GetContainerService returns the container service for testing purposes
func (o *Orchestrator) GetContainerService() *service.Service {
	return o.containerService
}

// DataService returns the data service for direct read access (e.g. by the API server).
func (o *Orchestrator) DataService() *services.DataService {
	return o.dataService
}

// GitServiceManager returns the git service manager for direct read access (e.g. by the API server).
func (o *Orchestrator) GitServiceManager() *services.GitServiceManager {
	return o.gitServiceManager
}

// Run starts the orchestrator's main loop
func (o *Orchestrator) Run(ctx context.Context) {
	getLog().Info().Msg("Orchestrator started")
	for {
		select {
		case <-ctx.Done():
			getLog().Info().Err(ctx.Err()).Msg("Orchestrator shutting down")
			return
		case cmd, ok := <-o.cmdChan:
			if !ok {
				getLog().Info().Msg("Command channel closed")
				return
			}
			getLog().Debug().Str("command_type", fmt.Sprintf("%T", cmd)).Msg("Processing command")
			o.handleCommand(ctx, cmd)
		case event, ok := <-o.internalEventChan:
			if !ok {
				getLog().Info().Msg("Internal event channel closed")
				return
			}
			getLog().Debug().Str("event_type", fmt.Sprintf("%T", event)).Msg("Processing internal event")
			select {
			case o.eventChan <- event:
			default:
				getLog().Warn().Str("event_type", fmt.Sprintf("%T", event)).Msg("Failed to forward internal event")
			}
		}
	}
}

// handleCommand processes commands with context support and timeout
func (o *Orchestrator) handleCommand(ctx context.Context, cmd protocol.Command) {
	switch c := cmd.(type) {
	case protocol.LoadProjectsCommand:
		o.handleLoadProjects(ctx, c.Metadata)
	case protocol.LoadTasksCommand:
		o.handleLoadTasks(ctx, c.Metadata, c.ProjectID)
	case protocol.LoadCommitsCommand:
		o.handleLoadCommits(ctx, c.Metadata, c.ProjectID, c.Limit)
	case protocol.ToggleTaskCommand:
		o.handleToggleTask(ctx, c.Metadata, c.ProjectID, c.TaskID)
	case protocol.DeleteTaskCommand:
		o.handleDeleteTask(ctx, c.Metadata, c.ProjectID, c.TaskID)
	case protocol.CreateTaskCommand:
		go o.handleCreateTask(ctx, c)
	case protocol.CreateProjectCommand:
		o.handleCreateProject(ctx, c.Metadata, c.Name, c.Description, c.RepositoryPath)
	case protocol.LoadAIActivityCommand:
		o.handleLoadAIActivity(ctx, c.Metadata, c.ProjectID, c.TaskID)
	case protocol.StartPipelineCommand:
		go o.handleStartPipeline(ctx, c)
	case protocol.LoadPipelineRunsCommand:
		o.handleLoadPipelineRuns(ctx, c.Metadata, c.ProjectID)
	case protocol.CancelPipelineCommand:
		go o.handleCancelPipeline(c)
	default:
		getLog().Warn().Str("command_type", fmt.Sprintf("%T", cmd)).Msg("Unknown command type")
	}
}

// --- Read-only handlers (stay in orchestrator — just DataService + event emit) ---

func (o *Orchestrator) handleLoadProjects(ctx context.Context, metadata protocol.Metadata) {
	projects, err := o.dataService.LoadProjects(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load projects", Context: err.Error()}
		return
	}
	o.eventChan <- protocol.ProjectsLoadedEvent{Metadata: metadata, Projects: projects}
}

func (o *Orchestrator) handleLoadTasks(ctx context.Context, metadata protocol.Metadata, projectID string) {
	project, err := o.dataService.GetProject(ctx, projectID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load project details for " + projectID, Context: err.Error()}
		return
	}

	tasks, err := o.dataService.LoadTasks(ctx, projectID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load tasks for project " + projectID, Context: err.Error()}
		return
	}
	o.eventChan <- protocol.TasksLoadedEvent{
		Metadata:       metadata,
		ProjectID:      projectID,
		ProjectName:    project.Name,
		RepositoryPath: project.RepositoryPath,
		Tasks:          tasks,
	}
}

func (o *Orchestrator) handleLoadPipelineRuns(ctx context.Context, metadata protocol.Metadata, projectID string) {
	project, err := o.dataService.GetProject(ctx, projectID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load project details for " + projectID, Context: err.Error()}
		return
	}

	runs, err := o.dataService.GetPipelineRunsByProject(ctx, projectID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load pipeline runs for project " + projectID, Context: err.Error()}
		return
	}

	runsMap := make(map[string]*models.PipelineRun, len(runs))
	for _, run := range runs {
		runsMap[run.ID] = run
	}

	o.eventChan <- protocol.PipelineRunsLoadedEvent{
		Metadata:       metadata,
		ProjectID:      projectID,
		ProjectName:    project.Name,
		RepositoryPath: project.RepositoryPath,
		Runs:           runsMap,
	}
}

func (o *Orchestrator) handleLoadAIActivity(ctx context.Context, metadata protocol.Metadata, projectID, taskID string) {
	events, err := o.dataService.GetAIActivityByTask(ctx, taskID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load AI activity events for task " + taskID, Context: err.Error()}
		return
	}
	o.eventChan <- protocol.AIActivityBatchEvent{Metadata: metadata, TaskID: taskID, ProjectID: projectID, Activities: events}
}

func (o *Orchestrator) handleLoadCommits(ctx context.Context, metadata protocol.Metadata, projectID string, limit int) {
	project, err := o.dataService.GetProject(ctx, projectID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load project details for " + projectID, Context: err.Error()}
		return
	}

	if project.RepositoryPath == "" {
		o.eventChan <- protocol.CommitsLoadedEvent{Metadata: metadata, ProjectID: projectID, RepositoryPath: "", Commits: []protocol.CommitInfo{}}
		return
	}

	gitServiceHandle, err := o.gitServiceManager.GetService(project.RepositoryPath)
	if err != nil {
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to access git repository", Context: err.Error()}
		return
	}
	defer gitServiceHandle.Release()

	commits, err := gitServiceHandle.GetGitService().GetCommitHistory(ctx, project.RepositoryPath, limit)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to load commit history", Context: err.Error()}
		return
	}

	commitInfos := make([]protocol.CommitInfo, len(commits))
	for i, commit := range commits {
		commitInfos[i] = protocol.CommitInfo{Hash: commit.Hash, Message: commit.Message, Author: commit.Author, Parents: commit.Parents}
	}

	o.eventChan <- protocol.CommitsLoadedEvent{Metadata: metadata, ProjectID: projectID, RepositoryPath: project.RepositoryPath, Commits: commitInfos}
}

// --- Mutation handlers (thin wrappers around PipelineService) ---

func (o *Orchestrator) handleCreateProject(ctx context.Context, metadata protocol.Metadata, name, description, repositoryPath string) {
	project, err := o.pipelineService.CreateProject(ctx, name, description, repositoryPath)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to create project", Context: err.Error()}
		return
	}
	o.eventChan <- protocol.ProjectCreatedEvent{Metadata: metadata, Project: project}
}

func (o *Orchestrator) handleToggleTask(ctx context.Context, metadata protocol.Metadata, projectID, taskID string) {
	newStatus, err := o.pipelineService.ToggleTask(ctx, projectID, taskID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to update task status", Context: err.Error()}
		return
	}
	event := protocol.NewTaskStatusUpdatedEvent(projectID, taskID, newStatus)
	event.Metadata = metadata
	o.eventChan <- event
}

func (o *Orchestrator) handleDeleteTask(ctx context.Context, metadata protocol.Metadata, projectID, taskID string) {
	if err := o.pipelineService.DeleteTask(ctx, projectID, taskID); err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: metadata, Message: "Failed to delete task", Context: err.Error()}
		return
	}
	// Reload tasks to reflect the deletion
	o.handleLoadTasks(ctx, metadata, projectID)
}

func (o *Orchestrator) handleCreateTask(ctx context.Context, cmd protocol.CreateTaskCommand) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := o.pipelineService.CreateTask(ctx, services.CreateTaskParams{
		ProjectID:     cmd.ProjectID,
		Title:         cmd.Title,
		Description:   cmd.Description,
		BaseCommitSHA: cmd.BaseCommitSHA,
		AgentConfig:   cmd.AgentConfig,
	})
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: cmd.Metadata, Message: "Failed to create task", Context: err.Error()}
		return
	}
	o.eventChan <- protocol.PipelineRunStartedEvent{
		Metadata:      cmd.Metadata,
		RunID:         result.RunID,
		ProjectID:     result.ProjectID,
		Name:          result.Name,
		WorkflowID:    result.WorkflowID,
		AlreadyExists: result.AlreadyExists,
		Status:        protocol.PipelineStatus(result.Status),
	}
}

func (o *Orchestrator) handleStartPipeline(ctx context.Context, cmd protocol.StartPipelineCommand) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := o.pipelineService.StartPipeline(ctx, services.StartPipelineParams{
		ProjectID:       cmd.ProjectID,
		Name:            cmd.Name,
		Steps:           cmd.Steps,
		BaseCommitSHA:   cmd.BaseCommitSHA,
		ForkFromRunID:   cmd.ForkFromRunID,
		ForkAfterStepID: cmd.ForkAfterStepID,
		NoAutoFork:      cmd.NoAutoFork,
	})
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		o.eventChan <- protocol.ErrorEvent{Metadata: cmd.Metadata, Message: "Failed to start pipeline", Context: err.Error()}
		return
	}
	o.eventChan <- protocol.PipelineRunStartedEvent{
		Metadata:        cmd.Metadata,
		RunID:           result.RunID,
		ProjectID:       result.ProjectID,
		Name:            result.Name,
		WorkflowID:      result.WorkflowID,
		AlreadyExists:   result.AlreadyExists,
		Status:          protocol.PipelineStatus(result.Status),
		ForkFromRunID:   result.ForkFromRunID,
		ForkAfterStepID: result.ForkAfterStepID,
		SkippedSteps:    result.SkippedSteps,
	}
}

func (o *Orchestrator) handleCancelPipeline(cmd protocol.CancelPipelineCommand) {
	result, err := o.pipelineService.CancelPipeline(context.Background(), cmd.RunID, cmd.Reason)
	if err != nil {
		o.eventChan <- protocol.ErrorEvent{Metadata: cmd.Metadata, Message: "Failed to cancel pipeline", Context: err.Error()}
		return
	}
	o.eventChan <- protocol.PipelineCancelledEvent{
		Metadata:       cmd.Metadata,
		RunID:          result.RunID,
		Reason:         result.Reason,
		WorkflowStatus: result.WorkflowStatus,
	}
}

// --- Lifecycle & accessors ---

// Close closes the orchestrator and cleans up resources
func (o *Orchestrator) Close() error {
	getLog().Info().Msg("Shutting down orchestrator...")
	var errs []error

	if o.temporalWorker != nil {
		if closeErr := o.temporalWorker.Stop(); closeErr != nil {
			getLog().Error().Err(closeErr).Msg("Error stopping temporal worker")
			errs = append(errs, closeErr)
		}
	}

	if closeErr := o.temporalClient.Close(); closeErr != nil {
		getLog().Error().Err(closeErr).Msg("Error closing temporal client")
		errs = append(errs, closeErr)
	}

	if closeErr := o.dataService.Close(); closeErr != nil {
		getLog().Error().Err(closeErr).Msg("Error closing data service")
		errs = append(errs, closeErr)
	}
	if closeErr := o.containerService.Close(); closeErr != nil {
		getLog().Error().Err(closeErr).Msg("Error closing container service")
		errs = append(errs, closeErr)
	}
	if closeErr := o.gitServiceManager.Close(); closeErr != nil {
		getLog().Error().Err(closeErr).Msg("Error closing git service manager")
		errs = append(errs, closeErr)
	}

	close(o.internalEventChan)
	getLog().Info().Msg("Orchestrator shutdown complete")
	return errors.Join(errs...)
}

// GetProjectRepositoryPath resolves the repository path for a project
func (o *Orchestrator) GetProjectRepositoryPath(ctx context.Context, projectID string) (string, error) {
	return o.dataService.GetProjectRepositoryPath(ctx, projectID)
}

// GetProjectGitServiceHandle gets a thread-safe git service handle for a specific project
func (o *Orchestrator) GetProjectGitServiceHandle(ctx context.Context, projectID string) (*services.GitServiceHandle, error) {
	repoPath, err := o.GetProjectRepositoryPath(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository path for project %s: %w", projectID, err)
	}
	return o.gitServiceManager.GetService(repoPath)
}

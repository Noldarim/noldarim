// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"fmt"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator/database"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	dataLog     *zerolog.Logger
	dataLogOnce sync.Once
)

func getDataLog() *zerolog.Logger {
	dataLogOnce.Do(func() {
		l := logger.GetDatabaseLogger().With().Str("component", "service").Logger()
		dataLog = &l
	})
	return dataLog
}

// DataService handles loading and managing data from various sources
type DataService struct {
	db *database.GormDB
}

// NewDataService creates a new data service
func NewDataService(cfg *config.AppConfig) (*DataService, error) {
	getDataLog().Debug().Msg("Initializing data service")

	// Initialize GORM database
	db, err := database.NewGormDB(&cfg.Database)
	if err != nil {
		getDataLog().Error().Err(err).Msg("Failed to initialize database")
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	// Validate schema to ensure models match database
	if err := db.ValidateSchema(); err != nil {
		getDataLog().Error().Err(err).Msg("Database schema validation failed")
		return nil, fmt.Errorf("database schema validation failed: %w", err)
	}

	getDataLog().Info().Msg("Data service initialized successfully")
	return &DataService{
		db: db,
	}, nil
}

// LoadProjects loads all projects from the database
func (ds *DataService) LoadProjects(ctx context.Context) (map[string]*models.Project, error) {
	return ds.db.GetAllProjects(ctx)
}

// GetProject gets a project by ID from the database
func (ds *DataService) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	return ds.db.GetProject(ctx, projectID)
}

// CreateProject creates a new project in the database
func (ds *DataService) CreateProject(ctx context.Context, name, description, repositoryPath string) (*models.Project, error) {
	// Generate new project ID using timestamp
	projectID := fmt.Sprintf("project-%d", time.Now().UnixNano())

	// Create GORM project model
	dbProject := &models.Project{
		ID:             projectID,
		Name:           name,
		Description:    description,
		RepositoryPath: repositoryPath,
		AgentID:        "", // Placeholder - will be set when agent assignment is implemented
	}

	if err := ds.db.CreateProject(ctx, dbProject); err != nil {
		return nil, err
	}

	return dbProject, nil
}

// LoadTasks loads tasks for a specific project from the database
func (ds *DataService) LoadTasks(ctx context.Context, projectID string) (map[string]*models.Task, error) {
	return ds.db.GetTasksByProject(ctx, projectID)
}

// UpdateTaskStatus updates a task's status in the database
func (ds *DataService) UpdateTaskStatus(ctx context.Context, taskID string, newStatus models.TaskStatus) error {
	return ds.db.UpdateTaskStatus(ctx, taskID, newStatus)
}

// UpdateTaskGitDiff updates a task's git diff in the database
func (ds *DataService) UpdateTaskGitDiff(ctx context.Context, taskID, gitDiff string) error {
	return ds.db.UpdateTaskGitDiff(ctx, taskID, gitDiff)
}

// DeleteTask deletes a task from the database
func (ds *DataService) DeleteTask(ctx context.Context, taskID string) error {
	return ds.db.DeleteTask(ctx, taskID)
}

// CreateTask creates a new task in the database
func (ds *DataService) CreateTask(ctx context.Context, projectID, taskID, title, description, taskFilePath string) (*models.Task, error) {
	// Validate inputs
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	if strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("project ID cannot be empty")
	}

	// Verify project exists
	project, err := ds.db.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify project exists: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project with ID '%s' does not exist", projectID)
	}

	// Create GORM task model using provided task ID
	dbTask := &models.Task{
		ID:           taskID,
		Title:        title,
		Description:  description,
		Status:       models.TaskStatusPending,
		ProjectID:    projectID,
		ExecHistory:  models.ExecHistory{},
		AgentID:      "",
		BranchName:   "",
		TaskFilePath: taskFilePath,
	}

	if err := ds.db.CreateTask(ctx, dbTask); err != nil {
		return nil, err
	}

	return dbTask, nil
}

// UpdateTask updates task details in the database
func (ds *DataService) UpdateTask(ctx context.Context, projectID, taskID, title, description string) (*models.Task, error) {
	if err := ds.db.UpdateTask(ctx, taskID, title, description); err != nil {
		return nil, err
	}

	// Fetch the updated task to return current state
	tasks, err := ds.db.GetTasksByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	task, exists := tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found after update", taskID)
	}

	return task, nil
}

// UpdateProject updates project details in the database
func (ds *DataService) UpdateProject(ctx context.Context, projectID, name, description string) (*models.Project, error) {
	if err := ds.db.UpdateProject(ctx, projectID, name, description); err != nil {
		return nil, err
	}

	// Fetch the updated project to return current state
	project, err := ds.db.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project %s not found after update: %w", projectID, err)
	}

	return project, nil
}

// GetProjectRepositoryPath gets the repository path for a project
func (ds *DataService) GetProjectRepositoryPath(ctx context.Context, projectID string) (string, error) {
	project, err := ds.db.GetProject(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("project %s not found: %w", projectID, err)
	}

	if project.RepositoryPath == "" {
		return "", fmt.Errorf("no repository path configured for project %s", projectID)
	}

	return project.RepositoryPath, nil
}

// DeleteProject deletes a project from the database
func (ds *DataService) DeleteProject(ctx context.Context, projectID string) error {
	return ds.db.DeleteProject(ctx, projectID)
}

// FindTaskByProjectAndTitle finds a task by project ID and title
func (ds *DataService) FindTaskByProjectAndTitle(ctx context.Context, projectID, title string) (*models.Task, error) {
	return ds.db.FindTaskByProjectAndTitle(ctx, projectID, title)
}

// GetTask gets a task by ID
func (ds *DataService) GetTask(ctx context.Context, taskID string) (*models.Task, error) {
	return ds.db.GetTask(ctx, taskID)
}

// GetLatestTask gets the most recently created task across all projects
func (ds *DataService) GetLatestTask(ctx context.Context) (*models.Task, error) {
	return ds.db.GetLatestTask(ctx)
}

// Close closes the database connection
func (ds *DataService) Close() error {
	return ds.db.Close()
}

// SaveAIActivityRecord saves an AI activity record to the database.
func (ds *DataService) SaveAIActivityRecord(ctx context.Context, record *models.AIActivityRecord) error {
	return ds.db.SaveAIActivityRecord(ctx, record)
}

// UpdateAIActivityRecord updates an existing AI activity record with parsed data
func (ds *DataService) UpdateAIActivityRecord(ctx context.Context, record *models.AIActivityRecord) error {
	return ds.db.UpdateAIActivityRecord(ctx, record)
}

// GetAIActivityByTask retrieves all AI activity records for a task
func (ds *DataService) GetAIActivityByTask(ctx context.Context, taskID string) ([]*models.AIActivityRecord, error) {
	return ds.db.GetAIActivityByTask(ctx, taskID)
}

// GetAIActivityByRunID retrieves all AI activity records for a pipeline run (all steps)
func (ds *DataService) GetAIActivityByRunID(ctx context.Context, runID string) ([]*models.AIActivityRecord, error) {
	return ds.db.GetAIActivityByRunID(ctx, runID)
}

// DeleteAIActivityByTask deletes all AI activity events for a task
func (ds *DataService) DeleteAIActivityByTask(ctx context.Context, taskID string) error {
	return ds.db.DeleteAIActivityByTask(ctx, taskID)
}

// GetAIActivityByEventType retrieves AI activity records filtered by event type.
// If limit is 0, returns all matching records.
func (ds *DataService) GetAIActivityByEventType(ctx context.Context, eventType string, limit int) ([]*models.AIActivityRecord, error) {
	return ds.db.GetAIActivityByEventType(ctx, eventType, limit)
}

// GetTokenTotalsByTask aggregates token counts from AI activity records for a task
func (ds *DataService) GetTokenTotalsByTask(ctx context.Context, taskID string) (*database.TokenTotals, error) {
	return ds.db.GetTokenTotalsByTask(ctx, taskID)
}

// ============================================================================
// Pipeline Operations
// ============================================================================

// CreatePipeline creates a new pipeline definition
func (ds *DataService) CreatePipeline(ctx context.Context, pipeline *models.Pipeline) error {
	return ds.db.CreatePipeline(ctx, pipeline)
}

// GetPipeline retrieves a pipeline by ID
func (ds *DataService) GetPipeline(ctx context.Context, pipelineID string) (*models.Pipeline, error) {
	return ds.db.GetPipeline(ctx, pipelineID)
}

// GetPipelinesByProject retrieves all pipelines for a project
func (ds *DataService) GetPipelinesByProject(ctx context.Context, projectID string) ([]*models.Pipeline, error) {
	return ds.db.GetPipelinesByProject(ctx, projectID)
}

// UpdatePipeline updates a pipeline's details
func (ds *DataService) UpdatePipeline(ctx context.Context, pipeline *models.Pipeline) error {
	return ds.db.UpdatePipeline(ctx, pipeline)
}

// DeletePipeline deletes a pipeline
func (ds *DataService) DeletePipeline(ctx context.Context, pipelineID string) error {
	return ds.db.DeletePipeline(ctx, pipelineID)
}

// ============================================================================
// PipelineRun Operations
// ============================================================================

// CreatePipelineRun creates a new pipeline run
func (ds *DataService) CreatePipelineRun(ctx context.Context, run *models.PipelineRun) error {
	return ds.db.CreatePipelineRun(ctx, run)
}

// GetPipelineRun retrieves a pipeline run by ID with step results
func (ds *DataService) GetPipelineRun(ctx context.Context, runID string) (*models.PipelineRun, error) {
	return ds.db.GetPipelineRun(ctx, runID)
}

// GetPipelineRunsByProject retrieves all pipeline runs for a project
func (ds *DataService) GetPipelineRunsByProject(ctx context.Context, projectID string) ([]*models.PipelineRun, error) {
	return ds.db.GetPipelineRunsByProject(ctx, projectID)
}

// GetPipelineRunsByPipeline retrieves all runs for a specific pipeline
func (ds *DataService) GetPipelineRunsByPipeline(ctx context.Context, pipelineID string) ([]*models.PipelineRun, error) {
	return ds.db.GetPipelineRunsByPipeline(ctx, pipelineID)
}

// GetLatestPipelineRun gets the most recently created pipeline run
func (ds *DataService) GetLatestPipelineRun(ctx context.Context) (*models.PipelineRun, error) {
	return ds.db.GetLatestPipelineRun(ctx)
}

// UpdatePipelineRunStatus updates a pipeline run's status and optional error message
func (ds *DataService) UpdatePipelineRunStatus(ctx context.Context, runID string, status models.PipelineRunStatus, errorMessage string) error {
	return ds.db.UpdatePipelineRunStatus(ctx, runID, status, errorMessage)
}

// UpdatePipelineRun updates a pipeline run
func (ds *DataService) UpdatePipelineRun(ctx context.Context, run *models.PipelineRun) error {
	return ds.db.UpdatePipelineRun(ctx, run)
}

// DeletePipelineRun deletes a pipeline run and its step results
func (ds *DataService) DeletePipelineRun(ctx context.Context, runID string) error {
	return ds.db.DeletePipelineRun(ctx, runID)
}

// ============================================================================
// StepResult Operations
// ============================================================================

// CreateStepResult creates a new step result
func (ds *DataService) CreateStepResult(ctx context.Context, result *models.StepResult) error {
	return ds.db.CreateStepResult(ctx, result)
}

// GetStepResult retrieves a step result by ID
func (ds *DataService) GetStepResult(ctx context.Context, resultID string) (*models.StepResult, error) {
	return ds.db.GetStepResult(ctx, resultID)
}

// GetStepResultsByRun retrieves all step results for a pipeline run
func (ds *DataService) GetStepResultsByRun(ctx context.Context, runID string) ([]*models.StepResult, error) {
	return ds.db.GetStepResultsByRun(ctx, runID)
}

// UpdateStepResult updates a step result
func (ds *DataService) UpdateStepResult(ctx context.Context, result *models.StepResult) error {
	return ds.db.UpdateStepResult(ctx, result)
}

// UpdateStepResultStatus updates a step result's status
func (ds *DataService) UpdateStepResultStatus(ctx context.Context, resultID string, status models.StepStatus) error {
	return ds.db.UpdateStepResultStatus(ctx, resultID, status)
}

// SaveRunStepSnapshots persists executed step configuration snapshots for a run.
func (ds *DataService) SaveRunStepSnapshots(ctx context.Context, snapshots []models.RunStepSnapshot) error {
	return ds.db.SaveRunStepSnapshots(ctx, snapshots)
}

// GetRecentSuccessfulRunsWithSteps retrieves recent successful pipeline runs for a project
// that started from the same base commit, with step results pre-loaded.
// Used for auto-fork detection to find candidate runs for forking.
func (ds *DataService) GetRecentSuccessfulRunsWithSteps(ctx context.Context, projectID string, baseCommitSHA string, maxRuns int) ([]*models.PipelineRun, error) {
	return ds.db.GetRecentSuccessfulRunsWithSteps(ctx, projectID, baseCommitSHA, maxRuns)
}

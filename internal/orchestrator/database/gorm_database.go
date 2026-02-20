// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"fmt"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// GormDB wraps the GORM database connection
type GormDB struct {
	db *gorm.DB
}

// NewGormDB creates a new GORM database connection
func NewGormDB(cfg *config.DatabaseConfig) (*GormDB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "sqlite":
		dialector = sqlite.Open(cfg.GetDSN())
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Reduce GORM log noise
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &GormDB{db: db}, nil
}

// AutoMigrate runs database migrations
func (db *GormDB) AutoMigrate() error {
	if err := db.db.AutoMigrate(
		&models.Project{},
		&models.Task{},
		&models.AIActivityRecord{},
		&models.Pipeline{},
		&models.PipelineRun{},
		&models.StepResult{},
		&models.RunStepSnapshot{},
	); err != nil {
		return err
	}

	// Migration path for existing databases: enforce deterministic snapshot ordering.
	if !db.db.Migrator().HasIndex(&models.RunStepSnapshot{}, "idx_run_step_snapshots_run_step_index") {
		if err := db.db.Migrator().CreateIndex(&models.RunStepSnapshot{}, "idx_run_step_snapshots_run_step_index"); err != nil {
			return fmt.Errorf("failed to create run_step_snapshots order index (run_id, step_index): %w", err)
		}
	}

	return nil
}

// ValidateSchema checks if GORM models match the database schema
func (db *GormDB) ValidateSchema() error {
	var missingTables []string
	var missingColumns []string
	var missingIndexes []string

	// Check if tables exist and have the correct structure
	if !db.db.Migrator().HasTable(&models.Project{}) {
		missingTables = append(missingTables, "projects")
	}

	if !db.db.Migrator().HasTable(&models.Task{}) {
		missingTables = append(missingTables, "tasks")
	}

	if !db.db.Migrator().HasTable(&models.AIActivityRecord{}) {
		missingTables = append(missingTables, "ai_activity_records")
	}

	if !db.db.Migrator().HasTable(&models.RunStepSnapshot{}) {
		missingTables = append(missingTables, "run_step_snapshots")
	}

	if len(missingTables) > 0 {
		return fmt.Errorf("missing tables: %v\n\n💡 Run 'make migrate' to create the required tables", missingTables)
	}

	// Check for required columns in projects table
	projectColumns := []string{"id", "name", "description", "last_updated_at", "agent_id", "created_at"}
	for _, col := range projectColumns {
		if !db.db.Migrator().HasColumn(&models.Project{}, col) {
			missingColumns = append(missingColumns, fmt.Sprintf("projects.%s", col))
		}
	}

	// Check for required columns in tasks table
	taskColumns := []string{
		"id", "title", "description", "status", "project_id", "exec_history",
		"last_updated_at", "agent_id", "created_at", "task_file_path", "branch_name",
	}
	for _, col := range taskColumns {
		if !db.db.Migrator().HasColumn(&models.Task{}, col) {
			missingColumns = append(missingColumns, fmt.Sprintf("tasks.%s", col))
		}
	}

	// Check for required columns in ai_activity_records table
	// run_id and step_id are required for real-time pipeline activity streaming.
	aiActivityColumns := []string{
		"event_id", "task_id", "run_id", "step_id", "event_type", "timestamp", "raw_payload",
	}
	for _, col := range aiActivityColumns {
		if !db.db.Migrator().HasColumn(&models.AIActivityRecord{}, col) {
			missingColumns = append(missingColumns, fmt.Sprintf("ai_activity_records.%s", col))
		}
	}

	// Check for required columns in run_step_snapshots table.
	runStepSnapshotColumns := []string{
		"run_id", "step_id", "step_index", "step_name", "agent_config_json", "definition_hash",
	}
	for _, col := range runStepSnapshotColumns {
		if !db.db.Migrator().HasColumn(&models.RunStepSnapshot{}, col) {
			missingColumns = append(missingColumns, fmt.Sprintf("run_step_snapshots.%s", col))
		}
	}

	if !db.db.Migrator().HasIndex(&models.RunStepSnapshot{}, "idx_run_step_snapshots_run_step_index") {
		missingIndexes = append(missingIndexes, "run_step_snapshots.idx_run_step_snapshots_run_step_index")
	}

	if len(missingColumns) > 0 {
		return fmt.Errorf("missing columns: %v\n\n💡 Run 'make migrate' to add the required columns", missingColumns)
	}

	if len(missingIndexes) > 0 {
		return fmt.Errorf("missing indexes: %v\n\n💡 Run 'make migrate' to add the required indexes", missingIndexes)
	}

	return nil
}

// Close closes the database connection
func (db *GormDB) Close() error {
	sqlDB, err := db.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetAllProjects retrieves all projects from the database
func (db *GormDB) GetAllProjects(ctx context.Context) (map[string]*models.Project, error) {
	var projects []models.Project

	err := db.db.WithContext(ctx).
		Order("last_updated_at DESC").
		Find(&projects).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]*models.Project)
	for _, project := range projects {
		result[project.ID] = &project
	}

	return result, nil
}

// GetTasksByProject retrieves all tasks for a specific project
func (db *GormDB) GetTasksByProject(ctx context.Context, projectID string) (map[string]*models.Task, error) {
	var tasks []models.Task

	err := db.db.WithContext(ctx).Where("project_id = ?", projectID).
		Order("last_updated_at DESC").
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]*models.Task)
	for _, task := range tasks {
		result[task.ID] = &task
	}

	return result, nil
}

// CreateProject creates a new project
func (db *GormDB) CreateProject(ctx context.Context, project *models.Project) error {
	return db.db.WithContext(ctx).Create(project).Error
}

// UpdateProject updates project details
func (db *GormDB) UpdateProject(ctx context.Context, projectID, name, description string) error {
	return db.db.WithContext(ctx).Model(&models.Project{}).
		Where("id = ?", projectID).
		Updates(map[string]any{
			"name":        name,
			"description": description,
		}).Error
}

// DeleteProject deletes a project
func (db *GormDB) DeleteProject(ctx context.Context, projectID string) error {
	return db.db.WithContext(ctx).Delete(&models.Project{}, "id = ?", projectID).Error
}

// CreateTask creates a new task
func (db *GormDB) CreateTask(ctx context.Context, task *models.Task) error {
	return db.db.WithContext(ctx).Create(task).Error
}

// UpdateTaskStatus updates a task's status
func (db *GormDB) UpdateTaskStatus(ctx context.Context, taskID string, status models.TaskStatus) error {
	return db.db.WithContext(ctx).Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("status", status).Error
}

// UpdateTask updates task details
func (db *GormDB) UpdateTask(ctx context.Context, taskID, title, description string) error {
	return db.db.WithContext(ctx).Model(&models.Task{}).
		Where("id = ?", taskID).
		Updates(map[string]any{
			"title":       title,
			"description": description,
		}).Error
}

// UpdateTaskGitDiff updates a task's git diff field
func (db *GormDB) UpdateTaskGitDiff(ctx context.Context, taskID, gitDiff string) error {
	return db.db.WithContext(ctx).Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("git_diff", gitDiff).Error
}

// DeleteTask deletes a task
func (db *GormDB) DeleteTask(ctx context.Context, taskID string) error {
	return db.db.WithContext(ctx).Delete(&models.Task{}, "id = ?", taskID).Error
}

// GetTask retrieves a single task by ID
func (db *GormDB) GetTask(ctx context.Context, taskID string) (*models.Task, error) {
	var task models.Task
	err := db.db.WithContext(ctx).First(&task, "id = ?", taskID).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetProject retrieves a single project by ID
func (db *GormDB) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	var project models.Project
	err := db.db.WithContext(ctx).First(&project, "id = ?", projectID).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// FindTaskByProjectAndTitle finds a task by project ID and title
func (db *GormDB) FindTaskByProjectAndTitle(ctx context.Context, projectID, title string) (*models.Task, error) {
	var task models.Task
	err := db.db.WithContext(ctx).Where("project_id = ? AND title = ?", projectID, title).First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Return nil, nil when not found for idempotency checks
		}
		return nil, err
	}
	return &task, nil
}

// GetLatestTask gets the most recently created task across all projects
func (db *GormDB) GetLatestTask(ctx context.Context) (*models.Task, error) {
	var task models.Task
	err := db.db.WithContext(ctx).Order("created_at DESC").First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no tasks found in database")
		}
		return nil, err
	}
	return &task, nil
}

// SaveAIActivityRecord saves an AI activity record to the database
func (db *GormDB) SaveAIActivityRecord(ctx context.Context, record *models.AIActivityRecord) error {
	// Use upsert to handle duplicate event IDs gracefully
	return db.db.WithContext(ctx).
		Where("event_id = ?", record.EventID).
		FirstOrCreate(record).Error
}

// UpdateAIActivityRecord updates an existing AI activity record with parsed data
func (db *GormDB) UpdateAIActivityRecord(ctx context.Context, record *models.AIActivityRecord) error {
	result := db.db.WithContext(ctx).
		Model(&models.AIActivityRecord{}).
		Where("event_id = ?", record.EventID).
		Updates(map[string]interface{}{
			// Identity
			"session_id": record.SessionID,
			// Conversation structure
			"message_uuid": record.MessageUUID,
			"parent_uuid":  record.ParentUUID,
			"request_id":   record.RequestID,
			// Classification
			"event_type":     record.EventType,
			"is_human_input": record.IsHumanInput,
			// Model info
			"model":       record.Model,
			"stop_reason": record.StopReason,
			// Token usage
			"input_tokens":        record.InputTokens,
			"output_tokens":       record.OutputTokens,
			"cache_read_tokens":   record.CacheReadTokens,
			"cache_create_tokens": record.CacheCreateTokens,
			// Context tracking
			"context_tokens": record.ContextTokens,
			"context_depth":  record.ContextDepth,
			// Tool info
			"tool_name":          record.ToolName,
			"tool_input_summary": record.ToolInputSummary,
			"tool_success":       record.ToolSuccess,
			"tool_error":         record.ToolError,
			"file_path":          record.FilePath,
			// Content
			"content_preview": record.ContentPreview,
			"content_length":  record.ContentLength,
			// Raw data (in case parsing enriches it)
			"raw_payload": record.RawPayload,
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no record found with event_id: %s", record.EventID)
	}
	return nil
}

// GetAIActivityByTask retrieves all AI activity records for a task, ordered by timestamp
func (db *GormDB) GetAIActivityByTask(ctx context.Context, taskID string) ([]*models.AIActivityRecord, error) {
	var records []*models.AIActivityRecord
	err := db.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetAIActivityByRunID retrieves all AI activity records for a pipeline run (all steps)
func (db *GormDB) GetAIActivityByRunID(ctx context.Context, runID string) ([]*models.AIActivityRecord, error) {
	var records []*models.AIActivityRecord
	err := db.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("created_at ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// GetAIActivityByTaskSince retrieves AI activity records for a task since a given event ID
func (db *GormDB) GetAIActivityByTaskSince(ctx context.Context, taskID string, sinceEventID string) ([]*models.AIActivityRecord, error) {
	var records []*models.AIActivityRecord

	// If no sinceEventID, return all
	if sinceEventID == "" {
		return db.GetAIActivityByTask(ctx, taskID)
	}

	// Find the record with sinceEventID to get its created_at
	var sinceRecord models.AIActivityRecord
	err := db.db.WithContext(ctx).
		Where("event_id = ?", sinceEventID).
		First(&sinceRecord).Error
	if err != nil {
		// If not found, return all records
		if err == gorm.ErrRecordNotFound {
			return db.GetAIActivityByTask(ctx, taskID)
		}
		return nil, err
	}

	// Get records after the sinceRecord
	err = db.db.WithContext(ctx).
		Where("task_id = ? AND created_at > ?", taskID, sinceRecord.CreatedAt).
		Order("created_at ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// DeleteAIActivityByTask deletes all AI activity records for a task
func (db *GormDB) DeleteAIActivityByTask(ctx context.Context, taskID string) error {
	return db.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Delete(&models.AIActivityRecord{}).Error
}

// GetAIActivityByEventType retrieves AI activity records filtered by event type.
// If limit is 0, returns all matching records.
func (db *GormDB) GetAIActivityByEventType(ctx context.Context, eventType string, limit int) ([]*models.AIActivityRecord, error) {
	var records []*models.AIActivityRecord
	query := db.db.WithContext(ctx).
		Where("event_type = ?", eventType).
		Where("raw_payload != ''").
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// TokenTotals represents aggregated token counts
type TokenTotals struct {
	InputTokens       int
	OutputTokens      int
	CacheReadTokens   int
	CacheCreateTokens int
}

// GetTokenTotalsByTask aggregates token counts from AI activity records for a task
func (db *GormDB) GetTokenTotalsByTask(ctx context.Context, taskID string) (*TokenTotals, error) {
	var result TokenTotals
	err := db.db.WithContext(ctx).
		Model(&models.AIActivityRecord{}).
		Where("task_id = ?", taskID).
		Select("COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(cache_read_tokens), 0) as cache_read_tokens, COALESCE(SUM(cache_create_tokens), 0) as cache_create_tokens").
		Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ============================================================================
// Pipeline Operations
// ============================================================================

// CreatePipeline creates a new pipeline definition
func (db *GormDB) CreatePipeline(ctx context.Context, pipeline *models.Pipeline) error {
	return db.db.WithContext(ctx).Create(pipeline).Error
}

// GetPipeline retrieves a pipeline by ID
func (db *GormDB) GetPipeline(ctx context.Context, pipelineID string) (*models.Pipeline, error) {
	var pipeline models.Pipeline
	err := db.db.WithContext(ctx).First(&pipeline, "id = ?", pipelineID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &pipeline, nil
}

// GetPipelinesByProject retrieves all pipelines for a project
func (db *GormDB) GetPipelinesByProject(ctx context.Context, projectID string) ([]*models.Pipeline, error) {
	var pipelines []*models.Pipeline
	err := db.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&pipelines).Error
	if err != nil {
		return nil, err
	}
	return pipelines, nil
}

// UpdatePipeline updates a pipeline's details
func (db *GormDB) UpdatePipeline(ctx context.Context, pipeline *models.Pipeline) error {
	return db.db.WithContext(ctx).Save(pipeline).Error
}

// DeletePipeline deletes a pipeline
func (db *GormDB) DeletePipeline(ctx context.Context, pipelineID string) error {
	return db.db.WithContext(ctx).Delete(&models.Pipeline{}, "id = ?", pipelineID).Error
}

// ============================================================================
// PipelineRun Operations
// ============================================================================

// CreatePipelineRun creates a new pipeline run
func (db *GormDB) CreatePipelineRun(ctx context.Context, run *models.PipelineRun) error {
	return db.db.WithContext(ctx).Create(run).Error
}

// GetPipelineRun retrieves a pipeline run by ID with step results
func (db *GormDB) GetPipelineRun(ctx context.Context, runID string) (*models.PipelineRun, error) {
	var run models.PipelineRun
	err := db.db.WithContext(ctx).
		Preload("StepResults", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_index ASC")
		}).
		Preload("StepSnapshots", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_index ASC")
		}).
		First(&run, "id = ?", runID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

// GetPipelineRunsByProject retrieves all pipeline runs for a project with step results
func (db *GormDB) GetPipelineRunsByProject(ctx context.Context, projectID string) ([]*models.PipelineRun, error) {
	var runs []*models.PipelineRun
	err := db.db.WithContext(ctx).
		Preload("StepResults", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_index ASC")
		}).
		Preload("StepSnapshots", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_index ASC")
		}).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&runs).Error
	if err != nil {
		return nil, err
	}
	return runs, nil
}

// GetPipelineRunsByPipeline retrieves all runs for a specific pipeline
func (db *GormDB) GetPipelineRunsByPipeline(ctx context.Context, pipelineID string) ([]*models.PipelineRun, error) {
	var runs []*models.PipelineRun
	err := db.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("created_at DESC").
		Find(&runs).Error
	if err != nil {
		return nil, err
	}
	return runs, nil
}

// GetLatestPipelineRun gets the most recently created pipeline run
func (db *GormDB) GetLatestPipelineRun(ctx context.Context) (*models.PipelineRun, error) {
	var run models.PipelineRun
	err := db.db.WithContext(ctx).
		Preload("StepResults", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_index ASC")
		}).
		Preload("StepSnapshots", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_index ASC")
		}).
		Order("created_at DESC").
		First(&run).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

// UpdatePipelineRunStatus updates a pipeline run's status and optional error message
func (db *GormDB) UpdatePipelineRunStatus(ctx context.Context, runID string, status models.PipelineRunStatus, errorMessage string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	return db.db.WithContext(ctx).
		Model(&models.PipelineRun{}).
		Where("id = ?", runID).
		Updates(updates).Error
}

// UpdatePipelineRun updates a pipeline run (only non-zero fields)
func (db *GormDB) UpdatePipelineRun(ctx context.Context, run *models.PipelineRun) error {
	return db.db.WithContext(ctx).Model(&models.PipelineRun{}).Where("id = ?", run.ID).Updates(run).Error
}

// DeletePipelineRun deletes a pipeline run and its step results
func (db *GormDB) DeletePipelineRun(ctx context.Context, runID string) error {
	return db.db.WithContext(ctx).Delete(&models.PipelineRun{}, "id = ?", runID).Error
}

// SaveRunStepSnapshots inserts or updates step snapshots for a run.
func (db *GormDB) SaveRunStepSnapshots(ctx context.Context, snapshots []models.RunStepSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	return db.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "run_id"}, {Name: "step_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"step_index",
				"step_name",
				"agent_config_json",
				"definition_hash",
			}),
		}).
		Create(&snapshots).Error
}

// ============================================================================
// StepResult Operations
// ============================================================================

// CreateStepResult creates a new step result
func (db *GormDB) CreateStepResult(ctx context.Context, result *models.StepResult) error {
	return db.db.WithContext(ctx).Create(result).Error
}

// GetStepResult retrieves a step result by ID
func (db *GormDB) GetStepResult(ctx context.Context, resultID string) (*models.StepResult, error) {
	var result models.StepResult
	err := db.db.WithContext(ctx).First(&result, "id = ?", resultID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// GetStepResultsByRun retrieves all step results for a pipeline run
func (db *GormDB) GetStepResultsByRun(ctx context.Context, runID string) ([]*models.StepResult, error) {
	var results []*models.StepResult
	err := db.db.WithContext(ctx).
		Where("pipeline_run_id = ?", runID).
		Order("step_index ASC").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

// UpdateStepResult updates a step result
func (db *GormDB) UpdateStepResult(ctx context.Context, result *models.StepResult) error {
	return db.db.WithContext(ctx).Save(result).Error
}

// UpdateStepResultStatus updates a step result's status
func (db *GormDB) UpdateStepResultStatus(ctx context.Context, resultID string, status models.StepStatus) error {
	return db.db.WithContext(ctx).
		Model(&models.StepResult{}).
		Where("id = ?", resultID).
		Update("status", status).Error
}

// GetRecentSuccessfulRunsWithSteps retrieves recent successful pipeline runs for a project,
// with their step results pre-loaded. Used for auto-fork detection.
func (db *GormDB) GetRecentSuccessfulRunsWithSteps(ctx context.Context, projectID string, baseCommitSHA string, maxRuns int) ([]*models.PipelineRun, error) {
	var runs []*models.PipelineRun
	err := db.db.WithContext(ctx).
		Preload("StepResults", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_index ASC")
		}).
		Where("project_id = ? AND status = ? AND base_commit_sha = ?", projectID, models.PipelineRunStatusCompleted, baseCommitSHA).
		Order("created_at DESC").
		Limit(maxRuns).
		Find(&runs).Error
	if err != nil {
		return nil, err
	}
	return runs, nil
}

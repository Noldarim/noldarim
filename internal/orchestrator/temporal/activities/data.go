// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
	"github.com/noldarim/noldarim/internal/protocol"
)

// DataActivities provides data-related activities
type DataActivities struct {
	dataService *services.DataService
	config      *config.AppConfig
}

// NewDataActivities creates a new instance of DataActivities
func NewDataActivities(dataService *services.DataService, config *config.AppConfig) *DataActivities {
	return &DataActivities{
		dataService: dataService,
		config:      config,
	}
}

// findExistingTask checks for an existing task by ID first (for retry idempotency),
// then by project+title (for duplicate prevention). Returns the task if found, nil otherwise.
func (a *DataActivities) findExistingTask(ctx context.Context, projectID, taskID, title string) (*models.Task, error) {
	// First, check by taskID (for retry idempotency)
	if taskID != "" {
		task, err := a.dataService.GetTask(ctx, taskID)
		if err == nil && task != nil {
			return task, nil
		}
		// Task not found by ID - continue to title check
	}

	// Check by project+title for duplicate prevention
	return a.dataService.FindTaskByProjectAndTitle(ctx, projectID, title)
}

// CreateTaskActivity creates a new task in the database with idempotency
func (a *DataActivities) CreateTaskActivity(ctx context.Context, input types.CreateTaskActivityInput) (*types.CreateTaskActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating task in database", "projectID", input.ProjectID, "taskID", input.TaskID, "title", input.Title)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Creating task")

	// Check for existing task (by ID for retries, or by title for duplicates)
	existingTask, err := a.findExistingTask(ctx, input.ProjectID, input.TaskID, input.Title)
	if err != nil {
		logger.Error("Failed to check for existing task", "error", err)
		return nil, fmt.Errorf("failed to check for existing task: %w", err)
	}

	if existingTask != nil {
		config := a.getConfig(ctx)
		// If found by ID (retry case) OR if it's a recent duplicate, return existing
		if existingTask.ID == input.TaskID || time.Since(existingTask.CreatedAt) < config.Container.Timeouts.TaskDuplicateWindow {
			logger.Info("Task already exists, returning existing", "taskID", existingTask.ID, "status", existingTask.Status)
			return &types.CreateTaskActivityOutput{
				Task: existingTask,
			}, nil
		}
		// Old task with same title (but different ID) - generate unique title
		input.Title = fmt.Sprintf("%s_%s", input.Title, time.Now().Format("20060102_150405"))
		logger.Info("Task with same title exists (old), using unique title", "newTitle", input.Title)
	}

	// Create the task using the data service with provided task ID
	task, err := a.dataService.CreateTask(ctx, input.ProjectID, input.TaskID, input.Title, input.Description, input.TaskFilePath)
	if err != nil {
		logger.Error("Failed to create task", "error", err)
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	logger.Info("Successfully created task", "taskID", task.ID)
	return &types.CreateTaskActivityOutput{
		Task: task,
	}, nil
}

// DeleteTaskActivity deletes a task from the database (compensation activity)
func (a *DataActivities) DeleteTaskActivity(ctx context.Context, taskID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Deleting task from database", "taskID", taskID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Deleting task")

	// Delete the task using the data service - idempotent (returns success if not found)
	if err := a.dataService.DeleteTask(ctx, "", taskID); err != nil {
		// Check if it's a not found error - that's OK for idempotency
		if err.Error() == "record not found" {
			logger.Info("Task already deleted or doesn't exist", "taskID", taskID)
			return nil
		}
		logger.Error("Failed to delete task", "error", err)
		return fmt.Errorf("failed to delete task: %w", err)
	}

	logger.Info("Successfully deleted task", "taskID", taskID)
	return nil
}

// UpdateTaskStatusActivity updates a task's status
func (a *DataActivities) UpdateTaskStatusActivity(ctx context.Context, input types.UpdateTaskStatusActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating task status", "taskID", input.TaskID, "status", input.Status.String())

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Updating task status")

	// Update the task status using the data service
	if err := a.dataService.UpdateTaskStatus(ctx, input.ProjectID, input.TaskID, input.Status); err != nil {
		logger.Error("Failed to update task status", "error", err)
		return fmt.Errorf("failed to update task status: %w", err)
	}

	logger.Info("Successfully updated task status")
	return nil
}

// UpdateTaskGitDiffActivity updates a task's git diff in the database
func (a *DataActivities) UpdateTaskGitDiffActivity(ctx context.Context, input types.UpdateTaskGitDiffActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating task git diff", "taskID", input.TaskID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Updating task git diff")

	// Update the task git diff using the data service
	if err := a.dataService.UpdateTaskGitDiff(ctx, input.TaskID, input.GitDiff); err != nil {
		logger.Error("Failed to update task git diff", "error", err)
		return fmt.Errorf("failed to update task git diff: %w", err)
	}

	logger.Info("Successfully updated task git diff")
	return nil
}

// LoadProjectsActivity loads all projects
func (a *DataActivities) LoadProjectsActivity(ctx context.Context) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Loading projects from database")

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Loading projects")

	// Load projects using the data service
	projects, err := a.dataService.LoadProjects(ctx)
	if err != nil {
		logger.Error("Failed to load projects", "error", err)
		return nil, fmt.Errorf("failed to load projects: %w", err)
	}

	logger.Info("Successfully loaded projects", "count", len(projects))
	return projects, nil
}

// LoadTasksActivity loads tasks for a specific project
func (a *DataActivities) LoadTasksActivity(ctx context.Context, projectID string) (interface{}, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Loading tasks from database", "projectID", projectID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Loading tasks")

	// Load tasks using the data service
	tasks, err := a.dataService.LoadTasks(ctx, projectID)
	if err != nil {
		logger.Error("Failed to load tasks", "error", err)
		return nil, fmt.Errorf("failed to load tasks: %w", err)
	}

	logger.Info("Successfully loaded tasks", "count", len(tasks))
	return tasks, nil
}

// getConfig returns the configuration, either from context or the default
func (a *DataActivities) getConfig(ctx context.Context) *config.AppConfig {
	// Try to get config from context first
	if cfg, ok := ctx.Value("config").(*config.AppConfig); ok && cfg != nil {
		return cfg
	}
	// Fall back to the config provided during initialization
	return a.config
}

// PrepareAgentCommandActivity prepares a command from an agent configuration
func (a *DataActivities) PrepareAgentCommandActivity(ctx context.Context, input *protocol.AgentConfigInput) ([]string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Preparing agent command", "tool", input.ToolName)

	command, err := PrepareAgentCommand(input)
	if err != nil {
		logger.Error("Failed to prepare agent command", "error", err)
		return nil, fmt.Errorf("failed to prepare agent command: %w", err)
	}

	logger.Info("Agent command prepared successfully", "command", command)
	return command, nil
}

// SaveAIActivityRecordActivity saves an AI activity record to the database
func (a *DataActivities) SaveAIActivityRecordActivity(ctx context.Context, record *models.AIActivityRecord) error {
	logger := activity.GetLogger(ctx)
	logger.Debug("Saving AI activity record", "taskID", record.TaskID, "eventID", record.EventID, "eventType", record.EventType)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Saving AI activity record")

	// Save the record using the data service
	if err := a.dataService.SaveAIActivityRecord(ctx, record); err != nil {
		logger.Error("Failed to save AI activity record", "error", err)
		return fmt.Errorf("failed to save AI activity record: %w", err)
	}

	logger.Debug("Successfully saved AI activity record")
	return nil
}

// LoadAIActivityByTaskActivity loads AI activity records for a task from the database
func (a *DataActivities) LoadAIActivityByTaskActivity(ctx context.Context, taskID string) ([]*models.AIActivityRecord, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Loading AI activity records", "taskID", taskID)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Loading AI activity records")

	// Load records using the data service
	records, err := a.dataService.GetAIActivityByTask(ctx, taskID)
	if err != nil {
		logger.Error("Failed to load AI activity records", "error", err)
		return nil, fmt.Errorf("failed to load AI activity records: %w", err)
	}

	logger.Info("Successfully loaded AI activity records", "count", len(records))
	return records, nil
}

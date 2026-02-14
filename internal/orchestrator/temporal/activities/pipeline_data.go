// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// PipelineDataActivities provides data activities for pipeline operations
type PipelineDataActivities struct {
	dataService *services.DataService
}

// NewPipelineDataActivities creates a new instance of PipelineDataActivities
func NewPipelineDataActivities(dataService *services.DataService) *PipelineDataActivities {
	return &PipelineDataActivities{
		dataService: dataService,
	}
}

// SavePipelineRunActivity saves or updates a pipeline run in the database
func (a *PipelineDataActivities) SavePipelineRunActivity(ctx context.Context, input types.SavePipelineRunActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Saving pipeline run to database", "runID", input.Run.ID, "status", input.Run.Status.String())

	activity.RecordHeartbeat(ctx, "Saving pipeline run")

	// Check if run exists
	existing, err := a.dataService.GetPipelineRun(ctx, input.Run.ID)
	if err != nil {
		logger.Debug("Pipeline run not found, will create new", "runID", input.Run.ID)
	}

	if existing != nil {
		// Update existing run (only non-zero fields)
		if err := a.dataService.UpdatePipelineRun(ctx, input.Run); err != nil {
			logger.Error("Failed to update pipeline run", "error", err)
			return fmt.Errorf("failed to update pipeline run: %w", err)
		}
		logger.Info("Successfully updated pipeline run", "runID", input.Run.ID)
	} else {
		// Create new run
		if err := a.dataService.CreatePipelineRun(ctx, input.Run); err != nil {
			logger.Error("Failed to create pipeline run", "error", err)
			return fmt.Errorf("failed to create pipeline run: %w", err)
		}
		logger.Info("Successfully created pipeline run", "runID", input.Run.ID)
	}

	return nil
}

// SaveStepResultActivity saves or updates a step result in the database
func (a *PipelineDataActivities) SaveStepResultActivity(ctx context.Context, input types.SaveStepResultActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Saving step result to database",
		"resultID", input.Result.ID,
		"runID", input.Result.PipelineRunID,
		"stepID", input.Result.StepID,
		"status", input.Result.Status.String())

	activity.RecordHeartbeat(ctx, "Saving step result")

	// Check if result exists
	existing, err := a.dataService.GetStepResult(ctx, input.Result.ID)
	if err != nil {
		logger.Debug("Step result not found, will create new", "resultID", input.Result.ID)
	}

	if existing != nil {
		// Update existing result
		if err := a.dataService.UpdateStepResult(ctx, input.Result); err != nil {
			logger.Error("Failed to update step result", "error", err)
			return fmt.Errorf("failed to update step result: %w", err)
		}
		logger.Info("Successfully updated step result", "resultID", input.Result.ID)
	} else {
		// Create new result
		if err := a.dataService.CreateStepResult(ctx, input.Result); err != nil {
			logger.Error("Failed to create step result", "error", err)
			return fmt.Errorf("failed to create step result: %w", err)
		}
		logger.Info("Successfully created step result", "resultID", input.Result.ID)
	}

	return nil
}

// GetPipelineRunActivity retrieves a pipeline run from the database
func (a *PipelineDataActivities) GetPipelineRunActivity(ctx context.Context, input types.GetPipelineRunActivityInput) (*types.GetPipelineRunActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting pipeline run from database", "runID", input.RunID)

	activity.RecordHeartbeat(ctx, "Getting pipeline run")

	run, err := a.dataService.GetPipelineRun(ctx, input.RunID)
	if err != nil {
		logger.Error("Failed to get pipeline run", "error", err)
		return nil, fmt.Errorf("failed to get pipeline run: %w", err)
	}

	logger.Info("Successfully retrieved pipeline run", "runID", input.RunID, "status", run.Status.String())
	return &types.GetPipelineRunActivityOutput{
		Run: run,
	}, nil
}

// UpdatePipelineRunStatusActivity updates a pipeline run's status
func (a *PipelineDataActivities) UpdatePipelineRunStatusActivity(ctx context.Context, input types.UpdatePipelineRunStatusActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Updating pipeline run status", "runID", input.RunID, "status", input.Status.String())

	activity.RecordHeartbeat(ctx, "Updating pipeline run status")

	if err := a.dataService.UpdatePipelineRunStatus(ctx, input.RunID, input.Status); err != nil {
		logger.Error("Failed to update pipeline run status", "error", err)
		return fmt.Errorf("failed to update pipeline run status: %w", err)
	}

	logger.Info("Successfully updated pipeline run status", "runID", input.RunID)
	return nil
}

// GetLatestPipelineRunActivity retrieves the most recent pipeline run
func (a *PipelineDataActivities) GetLatestPipelineRunActivity(ctx context.Context) (*types.GetPipelineRunActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting latest pipeline run from database")

	activity.RecordHeartbeat(ctx, "Getting latest pipeline run")

	run, err := a.dataService.GetLatestPipelineRun(ctx)
	if err != nil {
		logger.Error("Failed to get latest pipeline run", "error", err)
		return nil, fmt.Errorf("failed to get latest pipeline run: %w", err)
	}

	if run == nil {
		logger.Info("No pipeline runs found")
		return &types.GetPipelineRunActivityOutput{Run: nil}, nil
	}

	logger.Info("Successfully retrieved latest pipeline run", "runID", run.ID, "status", run.Status.String())
	return &types.GetPipelineRunActivityOutput{
		Run: run,
	}, nil
}

// GetTokenTotalsActivity retrieves aggregated token counts for a task
func (a *PipelineDataActivities) GetTokenTotalsActivity(ctx context.Context, input types.GetTokenTotalsActivityInput) (*types.GetTokenTotalsActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting token totals for task", "taskID", input.TaskID)

	activity.RecordHeartbeat(ctx, "Getting token totals")

	totals, err := a.dataService.GetTokenTotalsByTask(ctx, input.TaskID)
	if err != nil {
		logger.Error("Failed to get token totals", "error", err)
		return nil, fmt.Errorf("failed to get token totals: %w", err)
	}

	logger.Info("Successfully retrieved token totals",
		"taskID", input.TaskID,
		"inputTokens", totals.InputTokens,
		"outputTokens", totals.OutputTokens)

	return &types.GetTokenTotalsActivityOutput{
		InputTokens:       totals.InputTokens,
		OutputTokens:      totals.OutputTokens,
		CacheReadTokens:   totals.CacheReadTokens,
		CacheCreateTokens: totals.CacheCreateTokens,
	}, nil
}

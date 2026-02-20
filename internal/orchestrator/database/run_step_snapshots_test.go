// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"testing"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveRunStepSnapshots_UpsertAndPreload(t *testing.T) {
	cfg, _ := setupTestDB(t, "test_run_step_snapshots")
	db := createAndMigrateDB(t, cfg)
	ctx := context.Background()

	run := &models.PipelineRun{
		ID:             "run-1",
		ProjectID:      "proj-1",
		Name:           "Pipeline",
		Status:         models.PipelineRunStatusRunning,
		BaseCommitSHA:  "abc123",
		StartCommitSHA: "abc123",
	}
	require.NoError(t, db.CreatePipelineRun(ctx, run))

	require.NoError(t, db.SaveRunStepSnapshots(ctx, []models.RunStepSnapshot{
		{
			RunID:           run.ID,
			StepID:          "s2",
			StepIndex:       1,
			StepName:        "Step 2",
			AgentConfigJSON: `{"tool_name":"claude","prompt_template":"step2"}`,
			DefinitionHash:  "hash-s2-v1",
		},
		{
			RunID:           run.ID,
			StepID:          "s1",
			StepIndex:       0,
			StepName:        "Step 1",
			AgentConfigJSON: `{"tool_name":"claude","prompt_template":"step1"}`,
			DefinitionHash:  "hash-s1-v1",
		},
	}))

	loaded, err := db.GetPipelineRun(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, loaded.StepSnapshots, 2)
	require.Equal(t, "s1", loaded.StepSnapshots[0].StepID)
	require.Equal(t, "s2", loaded.StepSnapshots[1].StepID)

	require.NoError(t, db.SaveRunStepSnapshots(ctx, []models.RunStepSnapshot{
		{
			RunID:           run.ID,
			StepID:          "s1",
			StepIndex:       0,
			StepName:        "Step 1 updated",
			AgentConfigJSON: `{"tool_name":"claude","prompt_template":"step1-updated"}`,
			DefinitionHash:  "hash-s1-v2",
		},
	}))

	loaded, err = db.GetPipelineRun(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, loaded.StepSnapshots, 2)
	require.Equal(t, "Step 1 updated", loaded.StepSnapshots[0].StepName)
	require.Equal(t, "hash-s1-v2", loaded.StepSnapshots[0].DefinitionHash)
}

func TestSaveRunStepSnapshots_RejectsDuplicateStepIndexWithinRun(t *testing.T) {
	cfg, _ := setupTestDB(t, "test_run_step_snapshots_unique_step_index")
	db := createAndMigrateDB(t, cfg)
	ctx := context.Background()

	run := &models.PipelineRun{
		ID:             "run-unique",
		ProjectID:      "proj-1",
		Name:           "Pipeline",
		Status:         models.PipelineRunStatusRunning,
		BaseCommitSHA:  "abc123",
		StartCommitSHA: "abc123",
	}
	require.NoError(t, db.CreatePipelineRun(ctx, run))

	err := db.SaveRunStepSnapshots(ctx, []models.RunStepSnapshot{
		{
			RunID:           run.ID,
			StepID:          "s1",
			StepIndex:       0,
			StepName:        "Step 1",
			AgentConfigJSON: `{"tool_name":"claude","prompt_template":"step1"}`,
			DefinitionHash:  "hash-s1",
		},
		{
			RunID:           run.ID,
			StepID:          "s2",
			StepIndex:       0, // duplicate order slot in same run
			StepName:        "Step 2",
			AgentConfigJSON: `{"tool_name":"claude","prompt_template":"step2"}`,
			DefinitionHash:  "hash-s2",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE")
}

func TestValidateSchema_RequiresRunStepSnapshotsTableAndColumns(t *testing.T) {
	cfg, _ := setupTestDB(t, "test_run_step_snapshots_schema_validation")
	db := createAndMigrateDB(t, cfg)

	// Missing table should fail validation.
	require.NoError(t, db.db.Exec("DROP TABLE run_step_snapshots").Error)
	err := db.ValidateSchema()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing tables")
	assert.Contains(t, err.Error(), "run_step_snapshots")

	// Recreate table with missing required columns and verify column-level validation.
	require.NoError(t, db.db.Exec(`
		CREATE TABLE run_step_snapshots (
			run_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			step_name TEXT,
			created_at DATETIME,
			PRIMARY KEY (run_id, step_id)
		)
	`).Error)
	err = db.ValidateSchema()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing columns")
	assert.Contains(t, err.Error(), "run_step_snapshots.step_index")
	assert.Contains(t, err.Error(), "run_step_snapshots.agent_config_json")
	assert.Contains(t, err.Error(), "run_step_snapshots.definition_hash")
}

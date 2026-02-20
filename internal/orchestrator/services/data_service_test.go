// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"
	"os"
	"testing"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/database"
	"github.com/noldarim/noldarim/internal/orchestrator/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDataServiceCRUD tests all CRUD operations for the data service
func TestDataServiceCRUD(t *testing.T) {
	// Use fixture for data service setup - this creates both the database and service
	dsFixture := WithDataService(t)
	defer dsFixture.Cleanup()

	// Get the database connection from the data service
	ds := dsFixture.Service
	db := ds.db // Access the internal database connection

	// Test Projects CRUD
	t.Run("ProjectsCRUD", func(t *testing.T) {
		testProjectsCRUD(t, db)
	})

	// Test Tasks CRUD
	t.Run("TasksCRUD", func(t *testing.T) {
		testTasksCRUD(t, db, ds)
	})

	// Test Task Status Updates
	t.Run("TaskStatusUpdates", func(t *testing.T) {
		testTaskStatusUpdates(t, db, ds)
	})
}

func testProjectsCRUD(t *testing.T, db *database.GormDB) {
	ctx := context.Background()

	// Create test project
	project := &models.Project{
		ID:          "test-project-1",
		Name:        "Test Project",
		Description: "A test project",
		AgentID:     "agent-123",
	}

	// Test Create
	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create project")

	// Test Read
	projects, err := db.GetAllProjects(ctx)
	require.NoError(t, err, "Failed to get all projects")
	assert.Len(t, projects, 1, "Should have exactly 1 project")

	retrievedProject, exists := projects[project.ID]
	assert.True(t, exists, "Project should exist")
	assert.Equal(t, project.ID, retrievedProject.ID)
	assert.Equal(t, project.Name, retrievedProject.Name)
	assert.Equal(t, project.Description, retrievedProject.Description)
	assert.Equal(t, project.AgentID, retrievedProject.AgentID)
	// TaskCount is not a field on Project model, skip this assertion

	// Test direct project retrieval
	directProject, err := db.GetProject(ctx, project.ID)
	require.NoError(t, err, "Failed to get project directly")
	assert.Equal(t, project.ID, directProject.ID)
	assert.Equal(t, project.Name, directProject.Name)
}

func testTasksCRUD(t *testing.T, db *database.GormDB, ds *DataService) {
	ctx := context.Background()

	// Create test project first
	project := &models.Project{
		ID:          "test-project-2",
		Name:        "Test Project 2",
		Description: "Another test project",
		AgentID:     "agent-456",
	}
	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create test project")

	// Test Create Task via DataService
	taskView, err := ds.CreateTask(ctx, project.ID, "test-task-123", "Test Task", "A test task", "tasks/test-task.md")
	require.NoError(t, err, "Failed to create task")
	assert.NotEmpty(t, taskView.ID, "Task ID should not be empty")
	assert.Equal(t, "Test Task", taskView.Title)
	assert.Equal(t, "A test task", taskView.Description)
	assert.Equal(t, models.TaskStatusPending, taskView.Status)
	assert.Equal(t, project.ID, taskView.ProjectID)

	// Test Read Tasks
	tasks, err := db.GetTasksByProject(ctx, project.ID)
	require.NoError(t, err, "Failed to get tasks for project")
	assert.Len(t, tasks, 1, "Should have exactly 1 task")

	retrievedTask, exists := tasks[taskView.ID]
	assert.True(t, exists, "Task should exist")
	if exists {
		assert.Equal(t, taskView.ID, retrievedTask.ID)
		assert.Equal(t, taskView.Title, retrievedTask.Title)
		assert.Equal(t, taskView.Description, retrievedTask.Description)
		assert.Equal(t, taskView.Status, retrievedTask.Status)
		assert.Equal(t, taskView.ProjectID, retrievedTask.ProjectID)
	}

	// Test Update Task
	updatedTask, err := ds.UpdateTask(ctx, project.ID, taskView.ID, "Updated Task", "Updated description")
	require.NoError(t, err, "Failed to update task")
	assert.Equal(t, "Updated Task", updatedTask.Title)
	assert.Equal(t, "Updated description", updatedTask.Description)

	// Test Delete Task
	err = ds.DeleteTask(ctx, taskView.ID)
	require.NoError(t, err, "Failed to delete task")

	// Verify task is deleted
	tasks, err = db.GetTasksByProject(ctx, project.ID)
	require.NoError(t, err, "Failed to get tasks after deletion")
	assert.Len(t, tasks, 0, "Should have no tasks after deletion")
}

func testTaskStatusUpdates(t *testing.T, db *database.GormDB, ds *DataService) {
	ctx := context.Background()

	// Create test project
	project := &models.Project{
		ID:          "test-project-3",
		Name:        "Test Project 3",
		Description: "Status test project",
		AgentID:     "agent-789",
	}
	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create test project")

	// Create task
	taskView, err := ds.CreateTask(ctx, project.ID, "status-test-456", "Status Test Task", "Testing status updates", "tasks/status-test.md")
	require.NoError(t, err, "Failed to create task")

	// Test status updates
	statuses := []models.TaskStatus{
		models.TaskStatusInProgress,
		models.TaskStatusCompleted,
		models.TaskStatusPending,
	}

	for _, status := range statuses {
		err = ds.UpdateTaskStatus(ctx, taskView.ID, status)
		require.NoError(t, err, "Failed to update task status to %v", status)

		// Verify status was updated
		tasks, err := db.GetTasksByProject(ctx, project.ID)
		require.NoError(t, err, "Failed to get tasks")

		task, exists := tasks[taskView.ID]
		assert.True(t, exists, "Task should exist")
		assert.Equal(t, status, task.Status, "Status should be updated to %v", status)
	}
}

// TestSchemaValidationInDataService tests that schema validation is working
func TestSchemaValidationInDataService(t *testing.T) {
	// Create a temporary database with the wrong schema
	tmpDB := "test_wrong_schema.db"
	defer os.Remove(tmpDB)

	cfg := &config.AppConfig{
		Database: config.DatabaseConfig{
			Driver:   "sqlite",
			Database: tmpDB,
		},
	}

	// Create database connection without migrations
	db, err := database.NewGormDB(&cfg.Database)
	require.NoError(t, err, "Failed to create database")
	db.Close()

	// Try to create data service - should fail schema validation
	_, err = NewDataService(cfg)
	assert.Error(t, err, "Data service should fail with invalid schema")
	assert.Contains(t, err.Error(), "Run 'make migrate'", "Error should contain migration message")
}

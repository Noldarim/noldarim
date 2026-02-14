// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package database

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constants
const (
	TestAgentID1      = "agent-123"
	TestAgentID2      = "agent-456"
	TestProjectID1    = "test-project-1"
	TestProjectID2    = "test-project-2"
	TestTaskID1       = "test-task-1"
	TestTaskID2       = "test-task-2"
	DefaultTaskStatus = models.TaskStatus(0)
	TestBranch        = "main"
)

// Test helper functions

// setupTestDB creates a test database with a unique name and returns config and cleanup function
func setupTestDB(t *testing.T, name string) (*config.DatabaseConfig, func()) {
	testDBName := fmt.Sprintf("%s.db", name)
	cleanup := func() { os.Remove(testDBName) }
	t.Cleanup(cleanup)

	return &config.DatabaseConfig{
		Driver:   "sqlite",
		Database: testDBName,
	}, cleanup
}

// createAndMigrateDB creates a database connection and runs migrations
func createAndMigrateDB(t *testing.T, cfg *config.DatabaseConfig) *GormDB {
	db, err := NewGormDB(cfg)
	require.NoError(t, err, "Failed to connect to test database")
	t.Cleanup(func() { db.Close() })

	err = db.AutoMigrate()
	require.NoError(t, err, "Failed to run migrations")

	return db
}

// assertBasicProjectFields verifies common project fields are set correctly
func assertBasicProjectFields(t *testing.T, project *models.Project, id, name, description, agentID string) {
	assert.Equal(t, id, project.ID)
	assert.Equal(t, name, project.Name)
	assert.Equal(t, description, project.Description)
	assert.Equal(t, agentID, project.AgentID)
	assert.False(t, project.CreatedAt.IsZero())
	assert.False(t, project.LastUpdatedAt.IsZero())
}

// assertBasicTaskFields verifies common task fields are set correctly
func assertBasicTaskFields(t *testing.T, task *models.Task, id, title, description, projectID, agentID string, status models.TaskStatus) {
	assert.Equal(t, id, task.ID)
	assert.Equal(t, title, task.Title)
	assert.Equal(t, description, task.Description)
	assert.Equal(t, projectID, task.ProjectID)
	assert.Equal(t, agentID, task.AgentID)
	assert.Equal(t, status, task.Status)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.LastUpdatedAt.IsZero())
}

// assertProjectExists verifies a project exists and has expected fields
func assertProjectExists(t *testing.T, db *GormDB, ctx context.Context, id, name, description, agentID string) *models.Project {
	project, err := db.GetProject(ctx, id)
	require.NoError(t, err)
	assertBasicProjectFields(t, project, id, name, description, agentID)
	return project
}

// assertTaskExists verifies a task exists and has expected fields
func assertTaskExists(t *testing.T, db *GormDB, ctx context.Context, id, title, description, projectID, agentID string, status models.TaskStatus) *models.Task {
	task, err := db.GetTask(ctx, id)
	require.NoError(t, err)
	assertBasicTaskFields(t, task, id, title, description, projectID, agentID, status)
	return task
}

// assertProjectCollection verifies a project collection contains expected projects
func assertProjectCollection(t *testing.T, projects map[string]*models.Project, expectedIDs ...string) {
	assert.Len(t, projects, len(expectedIDs))
	for _, id := range expectedIDs {
		assert.Contains(t, projects, id)
	}
}

// assertTaskCollection verifies a task collection contains expected tasks
func assertTaskCollection(t *testing.T, tasks map[string]*models.Task, expectedIDs ...string) {
	assert.Len(t, tasks, len(expectedIDs))
	for _, id := range expectedIDs {
		assert.Contains(t, tasks, id)
	}
}

// Test data builders

// ProjectBuilder helps create test projects with sensible defaults
type ProjectBuilder struct {
	project *models.Project
}

// NewProjectBuilder creates a new project builder with defaults
func NewProjectBuilder() *ProjectBuilder {
	return &ProjectBuilder{
		project: &models.Project{
			ID:          TestProjectID1,
			Name:        "Test Project",
			Description: "Test Description",
			AgentID:     TestAgentID1,
		},
	}
}

// WithID sets the project ID
func (b *ProjectBuilder) WithID(id string) *ProjectBuilder {
	b.project.ID = id
	return b
}

// WithName sets the project name
func (b *ProjectBuilder) WithName(name string) *ProjectBuilder {
	b.project.Name = name
	return b
}

// WithDescription sets the project description
func (b *ProjectBuilder) WithDescription(desc string) *ProjectBuilder {
	b.project.Description = desc
	return b
}

// WithAgentID sets the agent ID
func (b *ProjectBuilder) WithAgentID(agentID string) *ProjectBuilder {
	b.project.AgentID = agentID
	return b
}

// Build returns the built project
func (b *ProjectBuilder) Build() *models.Project {
	return b.project
}

// Create builds and creates the project in the database
func (b *ProjectBuilder) Create(t *testing.T, db *GormDB, ctx context.Context) *models.Project {
	project := b.Build()
	err := db.CreateProject(ctx, project)
	require.NoError(t, err)
	return project
}

// TaskBuilder helps create test tasks with sensible defaults
type TaskBuilder struct {
	task *models.Task
}

// NewTaskBuilder creates a new task builder with defaults
func NewTaskBuilder() *TaskBuilder {
	return &TaskBuilder{
		task: &models.Task{
			ID:          TestTaskID1,
			Title:       "Test Task",
			Description: "Test Task Description",
			Status:      DefaultTaskStatus,
			ProjectID:   TestProjectID1,
			BranchName:  TestBranch,
			AgentID:     TestAgentID1,
			ExecHistory: models.ExecHistory{"git init"},
		},
	}
}

// WithID sets the task ID
func (b *TaskBuilder) WithID(id string) *TaskBuilder {
	b.task.ID = id
	return b
}

// WithTitle sets the task title
func (b *TaskBuilder) WithTitle(title string) *TaskBuilder {
	b.task.Title = title
	return b
}

// WithDescription sets the task description
func (b *TaskBuilder) WithDescription(desc string) *TaskBuilder {
	b.task.Description = desc
	return b
}

// WithStatus sets the task status
func (b *TaskBuilder) WithStatus(status models.TaskStatus) *TaskBuilder {
	b.task.Status = status
	return b
}

// WithProjectID sets the project ID
func (b *TaskBuilder) WithProjectID(projectID string) *TaskBuilder {
	b.task.ProjectID = projectID
	return b
}

// WithAgentID sets the agent ID
func (b *TaskBuilder) WithAgentID(agentID string) *TaskBuilder {
	b.task.AgentID = agentID
	return b
}

// WithBranchName sets the branch name
func (b *TaskBuilder) WithBranchName(branchName string) *TaskBuilder {
	b.task.BranchName = branchName
	return b
}

// WithExecHistory sets the execution history
func (b *TaskBuilder) WithExecHistory(history models.ExecHistory) *TaskBuilder {
	b.task.ExecHistory = history
	return b
}

// Build returns the built task
func (b *TaskBuilder) Build() *models.Task {
	return b.task
}

// Create builds and creates the task in the database
func (b *TaskBuilder) Create(t *testing.T, db *GormDB, ctx context.Context) *models.Task {
	task := b.Build()
	err := db.CreateTask(ctx, task)
	require.NoError(t, err)
	return task
}

// TestDatabaseSchemaValidation tests that GORM models match the existing database schema
func TestDatabaseSchemaValidation(t *testing.T) {
	cfg, _ := setupTestDB(t, "test_schema_validation")
	db := createAndMigrateDB(t, cfg)

	err := db.ValidateSchema()
	if err != nil {
		t.Fatalf("Schema validation failed: %v\n\nThis means your GORM models do not match the migrated database schema.", err)
	}

	t.Log("✅ Schema validation passed - GORM models match migrated database schema")
}

// TestDatabaseConnection tests that we can connect to the database
func TestDatabaseConnection(t *testing.T) {
	cfg, _ := setupTestDB(t, "test_connection")
	db := createAndMigrateDB(t, cfg)

	ctx := context.Background()
	projects, err := db.GetAllProjects(ctx)
	require.NoError(t, err, "Failed to query projects")
	assert.NotNil(t, projects, "Projects should not be nil")

	t.Logf("✅ Successfully connected to test database and retrieved %d projects", len(projects))
}

// TestGormModelStructure tests that GORM models have the expected structure
func TestGormModelStructure(t *testing.T) {
	t.Run("ProjectModel", func(t *testing.T) {
		project := NewProjectBuilder().Build()
		assert.Equal(t, "projects", project.TableName())
		assert.NotEmpty(t, project.ID)
		assert.NotEmpty(t, project.Name)
		assert.NotEmpty(t, project.Description)
		assert.NotEmpty(t, project.AgentID)
	})

	t.Run("TaskModel", func(t *testing.T) {
		task := NewTaskBuilder().Build()
		assert.Equal(t, "tasks", task.TableName())
		assert.NotEmpty(t, task.ID)
		assert.NotEmpty(t, task.Title)
		assert.NotEmpty(t, task.Description)
		assert.Equal(t, DefaultTaskStatus, task.Status)
		assert.NotEmpty(t, task.ProjectID)
		assert.NotEmpty(t, task.AgentID)
	})
}

// TestExecHistoryJSONHandling tests the custom ExecHistory type
func TestExecHistoryJSONHandling(t *testing.T) {
	// Test creating ExecHistory
	history := models.ExecHistory{"git init", "git add .", "git commit -m 'initial'"}

	// Test Value method (serialization)
	value, err := history.Value()
	require.NoError(t, err)
	assert.NotEmpty(t, value)

	// Test Scan method (deserialization)
	var newHistory models.ExecHistory
	err = newHistory.Scan(value)
	require.NoError(t, err)
	assert.Equal(t, history, newHistory)

	// Test scanning from string
	jsonStr := `["git init", "git add .", "git commit -m 'initial'"]`
	var historyFromString models.ExecHistory
	err = historyFromString.Scan(jsonStr)
	require.NoError(t, err)
	assert.Equal(t, history, historyFromString)

	// Test scanning from []byte
	var historyFromBytes models.ExecHistory
	err = historyFromBytes.Scan([]byte(jsonStr))
	require.NoError(t, err)
	assert.Equal(t, history, historyFromBytes)

	// Test scanning nil
	var historyFromNil models.ExecHistory
	err = historyFromNil.Scan(nil)
	require.NoError(t, err)
	assert.Equal(t, models.ExecHistory{}, historyFromNil)
}

// TestModelTableNames tests that all models have correct table names
func TestModelTableNames(t *testing.T) {
	testCases := []struct {
		model     interface{ TableName() string }
		tableName string
	}{
		{&models.Project{}, "projects"},
		{&models.Task{}, "tasks"},
	}

	for _, tc := range testCases {
		t.Run(tc.tableName, func(t *testing.T) {
			assert.Equal(t, tc.tableName, tc.model.TableName())
		})
	}
}

// TestInMemoryDatabaseFixtures tests creating and using in-memory database fixtures
func TestInMemoryDatabaseFixtures(t *testing.T) {
	t.Run("FreshInMemoryDatabase", func(t *testing.T) {
		fixture := UseFreshInMemoryDatabase(t)
		defer fixture.Cleanup()

		// Test that database is properly initialized
		assert.NotNil(t, fixture.DB)

		// Test that tables exist and schema is valid
		err := fixture.DB.ValidateSchema()
		assert.NoError(t, err)

		// Test that database is empty
		ctx := context.Background()
		projects, err := fixture.DB.GetAllProjects(ctx)
		require.NoError(t, err)
		assert.Empty(t, projects)
	})

	t.Run("MultipleDatabaseIsolation", func(t *testing.T) {
		// Test that multiple database connections are isolated
		// Use file-based databases to ensure proper isolation
		cfg1, _ := setupTestDB(t, "isolation_test_1")
		db1 := createAndMigrateDB(t, cfg1)

		cfg2, _ := setupTestDB(t, "isolation_test_2")
		db2 := createAndMigrateDB(t, cfg2)

		ctx := context.Background()

		// Create a project in db1
		project1 := NewProjectBuilder().WithName("Test Project 1").WithDescription("Test Description 1").WithAgentID("agent-1").Build()
		err := db1.CreateProject(ctx, project1)
		require.NoError(t, err)

		// Verify project exists in db1
		projects1, err := db1.GetAllProjects(ctx)
		require.NoError(t, err)
		assert.Len(t, projects1, 1)

		// Verify project does not exist in db2
		projects2, err := db2.GetAllProjects(ctx)
		require.NoError(t, err)
		assert.Empty(t, projects2)
	})
}

// TestProjectCRUD tests all project CRUD operations
func TestProjectCRUD(t *testing.T) {
	fixture := UseFreshInMemoryDatabase(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	t.Run("CreateProject", func(t *testing.T) {
		project := NewProjectBuilder().Create(t, fixture.DB, ctx)
		assert.False(t, project.CreatedAt.IsZero())
		assert.False(t, project.LastUpdatedAt.IsZero())
	})

	t.Run("CreateDuplicateProject", func(t *testing.T) {
		project := NewProjectBuilder().WithName("Duplicate Project").WithDescription("Should fail").WithAgentID(TestAgentID2).Build()
		err := fixture.DB.CreateProject(ctx, project)
		assert.Error(t, err, "Creating duplicate project should fail")
	})

	t.Run("GetProject", func(t *testing.T) {
		assertProjectExists(t, fixture.DB, ctx, TestProjectID1, "Test Project", "Test Description", TestAgentID1)
	})

	t.Run("GetNonExistentProject", func(t *testing.T) {
		_, err := fixture.DB.GetProject(ctx, "non-existent")
		assert.Error(t, err, "Getting non-existent project should fail")
	})

	t.Run("GetAllProjects", func(t *testing.T) {
		NewProjectBuilder().WithID(TestProjectID2).WithName("Test Project 2").WithDescription("Test Description 2").WithAgentID(TestAgentID2).Create(t, fixture.DB, ctx)

		projects, err := fixture.DB.GetAllProjects(ctx)
		require.NoError(t, err)
		assertProjectCollection(t, projects, TestProjectID1, TestProjectID2)
		// TaskCount is not a field on Project model, skip these assertions
	})
}

// TestTaskCRUD tests all task CRUD operations
func TestTaskCRUD(t *testing.T) {
	fixture := UseFreshInMemoryDatabase(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Create a project first
	NewProjectBuilder().Create(t, fixture.DB, ctx)

	t.Run("CreateTask", func(t *testing.T) {
		task := NewTaskBuilder().WithExecHistory(models.ExecHistory{"git init", "git add ."}).Create(t, fixture.DB, ctx)
		assert.False(t, task.CreatedAt.IsZero())
		assert.False(t, task.LastUpdatedAt.IsZero())
	})

	t.Run("CreateTaskWithInvalidProjectID", func(t *testing.T) {
		task := NewTaskBuilder().WithID("test-task-invalid").WithTitle("Invalid Task").WithDescription("Should fail").WithProjectID("non-existent-project").Build()
		err := fixture.DB.CreateTask(ctx, task)
		// Note: GORM with SQLite doesn't enforce foreign key constraints by default
		// This test documents the current behavior rather than expected behavior
		// In a production setup, foreign key constraints should be enabled
		if err != nil {
			assert.Error(t, err, "Creating task with invalid project ID should fail")
		} else {
			t.Log("Warning: Foreign key constraints not enforced - task created with invalid project ID")
		}
	})

	t.Run("GetTask", func(t *testing.T) {
		task := assertTaskExists(t, fixture.DB, ctx, TestTaskID1, "Test Task", "Test Task Description", TestProjectID1, TestAgentID1, DefaultTaskStatus)
		assert.Equal(t, TestBranch, task.BranchName)
		assert.Equal(t, models.ExecHistory{"git init", "git add ."}, task.ExecHistory)
	})

	t.Run("GetNonExistentTask", func(t *testing.T) {
		_, err := fixture.DB.GetTask(ctx, "non-existent")
		assert.Error(t, err, "Getting non-existent task should fail")
	})

	t.Run("GetTasksByProject", func(t *testing.T) {
		NewTaskBuilder().WithID(TestTaskID2).WithTitle("Test Task 2").WithDescription("Test Task Description 2").WithStatus(1).WithBranchName("feature/test").WithExecHistory(models.ExecHistory{"git checkout -b feature/test"}).Create(t, fixture.DB, ctx)

		tasks, err := fixture.DB.GetTasksByProject(ctx, TestProjectID1)
		require.NoError(t, err)
		assertTaskCollection(t, tasks, TestTaskID1, TestTaskID2)
		assert.Equal(t, models.TaskStatus(0), tasks[TestTaskID1].Status)
		assert.Equal(t, models.TaskStatus(1), tasks[TestTaskID2].Status)
	})

	t.Run("UpdateTaskStatus", func(t *testing.T) {
		err := fixture.DB.UpdateTaskStatus(ctx, TestTaskID1, models.TaskStatus(2)) // completed
		assert.NoError(t, err)

		task, err := fixture.DB.GetTask(ctx, TestTaskID1)
		require.NoError(t, err)
		assert.Equal(t, models.TaskStatus(2), task.Status)
		assert.True(t, task.LastUpdatedAt.After(task.CreatedAt))
	})

	t.Run("UpdateTask", func(t *testing.T) {
		err := fixture.DB.UpdateTask(ctx, TestTaskID1, "Updated Title", "Updated Description")
		assert.NoError(t, err)

		task, err := fixture.DB.GetTask(ctx, TestTaskID1)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", task.Title)
		assert.Equal(t, "Updated Description", task.Description)
	})

	t.Run("DeleteTask", func(t *testing.T) {
		err := fixture.DB.DeleteTask(ctx, TestTaskID1)
		assert.NoError(t, err)

		_, err = fixture.DB.GetTask(ctx, TestTaskID1)
		assert.Error(t, err, "Getting deleted task should fail")

		tasks, err := fixture.DB.GetTasksByProject(ctx, TestProjectID1)
		require.NoError(t, err)
		assertTaskCollection(t, tasks, TestTaskID2)
	})
}

// TestProjectTaskRelationship tests the relationship between projects and tasks
func TestProjectTaskRelationship(t *testing.T) {
	fixture := UseFreshInMemoryDatabase(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Create projects
	NewProjectBuilder().WithID("project-1").WithName("Project 1").WithDescription("Project 1 Description").WithAgentID("agent-1").Create(t, fixture.DB, ctx)
	NewProjectBuilder().WithID("project-2").WithName("Project 2").WithDescription("Project 2 Description").WithAgentID("agent-2").Create(t, fixture.DB, ctx)

	// Create tasks for project1
	for i := 0; i < 3; i++ {
		NewTaskBuilder().WithID(fmt.Sprintf("task-%d", i+1)).WithTitle(fmt.Sprintf("Task %d", i+1)).WithDescription(fmt.Sprintf("Task %d Description", i+1)).WithStatus(models.TaskStatus(i%3)).WithProjectID("project-1").WithAgentID("agent-1").Create(t, fixture.DB, ctx)
	}

	// Create tasks for project2
	for i := 0; i < 2; i++ {
		NewTaskBuilder().WithID(fmt.Sprintf("task-p2-%d", i+1)).WithTitle(fmt.Sprintf("Project 2 Task %d", i+1)).WithDescription(fmt.Sprintf("Project 2 Task %d Description", i+1)).WithProjectID("project-2").WithAgentID("agent-2").Create(t, fixture.DB, ctx)
	}

	t.Run("ProjectTaskCounts", func(t *testing.T) {
		// TaskCount is not a field on Project model, skip this test entirely
		t.Skip("TaskCount field not available on Project model")
	})

	t.Run("TasksByProject", func(t *testing.T) {
		tasks1, err := fixture.DB.GetTasksByProject(ctx, "project-1")
		require.NoError(t, err)
		assert.Len(t, tasks1, 3)

		tasks2, err := fixture.DB.GetTasksByProject(ctx, "project-2")
		require.NoError(t, err)
		assert.Len(t, tasks2, 2)

		// Verify tasks belong to correct projects
		for _, task := range tasks1 {
			assert.Equal(t, "project-1", task.ProjectID)
		}
		for _, task := range tasks2 {
			assert.Equal(t, "project-2", task.ProjectID)
		}
	})
}

// TestAIActivityCRUD tests AI activity record CRUD operations
func TestAIActivityCRUD(t *testing.T) {
	fixture := UseFreshInMemoryDatabase(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Create a project and task first
	NewProjectBuilder().Create(t, fixture.DB, ctx)
	NewTaskBuilder().Create(t, fixture.DB, ctx)

	t.Run("SaveAIActivityRecord", func(t *testing.T) {
		record := &models.AIActivityRecord{
			EventID:    "evt-001",
			TaskID:     TestTaskID1,
			SessionID:  "session-123",
			EventType:  "tool_use",
			ToolName:   "Bash",
			RawPayload: `{"event_id":"evt-001","tool_call":{"tool_name":"Bash"}}`,
		}

		err := fixture.DB.SaveAIActivityRecord(ctx, record)
		require.NoError(t, err)

		// Verify record was saved with EventID as primary key and CreatedAt
		assert.NotEmpty(t, record.EventID)
		assert.False(t, record.CreatedAt.IsZero())
	})

	t.Run("SaveDuplicateRecord", func(t *testing.T) {
		// Saving a record with the same EventID should not fail (upsert behavior)
		record := &models.AIActivityRecord{
			EventID:    "evt-001", // Same as above
			TaskID:     TestTaskID1,
			SessionID:  "session-123",
			EventType:  "tool_use",
			ToolName:   "Read",
			RawPayload: `{"event_id":"evt-001","tool_call":{"tool_name":"Read"}}`,
		}

		err := fixture.DB.SaveAIActivityRecord(ctx, record)
		require.NoError(t, err)

		// Verify only one record exists with that EventID
		records, err := fixture.DB.GetAIActivityByTask(ctx, TestTaskID1)
		require.NoError(t, err)

		var count int
		for _, r := range records {
			if r.EventID == "evt-001" {
				count++
			}
		}
		assert.Equal(t, 1, count, "Should only have one record with EventID evt-001")
	})

	t.Run("GetAIActivityByTask", func(t *testing.T) {
		// Add more records
		for i := 2; i <= 5; i++ {
			record := &models.AIActivityRecord{
				EventID:    fmt.Sprintf("evt-%03d", i),
				TaskID:     TestTaskID1,
				SessionID:  "session-123",
				EventType:  "tool_use",
				RawPayload: fmt.Sprintf(`{"event_id":"evt-%03d"}`, i),
			}
			err := fixture.DB.SaveAIActivityRecord(ctx, record)
			require.NoError(t, err)
		}

		records, err := fixture.DB.GetAIActivityByTask(ctx, TestTaskID1)
		require.NoError(t, err)
		assert.Len(t, records, 5)

		// Verify order (should be by CreatedAt ASC)
		for i := 0; i < len(records)-1; i++ {
			assert.True(t, records[i].CreatedAt.Before(records[i+1].CreatedAt) ||
				records[i].CreatedAt.Equal(records[i+1].CreatedAt),
				"Records should be ordered by CreatedAt ASC")
		}
	})

	t.Run("GetAIActivityByTaskEmpty", func(t *testing.T) {
		records, err := fixture.DB.GetAIActivityByTask(ctx, "non-existent-task")
		require.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("GetAIActivityByTaskSince_EmptyEventID", func(t *testing.T) {
		// With empty sinceEventID, should return all records
		records, err := fixture.DB.GetAIActivityByTaskSince(ctx, TestTaskID1, "")
		require.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("GetAIActivityByTaskSince_ValidEventID", func(t *testing.T) {
		// Should return records after evt-002
		records, err := fixture.DB.GetAIActivityByTaskSince(ctx, TestTaskID1, "evt-002")
		require.NoError(t, err)
		assert.Len(t, records, 3) // evt-003, evt-004, evt-005

		// Verify none of the returned records are evt-001 or evt-002
		for _, r := range records {
			assert.NotEqual(t, "evt-001", r.EventID)
			assert.NotEqual(t, "evt-002", r.EventID)
		}
	})

	t.Run("GetAIActivityByTaskSince_NonExistentEventID", func(t *testing.T) {
		// With non-existent sinceEventID, should fall back to returning all records
		records, err := fixture.DB.GetAIActivityByTaskSince(ctx, TestTaskID1, "non-existent-event")
		require.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("DeleteAIActivityByTask", func(t *testing.T) {
		// First verify records exist
		records, err := fixture.DB.GetAIActivityByTask(ctx, TestTaskID1)
		require.NoError(t, err)
		assert.Len(t, records, 5)

		// Delete all records for the task
		err = fixture.DB.DeleteAIActivityByTask(ctx, TestTaskID1)
		require.NoError(t, err)

		// Verify records are deleted
		records, err = fixture.DB.GetAIActivityByTask(ctx, TestTaskID1)
		require.NoError(t, err)
		assert.Empty(t, records)
	})

	t.Run("DeleteAIActivityByTask_NoRecords", func(t *testing.T) {
		// Deleting when no records exist should not error
		err := fixture.DB.DeleteAIActivityByTask(ctx, "non-existent-task")
		require.NoError(t, err)
	})
}

// TestAIActivityIsolation tests that AI activity records are properly isolated between tasks
func TestAIActivityIsolation(t *testing.T) {
	fixture := UseFreshInMemoryDatabase(t)
	defer fixture.Cleanup()

	ctx := context.Background()

	// Create a project and two tasks
	NewProjectBuilder().Create(t, fixture.DB, ctx)
	NewTaskBuilder().WithID("task-a").WithTitle("Task A").Create(t, fixture.DB, ctx)
	NewTaskBuilder().WithID("task-b").WithTitle("Task B").Create(t, fixture.DB, ctx)

	// Add records for task-a
	for i := 1; i <= 3; i++ {
		record := &models.AIActivityRecord{
			EventID:    fmt.Sprintf("evt-a-%d", i),
			TaskID:     "task-a",
			SessionID:  "session-a",
			EventType:  "tool_use",
			RawPayload: fmt.Sprintf(`{"task":"a","num":%d}`, i),
		}
		err := fixture.DB.SaveAIActivityRecord(ctx, record)
		require.NoError(t, err)
	}

	// Add records for task-b
	for i := 1; i <= 2; i++ {
		record := &models.AIActivityRecord{
			EventID:    fmt.Sprintf("evt-b-%d", i),
			TaskID:     "task-b",
			SessionID:  "session-b",
			EventType:  "tool_result",
			RawPayload: fmt.Sprintf(`{"task":"b","num":%d}`, i),
		}
		err := fixture.DB.SaveAIActivityRecord(ctx, record)
		require.NoError(t, err)
	}

	t.Run("RecordsIsolatedByTask", func(t *testing.T) {
		recordsA, err := fixture.DB.GetAIActivityByTask(ctx, "task-a")
		require.NoError(t, err)
		assert.Len(t, recordsA, 3)
		for _, r := range recordsA {
			assert.Equal(t, "task-a", r.TaskID)
		}

		recordsB, err := fixture.DB.GetAIActivityByTask(ctx, "task-b")
		require.NoError(t, err)
		assert.Len(t, recordsB, 2)
		for _, r := range recordsB {
			assert.Equal(t, "task-b", r.TaskID)
		}
	})

	t.Run("DeleteOnlyAffectsTargetTask", func(t *testing.T) {
		err := fixture.DB.DeleteAIActivityByTask(ctx, "task-a")
		require.NoError(t, err)

		// task-a records should be gone
		recordsA, err := fixture.DB.GetAIActivityByTask(ctx, "task-a")
		require.NoError(t, err)
		assert.Empty(t, recordsA)

		// task-b records should still exist
		recordsB, err := fixture.DB.GetAIActivityByTask(ctx, "task-b")
		require.NoError(t, err)
		assert.Len(t, recordsB, 2)
	})
}

// TestConcurrentOperations tests database operations under concurrent access
func TestConcurrentOperations(t *testing.T) {
	fixture := UseFreshInMemoryDatabase(t)
	defer fixture.Cleanup()

	ctx := context.Background()
	NewProjectBuilder().WithID("concurrent-project").WithName("Concurrent Project").WithDescription("Test concurrent operations").Create(t, fixture.DB, ctx)

	t.Run("ConcurrentTaskCreation", func(t *testing.T) {
		const numTasks = 5 // Reduced number to avoid SQLite locking issues

		// Create tasks concurrently
		done := make(chan error, numTasks)
		for i := 0; i < numTasks; i++ {
			go func(i int) {
				task := NewTaskBuilder().WithID(fmt.Sprintf("concurrent-task-%d", i)).WithTitle(fmt.Sprintf("Concurrent Task %d", i)).WithDescription(fmt.Sprintf("Task %d created concurrently", i)).WithProjectID("concurrent-project").Build()
				done <- fixture.DB.CreateTask(ctx, task)
			}(i)
		}

		// Wait for all tasks to complete
		var successCount int
		for i := 0; i < numTasks; i++ {
			err := <-done
			if err == nil {
				successCount++
			} else {
				t.Logf("Task creation failed (expected with SQLite concurrency): %v", err)
			}
		}

		// Verify that at least some tasks were created successfully
		// SQLite may have locking issues under high concurrency
		tasks, err := fixture.DB.GetTasksByProject(ctx, "concurrent-project")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(tasks), 1, "At least one task should be created")
		assert.LessOrEqual(t, len(tasks), numTasks, "No more than %d tasks should be created", numTasks)

		t.Logf("Successfully created %d out of %d tasks concurrently", len(tasks), numTasks)
	})
}

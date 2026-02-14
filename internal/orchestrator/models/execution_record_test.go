// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentConfigJSON_Scan(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    AgentConfigJSON
		wantErr bool
	}{
		{
			name: "valid JSON bytes",
			input: []byte(`{
				"tool_name": "claude",
				"tool_version": "4.5",
				"prompt_template": "Test {{.var}}",
				"variables": {"var": "value"},
				"tool_options": {"model": "claude-sonnet-4-5"}
			}`),
			want: AgentConfigJSON{
				ToolName:       "claude",
				ToolVersion:    "4.5",
				PromptTemplate: "Test {{.var}}",
				Variables:      map[string]string{"var": "value"},
				ToolOptions:    map[string]interface{}{"model": "claude-sonnet-4-5"},
			},
			wantErr: false,
		},
		{
			name: "valid JSON string",
			input: `{
				"tool_name": "claude",
				"prompt_template": "Simple prompt",
				"variables": {}
			}`,
			want: AgentConfigJSON{
				ToolName:       "claude",
				PromptTemplate: "Simple prompt",
				Variables:      map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "nil value",
			input:   nil,
			want:    AgentConfigJSON{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got AgentConfigJSON
			err := got.Scan(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Scan() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Scan() unexpected error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Scan() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAgentConfigJSON_Value(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfigJSON
		wantNil bool
		wantErr bool
	}{
		{
			name: "valid config",
			config: AgentConfigJSON{
				ToolName:       "claude",
				ToolVersion:    "4.5",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{"key": "value"},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "empty config",
			config:  AgentConfigJSON{},
			wantNil: true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.Value()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Value() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Value() unexpected error = %v", err)
				return
			}
			if tt.wantNil && got != nil {
				t.Errorf("Value() expected nil, got %v", got)
				return
			}
			if !tt.wantNil && got == nil {
				t.Errorf("Value() expected non-nil value")
				return
			}

			// If not nil, verify it's valid JSON
			if !tt.wantNil {
				var decoded AgentConfigJSON
				if err := json.Unmarshal(got.([]byte), &decoded); err != nil {
					t.Errorf("Value() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestExecutionRecord_DatabaseOperations(t *testing.T) {
	// Create in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&ExecutionRecord{}); err != nil {
		t.Fatalf("Failed to migrate schema: %v", err)
	}

	// Test data
	now := time.Now()
	testRecord := ExecutionRecord{
		ID:     "exec-001",
		TaskID: "task-001",
		AgentConfig: AgentConfigJSON{
			ToolName:       "claude",
			ToolVersion:    "4.5",
			PromptTemplate: "Analyze {{.file}}",
			Variables:      map[string]string{"file": "main.go"},
			ToolOptions:    map[string]interface{}{"model": "claude-sonnet-4-5"},
		},
		GitCommit:       "abc123",
		GitBranch:       "task/test",
		WorktreePath:    "/tmp/worktree",
		StartTime:       now,
		EndTime:         now.Add(5 * time.Minute),
		Duration:        5 * time.Minute,
		Success:         true,
		ExitCode:        0,
		RawOutput:       "Test output",
		IterationNumber: 1,
	}

	// Create record
	if err := db.Create(&testRecord).Error; err != nil {
		t.Fatalf("Failed to create record: %v", err)
	}

	// Read record back
	var retrieved ExecutionRecord
	if err := db.First(&retrieved, "id = ?", "exec-001").Error; err != nil {
		t.Fatalf("Failed to retrieve record: %v", err)
	}

	// Verify data
	if retrieved.ID != testRecord.ID {
		t.Errorf("ID mismatch: got %v, want %v", retrieved.ID, testRecord.ID)
	}
	if retrieved.TaskID != testRecord.TaskID {
		t.Errorf("TaskID mismatch: got %v, want %v", retrieved.TaskID, testRecord.TaskID)
	}
	if retrieved.AgentConfig.ToolName != testRecord.AgentConfig.ToolName {
		t.Errorf("ToolName mismatch: got %v, want %v", retrieved.AgentConfig.ToolName, testRecord.AgentConfig.ToolName)
	}
	if retrieved.GitCommit != testRecord.GitCommit {
		t.Errorf("GitCommit mismatch: got %v, want %v", retrieved.GitCommit, testRecord.GitCommit)
	}
	if retrieved.Success != testRecord.Success {
		t.Errorf("Success mismatch: got %v, want %v", retrieved.Success, testRecord.Success)
	}

	// Test query by task_id
	var records []ExecutionRecord
	if err := db.Where("task_id = ?", "task-001").Find(&records).Error; err != nil {
		t.Fatalf("Failed to query records: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

func TestExecutionRecord_IterationTracking(t *testing.T) {
	// Create in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&ExecutionRecord{}); err != nil {
		t.Fatalf("Failed to migrate schema: %v", err)
	}

	// Create parent execution
	parent := ExecutionRecord{
		ID:              "exec-001",
		TaskID:          "task-001",
		AgentConfig:     AgentConfigJSON{ToolName: "claude", PromptTemplate: "Test"},
		GitCommit:       "abc123",
		GitBranch:       "main",
		WorktreePath:    "/tmp/worktree",
		IterationNumber: 1,
	}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("Failed to create parent: %v", err)
	}

	// Create child execution
	parentID := "exec-001"
	child := ExecutionRecord{
		ID:                "exec-002",
		TaskID:            "task-001",
		AgentConfig:       AgentConfigJSON{ToolName: "claude", PromptTemplate: "Modified prompt"},
		GitCommit:         "abc123",
		GitBranch:         "main",
		WorktreePath:      "/tmp/worktree",
		ParentExecutionID: &parentID,
		IterationNumber:   2,
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("Failed to create child: %v", err)
	}

	// Query child and verify parent relationship
	var retrieved ExecutionRecord
	if err := db.First(&retrieved, "id = ?", "exec-002").Error; err != nil {
		t.Fatalf("Failed to retrieve child: %v", err)
	}

	if retrieved.ParentExecutionID == nil {
		t.Errorf("ParentExecutionID should not be nil")
	} else if *retrieved.ParentExecutionID != "exec-001" {
		t.Errorf("ParentExecutionID = %v, want exec-001", *retrieved.ParentExecutionID)
	}

	if retrieved.IterationNumber != 2 {
		t.Errorf("IterationNumber = %v, want 2", retrieved.IterationNumber)
	}
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ExecutionRecord tracks the complete history of an agent execution
type ExecutionRecord struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	TaskID    string         `gorm:"index;not null" json:"task_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Agent configuration (stored as JSON)
	AgentConfig AgentConfigJSON `gorm:"type:text;not null" json:"agent_config"`

	// Execution context
	GitCommit    string `gorm:"size:40;not null" json:"git_commit"`
	GitBranch    string `gorm:"size:255;not null" json:"git_branch"`
	WorktreePath string `gorm:"size:512;not null" json:"worktree_path"`

	// Execution results
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	ExitCode  int           `json:"exit_code"`

	// Output
	RawOutput string `gorm:"type:text" json:"raw_output"`
	ErrorMsg  string `gorm:"type:text" json:"error_msg"`

	// Iteration tracking
	ParentExecutionID *string `gorm:"index" json:"parent_execution_id,omitempty"`
	IterationNumber   int     `gorm:"default:1" json:"iteration_number"`
}

// AgentConfigJSON handles JSON serialization for AgentConfig
type AgentConfigJSON struct {
	ToolName       string                 `json:"tool_name"`
	ToolVersion    string                 `json:"tool_version"`
	PromptTemplate string                 `json:"prompt_template"`
	Variables      map[string]string      `json:"variables"`
	ToolOptions    map[string]interface{} `json:"tool_options,omitempty"`
}

// Scan implements the sql.Scanner interface
func (a *AgentConfigJSON) Scan(value any) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, a)
	case string:
		return json.Unmarshal([]byte(v), a)
	default:
		return nil
	}
}

// Value implements the driver.Valuer interface
func (a AgentConfigJSON) Value() (driver.Value, error) {
	if a.ToolName == "" {
		return nil, nil
	}
	return json.Marshal(a)
}

// TableName specifies the table name for ExecutionRecord
func (ExecutionRecord) TableName() string {
	return "execution_records"
}

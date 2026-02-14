// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/noldarim/noldarim/internal/aiobs/types"
	"github.com/noldarim/noldarim/internal/common"

	"gorm.io/gorm"
)

// TaskStatus represents the status of a task
type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusInProgress
	TaskStatusCompleted
	TaskStatusFailed
)

// String returns the string representation of TaskStatus
func (ts TaskStatus) String() string {
	switch ts {
	case TaskStatusPending:
		return "pending"
	case TaskStatusInProgress:
		return "in_progress"
	case TaskStatusCompleted:
		return "completed"
	case TaskStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// ExecHistory represents a JSON array of execution history
type ExecHistory []string

// Scan implements the sql.Scanner interface
func (h *ExecHistory) Scan(value any) error {
	if value == nil {
		*h = []string{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, h)
	case string:
		return json.Unmarshal([]byte(v), h)
	default:
		return errors.New("cannot scan ExecHistory from non-string/[]byte value")
	}
}

// Value implements the driver.Valuer interface
func (h ExecHistory) Value() (driver.Value, error) {
	if len(h) == 0 {
		return "[]", nil
	}
	return json.Marshal(h)
}

// Project represents the GORM model for projects
type Project struct {
	ID             string    `gorm:"primaryKey;type:text" json:"id"`
	Name           string    `gorm:"not null;type:text" json:"name"`
	Description    string    `gorm:"type:text" json:"description"`
	RepositoryPath string    `gorm:"type:text" json:"repository_path"`
	LastUpdatedAt  time.Time `gorm:"autoUpdateTime" json:"last_updated_at"`
	AgentID        string    `gorm:"type:text" json:"agent_id"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relations
	Tasks []Task `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE" json:"tasks,omitempty"`
}

// TableName returns the table name for Project
func (Project) TableName() string {
	return "projects"
}

// Task represents the GORM model for tasks
type Task struct {
	ID            string      `gorm:"primaryKey;type:text" json:"id"`
	Title         string      `gorm:"not null;type:text;uniqueIndex:idx_project_title" json:"title"`
	Description   string      `gorm:"type:text" json:"description"`
	Status        TaskStatus  `gorm:"not null;default:0" json:"status"`
	ProjectID     string      `gorm:"not null;type:text;index;constraint:OnDelete:CASCADE;uniqueIndex:idx_project_title" json:"project_id"`
	ExecHistory   ExecHistory `gorm:"type:text;column:exec_history" json:"exec_history"`
	LastUpdatedAt time.Time   `gorm:"autoUpdateTime" json:"last_updated_at"`
	AgentID       string      `gorm:"type:text;index" json:"agent_id"`
	CreatedAt     time.Time   `gorm:"autoCreateTime" json:"created_at"`
	TaskFilePath  string      `gorm:"type:text" json:"task_file_path"`

	BranchName string `gorm:"type:text" json:"branch_name"`
	GitDiff    string `gorm:"type:text" json:"git_diff"`
}

// TableName returns the table name for Task
func (Task) TableName() string {
	return "tasks"
}

// BeforeCreate is a GORM hook that runs before creating a record
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.LastUpdatedAt.IsZero() {
		p.LastUpdatedAt = now
	}
	return nil
}

// BeforeUpdate is a GORM hook that runs before updating a record
func (p *Project) BeforeUpdate(tx *gorm.DB) error {
	p.LastUpdatedAt = time.Now()
	return nil
}

// BeforeCreate is a GORM hook that runs before creating a record
func (t *Task) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.LastUpdatedAt.IsZero() {
		t.LastUpdatedAt = now
	}
	if t.ExecHistory == nil {
		t.ExecHistory = ExecHistory{}
	}
	return nil
}

// BeforeUpdate is a GORM hook that runs before updating a record
func (t *Task) BeforeUpdate(tx *gorm.DB) error {
	t.LastUpdatedAt = time.Now()
	return nil
}

// AIActivityRecord stores AI activity events for persistence
// EventID is the primary key (no separate auto-increment ID needed since EventID is unique)
type AIActivityRecord struct {
	// Identity
	EventID   string `gorm:"primaryKey;type:text" json:"event_id"`
	SessionID string `gorm:"type:text;index" json:"session_id"`
	TaskID    string `gorm:"type:text;index" json:"task_id"`
	RunID     string `gorm:"type:text;index" json:"run_id"` // Pipeline run ID for aggregating all steps

	// Conversation structure
	MessageUUID string `gorm:"type:text" json:"message_uuid"`
	ParentUUID  string `gorm:"type:text;index" json:"parent_uuid"`
	RequestID   string `gorm:"type:text;index" json:"request_id"` // Groups streaming chunks

	// Classification
	EventType    AIEventType `gorm:"type:text;index;not null" json:"event_type"`
	IsHumanInput *bool     `gorm:"type:boolean" json:"is_human_input"` // true = human typed
	Timestamp    time.Time `gorm:"index;not null" json:"timestamp"`

	// Model info
	Model      string `gorm:"type:text" json:"model"`
	StopReason string `gorm:"type:text" json:"stop_reason"`

	// Token usage
	InputTokens       int `gorm:"type:integer" json:"input_tokens"`
	OutputTokens      int `gorm:"type:integer" json:"output_tokens"`
	CacheReadTokens   int `gorm:"type:integer" json:"cache_read_tokens"`
	CacheCreateTokens int `gorm:"type:integer" json:"cache_create_tokens"`

	// Context tracking
	ContextTokens int `gorm:"type:integer" json:"context_tokens"` // input_tokens = context size
	ContextDepth  int `gorm:"type:integer" json:"context_depth"`  // Depth in parent chain

	// Tool info
	ToolName         string `gorm:"type:text;index" json:"tool_name"`
	ToolInputSummary string `gorm:"type:text" json:"tool_input_summary"` // Truncated human-readable
	ToolSuccess      *bool  `gorm:"type:boolean" json:"tool_success"`
	ToolError        string `gorm:"type:text" json:"tool_error"`
	FilePath         string `gorm:"type:text;index" json:"file_path"` // Extracted for file ops

	// Content
	ContentPreview string `gorm:"type:text" json:"content_preview"` // First 500 chars
	ContentLength  int    `gorm:"type:integer" json:"content_length"`

	// Raw data
	RawPayload string    `gorm:"type:text" json:"raw_payload"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName returns the table name for AIActivityRecord
func (AIActivityRecord) TableName() string {
	return "ai_activity_records"
}

// ToParsedEventFields extracts the fields for a ParsedEvent-style query result.
// This is for reading records back into a display-friendly format.
func (r *AIActivityRecord) ToParsedEventFields() map[string]interface{} {
	return map[string]interface{}{
		"event_id":           r.EventID,
		"session_id":         r.SessionID,
		"task_id":            r.TaskID,
		"message_uuid":       r.MessageUUID,
		"parent_uuid":        r.ParentUUID,
		"request_id":         r.RequestID,
		"event_type":         r.EventType,
		"is_human_input":     r.IsHumanInput,
		"timestamp":          r.Timestamp,
		"model":              r.Model,
		"stop_reason":        r.StopReason,
		"input_tokens":       r.InputTokens,
		"output_tokens":      r.OutputTokens,
		"cache_read_tokens":  r.CacheReadTokens,
		"cache_create_tokens": r.CacheCreateTokens,
		"context_tokens":     r.ContextTokens,
		"context_depth":      r.ContextDepth,
		"tool_name":          r.ToolName,
		"tool_input_summary": r.ToolInputSummary,
		"tool_success":       r.ToolSuccess,
		"tool_error":         r.ToolError,
		"file_path":          r.FilePath,
		"content_preview":    r.ContentPreview,
		"content_length":     r.ContentLength,
	}
}

// NewAIActivityRecordFromParsed creates a record from a ParsedEvent.
// This is THE canonical way to convert parsed transcript events to database records.
// taskID and runID come from workflow context since ParsedEvent doesn't include them.
func NewAIActivityRecordFromParsed(parsed types.ParsedEvent, taskID, runID string) *AIActivityRecord {
	isHuman := parsed.IsHumanInput
	return &AIActivityRecord{
		EventID:           parsed.EventID,
		SessionID:         parsed.SessionID,
		TaskID:            taskID,
		RunID:             runID,
		MessageUUID:       parsed.MessageUUID,
		ParentUUID:        parsed.ParentUUID,
		RequestID:         parsed.RequestID,
		EventType:         AIEventType(parsed.EventType),
		IsHumanInput:      &isHuman,
		Timestamp:         parsed.Timestamp,
		Model:             parsed.Model,
		StopReason:        parsed.StopReason,
		InputTokens:       parsed.InputTokens,
		OutputTokens:      parsed.OutputTokens,
		CacheReadTokens:   parsed.CacheReadTokens,
		CacheCreateTokens: parsed.CacheCreateTokens,
		ContextTokens:     parsed.InputTokens, // input_tokens represents context size
		ToolName:          parsed.ToolName,
		ToolInputSummary:  parsed.ToolInputSummary,
		ToolSuccess:       parsed.ToolSuccess,
		ToolError:         parsed.ToolError,
		FilePath:          parsed.FilePath,
		ContentPreview:    parsed.ContentPreview,
		ContentLength:     parsed.ContentLength,
		RawPayload:        string(parsed.RawPayload),
	}
}

// GetMetadata implements common.Event interface.
// This allows AIActivityRecord to be sent directly through the protocol event channel.
func (r *AIActivityRecord) GetMetadata() common.Metadata {
	return common.Metadata{
		TaskID:         r.TaskID,
		IdempotencyKey: r.EventID,
		Version:        common.CurrentProtocolVersion,
	}
}

// GetRawPayloadJSON returns the raw payload as json.RawMessage for unmarshaling.
// This provides type-safe access to the raw JSON data stored as a string.
func (r *AIActivityRecord) GetRawPayloadJSON() json.RawMessage {
	if r.RawPayload == "" {
		return nil
	}
	return json.RawMessage(r.RawPayload)
}

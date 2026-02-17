// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

// PipelineRunStatus represents the status of a pipeline run
type PipelineRunStatus int

const (
	PipelineRunStatusPending PipelineRunStatus = iota
	PipelineRunStatusRunning
	PipelineRunStatusCompleted
	PipelineRunStatusFailed
)

func (s PipelineRunStatus) String() string {
	switch s {
	case PipelineRunStatusPending:
		return "pending"
	case PipelineRunStatusRunning:
		return "running"
	case PipelineRunStatusCompleted:
		return "completed"
	case PipelineRunStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// StepStatus represents the status of a single step
type StepStatus int

const (
	StepStatusPending StepStatus = iota
	StepStatusRunning
	StepStatusCompleted
	StepStatusFailed
	StepStatusSkipped // For steps before fork point
)

func (s StepStatus) String() string {
	switch s {
	case StepStatusPending:
		return "pending"
	case StepStatusRunning:
		return "running"
	case StepStatusCompleted:
		return "completed"
	case StepStatusFailed:
		return "failed"
	case StepStatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// Pipeline represents a reusable pipeline definition (the "recipe")
// This defines what steps to run, but not the execution itself
type Pipeline struct {
	ID          string          `gorm:"primaryKey;type:text" json:"id"`
	Name        string          `gorm:"not null;type:text" json:"name"`
	Description string          `gorm:"type:text" json:"description"`
	ProjectID   string          `gorm:"type:text;index" json:"project_id"`
	Steps       StepDefinitions `gorm:"type:text" json:"steps"` // JSON array of step definitions

	// Prompt composition - applied to all steps
	PromptPrefix string `gorm:"type:text" json:"prompt_prefix"` // Prepended to all step prompts
	PromptSuffix string `gorm:"type:text" json:"prompt_suffix"` // Appended to all step prompts

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Pipeline) TableName() string {
	return "pipelines"
}

// StepDefinition defines a single step in a pipeline
type StepDefinition struct {
	StepID      string                 `json:"step_id"`      // e.g., "1a", "1b", "1c"
	Name        string                 `json:"name"`         // Human-readable name
	Description string                 `json:"description"`  // What this step does
	AgentConfig *StepAgentConfig       `json:"agent_config"` // Agent configuration for this step
	DependsOn   []string               `json:"depends_on"`   // Step IDs this step depends on (for future DAG support)
	Options     map[string]interface{} `json:"options"`      // Step-specific options
}

// StepAgentConfig holds agent configuration for a pipeline step
type StepAgentConfig struct {
	ToolName       string                 `json:"tool_name"`
	ToolVersion    string                 `json:"tool_version,omitempty"`
	PromptTemplate string                 `json:"prompt_template"`
	Variables      map[string]string      `json:"variables,omitempty"`
	ToolOptions    map[string]interface{} `json:"tool_options,omitempty"`
	FlagFormat     string                 `json:"flag_format,omitempty"`
}

// StepDefinitions is a JSON-serializable slice of StepDefinition
type StepDefinitions []StepDefinition

func (sd *StepDefinitions) Scan(value any) error {
	if value == nil {
		*sd = []StepDefinition{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, sd)
	case string:
		return json.Unmarshal([]byte(v), sd)
	default:
		return errors.New("cannot scan StepDefinitions from non-string/[]byte value")
	}
}

func (sd StepDefinitions) Value() (driver.Value, error) {
	if len(sd) == 0 {
		return "[]", nil
	}
	return json.Marshal(sd)
}

// PipelineRun represents a specific execution of a pipeline
type PipelineRun struct {
	ID         string            `gorm:"primaryKey;type:text" json:"id"`
	PipelineID string            `gorm:"type:text;index" json:"pipeline_id"`
	ProjectID  string            `gorm:"type:text;index" json:"project_id"`
	Name       string            `gorm:"type:text" json:"name"` // Human-readable name for display
	Status     PipelineRunStatus `gorm:"not null;default:0" json:"status"`

	// Fork information (if this run branched from another)
	ParentRunID     string `gorm:"type:text;index" json:"parent_run_id,omitempty"`
	ForkAfterStepID string `gorm:"type:text" json:"fork_after_step_id,omitempty"`
	StartCommitSHA  string `gorm:"type:text" json:"start_commit_sha"`

	// Git information
	BranchName    string `gorm:"type:text" json:"branch_name"`
	BaseCommitSHA string `gorm:"type:text" json:"base_commit_sha"` // Original base before any steps
	HeadCommitSHA string `gorm:"type:text" json:"head_commit_sha"` // Final commit after all steps

	// Prompt composition (stored for idempotency/fork validation)
	PromptPrefix string `gorm:"type:text" json:"prompt_prefix"`
	PromptSuffix string `gorm:"type:text" json:"prompt_suffix"`
	IdentityHash string `gorm:"type:text;index" json:"identity_hash"` // Hash of all inputs affecting output

	// Execution metadata
	WorktreePath string `gorm:"type:text" json:"worktree_path"`
	ContainerID  string `gorm:"type:text" json:"container_id"`

	// Temporal workflow tracking
	TemporalWorkflowID string `gorm:"type:text" json:"temporal_workflow_id"`

	// Timestamps
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	StartedAt   *time.Time `gorm:"type:timestamp" json:"started_at,omitempty"`
	CompletedAt *time.Time `gorm:"type:timestamp" json:"completed_at,omitempty"`

	// Error tracking
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`

	// Relations
	StepResults []StepResult `gorm:"foreignKey:PipelineRunID;constraint:OnDelete:CASCADE" json:"step_results,omitempty"`
}

func (PipelineRun) TableName() string {
	return "pipeline_runs"
}

// StepResult represents the result of executing a single step
type StepResult struct {
	ID            string     `gorm:"primaryKey;type:text" json:"id"`
	PipelineRunID string     `gorm:"type:text;index;not null" json:"pipeline_run_id"`
	StepID        string     `gorm:"type:text;not null" json:"step_id"` // Matches StepDefinition.StepID
	StepName      string     `gorm:"type:text" json:"step_name"`        // Human-readable name from StepDefinition
	StepIndex     int        `gorm:"type:integer" json:"step_index"`    // Order of execution (0, 1, 2...)
	Status        StepStatus `gorm:"not null;default:0" json:"status"`

	// Git results
	CommitSHA     string `gorm:"type:text" json:"commit_sha"`
	CommitMessage string `gorm:"type:text" json:"commit_message"`
	GitDiff       string `gorm:"type:text" json:"git_diff"`

	// Diff statistics
	FilesChanged int `gorm:"type:integer" json:"files_changed"`
	Insertions   int `gorm:"type:integer" json:"insertions"`
	Deletions    int `gorm:"type:integer" json:"deletions"`

	// Token usage
	InputTokens       int `gorm:"type:integer" json:"input_tokens"`
	OutputTokens      int `gorm:"type:integer" json:"output_tokens"`
	CacheReadTokens   int `gorm:"type:integer" json:"cache_read_tokens"`
	CacheCreateTokens int `gorm:"type:integer" json:"cache_create_tokens"`

	// Execution metadata
	AgentOutput  string        `gorm:"type:text" json:"agent_output"`
	Duration     time.Duration `gorm:"type:integer" json:"duration"` // Stored as nanoseconds
	ErrorMessage string        `gorm:"type:text" json:"error_message,omitempty"`

	// Step definition hash for fork comparison - allows detecting unchanged steps
	DefinitionHash string `gorm:"type:text;index" json:"definition_hash"`

	// Timestamps
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	StartedAt   *time.Time `gorm:"type:timestamp" json:"started_at,omitempty"`
	CompletedAt *time.Time `gorm:"type:timestamp" json:"completed_at,omitempty"`
}

func (StepResult) TableName() string {
	return "step_results"
}

// Helper methods

// TotalTokens returns the total tokens used in this step
func (sr *StepResult) TotalTokens() int {
	return sr.InputTokens + sr.OutputTokens
}

// GetCommitAfterStep returns the commit SHA produced by a specific step
func (pr *PipelineRun) GetCommitAfterStep(stepID string) string {
	for _, result := range pr.StepResults {
		if result.StepID == stepID && result.Status == StepStatusCompleted {
			return result.CommitSHA
		}
	}
	return ""
}

// GetLastCompletedStep returns the last successfully completed step
func (pr *PipelineRun) GetLastCompletedStep() *StepResult {
	var last *StepResult
	for i := range pr.StepResults {
		if pr.StepResults[i].Status == StepStatusCompleted {
			last = &pr.StepResults[i]
		}
	}
	return last
}

// IsForked returns true if this run was forked from another
func (pr *PipelineRun) IsForked() bool {
	return pr.ParentRunID != ""
}

// ComputePipelineIdentityHash computes a hash of all inputs that affect pipeline output.
// Used for idempotency checks and fork validation. Changing any of these inputs
// produces a different hash, indicating the pipeline would produce different results.
func ComputePipelineIdentityHash(
	pipelineID string,
	steps []StepDefinition,
	promptPrefix, promptSuffix string,
	baseCommitSHA string,
) string {
	data := struct {
		PipelineID   string           `json:"pipeline_id"`
		Steps        []StepDefinition `json:"steps"`
		PromptPrefix string           `json:"prompt_prefix"`
		PromptSuffix string           `json:"prompt_suffix"`
		BaseCommit   string           `json:"base_commit"`
	}{
		PipelineID:   pipelineID,
		Steps:        steps,
		PromptPrefix: promptPrefix,
		PromptSuffix: promptSuffix,
		BaseCommit:   baseCommitSHA,
	}

	jsonBytes, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:16]) // 32 char hex string
}

// ComputeStepDefinitionHash computes a deterministic hash of a step definition.
// Used for detecting unchanged steps when deciding whether to fork from a previous run.
// Two step definitions with the same hash will produce identical results from identical inputs.
func ComputeStepDefinitionHash(step StepDefinition) string {
	h := sha256.New()

	// Include step identity
	h.Write([]byte(step.StepID))
	h.Write([]byte(step.Name))

	// Include agent configuration (the core of what determines step behavior)
	if step.AgentConfig != nil {
		h.Write([]byte(step.AgentConfig.ToolName))
		h.Write([]byte(step.AgentConfig.ToolVersion))
		h.Write([]byte(step.AgentConfig.PromptTemplate))
		h.Write([]byte(step.AgentConfig.FlagFormat))

		// Sort variables for determinism
		if len(step.AgentConfig.Variables) > 0 {
			varKeys := make([]string, 0, len(step.AgentConfig.Variables))
			for k := range step.AgentConfig.Variables {
				varKeys = append(varKeys, k)
			}
			sort.Strings(varKeys)
			for _, k := range varKeys {
				h.Write([]byte(k))
				h.Write([]byte(step.AgentConfig.Variables[k]))
			}
		}

		// Sort and marshal ToolOptions for determinism
		if len(step.AgentConfig.ToolOptions) > 0 {
			optKeys := make([]string, 0, len(step.AgentConfig.ToolOptions))
			for k := range step.AgentConfig.ToolOptions {
				optKeys = append(optKeys, k)
			}
			sort.Strings(optKeys)
			for _, k := range optKeys {
				h.Write([]byte(k))
				if optBytes, err := json.Marshal(step.AgentConfig.ToolOptions[k]); err == nil {
					h.Write(optBytes)
				}
			}
		}
	}

	// Include step-level Options for completeness
	if len(step.Options) > 0 {
		optKeys := make([]string, 0, len(step.Options))
		for k := range step.Options {
			optKeys = append(optKeys, k)
		}
		sort.Strings(optKeys)
		for _, k := range optKeys {
			h.Write([]byte(k))
			if optBytes, err := json.Marshal(step.Options[k]); err == nil {
				h.Write(optBytes)
			}
		}
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

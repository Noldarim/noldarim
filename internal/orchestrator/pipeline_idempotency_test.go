// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestrator

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/orchestrator/services"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeRunID_Determinism verifies that the same inputs always produce the same run ID
func TestComputeRunID_Determinism(t *testing.T) {
	steps := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				ToolVersion:    "4.5",
				PromptTemplate: "Do something",
				Variables:      map[string]string{"key": "value"},
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps)

	assert.Equal(t, runID1, runID2, "Same inputs should produce same run ID")
	assert.True(t, len(runID1) > 0, "Run ID should not be empty")
	assert.Len(t, runID1, 16, "Run ID should be 16 hex characters")
}

// TestComputeRunID_DifferentCommit verifies different commits produce different IDs
func TestComputeRunID_DifferentCommit(t *testing.T) {
	steps := []models.StepDefinition{
		{StepID: "1", Name: "Test Step"},
	}

	runID1 := services.ComputeRunID("commit-aaa", "v1.0.0", steps)
	runID2 := services.ComputeRunID("commit-bbb", "v1.0.0", steps)

	assert.NotEqual(t, runID1, runID2, "Different commits should produce different run IDs")
}

// TestComputeRunID_DifferentWorkflowVersion verifies different workflow versions produce different IDs
func TestComputeRunID_DifferentWorkflowVersion(t *testing.T) {
	steps := []models.StepDefinition{
		{StepID: "1", Name: "Test Step"},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps)
	runID2 := services.ComputeRunID("abc123", "v2.0.0", steps)

	assert.NotEqual(t, runID1, runID2, "Different workflow versions should produce different run IDs")
}

// TestComputeRunID_DifferentSteps verifies different step definitions produce different IDs
func TestComputeRunID_DifferentSteps(t *testing.T) {
	steps1 := []models.StepDefinition{
		{StepID: "1", Name: "Step One"},
	}
	steps2 := []models.StepDefinition{
		{StepID: "1", Name: "Step Two"},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Different step names should produce different run IDs")
}

// TestComputeRunID_DifferentStepID verifies different step IDs produce different IDs
func TestComputeRunID_DifferentStepID(t *testing.T) {
	steps1 := []models.StepDefinition{
		{StepID: "1a", Name: "Test Step"},
	}
	steps2 := []models.StepDefinition{
		{StepID: "1b", Name: "Test Step"},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Different step IDs should produce different run IDs")
}

// TestComputeRunID_DifferentAgentConfig verifies different agent configs produce different IDs
func TestComputeRunID_DifferentAgentConfig(t *testing.T) {
	steps1 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt A",
			},
		},
	}
	steps2 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt B",
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Different prompts should produce different run IDs")
}

// TestComputeRunID_DifferentVariables verifies different variables produce different IDs
func TestComputeRunID_DifferentVariables(t *testing.T) {
	steps1 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				Variables:      map[string]string{"env": "prod"},
			},
		},
	}
	steps2 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				Variables:      map[string]string{"env": "staging"},
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Different variables should produce different run IDs")
}

// TestComputeRunID_VariableOrderIndependent verifies variable order doesn't affect the hash
func TestComputeRunID_VariableOrderIndependent(t *testing.T) {
	// Create steps with variables in different declaration orders
	// Go maps don't guarantee iteration order, but our code sorts keys
	steps1 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				Variables:      map[string]string{"aaa": "1", "bbb": "2", "ccc": "3"},
			},
		},
	}
	steps2 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				Variables:      map[string]string{"ccc": "3", "aaa": "1", "bbb": "2"},
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.Equal(t, runID1, runID2, "Variable order should not affect run ID (sorted internally)")
}

// TestComputeRunID_NilAgentConfig verifies steps without agent config work correctly
func TestComputeRunID_NilAgentConfig(t *testing.T) {
	steps := []models.StepDefinition{
		{
			StepID:      "1",
			Name:        "Test Step",
			AgentConfig: nil, // No agent config
		},
	}

	runID := services.ComputeRunID("abc123", "v1.0.0", steps)

	assert.NotEmpty(t, runID, "Should handle nil AgentConfig without panic")
	assert.Len(t, runID, 16, "Run ID should be 16 hex characters")
}

// TestComputeRunID_EmptySteps verifies empty steps list works
func TestComputeRunID_EmptySteps(t *testing.T) {
	steps := []models.StepDefinition{}

	runID := services.ComputeRunID("abc123", "v1.0.0", steps)

	assert.NotEmpty(t, runID, "Should handle empty steps list")
	assert.Len(t, runID, 16, "Run ID should be 16 hex characters")
}

// TestComputeRunID_MultipleSteps verifies multiple steps are all included in hash
func TestComputeRunID_MultipleSteps(t *testing.T) {
	steps1 := []models.StepDefinition{
		{StepID: "1", Name: "Step One"},
		{StepID: "2", Name: "Step Two"},
	}
	steps2 := []models.StepDefinition{
		{StepID: "1", Name: "Step One"},
		{StepID: "2", Name: "Step Two Modified"},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Changes in later steps should affect run ID")
}

// TestComputeRunID_StepOrderMatters verifies step order affects the hash
func TestComputeRunID_StepOrderMatters(t *testing.T) {
	steps1 := []models.StepDefinition{
		{StepID: "1", Name: "First"},
		{StepID: "2", Name: "Second"},
	}
	steps2 := []models.StepDefinition{
		{StepID: "2", Name: "Second"},
		{StepID: "1", Name: "First"},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Step order should affect run ID")
}

// TestComputeRunID_ToolVersionIncluded verifies tool version is included in hash
func TestComputeRunID_ToolVersionIncluded(t *testing.T) {
	steps1 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:    "claude",
				ToolVersion: "4.5",
			},
		},
	}
	steps2 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:    "claude",
				ToolVersion: "5.0",
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Different tool versions should produce different run IDs")
}

// TestComputeRunID_DifferentToolOptions verifies different tool options produce different IDs
func TestComputeRunID_DifferentToolOptions(t *testing.T) {
	steps1 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				ToolOptions:    map[string]interface{}{"max_tokens": 1000},
			},
		},
	}
	steps2 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				ToolOptions:    map[string]interface{}{"max_tokens": 2000},
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.NotEqual(t, runID1, runID2, "Different tool options should produce different run IDs")
}

// TestComputeRunID_ToolOptionsOrderIndependent verifies tool options order doesn't affect the hash
func TestComputeRunID_ToolOptionsOrderIndependent(t *testing.T) {
	steps1 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				ToolOptions:    map[string]interface{}{"aaa": 1, "bbb": 2, "ccc": 3},
			},
		},
	}
	steps2 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Prompt",
				ToolOptions:    map[string]interface{}{"ccc": 3, "aaa": 1, "bbb": 2},
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.Equal(t, runID1, runID2, "Tool options order should not affect run ID (sorted internally)")
}

// TestComputeRunID_EmptyToolOptions verifies empty vs nil tool options produce same ID
func TestComputeRunID_EmptyToolOptions(t *testing.T) {
	steps1 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:    "claude",
				ToolOptions: nil,
			},
		},
	}
	steps2 := []models.StepDefinition{
		{
			StepID: "1",
			Name:   "Test Step",
			AgentConfig: &models.StepAgentConfig{
				ToolName:    "claude",
				ToolOptions: map[string]interface{}{},
			},
		},
	}

	runID1 := services.ComputeRunID("abc123", "v1.0.0", steps1)
	runID2 := services.ComputeRunID("abc123", "v1.0.0", steps2)

	assert.Equal(t, runID1, runID2, "Empty and nil tool options should produce same run ID")
}

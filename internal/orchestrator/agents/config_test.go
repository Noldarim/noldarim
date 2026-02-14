// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"testing"
)

func TestAgentConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  AgentConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: AgentConfig{
				ToolName:       "claude",
				ToolVersion:    "4.5",
				PromptTemplate: "Analyze this code: {{.file}}",
				Variables: map[string]string{
					"file": "main.go",
				},
			},
			wantErr: false,
		},
		{
			name: "missing tool name",
			config: AgentConfig{
				PromptTemplate: "Test prompt",
			},
			wantErr: true,
			errMsg:  "tool_name is required",
		},
		{
			name: "missing prompt template",
			config: AgentConfig{
				ToolName: "claude",
			},
			wantErr: true,
			errMsg:  "prompt_template is required",
		},
		{
			name: "unsupported tool",
			config: AgentConfig{
				ToolName:       "gemini",
				PromptTemplate: "Test prompt",
			},
			wantErr: true,
			errMsg:  "unsupported tool: gemini (supported: [claude test])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestAgentConfig_HasVariable(t *testing.T) {
	config := AgentConfig{
		PromptTemplate: "Analyze {{.file}} for {{.issue_type}} issues",
	}

	tests := []struct {
		name     string
		varName  string
		expected bool
	}{
		{
			name:     "existing variable file",
			varName:  "file",
			expected: true,
		},
		{
			name:     "existing variable issue_type",
			varName:  "issue_type",
			expected: true,
		},
		{
			name:     "non-existing variable",
			varName:  "author",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.HasVariable(tt.varName)
			if result != tt.expected {
				t.Errorf("HasVariable(%s) = %v, want %v", tt.varName, result, tt.expected)
			}
		})
	}
}

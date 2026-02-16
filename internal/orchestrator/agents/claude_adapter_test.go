// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"reflect"
	"testing"
)

func TestClaudeAdapter_RenderPrompt(t *testing.T) {
	adapter := NewClaudeAdapter()

	tests := []struct {
		name      string
		template  string
		variables map[string]string
		want      string
		wantErr   bool
	}{
		{
			name:     "simple variable substitution",
			template: "Analyze {{.file}}",
			variables: map[string]string{
				"file": "main.go",
			},
			want:    "Analyze main.go",
			wantErr: false,
		},
		{
			name:     "multiple variables",
			template: "Analyze {{.file}} for {{.issue_type}} issues",
			variables: map[string]string{
				"file":       "main.go",
				"issue_type": "memory",
			},
			want:    "Analyze main.go for memory issues",
			wantErr: false,
		},
		{
			name:      "no variables",
			template:  "Simple prompt with no variables",
			variables: map[string]string{},
			want:      "Simple prompt with no variables",
			wantErr:   false,
		},
		{
			name:     "multiline template",
			template: "Analyze {{.file}}\n\nFocus on:\n- {{.focus}}",
			variables: map[string]string{
				"file":  "main.go",
				"focus": "performance",
			},
			want:    "Analyze main.go\n\nFocus on:\n- performance",
			wantErr: false,
		},
		{
			name:      "unclosed braces are left as-is",
			template:  "Analyze {{.file",
			variables: map[string]string{},
			want:      "Analyze {{.file",
			wantErr:   false,
		},
		{
			name:     "spaced variable syntax",
			template: "Analyze {{ .file }} for {{ .issue_type }}",
			variables: map[string]string{
				"file":       "main.go",
				"issue_type": "memory",
			},
			want:    "Analyze main.go for memory",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adapter.RenderPrompt(tt.template, tt.variables)
			if tt.wantErr {
				if err == nil {
					t.Errorf("RenderPrompt() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("RenderPrompt() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("RenderPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClaudeAdapter_PrepareCommand(t *testing.T) {
	adapter := NewClaudeAdapter()

	tests := []struct {
		name    string
		config  AgentConfig
		want    []string
		wantErr bool
	}{
		{
			name: "basic command",
			config: AgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Analyze {{.file}}",
				Variables: map[string]string{
					"file": "main.go",
				},
			},
			want: []string{
				"claude",
				"--print",
				"Analyze main.go",
			},
			wantErr: false,
		},
		{
			name: "command with model option",
			config: AgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{},
				ToolOptions: map[string]interface{}{
					"model": "claude-sonnet-4-5",
				},
			},
			want: []string{
				"claude",
				"--print",
				"--model",
				"claude-sonnet-4-5",
				"Test prompt",
			},
			wantErr: false,
		},
		{
			name: "command with max_tokens option (ignored)",
			config: AgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{},
				ToolOptions: map[string]interface{}{
					"max_tokens": 4000,
				},
			},
			want: []string{
				"claude",
				"--print",
				"Test prompt",
			},
			wantErr: false,
		},
		{
			name: "command with model option (max_tokens ignored)",
			config: AgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Analyze {{.file}}",
				Variables: map[string]string{
					"file": "main.go",
				},
				ToolOptions: map[string]interface{}{
					"model":      "claude-sonnet-4-5",
					"max_tokens": 2000,
				},
			},
			want: []string{
				"claude",
				"--print",
				"--model",
				"claude-sonnet-4-5",
				"Analyze main.go",
			},
			wantErr: false,
		},
		{
			name: "invalid config - missing tool name",
			config: AgentConfig{
				PromptTemplate: "Test prompt",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid config - unsupported tool",
			config: AgentConfig{
				ToolName:       "gemini",
				PromptTemplate: "Test prompt",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "equals format with model option",
			config: AgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{},
				ToolOptions: map[string]interface{}{
					"model": "claude-sonnet-4-5",
				},
				FlagFormat: "equals",
			},
			want: []string{
				"claude",
				"--print",
				"--model=claude-sonnet-4-5",
				"Test prompt",
			},
			wantErr: false,
		},
		{
			name: "space format with custom flags",
			config: AgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{},
				ToolOptions: map[string]interface{}{
					"model":   "claude-sonnet-4-5",
					"timeout": 30,
					"verbose": true,
				},
				FlagFormat: "space",
			},
			want: []string{
				"claude",
				"--print",
				"--model",
				"claude-sonnet-4-5",
				"--timeout",
				"30",
				"--verbose",
				"Test prompt",
			},
			wantErr: false,
		},
		{
			name: "equals format with custom flags",
			config: AgentConfig{
				ToolName:       "claude",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{},
				ToolOptions: map[string]interface{}{
					"model":   "claude-sonnet-4-5",
					"timeout": 30,
				},
				FlagFormat: "equals",
			},
			want: []string{
				"claude",
				"--print",
				"--model=claude-sonnet-4-5",
				"--timeout=30",
				"Test prompt",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adapter.PrepareCommand(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("PrepareCommand() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("PrepareCommand() unexpected error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrepareCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"reflect"
	"testing"

	"github.com/noldarim/noldarim/internal/protocol"
)

func TestPrepareAgentCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   *protocol.AgentConfigInput
		want    []string
		wantErr bool
	}{
		{
			name: "basic claude config",
			input: &protocol.AgentConfigInput{
				ToolName:       "claude",
				ToolVersion:    "4.5",
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
			name: "claude config with model option",
			input: &protocol.AgentConfigInput{
				ToolName:       "claude",
				PromptTemplate: "Test prompt",
				Variables:      map[string]string{},
				ToolOptions: map[string]interface{}{
					"model":      "claude-sonnet-4-5",
					"max_tokens": 2000, // Ignored by Claude CLI
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
			name:    "nil config",
			input:   nil,
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid config - missing tool name",
			input: &protocol.AgentConfigInput{
				PromptTemplate: "Test",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "test adapter - basic shell command",
			input: &protocol.AgentConfigInput{
				ToolName:       "test",
				PromptTemplate: "echo 'Hello World'",
				Variables:      map[string]string{},
				FlagFormat:     "space",
			},
			want: []string{
				"sh",
				"-c",
				"echo 'Hello World'",
			},
			wantErr: false,
		},
		{
			name: "test adapter - command with template variables",
			input: &protocol.AgentConfigInput{
				ToolName:       "test",
				PromptTemplate: "echo '{{.message}}' > {{.output}}",
				Variables: map[string]string{
					"message": "Test output",
					"output":  "test.txt",
				},
				FlagFormat: "space",
			},
			want: []string{
				"sh",
				"-c",
				"echo 'Test output' > test.txt",
			},
			wantErr: false,
		},
		{
			name: "unsupported tool name",
			input: &protocol.AgentConfigInput{
				ToolName:       "unknown-tool",
				PromptTemplate: "test",
				Variables:      map[string]string{},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PrepareAgentCommand(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("PrepareAgentCommand() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("PrepareAgentCommand() unexpected error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrepareAgentCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

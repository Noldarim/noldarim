// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"bytes"
	"fmt"
	"text/template"
)

// TestAdapter handles test command execution for integration tests
// It executes shell commands directly without invoking external tools
type TestAdapter struct{}

// NewTestAdapter creates a new test adapter
func NewTestAdapter() *TestAdapter {
	return &TestAdapter{}
}

// PrepareCommand prepares a shell command for test execution
// The PromptTemplate contains the actual shell command to execute
func (ta *TestAdapter) PrepareCommand(config AgentConfig) ([]string, error) {
	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Validate that ToolName is "test"
	if config.ToolName != "test" {
		return nil, fmt.Errorf("TestAdapter requires ToolName='test', got '%s'", config.ToolName)
	}

	// Validate that PromptTemplate is not empty
	if config.PromptTemplate == "" {
		return nil, fmt.Errorf("TestAdapter requires non-empty PromptTemplate (contains the shell command)")
	}

	// Render the prompt template with variables
	// For test adapter, the template contains the actual shell command
	renderedCommand, err := ta.renderPrompt(config.PromptTemplate, config.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render command template: %w", err)
	}

	// Return shell command that executes the rendered command
	return []string{"sh", "-c", renderedCommand}, nil
}

// renderPrompt renders a prompt template with variables
// Reuses the same template rendering logic as ClaudeAdapter
func (ta *TestAdapter) renderPrompt(promptTemplate string, variables map[string]string) (string, error) {
	// Parse template
	tmpl, err := template.New("command").Parse(promptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template with variables
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

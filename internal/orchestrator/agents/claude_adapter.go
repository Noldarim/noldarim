// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"bytes"
	"fmt"
	"sort"
	"text/template"
)

// ClaudeAdapter handles execution configuration for Claude AI agent
type ClaudeAdapter struct{}

// NewClaudeAdapter creates a new Claude adapter
func NewClaudeAdapter() *ClaudeAdapter {
	return &ClaudeAdapter{}
}

// PrepareCommand prepares the command to execute Claude with the given configuration
func (ca *ClaudeAdapter) PrepareCommand(config AgentConfig) ([]string, error) {
	// Validate config first
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Render the prompt template with variables
	renderedPrompt, err := ca.RenderPrompt(config.PromptTemplate, config.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	// Build Claude command
	// Format: claude [options] [prompt]
	// --print is required for non-interactive output (automation)
	command := []string{
		"claude",
		"--print", // Non-interactive mode - critical for automation
	}

	// Add optional parameters from ToolOptions
	if config.ToolOptions != nil {
		// Determine flag format (default to "space" if not specified)
		useEquals := config.FlagFormat == "equals"

		// Sort keys for deterministic output
		keys := make([]string, 0, len(config.ToolOptions))
		for key := range config.ToolOptions {
			if key != "max_tokens" { // Skip max_tokens
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := config.ToolOptions[key]
			flagName := "--" + key

			// Handle different value types
			switch v := value.(type) {
			case string:
				if v != "" {
					if useEquals {
						command = append(command, fmt.Sprintf("%s=%s", flagName, v))
					} else {
						command = append(command, flagName, v)
					}
				}
			case bool:
				// Boolean flags are included only if true (no value)
				if v {
					command = append(command, flagName)
				}
			case int, int32, int64:
				valueStr := fmt.Sprintf("%d", v)
				if useEquals {
					command = append(command, fmt.Sprintf("%s=%s", flagName, valueStr))
				} else {
					command = append(command, flagName, valueStr)
				}
			case float32, float64:
				valueStr := fmt.Sprintf("%f", v)
				if useEquals {
					command = append(command, fmt.Sprintf("%s=%s", flagName, valueStr))
				} else {
					command = append(command, flagName, valueStr)
				}
			}
		}
	}

	// Prompt is the last positional argument
	command = append(command, renderedPrompt)

	return command, nil
}

// RenderPrompt renders a prompt template with variables
func (ca *ClaudeAdapter) RenderPrompt(promptTemplate string, variables map[string]string) (string, error) {
	// Parse template
	tmpl, err := template.New("prompt").Parse(promptTemplate)
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

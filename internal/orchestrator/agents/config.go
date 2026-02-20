// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"fmt"
	"strings"
)

// AgentConfig represents the configuration for an AI agent execution
type AgentConfig struct {
	// Tool identification
	ToolName    string `json:"tool_name"`    // "claude" or "test"
	ToolVersion string `json:"tool_version"` // e.g., "4.5"

	// Prompt configuration
	PromptTemplate string            `json:"prompt_template"` // The prompt text, may contain {{.variables}}
	Variables      map[string]string `json:"variables"`       // Values to substitute in template

	// Tool-specific options
	ToolOptions map[string]interface{} `json:"tool_options"` // e.g., model, max_tokens, etc.
	FlagFormat  string                 `json:"flag_format"`  // Format for flags: "space" (--flag value) or "equals" (--flag=value)
}

// Validate checks if the agent configuration is valid
func (ac *AgentConfig) Validate() error {
	if ac.ToolName == "" {
		return fmt.Errorf("tool_name is required")
	}

	if ac.PromptTemplate == "" {
		return fmt.Errorf("prompt_template is required")
	}

	// Supported tools: claude, test
	supportedTools := []string{"claude", "test"}
	isSupported := false
	for _, tool := range supportedTools {
		if ac.ToolName == tool {
			isSupported = true
			break
		}
	}
	if !isSupported {
		return fmt.Errorf("unsupported tool: %s (supported: %v)", ac.ToolName, supportedTools)
	}

	return nil
}

// HasVariable checks if the prompt template contains a specific variable
func (ac *AgentConfig) HasVariable(varName string) bool {
	placeholder := fmt.Sprintf("{{.%s}}", varName)
	return strings.Contains(ac.PromptTemplate, placeholder)
}

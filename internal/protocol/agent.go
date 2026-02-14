// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol

// AgentConfigInput represents the agent configuration for task processing
// This defines how an AI agent or tool should execute a task
type AgentConfigInput struct {
	ToolName       string                 `json:"tool_name"`        // Tool to use: "claude", "test", etc.
	ToolVersion    string                 `json:"tool_version"`     // Version of the tool (e.g., "4.5")
	PromptTemplate string                 `json:"prompt_template"`  // Template with {{.variables}} placeholders
	Variables      map[string]string      `json:"variables"`        // Values to substitute in template
	ToolOptions    map[string]interface{} `json:"tool_options,omitempty"` // Tool-specific options
	FlagFormat     string                 `json:"flag_format,omitempty"`  // "space" or "equals" for CLI flags
}

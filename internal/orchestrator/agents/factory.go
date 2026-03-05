// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import "fmt"

// AgentAdapter defines the interface for all agent adapters
// Adapters translate AgentConfig into tool-specific command arrays
type AgentAdapter interface {
	// PrepareCommand prepares the command array to execute the agent tool
	PrepareCommand(config AgentConfig) ([]string, error)
}

// GetAdapter returns the appropriate adapter for the given tool name
func GetAdapter(toolName string) (AgentAdapter, error) {
	if runtime, ok := GetRuntime(toolName); ok {
		return runtime, nil
	}

	switch toolName {
	case "claude":
		return NewClaudeAdapter(), nil
	case "opencode":
		return NewOpenCodeAdapter(), nil
	case "test":
		return NewTestAdapter(), nil
	default:
		return nil, fmt.Errorf("unsupported tool: %s (supported: claude, opencode, test)", toolName)
	}
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"fmt"

	"github.com/noldarim/noldarim/internal/orchestrator/agents"
	"github.com/noldarim/noldarim/internal/protocol"
)

// PrepareAgentCommand converts an AgentConfigInput into a command ready to execute
func PrepareAgentCommand(input *protocol.AgentConfigInput) ([]string, error) {
	if input == nil {
		return nil, fmt.Errorf("agent config is nil")
	}

	// Convert protocol.AgentConfigInput to agents.AgentConfig
	agentConfig := agents.AgentConfig{
		ToolName:       input.ToolName,
		ToolVersion:    input.ToolVersion,
		PromptTemplate: input.PromptTemplate,
		Variables:      input.Variables,
		ToolOptions:    input.ToolOptions,
		FlagFormat:     input.FlagFormat,
	}

	// Get the appropriate adapter for the tool
	adapter, err := agents.GetAdapter(input.ToolName)
	if err != nil {
		return nil, fmt.Errorf("failed to get adapter for tool '%s': %w", input.ToolName, err)
	}

	// Use adapter to prepare the command
	command, err := adapter.PrepareCommand(agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare command: %w", err)
	}

	return command, nil
}

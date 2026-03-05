// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"fmt"
	"sort"
	"strings"
)

type OpenCodeAdapter struct{}

func NewOpenCodeAdapter() *OpenCodeAdapter {
	return &OpenCodeAdapter{}
}

func (oa *OpenCodeAdapter) PrepareCommand(config AgentConfig) ([]string, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	renderedPrompt, err := oa.renderPrompt(config.PromptTemplate, config.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}

	command := []string{"opencode", "run"}

	if config.ToolOptions != nil {
		useEquals := config.FlagFormat == "equals"
		keys := make([]string, 0, len(config.ToolOptions))
		for key := range config.ToolOptions {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			value := config.ToolOptions[key]
			flag := fmt.Sprintf("--%s", key)

			switch v := value.(type) {
			case bool:
				if v {
					command = append(command, flag)
				}
			case string:
				if useEquals {
					command = append(command, fmt.Sprintf("%s=%s", flag, v))
				} else {
					command = append(command, flag, v)
				}
			default:
				strVal := fmt.Sprintf("%v", value)
				if useEquals {
					command = append(command, fmt.Sprintf("%s=%s", flag, strVal))
				} else {
					command = append(command, flag, strVal)
				}
			}
		}
	}

	command = append(command, "--message", renderedPrompt)

	return command, nil
}

func (oa *OpenCodeAdapter) renderPrompt(template string, vars map[string]string) (string, error) {
	result := template
	for key, value := range vars {
		result = strings.ReplaceAll(result, "{{."+key+"}}", value)
		result = strings.ReplaceAll(result, "{{ ."+key+" }}", value)
	}
	return result, nil
}

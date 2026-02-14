// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// summaryPattern matches the structured summary block in agent output
var summaryPattern = regexp.MustCompile(`(?s)---SUMMARY---\s*(\{.*?\})\s*---END SUMMARY---`)

// ParseStepSummary extracts the structured summary from agent output.
// Returns nil (not an error) if no summary block is found.
func ParseStepSummary(agentOutput string) (*types.StepSummary, error) {
	matches := summaryPattern.FindStringSubmatch(agentOutput)
	if len(matches) < 2 {
		// No summary found - this is not an error, just means AI didn't include one
		return nil, nil
	}

	jsonStr := strings.TrimSpace(matches[1])

	var summary types.StepSummary
	if err := json.Unmarshal([]byte(jsonStr), &summary); err != nil {
		return nil, fmt.Errorf("failed to parse summary JSON: %w", err)
	}

	return &summary, nil
}

// GetAgentOutputWithoutSummary returns the agent output with the summary block removed.
// Useful for displaying cleaner output or storing the "real" response separately.
func GetAgentOutputWithoutSummary(agentOutput string) string {
	return strings.TrimSpace(summaryPattern.ReplaceAllString(agentOutput, ""))
}

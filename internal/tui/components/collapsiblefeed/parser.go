// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package collapsiblefeed

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// ParseRecords converts a slice of AIActivityRecords into ActivityGroups.
// It pairs tool_use events with their corresponding tool_result events.
func ParseRecords(records []*models.AIActivityRecord) []ActivityGroup {
	var groups []ActivityGroup
	// Queue of pending group indices per tool name (FIFO for matching)
	pendingQueues := make(map[string][]int)

	for _, record := range records {
		if record == nil {
			continue
		}

		switch record.EventType {
		case models.AIEventToolUse:
			group := parseToolUse(record)
			groups = append(groups, group)
			// Add to pending queue for this tool
			idx := len(groups) - 1
			pendingQueues[record.ToolName] = append(pendingQueues[record.ToolName], idx)

		case models.AIEventToolResult:
			// Find oldest matching pending group (FIFO)
			if queue, ok := pendingQueues[record.ToolName]; ok && len(queue) > 0 {
				idx := queue[0]
				pendingQueues[record.ToolName] = queue[1:] // Pop from front

				result := parseToolResult(record, groups[idx].ToolName)
				groups[idx].Result = &result
				if result.Success {
					groups[idx].State = StateCompleted
				} else {
					groups[idx].State = StateFailed
				}
			}
			// If no matching group found, skip the orphan result

		case models.AIEventThinking:
			// Could add thinking as a special group type if desired
			// For now, skip thinking events

		case models.AIEventAIOutput:
			// Could add AI output as inline text if desired
			// For now, skip
		}
	}

	return groups
}

// parseToolUse creates an ActivityGroup from a tool_use record
func parseToolUse(record *models.AIActivityRecord) ActivityGroup {
	group := ActivityGroup{
		ID:       record.EventID,
		ToolName: record.ToolName,
		State:    StatePending,
		Expanded: false,
		Input:    parseToolInput(record),
	}

	return group
}

// parseToolInput extracts structured input from the record
func parseToolInput(record *models.AIActivityRecord) ToolInput {
	input := ToolInput{
		Raw:      record.ContentPreview, // ContentPreview usually has the JSON
		FilePath: record.FilePath,
	}

	// If no content preview, fall back to tool input summary
	if input.Raw == "" {
		input.Raw = record.ToolInputSummary
	}

	// Try to parse tool-specific data
	switch record.ToolName {
	case "Bash":
		input.Command, input.Description = parseBashInput(record)
	case "TodoWrite":
		input.Todos = parseTodoWriteInput(record)
	case "Grep":
		input.Pattern = parseGrepPattern(record)
	case "Task":
		input.Preview = parseTaskDescription(record)
	}

	return input
}

// parseBashInput extracts command and description from Bash tool input
func parseBashInput(record *models.AIActivityRecord) (command, description string) {
	// Try content preview first - this is where the JSON usually lives
	if record.ContentPreview != "" {
		var simple struct {
			Command     string `json:"command"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal([]byte(record.ContentPreview), &simple); err == nil {
			if simple.Command != "" || simple.Description != "" {
				return simple.Command, simple.Description
			}
		}
	}

	// Try tool input summary (might be plain text command)
	if record.ToolInputSummary != "" {
		// Check if it's JSON
		var simple struct {
			Command     string `json:"command"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal([]byte(record.ToolInputSummary), &simple); err == nil {
			return simple.Command, simple.Description
		}
		// It's plain text - treat as command
		return record.ToolInputSummary, ""
	}

	// Try raw payload with nested input structure
	if record.RawPayload != "" {
		var payload struct {
			Input struct {
				Command     string `json:"command"`
				Description string `json:"description"`
			} `json:"input"`
		}
		if err := json.Unmarshal([]byte(record.RawPayload), &payload); err == nil {
			return payload.Input.Command, payload.Input.Description
		}
	}

	return command, description
}

// parseTodoWriteInput extracts todos from TodoWrite input
func parseTodoWriteInput(record *models.AIActivityRecord) []TodoItem {
	// Try content preview first - this is where the JSON usually lives
	if record.ContentPreview != "" {
		var direct struct {
			Todos []TodoItem `json:"todos"`
		}
		if err := json.Unmarshal([]byte(record.ContentPreview), &direct); err == nil && len(direct.Todos) > 0 {
			return direct.Todos
		}
	}

	// Try tool input summary
	if record.ToolInputSummary != "" {
		var direct struct {
			Todos []TodoItem `json:"todos"`
		}
		if err := json.Unmarshal([]byte(record.ToolInputSummary), &direct); err == nil && len(direct.Todos) > 0 {
			return direct.Todos
		}
	}

	// Try raw payload with nested input structure
	if record.RawPayload != "" {
		var payload struct {
			Input struct {
				Todos []TodoItem `json:"todos"`
			} `json:"input"`
		}
		if err := json.Unmarshal([]byte(record.RawPayload), &payload); err == nil && len(payload.Input.Todos) > 0 {
			return payload.Input.Todos
		}
	}

	return nil
}

// parseGrepPattern extracts the search pattern from Grep input
func parseGrepPattern(record *models.AIActivityRecord) string {
	// Try content preview first
	if record.ContentPreview != "" {
		var simple struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(record.ContentPreview), &simple); err == nil && simple.Pattern != "" {
			return simple.Pattern
		}
	}

	// Try tool input summary
	if record.ToolInputSummary != "" {
		var simple struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal([]byte(record.ToolInputSummary), &simple); err == nil && simple.Pattern != "" {
			return simple.Pattern
		}
	}

	// Try raw payload
	if record.RawPayload != "" {
		var payload struct {
			Input struct {
				Pattern string `json:"pattern"`
			} `json:"input"`
		}
		if err := json.Unmarshal([]byte(record.RawPayload), &payload); err == nil {
			return payload.Input.Pattern
		}
	}

	return ""
}

// parseTaskDescription extracts description from Task tool input
func parseTaskDescription(record *models.AIActivityRecord) string {
	// Try content preview first
	if record.ContentPreview != "" {
		var simple struct {
			Description string `json:"description"`
			Prompt      string `json:"prompt"`
		}
		if err := json.Unmarshal([]byte(record.ContentPreview), &simple); err == nil {
			if simple.Description != "" {
				return simple.Description
			}
			if simple.Prompt != "" {
				return simple.Prompt
			}
		}
	}

	// Try raw payload
	if record.RawPayload != "" {
		var payload struct {
			Input struct {
				Description string `json:"description"`
				Prompt      string `json:"prompt"`
			} `json:"input"`
		}
		if err := json.Unmarshal([]byte(record.RawPayload), &payload); err == nil {
			if payload.Input.Description != "" {
				return payload.Input.Description
			}
			return payload.Input.Prompt
		}
	}
	return ""
}

// parseToolResult creates a ToolResult from a tool_result record
func parseToolResult(record *models.AIActivityRecord, toolName string) ToolResult {
	result := ToolResult{
		Success: record.ToolSuccess != nil && *record.ToolSuccess,
		Error:   record.ToolError,
		Output:  record.ContentPreview,
	}

	// Parse tool-specific result data
	switch toolName {
	case "Read":
		result.LineCount = parseLineCount(record)
	case "TodoWrite":
		result.TodoCount = parseTodoCount(record)
	}

	// Generate summary output if not present
	if result.Output == "" && result.Success {
		result.Output = "done"
	}

	return result
}

// parseLineCount extracts line count from Read result
func parseLineCount(record *models.AIActivityRecord) int {
	// Try to extract from content preview like "[file.go] 150 lines"
	if record.ContentPreview != "" {
		re := regexp.MustCompile(`(\d+)\s+lines?`)
		if matches := re.FindStringSubmatch(record.ContentPreview); len(matches) > 1 {
			var count int
			fmt.Sscanf(matches[1], "%d", &count)
			return count
		}
	}
	return 0
}

// parseTodoCount extracts todo count from TodoWrite result
func parseTodoCount(record *models.AIActivityRecord) int {
	// Try to extract from content preview like "Updated todos (7 items)"
	if record.ContentPreview != "" {
		re := regexp.MustCompile(`(\d+)\s+items?`)
		if matches := re.FindStringSubmatch(record.ContentPreview); len(matches) > 1 {
			var count int
			fmt.Sscanf(matches[1], "%d", &count)
			return count
		}
	}
	return 0
}

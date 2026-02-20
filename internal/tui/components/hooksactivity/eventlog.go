// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package hooksactivity

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// Event icons for different event types
const (
	iconToolCall   = ">"
	iconToolResult = "<"
	iconStop       = "X"
	iconError      = "!"
	iconSession    = "*"
	iconThinking   = "~"
	iconOutput     = "-"
)

var (
	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	toolCallStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")). // Cyan
			Bold(true)

	toolResultOKStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	toolResultErrStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")) // Red

	stopStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")). // Orange
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			Bold(true)

	sessionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")) // Purple

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")) // Light gray
)

// RenderEventLog renders the chronological activity log
func RenderEventLog(events []*models.AIActivityRecord, width int) string {
	if len(events) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Render("No activity yet...")
	}

	var lines []string
	for _, record := range events {
		line := renderEventLine(record, width)
		if line != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// renderEventLine renders a single record as a log line
func renderEventLine(record *models.AIActivityRecord, width int) string {
	timestamp := record.Timestamp.Format("15:04:05")
	ts := timestampStyle.Render(timestamp)

	// Calculate available width for content
	contentWidth := width - 12 // timestamp + icon + spaces

	switch record.EventType {
	case models.AIEventToolUse:
		return renderToolCall(ts, record, contentWidth)

	case models.AIEventToolResult:
		return renderToolResult(ts, record, contentWidth)

	case models.AIEventSessionEnd, models.AIEventStop:
		return renderStop(ts, record)

	case models.AIEventError:
		return renderError(ts, record, contentWidth)

	case models.AIEventSessionStart:
		return fmt.Sprintf("%s %s Session started",
			ts, sessionStyle.Render(iconSession))

	case models.AIEventSubagentStart:
		return fmt.Sprintf("%s %s Subagent started",
			ts, sessionStyle.Render("🤖"))

	case models.AIEventSubagentStop:
		return fmt.Sprintf("%s %s Subagent stopped",
			ts, sessionStyle.Render("🤖"))

	case models.AIEventUserPrompt:
		prompt := extractUserPrompt(record)
		if prompt != "" {
			prompt = truncate(prompt, contentWidth-20)
			return fmt.Sprintf("%s %s User: %s",
				ts, sessionStyle.Render("💬"), inputStyle.Render(prompt))
		}
		return fmt.Sprintf("%s %s User prompt submitted",
			ts, sessionStyle.Render("💬"))

	case models.AIEventThinking:
		content := truncate(record.ContentPreview, contentWidth-10)
		return fmt.Sprintf("%s %s %s",
			ts, sessionStyle.Render(iconThinking), inputStyle.Render(content))

	case models.AIEventAIOutput:
		content := truncate(record.ContentPreview, contentWidth-10)
		return fmt.Sprintf("%s %s %s",
			ts, sessionStyle.Render(iconOutput), inputStyle.Render(content))

	default:
		// Show unknown event types for debugging
		if record.EventType != "" {
			return fmt.Sprintf("%s %s %s",
				ts, sessionStyle.Render("❓"), string(record.EventType))
		}
		// Event with no type - likely old data
		return fmt.Sprintf("%s %s error no type",
			ts, errorStyle.Render(iconError))
	}
}

func renderToolCall(ts string, record *models.AIActivityRecord, maxWidth int) string {
	if record.ToolName == "" {
		return ""
	}

	icon := toolCallStyle.Render(iconToolCall)
	toolName := toolCallStyle.Render(record.ToolName)

	// Use ToolInputSummary for display
	inputSummary := ""
	if record.ToolInputSummary != "" {
		inputSummary = truncate(record.ToolInputSummary, maxWidth-len(record.ToolName)-5)
		inputSummary = inputStyle.Render(": " + inputSummary)
	}

	return fmt.Sprintf("%s %s %s%s", ts, icon, toolName, inputSummary)
}

func renderToolResult(ts string, record *models.AIActivityRecord, maxWidth int) string {
	icon := toolResultOKStyle.Render(iconToolResult)
	status := toolResultOKStyle.Render("[OK]")

	// Check success status
	if record.ToolSuccess != nil && !*record.ToolSuccess {
		icon = toolResultErrStyle.Render(iconToolResult)
		status = toolResultErrStyle.Render("[ERR]")
	}

	toolName := record.ToolName

	// Add error message if present
	extra := ""
	if record.ToolError != "" {
		extra = " " + truncate(record.ToolError, maxWidth-len(toolName)-10)
	}

	return fmt.Sprintf("%s %s %s %s%s", ts, icon, toolName, status, extra)
}

func renderStop(ts string, record *models.AIActivityRecord) string {
	icon := stopStyle.Render(iconStop)

	reason := "completed"
	if record.StopReason != "" {
		reason = record.StopReason
	}

	stats := ""
	if record.InputTokens > 0 || record.OutputTokens > 0 {
		totalTokens := record.InputTokens + record.OutputTokens
		stats = fmt.Sprintf(" (tokens: %d)", totalTokens)
	}

	return fmt.Sprintf("%s %s Session ended: %s%s", ts, icon, reason, stats)
}

func renderError(ts string, record *models.AIActivityRecord, maxWidth int) string {
	icon := errorStyle.Render(iconError)

	msg := "Unknown error"
	if record.ContentPreview != "" {
		msg = truncate(record.ContentPreview, maxWidth-5)
	}

	return fmt.Sprintf("%s %s %s", ts, icon, errorStyle.Render(msg))
}

// truncate shortens a string to max length with ellipsis
func truncate(s string, max int) string {
	if max <= 3 {
		return "..."
	}
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// extractUserPrompt attempts to extract the user prompt from a record's raw payload
func extractUserPrompt(record *models.AIActivityRecord) string {
	if record.RawPayload == "" {
		return ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(record.RawPayload), &data); err != nil {
		return ""
	}

	// Claude hooks may include the prompt in various fields
	for _, key := range []string{"prompt", "message", "content", "user_message"} {
		if v, ok := data[key].(string); ok && v != "" {
			return v
		}
	}

	return ""
}

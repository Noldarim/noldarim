// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/noldarim/noldarim/internal/orchestrator/agents"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// StepDocumentationActivities provides activities for generating step documentation
type StepDocumentationActivities struct{}

// NewStepDocumentationActivities creates a new instance of StepDocumentationActivities
func NewStepDocumentationActivities() *StepDocumentationActivities {
	return &StepDocumentationActivities{}
}

// GenerateStepDocumentationActivity creates a markdown documentation file for a pipeline step.
// The file is written to the worktree so it gets committed with the step's changes.
func (a *StepDocumentationActivities) GenerateStepDocumentationActivity(
	ctx context.Context,
	input types.GenerateStepDocumentationActivityInput,
) (*types.GenerateStepDocumentationActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Generating step documentation",
		"runID", input.RunID,
		"stepID", input.StepID,
		"stepName", input.StepName)

	activity.RecordHeartbeat(ctx, "Parsing agent output for summary")

	// Parse summary from agent output
	summary, err := agents.ParseStepSummary(input.AgentOutput)
	if err != nil {
		logger.Warn("Failed to parse summary from agent output", "error", err)
		// Continue without summary - not fatal
	}

	activity.RecordHeartbeat(ctx, "Generating markdown content")

	// Generate markdown content
	content := generateStepMarkdown(input, summary)

	// Create directory structure: docs/ai-history/runs/{runID}/
	docDir := filepath.Join(input.WorktreePath, "docs", "ai-history", "runs", input.RunID)
	if err := os.MkdirAll(docDir, 0755); err != nil {
		logger.Error("Failed to create documentation directory", "error", err, "path", docDir)
		return &types.GenerateStepDocumentationActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to create directory: %v", err),
		}, nil // Return non-fatal error
	}

	// Write documentation file
	docFileName := fmt.Sprintf("step-%s.md", input.StepID)
	docPath := filepath.Join(docDir, docFileName)

	if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
		logger.Error("Failed to write documentation file", "error", err, "path", docPath)
		return &types.GenerateStepDocumentationActivityOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to write file: %v", err),
		}, nil // Return non-fatal error
	}

	// Calculate relative path for return value
	relPath, _ := filepath.Rel(input.WorktreePath, docPath)

	logger.Info("Step documentation generated successfully",
		"path", relPath,
		"hasSummary", summary != nil)

	return &types.GenerateStepDocumentationActivityOutput{
		Success:      true,
		DocumentPath: relPath,
		Summary:      summary,
	}, nil
}

// generateStepMarkdown creates the markdown content for a step's documentation
func generateStepMarkdown(input types.GenerateStepDocumentationActivityInput, summary *types.StepSummary) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Step %s: %s\n\n", input.StepID, input.StepName))
	sb.WriteString(fmt.Sprintf("**Run ID**: `%s`\n", input.RunID))
	sb.WriteString(fmt.Sprintf("**Step Index**: %d\n", input.StepIndex))
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n\n", time.Now().Format(time.RFC3339)))

	// Summary section (if AI provided one)
	if summary != nil {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(fmt.Sprintf("**Reason**: %s\n\n", summary.Reason))
		if len(summary.Changes) > 0 {
			sb.WriteString("**Changes**:\n")
			for _, change := range summary.Changes {
				sb.WriteString(fmt.Sprintf("- %s\n", change))
			}
			sb.WriteString("\n")
		}
	}

	// Change statistics
	sb.WriteString("## Changes\n\n")
	sb.WriteString(fmt.Sprintf("- **Files changed**: %d\n", len(input.FilesChanged)))
	sb.WriteString(fmt.Sprintf("- **Insertions**: +%d\n", input.Insertions))
	sb.WriteString(fmt.Sprintf("- **Deletions**: -%d\n", input.Deletions))
	sb.WriteString("\n")

	// Files list
	if len(input.FilesChanged) > 0 {
		sb.WriteString("### Modified Files\n\n")
		for _, f := range input.FilesChanged {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
		sb.WriteString("\n")
	}

	// Prompt that was used (original template, without system instructions)
	sb.WriteString("## Prompt\n\n")
	sb.WriteString("```\n")
	sb.WriteString(strings.TrimSpace(input.PromptUsed))
	sb.WriteString("\n```\n\n")

	// Git diff (collapsed for readability)
	if input.GitDiff != "" {
		sb.WriteString("<details>\n<summary>Git Diff</summary>\n\n")
		sb.WriteString("```diff\n")
		sb.WriteString(input.GitDiff)
		sb.WriteString("\n```\n\n")
		sb.WriteString("</details>\n")
	}

	return sb.String()
}

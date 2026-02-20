// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal/types"
)

// TaskFileActivities provides task file-related activities
type TaskFileActivities struct {
	config *config.AppConfig
}

// NewTaskFileActivities creates a new instance of TaskFileActivities
func NewTaskFileActivities(config *config.AppConfig) *TaskFileActivities {
	return &TaskFileActivities{
		config: config,
	}
}

// sanitizeFilename creates a safe filename from the task title
func sanitizeFilename(title string) string {
	// Remove special characters and convert to lowercase
	reg := regexp.MustCompile(`[^a-zA-Z0-9\s-]`)
	sanitized := reg.ReplaceAllString(title, "")

	// Replace spaces with hyphens and convert to lowercase
	sanitized = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(sanitized)), " ", "-")

	// Remove multiple consecutive hyphens
	reg = regexp.MustCompile(`-+`)
	sanitized = reg.ReplaceAllString(sanitized, "-")

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "task"
	}

	// Limit length to avoid filesystem limits
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
		sanitized = strings.Trim(sanitized, "-")
	}

	return sanitized
}

// WriteTaskFileActivity writes task details to a markdown file in the repository
func (a *TaskFileActivities) WriteTaskFileActivity(ctx context.Context, input types.WriteTaskFileActivityInput) (*types.WriteTaskFileActivityOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Writing task to file", "taskID", input.TaskID, "title", input.Title, "repositoryPath", input.RepositoryPath)

	// Record heartbeat
	activity.RecordHeartbeat(ctx, "Writing task file")

	// Create the directory structure
	tasksDir := filepath.Join(input.RepositoryPath, "noldarim-system-progress-log", "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		logger.Error("Failed to create tasks directory", "error", err, "path", tasksDir)
		return nil, fmt.Errorf("failed to create tasks directory: %w", err)
	}

	// Generate filename with timestamp and sanitized title
	timestamp := time.Now().Format("20060102-150405")
	sanitizedTitle := sanitizeFilename(input.Title)
	filename := fmt.Sprintf("%s-%s.md", timestamp, sanitizedTitle)
	fullPath := filepath.Join(tasksDir, filename)

	// Check if file already exists for idempotency
	// We'll check for files with the same task ID pattern to handle retries
	existingFiles, err := filepath.Glob(filepath.Join(tasksDir, fmt.Sprintf("*-%s.md", sanitizedTitle)))
	if err != nil {
		logger.Error("Failed to check for existing files", "error", err)
		return nil, fmt.Errorf("failed to check for existing files: %w", err)
	}

	// If we found existing files with the same sanitized title, check their content
	for _, existingFile := range existingFiles {
		content, err := os.ReadFile(existingFile)
		if err != nil {
			continue
		}
		// Check if this file is for the same task (by checking title match)
		if strings.Contains(string(content), fmt.Sprintf("Title: %s", input.Title)) {
			// Found an existing file for this task, return its path for idempotency
			relativePath, err := filepath.Rel(input.RepositoryPath, existingFile)
			if err != nil {
				relativePath = existingFile
			}
			logger.Info("Task file already exists, returning existing", "path", relativePath)
			return &types.WriteTaskFileActivityOutput{
				FilePath: relativePath,
			}, nil
		}
	}

	// Create the markdown content
	content := fmt.Sprintf("Title: %s\n\nDescription: %s\n", input.Title, input.Description)

	// Write the file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logger.Error("Failed to write task file", "error", err, "path", fullPath)
		return nil, fmt.Errorf("failed to write task file: %w", err)
	}

	// Calculate relative path from repository root
	relativePath, err := filepath.Rel(input.RepositoryPath, fullPath)
	if err != nil {
		// If we can't calculate relative path, use the full path
		relativePath = fullPath
	}

	// Extract just the filename for commit purposes
	fileName := filepath.Base(fullPath)

	logger.Info("Successfully wrote task file", "path", relativePath, "fileName", fileName)
	return &types.WriteTaskFileActivityOutput{
		FilePath: relativePath,
		FileName: fileName,
	}, nil
}

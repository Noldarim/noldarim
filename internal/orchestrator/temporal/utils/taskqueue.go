// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package utils provides shared utility functions for Temporal workflows and activities.
package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// GenerateTaskQueueName creates a sanitized task queue name from task title and ID.
// This function is used by both CreateTaskWorkflow and dev tools to ensure
// consistent task queue naming across the system.
func GenerateTaskQueueName(title, taskID string) string {
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

	// Limit length to avoid Temporal limits (max 1000 chars for task queue)
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
		sanitized = strings.Trim(sanitized, "-")
	}

	return fmt.Sprintf("task-queue-%s-%s", sanitized, taskID)
}

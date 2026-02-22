// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package utils provides shared utility functions for Temporal workflows and activities.
package utils

import "fmt"

// GenerateTaskQueueName creates a deterministic task queue name from a task/run ID.
// The name is an opaque Temporal identifier — only the ID is needed for uniqueness.
// This function is used by both workflows and dev tools to ensure consistent naming.
func GenerateTaskQueueName(taskID string) string {
	return fmt.Sprintf("task-queue-%s", taskID)
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// PublishEventInput is a unified input type for all event publishing activities.
// This replaces the individual input types like PublishTaskCreatedEventInput,
// PublishTaskInProgressEventInput, etc.
//
// Use the typed payload fields to preserve type info during Temporal serialization:
// - Task: for TaskCreated events
// - AIRecord: for AIActivity events
// - For simple lifecycle events (InProgress, Finished, etc.): leave payloads nil
type PublishEventInput struct {
	ProjectID string
	TaskID    string
	// Task is used for TaskCreated events
	Task *models.Task
	// AIRecord is used for AIActivity events
	AIRecord *models.AIActivityRecord
	// Status is used for TaskStatusUpdated events
	Status models.TaskStatus
}

// PublishErrorEventInput remains separate as it has different fields
type PublishErrorEventInput struct {
	Message      string
	ErrorContext string
	TaskID       string
}

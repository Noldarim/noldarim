// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package messages

import "github.com/noldarim/noldarim/internal/orchestrator/models"

// Navigation messages for screen transitions within the TUI
type GoBackMsg struct{}

type GoToTasksScreenMsg struct {
	ProjectID string
}

type GoToTaskDetailsMsg struct {
	Task      *models.Task
	ProjectID string
}

type GoToSettingsMsg struct{}

type GoToProjectListMsg struct{}

type GoToProjectCreationMsg struct{}

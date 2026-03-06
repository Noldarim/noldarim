// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol

import "github.com/noldarim/noldarim/internal/orchestrator/models"

// ContainerLogEvent wraps container logs for API responses.
type ContainerLogEvent struct {
	Metadata
	RunID string               `json:"run_id"`
	Logs  []*models.ContainerLog `json:"logs"`
}

func (e ContainerLogEvent) GetMetadata() Metadata {
	return e.Metadata
}

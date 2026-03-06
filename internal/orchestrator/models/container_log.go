// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import "time"

// ContainerLog stores captured container output (stdout/stderr) for a pipeline step.
type ContainerLog struct {
	ID          string    `gorm:"primaryKey;type:text" json:"id"`
	RunID       string    `gorm:"type:text;index;not null" json:"run_id"`
	StepID      string    `gorm:"type:text;index" json:"step_id"`
	ContainerID string    `gorm:"type:text;index" json:"container_id"`
	Stream      string    `gorm:"type:text;not null" json:"stream"` // "stdout" or "stderr"
	Content     string    `gorm:"type:text" json:"content"`
	Timestamp   time.Time `gorm:"index;not null" json:"timestamp"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName returns the table name for ContainerLog.
func (ContainerLog) TableName() string {
	return "container_logs"
}

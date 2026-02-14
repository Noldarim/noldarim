// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package utils

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"github.com/noldarim/noldarim/internal/config"
)

// GetActivityOptions returns workflow.ActivityOptions from config
func GetActivityOptions(cfg *config.AppConfig) workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout:    cfg.Temporal.Activity.StartToCloseTimeout,
		ScheduleToCloseTimeout: cfg.Temporal.Activity.ScheduleToCloseTimeout,
		HeartbeatTimeout:       cfg.Temporal.Activity.HeartbeatTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    cfg.Temporal.Activity.RetryPolicy.InitialInterval,
			BackoffCoefficient: cfg.Temporal.Activity.RetryPolicy.BackoffCoefficient,
			MaximumInterval:    cfg.Temporal.Activity.RetryPolicy.MaximumInterval,
			MaximumAttempts:    cfg.Temporal.Activity.RetryPolicy.MaximumAttempts,
		},
	}
}

// GetWorkflowExecutionTimeout returns the workflow execution timeout from config
func GetWorkflowExecutionTimeout(cfg *config.AppConfig) time.Duration {
	return cfg.Temporal.Workflow.WorkflowExecutionTimeout
}

// GetWorkflowRunTimeout returns the workflow run timeout from config
func GetWorkflowRunTimeout(cfg *config.AppConfig) time.Duration {
	return cfg.Temporal.Workflow.WorkflowRunTimeout
}

// GetWorkflowTaskTimeout returns the workflow task timeout from config
func GetWorkflowTaskTimeout(cfg *config.AppConfig) time.Duration {
	return cfg.Temporal.Workflow.WorkflowTaskTimeout
}

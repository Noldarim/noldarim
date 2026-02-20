// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package services

import (
	"context"

	"go.temporal.io/sdk/client"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal"
)

// TemporalClient defines the methods used by services from the temporal client.
// Owned by the services package so both orchestrator and temporal/client satisfy it.
type TemporalClient interface {
	StartWorkflow(ctx context.Context, workflowID string, workflow interface{}, args ...interface{}) (client.WorkflowRun, error)
	GetWorkflowStatus(ctx context.Context, workflowID string) (temporal.WorkflowStatus, error)
	SignalWorkflow(ctx context.Context, workflowID, signalName string, arg interface{}) error
	QueryWorkflow(ctx context.Context, workflowID, queryType string, args ...interface{}) (interface{}, error)
	CancelWorkflow(ctx context.Context, workflowID string) error
	GetTemporalClient() client.Client
	GetTaskQueue() string
	Close() error
}

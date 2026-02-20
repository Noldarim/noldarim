// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package temporal

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"github.com/noldarim/noldarim/internal/logger"
)

// WorkflowStatus represents the current status of a workflow
type WorkflowStatus int

const (
	WorkflowStatusUnknown WorkflowStatus = iota
	WorkflowStatusRunning
	WorkflowStatusCompleted
	WorkflowStatusFailed
	WorkflowStatusCanceled
	WorkflowStatusTerminated
	WorkflowStatusTimedOut
)

// String returns the string representation of WorkflowStatus
func (s WorkflowStatus) String() string {
	switch s {
	case WorkflowStatusRunning:
		return "running"
	case WorkflowStatusCompleted:
		return "completed"
	case WorkflowStatusFailed:
		return "failed"
	case WorkflowStatusCanceled:
		return "canceled"
	case WorkflowStatusTerminated:
		return "terminated"
	case WorkflowStatusTimedOut:
		return "timed_out"
	default:
		return "unknown"
	}
}

var (
	temporalLog     *zerolog.Logger
	temporalLogOnce sync.Once
)

func getTemporalLog() *zerolog.Logger {
	temporalLogOnce.Do(func() {
		l := logger.GetTemporalLogger().With().Str("component", "client").Logger()
		temporalLog = &l
	})
	return temporalLog
}

// Client wraps the Temporal client and provides additional functionality
type Client struct {
	temporalClient client.Client
	namespace      string
	taskQueue      string
}

// NewClient creates a new Temporal client wrapper
func NewClient(hostPort, namespace, taskQueue string) (*Client, error) {
	// Create Temporal client options with our logger
	options := client.Options{
		HostPort:  hostPort,
		Namespace: namespace,
		Logger:    logger.GetTemporalLogAdapter("temporal"),
	}

	// Create the Temporal client
	temporalClient, err := client.Dial(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	getTemporalLog().Info().Msgf("Connected to Temporal at %s, namespace: %s", hostPort, namespace)

	return &Client{
		temporalClient: temporalClient,
		namespace:      namespace,
		taskQueue:      taskQueue,
	}, nil
}

// GetTemporalClient returns the underlying Temporal client
func (c *Client) GetTemporalClient() client.Client {
	return c.temporalClient
}

// GetTaskQueue returns the task queue name
func (c *Client) GetTaskQueue() string {
	return c.taskQueue
}

// StartWorkflow starts a new workflow execution.
// Uses ALLOW_DUPLICATE_FAILED_ONLY policy for idempotency:
// - Running workflows: Rejects (caller should check status first)
// - Completed workflows: Rejects (caller should check status first)
// - Failed workflows: Allows retry with same ID
// - Not found: Starts new workflow
func (c *Client) StartWorkflow(ctx context.Context, workflowID string, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	options := client.StartWorkflowOptions{
		ID:                       workflowID,
		TaskQueue:                c.taskQueue,
		WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
	}

	we, err := c.temporalClient.ExecuteWorkflow(ctx, options, workflow, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	getTemporalLog().Info().Msgf("Started workflow %s with ID: %s", workflow, workflowID)
	return we, nil
}

// SignalWorkflow sends a signal to a running workflow
func (c *Client) SignalWorkflow(ctx context.Context, workflowID, signalName string, arg interface{}) error {
	err := c.temporalClient.SignalWorkflow(ctx, workflowID, "", signalName, arg)
	if err != nil {
		return fmt.Errorf("failed to signal workflow: %w", err)
	}

	getTemporalLog().Debug().Msgf("Sent signal %s to workflow %s", signalName, workflowID)
	return nil
}

// QueryWorkflow queries a running workflow
func (c *Client) QueryWorkflow(ctx context.Context, workflowID, queryType string, args ...interface{}) (interface{}, error) {
	resp, err := c.temporalClient.QueryWorkflow(ctx, workflowID, "", queryType, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow: %w", err)
	}

	var result interface{}
	if err := resp.Get(&result); err != nil {
		return nil, fmt.Errorf("failed to get query result: %w", err)
	}

	return result, nil
}

// MapWorkflowExecutionStatus maps Temporal's WorkflowExecutionStatus to our WorkflowStatus type.
// Exported for testing purposes.
func MapWorkflowExecutionStatus(status enums.WorkflowExecutionStatus) WorkflowStatus {
	switch status {
	case enums.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return WorkflowStatusRunning
	case enums.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return WorkflowStatusCompleted
	case enums.WORKFLOW_EXECUTION_STATUS_FAILED:
		return WorkflowStatusFailed
	case enums.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return WorkflowStatusCanceled
	case enums.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return WorkflowStatusTerminated
	case enums.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return WorkflowStatusTimedOut
	default:
		return WorkflowStatusUnknown
	}
}

// GetWorkflowStatus returns the current status of a workflow by ID.
// Returns an error if the workflow doesn't exist.
func (c *Client) GetWorkflowStatus(ctx context.Context, workflowID string) (WorkflowStatus, error) {
	desc, err := c.temporalClient.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		return WorkflowStatusUnknown, fmt.Errorf("failed to describe workflow: %w", err)
	}

	return MapWorkflowExecutionStatus(desc.WorkflowExecutionInfo.Status), nil
}

// CancelWorkflow requests cancellation of a running workflow.
// The workflow will receive a cancellation signal and can clean up gracefully.
func (c *Client) CancelWorkflow(ctx context.Context, workflowID string) error {
	err := c.temporalClient.CancelWorkflow(ctx, workflowID, "")
	if err != nil {
		return fmt.Errorf("failed to cancel workflow: %w", err)
	}

	getTemporalLog().Info().Msgf("Cancelled workflow %s", workflowID)
	return nil
}

// Close closes the Temporal client connection
func (c *Client) Close() error {
	if c.temporalClient != nil {
		c.temporalClient.Close()
		getTemporalLog().Info().Msg("Temporal client closed")
	}
	return nil
}

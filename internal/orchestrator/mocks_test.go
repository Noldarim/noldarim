// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestrator

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	"github.com/noldarim/noldarim/internal/orchestrator/temporal"
)

// MockTemporalClient is a shared mock implementation of TemporalClient interface.
// Use this in all orchestrator tests that need to mock the temporal client.
type MockTemporalClient struct {
	mock.Mock
}

func (m *MockTemporalClient) StartWorkflow(ctx context.Context, workflowID string, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	callArgs := m.Called(ctx, workflowID, workflow, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(client.WorkflowRun), callArgs.Error(1)
}

func (m *MockTemporalClient) GetWorkflowStatus(ctx context.Context, workflowID string) (temporal.WorkflowStatus, error) {
	args := m.Called(ctx, workflowID)
	return args.Get(0).(temporal.WorkflowStatus), args.Error(1)
}

func (m *MockTemporalClient) SignalWorkflow(ctx context.Context, workflowID, signalName string, arg interface{}) error {
	args := m.Called(ctx, workflowID, signalName, arg)
	return args.Error(0)
}

func (m *MockTemporalClient) QueryWorkflow(ctx context.Context, workflowID, queryType string, args ...interface{}) (interface{}, error) {
	callArgs := m.Called(ctx, workflowID, queryType, args)
	return callArgs.Get(0), callArgs.Error(1)
}

func (m *MockTemporalClient) CancelWorkflow(ctx context.Context, workflowID string) error {
	args := m.Called(ctx, workflowID)
	return args.Error(0)
}

func (m *MockTemporalClient) GetTemporalClient() client.Client {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(client.Client)
}

func (m *MockTemporalClient) GetTaskQueue() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockTemporalClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockWorkflowRun implements client.WorkflowRun for testing.
// Use this when mocking StartWorkflow return values.
type MockWorkflowRun struct {
	mock.Mock
}

func (m *MockWorkflowRun) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWorkflowRun) GetRunID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockWorkflowRun) Get(ctx context.Context, valuePtr interface{}) error {
	args := m.Called(ctx, valuePtr)
	return args.Error(0)
}

func (m *MockWorkflowRun) GetWithOptions(ctx context.Context, valuePtr interface{}, options client.WorkflowRunGetOptions) error {
	args := m.Called(ctx, valuePtr, options)
	return args.Error(0)
}

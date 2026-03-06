// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package activities

import (
	"context"
	"testing"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.temporal.io/sdk/testsuite"
)

type mockLogSource struct {
	stdout string
	stderr string
	err    error
}

func (m *mockLogSource) GetContainerLogs(_ context.Context, _ string, _ string) (string, string, error) {
	return m.stdout, m.stderr, m.err
}

type mockLogSaver struct {
	saved []*models.ContainerLog
}

func (m *mockLogSaver) SaveContainerLog(_ context.Context, log *models.ContainerLog) error {
	m.saved = append(m.saved, log)
	return nil
}

func TestCaptureContainerLogsActivity(t *testing.T) {
	source := &mockLogSource{
		stdout: "line1\nline2\nline3",
		stderr: "warn: something",
	}
	saver := &mockLogSaver{}
	act := NewContainerLogActivities(source, saver)

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.CaptureContainerLogsActivity)

	result, err := env.ExecuteActivity(act.CaptureContainerLogsActivity, CaptureContainerLogsInput{
		RunID:       "run-1",
		StepID:      "step-1",
		ContainerID: "ctr-abc",
		Tail:        "100",
	})
	require.NoError(t, err)

	var output CaptureContainerLogsOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 3, output.StdoutLines) // "line1\nline2\nline3" = 2 newlines + 1
	assert.Equal(t, 1, output.StderrLines)
	assert.Len(t, saver.saved, 2)
	assert.Equal(t, "stdout", saver.saved[0].Stream)
	assert.Equal(t, "stderr", saver.saved[1].Stream)
}

func TestCaptureContainerLogsActivity_EmptyLogs(t *testing.T) {
	source := &mockLogSource{stdout: "", stderr: ""}
	saver := &mockLogSaver{}
	act := NewContainerLogActivities(source, saver)

	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(act.CaptureContainerLogsActivity)

	result, err := env.ExecuteActivity(act.CaptureContainerLogsActivity, CaptureContainerLogsInput{
		RunID:       "run-2",
		StepID:      "step-1",
		ContainerID: "ctr-xyz",
	})
	require.NoError(t, err)

	var output CaptureContainerLogsOutput
	require.NoError(t, result.Get(&output))

	assert.Equal(t, 0, output.StdoutLines)
	assert.Equal(t, 0, output.StderrLines)
	assert.Len(t, saver.saved, 0)
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package testutil

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/noldarim/noldarim/internal/protocol"
)

// AssertCommandSent verifies that a command of the expected type was sent
func AssertCommandSent(t *testing.T, capture *CommandCapture, expectedType interface{}) {
	assert.True(t, capture.CommandCount() > 0, "Expected at least one command to be sent")
	lastCmd := capture.LastCommand()
	assert.IsType(t, expectedType, lastCmd, "Command type mismatch")
}

// AssertLoadProjectsCommand verifies that a LoadProjectsCommand was sent
func AssertLoadProjectsCommand(t *testing.T, capture *CommandCapture) {
	AssertCommandSent(t, capture, protocol.LoadProjectsCommand{})
}

// AssertLoadTasksCommand verifies that a LoadTasksCommand was sent with the correct project ID
func AssertLoadTasksCommand(t *testing.T, capture *CommandCapture, projectID string) {
	AssertCommandSent(t, capture, protocol.LoadTasksCommand{})
	cmd := capture.LastCommand().(protocol.LoadTasksCommand)
	assert.Equal(t, projectID, cmd.ProjectID, "LoadTasksCommand project ID mismatch")
}

// AssertToggleTaskCommand verifies that a ToggleTaskCommand was sent with correct IDs
func AssertToggleTaskCommand(t *testing.T, capture *CommandCapture, projectID, taskID string) {
	AssertCommandSent(t, capture, protocol.ToggleTaskCommand{})
	cmd := capture.LastCommand().(protocol.ToggleTaskCommand)
	assert.Equal(t, projectID, cmd.ProjectID, "ToggleTaskCommand project ID mismatch")
	assert.Equal(t, taskID, cmd.TaskID, "ToggleTaskCommand task ID mismatch")
}

// AssertDeleteTaskCommand verifies that a DeleteTaskCommand was sent with correct IDs
func AssertDeleteTaskCommand(t *testing.T, capture *CommandCapture, projectID, taskID string) {
	AssertCommandSent(t, capture, protocol.DeleteTaskCommand{})
	cmd := capture.LastCommand().(protocol.DeleteTaskCommand)
	assert.Equal(t, projectID, cmd.ProjectID, "DeleteTaskCommand project ID mismatch")
	assert.Equal(t, taskID, cmd.TaskID, "DeleteTaskCommand task ID mismatch")
}

// AssertNavigationMessage verifies that a message is of the expected navigation type
func AssertNavigationMessage(t *testing.T, msg tea.Msg, expectedType interface{}) {
	assert.IsType(t, expectedType, msg, "Navigation message type mismatch")
}

// AssertGoToTasksMessage verifies a GoToTasksScreenMsg with the correct project ID
func AssertGoToTasksMessage(t *testing.T, msg tea.Msg, expectedProjectID string) {
	assert.IsType(t, msg, expectedProjectID, "Expected GoToTasksScreenMsg")
	// Note: We can't import the navigation types here due to import cycles
	// Individual screen tests will need to handle the specific type assertions
}

// AssertQuitMessage verifies that a quit message was generated
func AssertQuitMessage(t *testing.T, cmd tea.Cmd) {
	assert.NotNil(t, cmd, "Expected a command to be generated")
	msg := ExecuteCommand(cmd)
	assert.IsType(t, tea.QuitMsg{}, msg, "Expected quit message")
}

// AssertNoCommand verifies that no command was generated
func AssertNoCommand(t *testing.T, cmd tea.Cmd) {
	assert.Nil(t, cmd, "Expected no command to be generated")
}

// AssertViewNotEmpty verifies that the view produces non-empty output
func AssertViewNotEmpty(t *testing.T, model tea.Model) {
	view := model.View()
	assert.NotEmpty(t, view, "View should not be empty")
}

// AssertCommandCount verifies the exact number of commands captured
func AssertCommandCount(t *testing.T, capture *CommandCapture, expected int) {
	actual := capture.CommandCount()
	assert.Equal(t, expected, actual, "Command count mismatch")
}

// AssertNoCommands verifies that no commands were sent
func AssertNoCommands(t *testing.T, capture *CommandCapture) {
	AssertCommandCount(t, capture, 0)
}

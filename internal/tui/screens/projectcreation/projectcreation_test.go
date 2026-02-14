// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectcreation

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/messages"
)

func TestNewModel(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	assert.Equal(t, DirSelection, model.stage)
	assert.NotNil(t, model.filePicker)
	assert.NotNil(t, model.form)
	// Check that cmdChan is not nil (we can't directly compare send-only channels)
	assert.NotNil(t, model.cmdChan)
	assert.Empty(t, model.selectedPath)
	assert.Empty(t, model.formTitle)
	assert.Empty(t, model.formDesc)
}

func TestNavigationFromProjectList(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	// Set larger dimensions so all content is visible
	model.SetSize(120, 40)

	// Verify initial state
	assert.Equal(t, DirSelection, model.stage)
	view := model.View()
	assert.Contains(t, view, "Navigate to your project directory")
	assert.Contains(t, view, "Current:")
}

func TestSpaceKeySelectsDirectory(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	// Set a test directory
	testDir := "/test/directory"
	model.filePicker.CurrentDirectory = testDir

	// Press space to select directory
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model = updatedModel.(Model)

	assert.Equal(t, FormInput, model.stage)
	assert.Equal(t, testDir, model.selectedPath)
}

func TestEscapeInDirSelectionReturnsToProjectList(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	// Press escape in directory selection
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEscape})

	// Should return navigation message
	assert.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(messages.GoToProjectListMsg)
	assert.True(t, ok, "Expected GoToProjectListMsg")
}

func TestEscapeInFormReturnsToDirectorySelection(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	// Move to form stage
	model.stage = FormInput
	model.selectedPath = "/test/dir"

	// Press escape in form
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = updatedModel.(Model)

	assert.Equal(t, DirSelection, model.stage)
	assert.Empty(t, model.formTitle)
	assert.Empty(t, model.formDesc)
}

func TestFormSubmissionSendsCommand(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	// Setup form stage with data
	model.stage = FormInput
	model.selectedPath = "/test/project"
	model.formTitle = "Test Project"
	model.formDesc = "Test Description"

	// Re-init form with values
	model.initForm()

	// Simulate form completion
	model.form.State = huh.StateCompleted

	// Submit form
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should return navigation message
	assert.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(messages.GoToProjectListMsg)
	assert.True(t, ok, "Expected GoToProjectListMsg after form submission")

	// Check command was sent
	select {
	case sentCmd := <-cmdChan:
		createCmd, ok := sentCmd.(protocol.CreateProjectCommand)
		assert.True(t, ok, "Expected CreateProjectCommand")
		assert.Equal(t, "Test Project", createCmd.Name)
		assert.Equal(t, "Test Description", createCmd.Description)
		assert.Equal(t, "/test/project", createCmd.RepositoryPath)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("No command received")
	}
}

func TestWindowResize(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	// Send window resize message
	newWidth, newHeight := 100, 50
	updatedModel, _ := model.Update(tea.WindowSizeMsg{
		Width:  newWidth,
		Height: newHeight,
	})
	model = updatedModel.(Model)

	// The Update method doesn't update size directly, it's handled by SetSize
	// Window resize messages are handled at top level, not in Update
	// So widths and heights remain the initial values (50, 10)
	assert.Equal(t, 50, model.width)
	assert.Equal(t, 10, model.height)
}

func TestViewRendersCorrectStage(t *testing.T) {
	cmdChan := make(chan protocol.Command, 1)
	model := NewModel(cmdChan)

	// Test directory selection view
	view := model.View()
	assert.Contains(t, view, "Navigate to your project directory")
	assert.Contains(t, view, "space")

	// Switch to form stage
	model.stage = FormInput
	model.selectedPath = "/test/selected"

	// Test form view
	view = model.View()
	assert.Contains(t, view, "Selected Directory:")
	assert.Contains(t, view, "/test/selected")
	// The form may not render "Project Title" immediately without initialization
	// Just check that we have the directory showing
}

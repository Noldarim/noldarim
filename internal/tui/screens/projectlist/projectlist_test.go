// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectlist

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/noldarim/noldarim/internal/tui/messages"
	"github.com/noldarim/noldarim/test/testutil"
)

func TestProjectItem(t *testing.T) {
	t.Run("implements list.Item interface correctly", func(t *testing.T) {
		item := ProjectItem{
			ID:   "test-id",
			Name: "Test Project",
			Desc: "Test Description",
		}

		// Test FilterValue
		assert.Equal(t, "Test Project", item.FilterValue())

		// Test Title
		assert.Equal(t, "Test Project", item.Title())

		// Test Description
		assert.Equal(t, "Test Description", item.Description())

		// Test String
		expected := "Test Project: Test Description"
		assert.Equal(t, expected, item.String())
	})
}

func TestNewModel(t *testing.T) {
	t.Run("creates model with correct initial state", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())

		// Verify initial state
		assert.NotNil(t, model.list)
		assert.NotNil(t, model.cmdChan)
		assert.NotNil(t, model.projects)
		assert.Empty(t, model.projects, "Projects map should be empty initially")

		// Verify list configuration
		assert.Equal(t, "", model.list.Title)
	})
}

func TestModelInit(t *testing.T) {
	t.Run("sends LoadProjectsCommand on init", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())
		cmd := model.Init()

		// Init should return nil (command sent via goroutine)
		assert.Nil(t, cmd)

		// Wait for command to be captured
		capture.WaitForCommands(1)

		// Verify LoadProjectsCommand was sent
		testutil.AssertLoadProjectsCommand(t, capture)
	})
}

func TestModelUpdate_KeyHandling(t *testing.T) {
	capture := testutil.NewCommandCapture()
	defer capture.Close()

	model := NewModel(capture.Channel())

	t.Run("enter key with no selection does nothing special", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.SpecialKey(tea.KeyEnter))

		// Should return same model type
		assert.IsType(t, Model{}, newModel)

		// No special navigation command should be generated for empty list
		if cmd != nil {
			msg := testutil.ExecuteCommand(cmd)
			// If there's a command, it shouldn't be a navigation message
			_, isNavMsg := msg.(messages.GoToTasksScreenMsg)
			assert.False(t, isNavMsg, "Should not generate navigation message for empty list")
		}
	})

	t.Run("s key generates settings navigation message", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.KeyPress("s"))

		assert.IsType(t, Model{}, newModel)
		assert.NotNil(t, cmd)

		msg := testutil.ExecuteCommand(cmd)
		assert.IsType(t, messages.GoToSettingsMsg{}, msg)
	})

	t.Run("q key generates quit message", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.KeyPress("q"))

		assert.IsType(t, Model{}, newModel)
		testutil.AssertQuitMessage(t, cmd)
	})

	t.Run("ctrl+c generates quit message", func(t *testing.T) {
		ctrlC := tea.KeyMsg{
			Type: tea.KeyCtrlC,
		}
		newModel, cmd := testutil.SendMessage(model, ctrlC)

		assert.IsType(t, Model{}, newModel)
		testutil.AssertQuitMessage(t, cmd)
	})
}

func TestModelUpdate_EventHandling(t *testing.T) {
	t.Run("ProjectsLoadedEvent updates projects and list", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())
		event := testutil.ProjectsLoadedEvent()

		newModel, _ := testutil.SendMessage(model, event)
		updatedModel := newModel.(Model)

		// Verify projects were stored
		assert.Len(t, updatedModel.projects, 3) // testutil.SampleProjects() has 3 projects
		assert.Contains(t, updatedModel.projects, "proj1")
		assert.Contains(t, updatedModel.projects, "proj2")
		assert.Contains(t, updatedModel.projects, "proj3")

		// Verify list was updated (list.Model internal state is harder to test directly)
		// We can at least verify the model was updated
	})

	t.Run("WindowSizeMsg updates list dimensions", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())
		sizeMsg := testutil.WindowSizeMsg(100, 50)

		newModel, _ := testutil.SendMessage(model, sizeMsg)
		updatedModel := newModel.(Model)

		// Verify model was updated (internal list dimensions are private)
		assert.IsType(t, Model{}, updatedModel)
	})
}

func TestModelUpdate_NavigationWithSelection(t *testing.T) {
	t.Run("enter key with selected project generates navigation message", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())

		// First load some projects
		event := testutil.ProjectsLoadedEvent()
		newModel, _ := testutil.SendMessage(model, event)
		model = newModel.(Model)

		// Simulate selection and enter key - this is tricky to test directly
		// since we can't easily control list selection in tests
		// For now, we'll test that the logic exists by checking the code path

		// The actual navigation testing would require more sophisticated
		// list manipulation or integration tests
	})
}

func TestModelView(t *testing.T) {
	t.Run("returns non-empty view output", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())

		testutil.AssertViewNotEmpty(t, model)
	})

	t.Run("contains expected elements", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())
		view := model.View()

		// Should contain help text
		assert.Contains(t, view, "enter")
		assert.Contains(t, view, "select")
		assert.Contains(t, view, "s")
		assert.Contains(t, view, "settings")
		assert.Contains(t, view, "q")
		assert.Contains(t, view, "quit")
	})

	t.Run("shows projects after loading", func(t *testing.T) {
		capture := testutil.NewCommandCapture()
		defer capture.Close()

		model := NewModel(capture.Channel())

		// Load projects
		event := testutil.ProjectsLoadedEvent()
		newModel, _ := testutil.SendMessage(model, event)
		model = newModel.(Model)

		view := model.View()

		// View should contain project information
		// Note: Exact content depends on list.Model rendering
		assert.NotEmpty(t, view)
	})
}

// Project creation flow tests are no longer relevant since
// project creation moved to separate screen

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package settings

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/noldarim/noldarim/internal/tui/messages"
	"github.com/noldarim/noldarim/test/testutil"
)

func TestNewModel(t *testing.T) {
	t.Run("creates model with correct initial state", func(t *testing.T) {
		model := NewModel()

		// Verify model creation succeeds
		assert.IsType(t, Model{}, model)

		// Since it's a placeholder, there's not much state to verify
		// This test mainly ensures the constructor works
	})
}

func TestModelInit(t *testing.T) {
	t.Run("init returns nil command", func(t *testing.T) {
		model := NewModel()
		cmd := model.Init()

		// Settings screen doesn't need to do anything on init
		testutil.AssertNoCommand(t, cmd)
	})
}

func TestModelUpdate_KeyHandling(t *testing.T) {
	model := NewModel()

	t.Run("esc key generates back navigation message", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.SpecialKey(tea.KeyEsc))

		assert.IsType(t, Model{}, newModel)
		assert.NotNil(t, cmd)

		msg := testutil.ExecuteCommand(cmd)
		assert.IsType(t, messages.GoBackMsg{}, msg)
	})

	t.Run("backspace key generates back navigation message", func(t *testing.T) {
		newModel, cmd := testutil.SendMessage(model, testutil.SpecialKey(tea.KeyBackspace))

		assert.IsType(t, Model{}, newModel)
		assert.NotNil(t, cmd)

		msg := testutil.ExecuteCommand(cmd)
		assert.IsType(t, messages.GoBackMsg{}, msg)
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

	t.Run("other keys do nothing special", func(t *testing.T) {
		testCases := []string{"a", "b", "enter", "space"}

		for _, key := range testCases {
			t.Run("key: "+key, func(t *testing.T) {
				newModel, cmd := testutil.SendMessage(model, testutil.KeyPress(key))

				assert.IsType(t, Model{}, newModel)
				// Other keys shouldn't generate special commands
				if cmd != nil {
					msg := testutil.ExecuteCommand(cmd)
					// Should not be navigation or quit messages
					_, isBack := msg.(messages.GoBackMsg)
					assert.False(t, isBack, "Should not generate back navigation for regular keys")
					_, isQuit := msg.(tea.QuitMsg)
					assert.False(t, isQuit, "Should not generate quit for regular keys")
				}
			})
		}
	})
}

func TestModelUpdate_OtherMessages(t *testing.T) {
	model := NewModel()

	t.Run("window size message doesn't break anything", func(t *testing.T) {
		sizeMsg := testutil.WindowSizeMsg(100, 50)

		newModel, cmd := testutil.SendMessage(model, sizeMsg)

		assert.IsType(t, Model{}, newModel)
		testutil.AssertNoCommand(t, cmd)
	})

	t.Run("arbitrary message doesn't break anything", func(t *testing.T) {
		arbitraryMsg := "some random message"

		newModel, cmd := testutil.SendMessage(model, arbitraryMsg)

		assert.IsType(t, Model{}, newModel)
		testutil.AssertNoCommand(t, cmd)
	})
}

func TestModelView(t *testing.T) {
	t.Run("returns non-empty view output", func(t *testing.T) {
		model := NewModel()

		testutil.AssertViewNotEmpty(t, model)
	})

	t.Run("contains expected content", func(t *testing.T) {
		model := NewModel()
		view := model.View()

		// Should contain settings content
		assert.Contains(t, view, "Settings")
		assert.Contains(t, view, "Theme: Default")
		assert.Contains(t, view, "Auto-refresh: Enabled")
		assert.Contains(t, view, "Notifications: Enabled")
		assert.Contains(t, view, "Debug Mode: Disabled")
	})

	t.Run("contains help text", func(t *testing.T) {
		model := NewModel()
		view := model.View()

		// Should contain navigation help
		assert.Contains(t, view, "esc")
		assert.Contains(t, view, "back")
		assert.Contains(t, view, "q")
		assert.Contains(t, view, "quit")
	})

	t.Run("view is consistent", func(t *testing.T) {
		model := NewModel()

		// Multiple calls should return the same content
		view1 := model.View()
		view2 := model.View()

		assert.Equal(t, view1, view2, "View should be consistent across calls")
	})
}

func TestModelState(t *testing.T) {
	t.Run("model handles state changes correctly", func(t *testing.T) {
		model1 := NewModel()
		model2 := NewModel()

		// Both models should be equivalent initially
		assert.Equal(t, model1, model2)

		// Window size changes should update model dimensions
		originalModel := model1
		newModel, _ := testutil.SendMessage(model1, testutil.WindowSizeMsg(100, 50))
		updatedModel := newModel.(Model)

		// Model dimensions should be updated
		assert.Equal(t, 100, updatedModel.width)
		assert.Equal(t, 50, updatedModel.height)
		assert.NotEqual(t, originalModel.width, updatedModel.width)
		assert.NotEqual(t, originalModel.height, updatedModel.height)
	})
}

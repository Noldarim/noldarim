// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package testutil

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/noldarim/noldarim/internal/protocol"
)

// MockCommandChannel creates a buffered channel for testing
// Captures commands sent by screens for verification
func MockCommandChannel() (chan<- protocol.Command, <-chan protocol.Command) {
	ch := make(chan protocol.Command, 100)
	return ch, ch
}

// SendMessage simulates sending a message to a Bubble Tea model
// Returns the updated model and any commands generated
func SendMessage(model tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	return model.Update(msg)
}

// ExecuteCommand executes a tea.Cmd and returns the resulting message
// Useful for testing command chains
func ExecuteCommand(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// AssertViewContains checks if view output contains expected string
func AssertViewContains(t *testing.T, model tea.Model, expected string) {
	view := model.View()
	assert.Contains(t, view, expected)
}

// KeyPress creates a tea.KeyMsg for testing keyboard input
func KeyPress(key string) tea.KeyMsg {
	return tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(key),
	}
}

// SpecialKey creates special key messages (Enter, Esc, etc.)
func SpecialKey(keyType tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: keyType}
}

// WindowSizeMsg creates a window size message for testing
func WindowSizeMsg(width, height int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	}
}

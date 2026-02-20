// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package testutil

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/protocol"
)

// MockScreen implements the Screen interface for testing
type MockScreen struct {
	InitCalled   bool
	UpdateCalled bool
	ViewCalled   bool
	LastMessage  tea.Msg
	LastCommand  tea.Cmd
	ViewOutput   string
}

// NewMockScreen creates a new mock screen with default view output
func NewMockScreen() *MockScreen {
	return &MockScreen{
		ViewOutput: "Mock Screen View",
	}
}

func (m *MockScreen) Init() tea.Cmd {
	m.InitCalled = true
	return nil
}

func (m *MockScreen) Update(msg tea.Msg) (*MockScreen, tea.Cmd) {
	m.UpdateCalled = true
	m.LastMessage = msg
	return m, m.LastCommand
}

func (m *MockScreen) View() string {
	m.ViewCalled = true
	return m.ViewOutput
}

// SetNextCommand sets the command that will be returned on the next Update call
func (m *MockScreen) SetNextCommand(cmd tea.Cmd) {
	m.LastCommand = cmd
}

// SetViewOutput sets the output that will be returned by View()
func (m *MockScreen) SetViewOutput(output string) {
	m.ViewOutput = output
}

// CommandCapture captures commands sent through a channel
type CommandCapture struct {
	Commands []protocol.Command
	ch       chan protocol.Command
	mu       sync.RWMutex
	closed   bool
}

// NewCommandCapture creates a new command capture instance
func NewCommandCapture() *CommandCapture {
	ch := make(chan protocol.Command, 100)
	capture := &CommandCapture{
		Commands: make([]protocol.Command, 0),
		ch:       ch,
	}

	// Start capturing in background
	go func() {
		for cmd := range ch {
			capture.mu.Lock()
			capture.Commands = append(capture.Commands, cmd)
			capture.mu.Unlock()
		}
		capture.mu.Lock()
		capture.closed = true
		capture.mu.Unlock()
	}()

	return capture
}

// Channel returns the send channel for commands
func (c *CommandCapture) Channel() chan<- protocol.Command {
	return c.ch
}

// LastCommand returns the most recent command sent, or nil if none
func (c *CommandCapture) LastCommand() protocol.Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.Commands) == 0 {
		return nil
	}
	return c.Commands[len(c.Commands)-1]
}

// CommandCount returns the number of commands captured
func (c *CommandCapture) CommandCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.Commands)
}

// AllCommands returns a copy of all captured commands
func (c *CommandCapture) AllCommands() []protocol.Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]protocol.Command, len(c.Commands))
	copy(result, c.Commands)
	return result
}

// Clear clears all captured commands
func (c *CommandCapture) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Commands = c.Commands[:0]
}

// Close closes the capture channel
func (c *CommandCapture) Close() {
	close(c.ch)
}

// WaitForCommands waits until at least n commands have been captured
// This is useful for testing async command sending
func (c *CommandCapture) WaitForCommands(n int) {
	for {
		c.mu.RLock()
		count := len(c.Commands)
		c.mu.RUnlock()

		if count >= n {
			break
		}

		// Small sleep to avoid busy waiting
		// In real tests you might want to use a proper wait mechanism
	}
}

// MockList is a mock list.Model for testing delegate rendering
type MockList struct {
	width  int
	height int
	index  int
}

// NewMockList creates a new mock list model
func NewMockList() *MockList {
	return &MockList{
		width:  80,
		height: 20,
		index:  0,
	}
}

// Width returns the width of the list
func (m *MockList) Width() int {
	return m.width
}

// Height returns the height of the list
func (m *MockList) Height() int {
	return m.height
}

// Index returns the current selection index
func (m *MockList) Index() int {
	return m.index
}

// SetWidth sets the width of the list
func (m *MockList) SetWidth(width int) {
	m.width = width
}

// SetHeight sets the height of the list
func (m *MockList) SetHeight(height int) {
	m.height = height
}

// SetIndex sets the current selection index
func (m *MockList) SetIndex(index int) {
	m.index = index
}

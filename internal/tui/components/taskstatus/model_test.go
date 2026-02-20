// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskstatus

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

func TestNew(t *testing.T) {
	text := "Test task"
	status := models.TaskStatusPending

	model := New(text, status)

	if model.text != text {
		t.Errorf("expected text %q, got %q", text, model.text)
	}

	if model.status != status {
		t.Errorf("expected status %v, got %v", status, model.status)
	}

	if model.width != 0 {
		t.Errorf("expected width 0, got %d", model.width)
	}
}

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		status  models.TaskStatus
		uiState UIState
		hasCmd  bool
	}{
		{
			name:    "pending status has no command",
			status:  models.TaskStatusPending,
			uiState: UIStateNormal,
			hasCmd:  false,
		},
		{
			name:    "in progress status has spinner command",
			status:  models.TaskStatusInProgress,
			uiState: UIStateNormal,
			hasCmd:  true,
		},
		{
			name:    "completed status has no command",
			status:  models.TaskStatusCompleted,
			uiState: UIStateNormal,
			hasCmd:  false,
		},
		{
			name:    "pending UI state has spinner command",
			status:  models.TaskStatusPending,
			uiState: UIStatePending,
			hasCmd:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New("test", tt.status)
			if tt.uiState != UIStateNormal {
				model = model.SetUIState(tt.uiState)
			}
			cmd := model.Init()

			if tt.hasCmd && cmd == nil {
				t.Error("expected command, got nil")
			}

			if !tt.hasCmd && cmd != nil {
				t.Error("expected no command, got command")
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	model := New("test", models.TaskStatusInProgress)

	// Test with TickMsg
	tickMsg := TickMsg(time.Now())
	updatedModel, cmd := model.Update(tickMsg)

	if cmd == nil {
		t.Error("expected command from tick message, got nil")
	}

	// Test with spinner.TickMsg
	spinnerTickMsg := spinner.TickMsg{}
	updatedModel, cmd = updatedModel.Update(spinnerTickMsg)

	if cmd == nil {
		t.Error("expected command from spinner tick message, got nil")
	}

	// Test with pending status (should not respond to tick)
	pendingModel := New("test", models.TaskStatusPending)
	updatedModel, cmd = pendingModel.Update(tickMsg)

	if cmd != nil {
		t.Error("expected no command for pending status, got command")
	}
}

func TestView(t *testing.T) {
	tests := []struct {
		name     string
		status   models.TaskStatus
		width    int
		contains []string
	}{
		{
			name:     "pending full view",
			status:   models.TaskStatusPending,
			width:    50,
			contains: []string{"○", "PENDING"},
		},
		{
			name:     "completed full view",
			status:   models.TaskStatusCompleted,
			width:    50,
			contains: []string{"✓", "COMPLETED"},
		},
		{
			name:     "pending compact view",
			status:   models.TaskStatusPending,
			width:    10,
			contains: []string{"○"},
		},
		{
			name:     "completed compact view",
			status:   models.TaskStatusCompleted,
			width:    10,
			contains: []string{"✓"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New("test task", tt.status)
			model = model.SetWidth(tt.width)

			view := model.View()

			for _, expected := range tt.contains {
				if !strings.Contains(view, expected) {
					t.Errorf("expected view to contain %q, got %q", expected, view)
				}
			}

			// For compact view, should not contain status text
			if tt.width < 20 {
				if strings.Contains(view, "PENDING") || strings.Contains(view, "COMPLETED") {
					t.Errorf("compact view should not contain status text, got %q", view)
				}
			}
		})
	}
}

func TestSetters(t *testing.T) {
	model := New("original", models.TaskStatusPending)

	// Test SetWidth
	model = model.SetWidth(100)
	if model.width != 100 {
		t.Errorf("expected width 100, got %d", model.width)
	}

	// Test SetStatus
	model = model.SetStatus(models.TaskStatusCompleted)
	if model.status != models.TaskStatusCompleted {
		t.Errorf("expected status %v, got %v", models.TaskStatusCompleted, model.status)
	}

	// Test SetText
	model = model.SetText("new text")
	if model.text != "new text" {
		t.Errorf("expected text %q, got %q", "new text", model.text)
	}
}

func TestGetIcon(t *testing.T) {
	tests := []struct {
		name     string
		status   models.TaskStatus
		expected string
	}{
		{
			name:     "pending icon",
			status:   models.TaskStatusPending,
			expected: "○",
		},
		{
			name:     "completed icon",
			status:   models.TaskStatusCompleted,
			expected: "✓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New("test", tt.status)
			icon := model.getIcon()

			if icon != tt.expected {
				t.Errorf("expected icon %q, got %q", tt.expected, icon)
			}
		})
	}
}

func TestRenderModes(t *testing.T) {
	model := New("test task", models.TaskStatusCompleted)

	// Test full render
	fullView := model.renderFull()
	if !strings.Contains(fullView, "✓") {
		t.Error("full view should contain icon")
	}
	if !strings.Contains(fullView, "COMPLETED") {
		t.Error("full view should contain status text")
	}

	// Test compact render
	compactView := model.renderCompact()
	if !strings.Contains(compactView, "✓") {
		t.Error("compact view should contain icon")
	}
	if strings.Contains(compactView, "COMPLETED") {
		t.Error("compact view should not contain status text")
	}
}

func TestInteractivity(t *testing.T) {
	model := New("test", models.TaskStatusInProgress)

	// Initialize the model
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected init command for in-progress status")
	}

	// Run a few updates to ensure the app stays interactive
	for i := 0; i < 5; i++ {
		tickMsg := TickMsg(time.Now())
		var newCmd tea.Cmd
		model, newCmd = model.Update(tickMsg)

		if newCmd == nil {
			t.Error("expected command to keep animation running")
		}
	}
}

func TestUIStates(t *testing.T) {
	tests := []struct {
		name     string
		uiState  UIState
		expected string
	}{
		{
			name:     "pending UI state",
			uiState:  UIStatePending,
			expected: "PENDING",
		},
		{
			name:     "failed UI state",
			uiState:  UIStateFailed,
			expected: "FAILED",
		},
		{
			name:     "created UI state",
			uiState:  UIStateCreated,
			expected: "CREATED",
		},
		{
			name:     "normal UI state shows task status",
			uiState:  UIStateNormal,
			expected: "PENDING", // Should show the task status
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New("test task", models.TaskStatusPending)
			model = model.SetUIState(tt.uiState)

			view := model.View()
			if !strings.Contains(view, tt.expected) {
				t.Errorf("expected view to contain %q, got %q", tt.expected, view)
			}
		})
	}
}

func TestSetUIState(t *testing.T) {
	model := New("test", models.TaskStatusPending)

	// Test SetUIState
	model = model.SetUIState(UIStateFailed)
	if model.uiState != UIStateFailed {
		t.Errorf("expected UI state %v, got %v", UIStateFailed, model.uiState)
	}
}

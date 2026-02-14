// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/protocol"
)

// StartTUI initializes and runs the TUI application
func StartTUI(cmdChan chan<- protocol.Command, eventChan <-chan protocol.Event) error {
	// Create the main model
	mainModel := NewMainModel(cmdChan, eventChan)

	// Create event deduplicator
	deduplicator := NewEventDeduplicator()

	// Create the Bubble Tea program
	p := tea.NewProgram(mainModel, tea.WithAltScreen())

	// Start listening for events in a separate goroutine
	go func() {
		for event := range eventChan {
			// Check for critical errors and handle them by printing and exiting
			if criticalErr, ok := event.(*protocol.CriticalErrorEvent); ok {
				handleCriticalError(criticalErr)
				return
			}

			// Apply deduplication
			if deduplicator.ShouldProcess(event) {
				p.Send(event)
			}
		}
	}()

	// Run the program
	_, err := p.Run()
	return err
}

// handleCriticalError prints a red error message and exits the application
func handleCriticalError(event *protocol.CriticalErrorEvent) {
	errorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("9")). // Red color
		Render

	// Log critical TUI errors to stderr to avoid corrupting display
	fmt.Fprintf(os.Stderr, "\n%s\n", errorStyle("CRITICAL ERROR: "+event.Message))
	if event.Context != "" {
		fmt.Fprintf(os.Stderr, "%s\n", errorStyle("Context: "+event.Context))
	}
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

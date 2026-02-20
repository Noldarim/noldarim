// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectcreation

import (
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// Update handles messages and updates the model state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log := logger.GetTUILogger().With().Str("component", "projectcreation").Logger()

	var cmd tea.Cmd

	switch m.stage {
	case DirSelection:
		// Handle directory selection stage
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case " ", "s":
				// Select current directory
				m.selectedPath = m.filePicker.CurrentDirectory
				m.stage = FormInput
				// Re-init form to reset state
				m.initForm()
				return m, m.form.Init()

			case "esc":
				// Cancel and go back to project list
				return m, func() tea.Msg {
					return messages.GoToProjectListMsg{}
				}

			case "ctrl+c":
				return m, tea.Quit
			}
		}

		// Update file picker
		m.filePicker, cmd = m.filePicker.Update(msg)
		return m, cmd

	case FormInput:
		// Handle form input stage
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				// Go back to directory selection
				m.stage = DirSelection
				// Reset form values
				m.formTitle = ""
				m.formDesc = ""
				m.initForm()
				return m, nil

			case "ctrl+c":
				return m, tea.Quit
			}
		}

		// Update the form and check if it's complete
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}

		if m.form.State == huh.StateCompleted {
			// Form is completed, extract values
			title := m.form.GetString("title")
			description := m.form.GetString("description")

			// Fallback to model fields if form values are empty
			if title == "" {
				title = m.formTitle
			}
			if description == "" {
				description = m.formDesc
			}

			// Log the command details for debugging
			log.Info().Str("title", title).Str("description", description).Str("repository_path", m.selectedPath).Msg("Sending CreateProjectCommand")

			// Send CreateProjectCommand
			go func() {
				cmd := protocol.CreateProjectCommand{
					Name:           title,
					Description:    description,
					RepositoryPath: m.selectedPath,
				}
				m.cmdChan <- cmd
			}()

			// Navigate back to project list
			return m, func() tea.Msg {
				return messages.GoToProjectListMsg{}
			}
		}

		return m, cmd
	}

	// Handle window resize
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.SetSize(msg.Width, msg.Height)
	}

	return m, nil
}

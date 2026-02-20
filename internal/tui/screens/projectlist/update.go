// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectlist

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/messages"
)

// Update handles messages and updates the model state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle key messages
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {

		case "enter":
			// Get selected project and navigate to tasks
			if selectedItem := m.list.SelectedItem(); selectedItem != nil {
				if projectItem, ok := selectedItem.(ProjectItem); ok {
					return m, func() tea.Msg {
						// Return a custom message that the main model will handle
						return messages.GoToTasksScreenMsg{ProjectID: projectItem.ID}
					}
				}
			}

		case "n":
			// Go to project creation screen
			return m, func() tea.Msg {
				return messages.GoToProjectCreationMsg{}
			}

		case "s":
			// Go to settings
			return m, func() tea.Msg {
				return messages.GoToSettingsMsg{}
			}

		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case protocol.ProjectsLoadedEvent:
		// Update projects and list items
		m.projects = msg.Projects
		items := make([]list.Item, 0, len(msg.Projects))
		for _, project := range msg.Projects {
			items = append(items, ProjectItem{
				ID:             project.ID,
				Name:           project.Name,
				Desc:           project.RepositoryPath,
				RepositoryPath: project.RepositoryPath,
			})
		}
		m.list.SetItems(items)

	case protocol.ErrorEvent:
		// Handle error events with detailed logging
		if msg.Context != "" {
			m.statusMessage = fmt.Sprintf("Error: %s - %s", msg.Message, msg.Context)
		} else {
			m.statusMessage = fmt.Sprintf("Error: %s", msg.Message)
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	// Update the list component
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskdetails

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noldarim/noldarim/internal/logger"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/messages"
)

// Update handles messages and updates the model state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			// Go back to task view
			return m, func() tea.Msg {
				return messages.GoBackMsg{}
			}
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			// Switch to next tab
			m.tabBar.NextTab()
			m.updateFocus()
			return m, nil

		case "shift+tab":
			// Switch to previous tab
			m.tabBar.PrevTab()
			m.updateFocus()
			return m, nil

		case "1":
			// Switch to Task Info tab
			m.tabBar.SetActiveTab(0)
			m.updateFocus()
			return m, nil

		case "2":
			// Switch to Git Diff tab
			m.tabBar.SetActiveTab(1)
			m.updateFocus()
			return m, nil

		case "3":
			// Switch to Hooks Activity tab
			m.tabBar.SetActiveTab(2)
			m.updateFocus()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	// Handle AI Activity events
	// AIActivityRecord implements common.Event directly (no protocol wrapper)
	case *models.AIActivityRecord:
		if m.task != nil && msg.TaskID == m.task.ID {
			m.AddAIActivityRecord(msg)
		}
		return m, nil

	case protocol.AIActivityBatchEvent:
		log := logger.GetTUILogger().With().Str("component", "taskdetails").Logger()
		log.Info().
			Str("msgTaskID", msg.TaskID).
			Str("myTaskID", m.task.ID).
			Int("count", len(msg.Activities)).
			Msg("AIActivityBatchEvent received")
		if m.task != nil && msg.TaskID == m.task.ID {
			for _, activity := range msg.Activities {
				if activity != nil {
					m.hooksActivity.AddEvent(activity)
				}
			}
			log.Info().
				Int("hooksActivityEventCount", m.hooksActivity.GetEventCount()).
				Msg("After adding events to hooksActivity")
		}
		return m, nil

	case protocol.AIStreamStartEvent:
		if m.task != nil && msg.TaskID == m.task.ID {
			m.StartAIStream()
		}
		return m, nil

	case protocol.AIStreamEndEvent:
		if m.task != nil && msg.TaskID == m.task.ID {
			m.EndAIStream(msg.FinalStatus)
		}
		return m, nil
	}

	// Forward message to the focused component based on active tab
	activeTab := m.tabBar.GetActiveTab()
	switch activeTab {
	case 0: // Task Info
		if len(m.cards) > 0 {
			m.cards[0], cmd = m.cards[0].Update(msg)
			cmds = append(cmds, cmd)
		}
	case 1: // Git Diff
		if len(m.cards) > 1 {
			m.cards[1], cmd = m.cards[1].Update(msg)
			cmds = append(cmds, cmd)
		}
	case 2: // Hooks Activity
		m.hooksActivity, cmd = m.hooksActivity.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

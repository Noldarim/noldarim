// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package settings

import (
	"fmt"
	"strings"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// View renders the settings screen
func (m Model) View() string {
	layoutInfo := m.GetLayoutInfo()

	// Build the content
	var content strings.Builder

	for i, option := range m.options {
		cursor := " "
		if i == m.selectedIndex {
			cursor = ">"
		}
		content.WriteString(fmt.Sprintf("%s %s\n", cursor, option))
	}

	// Wrap with layout
	return layout.RenderLayout(content.String(), layoutInfo, m.width, m.height)
}

// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectlist

import (
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// View renders the project list screen
func (m Model) View() string {
	layoutInfo := m.GetLayoutInfo()
	return layout.RenderLayout(m.list.View(), layoutInfo, m.width, m.height)
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskstatus

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

type Option func(*Model)

func WithSpinner(s spinner.Spinner) Option {
	return func(m *Model) {
		m.spinner.Spinner = s
	}
}

func WithSpinnerStyle(style lipgloss.Style) Option {
	return func(m *Model) {
		m.spinner.Style = style
	}
}

func WithWidth(width int) Option {
	return func(m *Model) {
		m.width = width
	}
}

func NewWithOptions(text string, status models.TaskStatus, opts ...Option) Model {
	m := New(text, status)
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

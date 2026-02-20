// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package pipelinesummary

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Status represents the pipeline run status
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusCompleted
	StatusFailed
)

// SummaryData holds all the data for the pipeline summary
type SummaryData struct {
	Status         Status
	Duration       time.Duration
	TotalSteps     int
	CompletedSteps int
	FailedSteps    int
	TotalTokens    int
	CacheHitTokens int
	FilesChanged   int
	Insertions     int
	Deletions      int
	BranchName     string
	BaseCommitSHA  string
	HeadCommitSHA  string
	ErrorMessage   string
}

// Model represents the pipeline summary component
type Model struct {
	data SummaryData
}

// New creates a new pipeline summary model
func New() Model {
	return Model{}
}

// SetData updates the summary data
func (m Model) SetData(data SummaryData) Model {
	m.data = data
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	return m, nil
}

// View renders the pipeline summary
func (m Model) View() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("35"))
	fail := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

	var lines []string

	// Status
	lines = append(lines, renderStatus(m.data.Status, success, fail, accent, label))

	// Duration
	if m.data.Duration > 0 {
		lines = append(lines, fmt.Sprintf("%s %s", label.Render("Duration:"), value.Render(formatDuration(m.data.Duration))))
	}

	// Steps
	stepsInfo := fmt.Sprintf("%d/%d", m.data.CompletedSteps, m.data.TotalSteps)
	if m.data.FailedSteps > 0 {
		stepsInfo += fail.Render(fmt.Sprintf(" (%d failed)", m.data.FailedSteps))
	}
	lines = append(lines, fmt.Sprintf("%s %s", label.Render("Steps:"), value.Render(stepsInfo)))

	// Tokens
	if m.data.TotalTokens > 0 {
		tokenLine := fmt.Sprintf("%s %s", label.Render("Tokens:"), value.Render(formatNumber(m.data.TotalTokens)))
		if m.data.CacheHitTokens > 0 {
			tokenLine += dim.Render(fmt.Sprintf(" (%s cache)", formatCompact(m.data.CacheHitTokens)))
		}
		lines = append(lines, tokenLine)
	}

	// Diff stats
	if m.data.FilesChanged > 0 {
		diffLine := fmt.Sprintf("%s %d files", label.Render("Changes:"), m.data.FilesChanged)
		diffLine += " " + success.Render(fmt.Sprintf("+%d", m.data.Insertions))
		diffLine += " " + fail.Render(fmt.Sprintf("-%d", m.data.Deletions))
		lines = append(lines, diffLine)
	}

	// Git info
	if m.data.BranchName != "" {
		lines = append(lines, fmt.Sprintf("%s %s", label.Render("Branch:"), accent.Render(m.data.BranchName)))
	}
	if m.data.BaseCommitSHA != "" && m.data.HeadCommitSHA != "" {
		lines = append(lines, fmt.Sprintf("%s %s → %s",
			label.Render("Commits:"),
			dim.Render(truncateSHA(m.data.BaseCommitSHA)),
			value.Render(truncateSHA(m.data.HeadCommitSHA))))
	}

	// Error
	if m.data.ErrorMessage != "" && m.data.Status == StatusFailed {
		lines = append(lines, fail.Render("Error: "+m.data.ErrorMessage))
	}

	return strings.Join(lines, "\n")
}

func renderStatus(s Status, success, fail, accent, label lipgloss.Style) string {
	switch s {
	case StatusCompleted:
		return success.Render("✓") + " " + success.Bold(true).Render("Completed")
	case StatusFailed:
		return fail.Render("✗") + " " + fail.Bold(true).Render("Failed")
	case StatusRunning:
		return accent.Render("◦") + " " + accent.Bold(true).Render("Running")
	default:
		return label.Render("○") + " " + label.Bold(true).Render("Pending")
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	str := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(str)+(len(str)-1)/3)
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatCompact(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.0fk", float64(n)/1000)
}

func truncateSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

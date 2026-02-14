// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package hooksactivity

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

// Summary holds aggregated metrics from AI activity events
type Summary struct {
	TurnCount       int
	TotalTokens     int
	ToolsInvoked    map[string]int // tool name -> invocation count
	SessionDuration time.Duration
	SessionID       string
	IsComplete      bool
	FinalReason     string
	StartTime       time.Time
	LastEventTime   time.Time
}

// Model represents the hooks activity display component
type Model struct {
	taskID      string
	events      []*models.AIActivityRecord
	streaming   bool
	summary     Summary
	logViewport viewport.Model
	width       int
	height      int
	focused     bool
	ready       bool
}

// New creates a new hooks activity model
func New(taskID string, width, height int) Model {
	vp := viewport.New(width, height-6) // Reserve space for summary
	vp.SetContent("No activity yet...")

	return Model{
		taskID:      taskID,
		events:      make([]*models.AIActivityRecord, 0),
		streaming:   false,
		summary:     Summary{ToolsInvoked: make(map[string]int)},
		logViewport: vp,
		width:       width,
		height:      height,
		focused:     false,
		ready:       true,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the hooks activity component
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd
	m.logViewport, cmd = m.logViewport.Update(msg)
	return m, cmd
}

// SetFocus sets the focus state
func (m *Model) SetFocus(focused bool) {
	m.focused = focused
}

// IsFocused returns whether the component is focused
func (m Model) IsFocused() bool {
	return m.focused
}

// SetSize updates the component dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Reserve 6 lines for summary panel
	logHeight := height - 6
	if logHeight < 3 {
		logHeight = 3
	}

	m.logViewport.Width = width
	m.logViewport.Height = logHeight
}

// AddEvent adds a new AI activity record and updates the summary
func (m *Model) AddEvent(record *models.AIActivityRecord) {
	if record == nil {
		return
	}

	m.events = append(m.events, record)
	m.updateSummary(record)
	m.refreshLogContent()
}

// LoadBatch loads multiple records at once
func (m *Model) LoadBatch(records []*models.AIActivityRecord) {
	for _, record := range records {
		if record != nil {
			m.events = append(m.events, record)
			m.updateSummary(record)
		}
	}
	m.refreshLogContent()
}

// StartStream marks that streaming has started
func (m *Model) StartStream() {
	m.streaming = true
	m.summary.StartTime = time.Now()
}

// EndStream marks that streaming has ended
func (m *Model) EndStream(finalStatus string) {
	m.streaming = false
	m.summary.IsComplete = true
	m.summary.FinalReason = finalStatus
	if !m.summary.StartTime.IsZero() {
		m.summary.SessionDuration = time.Since(m.summary.StartTime)
	}
}

// GetEventCount returns the number of events
func (m Model) GetEventCount() int {
	return len(m.events)
}

// IsStreaming returns whether the component is receiving streaming events
func (m Model) IsStreaming() bool {
	return m.streaming
}

// updateSummary updates the summary based on a new record
func (m *Model) updateSummary(record *models.AIActivityRecord) {
	// Update session ID
	if record.SessionID != "" && m.summary.SessionID == "" {
		m.summary.SessionID = record.SessionID
	}

	// Update last event time
	m.summary.LastEventTime = record.Timestamp

	// Update start time if not set
	if m.summary.StartTime.IsZero() {
		m.summary.StartTime = record.Timestamp
	}

	// Accumulate tokens from all events
	m.summary.TotalTokens += record.InputTokens + record.OutputTokens

	// Process event-specific data based on flat fields
	switch record.EventType {
	case models.AIEventToolUse:
		if record.ToolName != "" {
			m.summary.ToolsInvoked[record.ToolName]++
		}
		m.summary.TurnCount++

	case models.AIEventSessionEnd:
		m.summary.FinalReason = record.StopReason
		m.summary.IsComplete = true
	}

	// Update duration
	if !m.summary.StartTime.IsZero() && !m.summary.LastEventTime.IsZero() {
		m.summary.SessionDuration = m.summary.LastEventTime.Sub(m.summary.StartTime)
	}
}

// refreshLogContent updates the viewport content with the event log
func (m *Model) refreshLogContent() {
	content := RenderEventLog(m.events, m.width)
	m.logViewport.SetContent(content)

	// Auto-scroll to bottom for new events
	m.logViewport.GotoBottom()
}

// ClearEvents clears all events and resets summary
func (m *Model) ClearEvents() {
	m.events = make([]*models.AIActivityRecord, 0)
	m.summary = Summary{ToolsInvoked: make(map[string]int)}
	m.refreshLogContent()
}

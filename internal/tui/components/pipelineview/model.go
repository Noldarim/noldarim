// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package pipelineview

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noldarim/noldarim/internal/tui/components/collapsiblefeed"
	"github.com/noldarim/noldarim/internal/tui/components/elapsedtimer"
	"github.com/noldarim/noldarim/internal/tui/components/pipelinesummary"
	"github.com/noldarim/noldarim/internal/tui/components/stepprogress"
	"github.com/noldarim/noldarim/internal/tui/components/tokendisplay"
)

// PollInterval is how often we check for new data
const PollInterval = 500 * time.Millisecond

// PollMsg triggers a data fetch
type PollMsg struct{}

// DataMsg carries fetched data to update the model
type DataMsg struct {
	Groups  []collapsiblefeed.ActivityGroup
	Steps   []stepprogress.Step
	Tokens  tokendisplay.TokenData
	Status  RunStatus
	Summary *pipelinesummary.SummaryData // non-nil when run is complete
}

// RunStatus represents pipeline execution status
type RunStatus int

const (
	StatusRunning RunStatus = iota
	StatusCompleted
	StatusFailed
	StatusCancelling // User requested cancellation, waiting for confirmation
	StatusCancelled  // Cancellation confirmed
)

// CancelRequestFunc is called when user presses Ctrl+C to request cancellation
type CancelRequestFunc func()

// CancelConfirmedMsg signals that cancellation has been confirmed by the orchestrator
type CancelConfirmedMsg struct {
	Status string // Final workflow status (e.g., "canceled", "terminated")
}

// DataFetcher is a function that fetches the latest pipeline data
// It's called on each poll tick. Return nil DataMsg to skip update.
type DataFetcher func(ctx context.Context) (*DataMsg, error)

// Model is the pipeline view component - a scrollable activity viewport
// with a fixed status bar at the bottom
type Model struct {
	// Layout
	viewport viewport.Model
	width    int
	height   int
	ready    bool

	// Sub-components
	feed     collapsiblefeed.Model
	timer    elapsedtimer.Model
	tokens   tokendisplay.Model
	progress stepprogress.Model
	summary  pipelinesummary.Model

	// State
	groups      []collapsiblefeed.ActivityGroup
	steps       []stepprogress.Step
	tokenData   tokendisplay.TokenData
	showSummary bool
	status      RunStatus

	// Polling
	fetcher DataFetcher
	ctx     context.Context
	cancel  context.CancelFunc

	// Cancellation
	cancelRequest CancelRequestFunc // Called when user presses Ctrl+C
}

// New creates a new pipeline view model
func New(width, height int, fetcher DataFetcher) Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Reserve space for status bar (2 lines: separator + status)
	statusBarHeight := 2
	vpHeight := height - statusBarHeight
	if vpHeight < 3 {
		vpHeight = 3
	}

	vp := viewport.New(width, vpHeight)
	vp.SetContent("Waiting for activity...")

	// Default to single step if not yet known
	steps := []stepprogress.Step{{Name: "", Status: stepprogress.StatusRunning}}

	// Calculate feed width (leave room for todo panel)
	feedWidth := width
	if width > 80 {
		feedWidth = width - 40 // Reserve space for todo panel
	}

	return Model{
		viewport: vp,
		width:    width,
		height:   height,
		feed:     collapsiblefeed.New(feedWidth, vpHeight),
		timer:    elapsedtimer.New().Start(),
		tokens:   tokendisplay.New(),
		progress: stepprogress.New().SetSteps(steps).SetWidth(15),
		summary:  pipelinesummary.New(),
		steps:    steps,
		fetcher:  fetcher,
		ctx:      ctx,
		cancel:   cancel,
		status:   StatusRunning,
	}
}

// Init starts the polling and timer
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		pollTick(),
		m.timer.Init(),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// If already cancelling, force quit on second Ctrl+C
			if m.status == StatusCancelling {
				m.cancel()
				return m, tea.Quit
			}
			// Request cancellation - don't quit immediately
			if m.cancelRequest != nil && m.status == StatusRunning {
				m.status = StatusCancelling
				m.cancelRequest()
				// Don't quit - wait for CancelConfirmedMsg
				return m, nil
			}
			// Fallback: if no cancel handler, just quit
			m.cancel()
			return m, tea.Quit
		case "q":
			// 'q' still quits immediately (for when pipeline is done)
			m.cancel()
			return m, tea.Quit
		}
		// Pass key events to viewport for scrolling
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case CancelConfirmedMsg:
		// Orchestrator confirmed cancellation - now we can quit
		m.status = StatusCancelled
		m.cancel()
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewportSize()

	case elapsedtimer.TickMsg:
		var cmd tea.Cmd
		m.timer, cmd = m.timer.Update(msg)
		cmds = append(cmds, cmd)

	case PollMsg:
		// Don't poll if we're done or cancelling
		if m.status != StatusRunning && m.status != StatusCancelling {
			return m, nil
		}
		// Fetch new data
		if m.fetcher != nil {
			data, err := m.fetcher(m.ctx)
			if err == nil && data != nil {
				cmds = append(cmds, func() tea.Msg { return *data })
			}
		}
		// Schedule next poll
		cmds = append(cmds, pollTick())

	case DataMsg:
		m = m.applyData(msg)

		// Check if pipeline is done (but not if we're cancelling - wait for CancelConfirmedMsg)
		if m.status != StatusCancelling && (msg.Status == StatusCompleted || msg.Status == StatusFailed) {
			m.status = msg.Status
			if msg.Summary != nil {
				m.summary = m.summary.SetData(*msg.Summary)
				m.showSummary = true
			}
			return m, tea.Quit
		}
	}

	return m, tea.Batch(cmds...)
}

// applyData updates the model with fetched data
func (m Model) applyData(data DataMsg) Model {
	// Update activity groups
	if len(data.Groups) > len(m.groups) {
		m.groups = data.Groups
		m.feed = m.feed.SetGroups(m.groups)
		m.refreshViewportContent()
	}

	// Update steps
	if len(data.Steps) > 0 {
		m.steps = data.Steps
		m.progress = m.progress.SetSteps(m.steps)
	}

	// Update tokens
	m.tokenData = data.Tokens
	m.tokens = m.tokens.SetData(m.tokenData)

	return m
}

// refreshViewportContent renders activity groups into the viewport
func (m *Model) refreshViewportContent() {
	// Use RenderContent() to get raw content without nested viewport
	content := m.feed.RenderContent()
	if content == "" {
		content = "Waiting for activity..."
	}
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

// updateViewportSize recalculates viewport dimensions
func (m *Model) updateViewportSize() {
	statusBarHeight := 2
	vpHeight := m.height - statusBarHeight
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
}

// SetSteps sets the initial step configuration
func (m Model) SetSteps(steps []stepprogress.Step) Model {
	m.steps = steps
	m.progress = m.progress.SetSteps(steps)
	return m
}

// SetCancelRequest sets the function to call when user requests cancellation (Ctrl+C)
func (m Model) SetCancelRequest(fn CancelRequestFunc) Model {
	m.cancelRequest = fn
	return m
}

// GetSummary returns the summary model for final display
func (m Model) GetSummary() pipelinesummary.Model {
	return m.summary
}

// ShowSummary returns whether the summary should be displayed
func (m Model) ShowSummary() bool {
	return m.showSummary
}

// Status returns the final run status
func (m Model) Status() RunStatus {
	return m.status
}

// Groups returns the collected activity groups for final display
func (m Model) Groups() []collapsiblefeed.ActivityGroup {
	return m.groups
}

// CurrentTodos returns the current todo list from the feed
func (m Model) CurrentTodos() []collapsiblefeed.TodoItem {
	return m.feed.CurrentTodos()
}

// pollTick returns a command that sends a PollMsg after the interval
func pollTick() tea.Cmd {
	return tea.Tick(PollInterval, func(time.Time) tea.Msg {
		return PollMsg{}
	})
}

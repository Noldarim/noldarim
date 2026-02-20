// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskview

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/noldarim/noldarim/internal/orchestrator/models"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/components/commitgraph"
	"github.com/noldarim/noldarim/internal/tui/components/taskstatus"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// TaskItem represents a task in the list
type TaskItem struct {
	ID        string
	TaskTitle string
	Desc      string
	Status    models.TaskStatus
}

// FilterValue returns the value to filter against
func (t TaskItem) FilterValue() string {
	return t.TaskTitle
}

// Title returns the task title without status indicator (status will be shown separately)
func (t TaskItem) Title() string {
	return t.TaskTitle
}

// Description returns the task description
func (t TaskItem) Description() string {
	return t.Desc
}

// String returns a string representation of the task item
func (t TaskItem) String() string {
	return fmt.Sprintf("%s: %s", t.Title(), t.Desc)
}

// Model is the model for the task view screen.
type Model struct {
	projectID      string
	projectName    string
	repositoryPath string
	list           list.Model
	delegate       TaskDelegate // Store reference to delegate
	cmdChan        chan<- protocol.Command
	tasks          map[string]*models.Task        // Legacy task storage
	pipelineRuns   map[string]*models.PipelineRun // New unified pipeline runs
	taskStatuses   map[string]taskstatus.Model    // Task status components (works for both)
	pendingTasks   map[string]bool                // Track tasks/runs that are pending creation
	failedTasks    map[string]time.Time           // Track failed tasks/runs with timestamp for cleanup
	showForm       bool
	form           *huh.Form
	formTitle      string
	formDesc       string
	width          int // Terminal width for layout
	height         int // Terminal height for layout

	// Tab navigation
	tabs      []string
	activeTab int

	// Commit graph related fields
	commits        []*commitgraph.Commit
	commitsLoaded  bool
	selectedCommit int
	commitLanes    map[int]int16 // Maps commit index to its lane position
	currentLane    int16         // Current lane we're navigating in
	hashPool       *commitgraph.StringPool
}

// NewModel creates a new task view model
func NewModel(projectID string, cmdChan chan<- protocol.Command) Model {
	// Create list with standard configuration
	delegate := NewTaskDelegate()
	l := list.New([]list.Item{}, delegate, 50, 10)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.Title = ""

	m := Model{
		projectID:      projectID,
		list:           l,
		delegate:       delegate,
		cmdChan:        cmdChan,
		tasks:          make(map[string]*models.Task),
		pipelineRuns:   make(map[string]*models.PipelineRun),
		taskStatuses:   make(map[string]taskstatus.Model),
		pendingTasks:   make(map[string]bool),
		failedTasks:    make(map[string]time.Time),
		showForm:       false,
		width:          80, // Default width
		height:         24, // Default height
		tabs:           []string{"Tasks", "Commits"},
		activeTab:      0,
		commits:        []*commitgraph.Commit{},
		commitsLoaded:  false,
		selectedCommit: 0,
		commitLanes:    make(map[int]int16),
		currentLane:    0,
		hashPool:       commitgraph.NewStringPool(),
	}

	m.initForm()
	return m
}

// initForm initializes the huh form for creating new tasks
func (m *Model) initForm() {
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("title").
				Title("Task Title").
				Placeholder("Enter task title...").
				Value(&m.formTitle),

			huh.NewText().
				Key("description").
				Title("Task Description").
				Placeholder("Enter task description...").
				Value(&m.formDesc),
		),
	).WithTheme(huh.ThemeCharm())
}

func (m Model) Init() tea.Cmd {
	// Send commands to load both tasks and pipeline runs for this project
	go func() {
		m.cmdChan <- protocol.LoadTasksCommand{ProjectID: m.projectID}
		m.cmdChan <- protocol.LoadPipelineRunsCommand{ProjectID: m.projectID}
	}()

	// Initialize any existing task status components
	var initCmds []tea.Cmd
	for _, statusModel := range m.taskStatuses {
		if cmd := statusModel.Init(); cmd != nil {
			initCmds = append(initCmds, cmd)
		}
	}

	if len(initCmds) > 0 {
		return tea.Batch(initCmds...)
	}
	return nil
}

// GetLayoutInfo returns layout information for the task view screen
func (m Model) GetLayoutInfo() layout.LayoutInfo {
	// Use project name in breadcrumb if available, fallback to project ID
	projectDisplayName := m.projectName
	if projectDisplayName == "" {
		projectDisplayName = m.projectID
	}
	// Ensure we always have a non-empty display name
	if projectDisplayName == "" {
		projectDisplayName = "Unknown Project"
	}

	// Keep consistent header regardless of active tab to prevent rerender jumps
	title := "Project View"

	// Count tasks
	completedCount := 0
	for _, task := range m.tasks {
		if task.Status == models.TaskStatusCompleted {
			completedCount++
		}
	}

	// Build unified status text showing both task and commit info
	var statusText string
	commitCount := len(m.commits)
	if m.repositoryPath != "" {
		statusText = fmt.Sprintf("Repository: %s | Tasks: %d (%d completed) | Commits: %d",
			m.repositoryPath, len(m.tasks), completedCount, commitCount)
	} else {
		statusText = fmt.Sprintf("Tasks: %d (%d completed) | Commits: %d",
			len(m.tasks), completedCount, commitCount)
	}

	// Provide help items that work for both tabs
	helpItems := []layout.HelpItem{
		{Key: "tab/1/2", Description: "switch tabs"},
		{Key: "enter", Description: "details"},
		{Key: "n", Description: "new"},
		{Key: "r", Description: "retry (failed)"},
		{Key: "d", Description: "delete"},
		{Key: "esc", Description: "back"},
		{Key: "q", Description: "quit"},
	}

	return layout.LayoutInfo{
		Title:       title,
		Breadcrumbs: []string{"Projects", projectDisplayName},
		Status:      statusText,
		HelpItems:   helpItems,
	}
}

// SetSize updates the terminal dimensions for layout
func (m *Model) SetSize(width, height int) {
	// Store dimensions for layout system
	m.width = width
	m.height = height

	// Update list component with appropriate size
	// The list gets most of the width and a reasonable height
	m.list.SetWidth(width - 4)   // Account for padding
	m.list.SetHeight(height - 8) // Account for headers/footers
}

// pipelineRunStatusToTaskStatus converts PipelineRunStatus to TaskStatus for unified display
func pipelineRunStatusToTaskStatus(status models.PipelineRunStatus) models.TaskStatus {
	switch status {
	case models.PipelineRunStatusPending:
		return models.TaskStatusPending
	case models.PipelineRunStatusRunning:
		return models.TaskStatusInProgress
	case models.PipelineRunStatusCompleted:
		return models.TaskStatusCompleted
	case models.PipelineRunStatusFailed:
		return models.TaskStatusFailed
	default:
		return models.TaskStatusPending
	}
}

// displayItem represents a unified display item (either Task or PipelineRun)
type displayItem struct {
	ID        string
	Title     string
	Desc      string
	Status    models.TaskStatus
	CreatedAt time.Time
}

// refreshTaskList updates the list items with current tasks/runs and creates/updates taskstatus components
func (m *Model) refreshTaskList() {
	// Collect all display items (both legacy tasks and pipeline runs)
	items := make([]displayItem, 0, len(m.tasks)+len(m.pipelineRuns))

	// Add legacy tasks
	for _, task := range m.tasks {
		items = append(items, displayItem{
			ID:        task.ID,
			Title:     task.Title,
			Desc:      task.Description,
			Status:    task.Status,
			CreatedAt: task.CreatedAt,
		})
	}

	// Add pipeline runs
	for _, run := range m.pipelineRuns {
		items = append(items, displayItem{
			ID:        run.ID,
			Title:     run.Name,
			Desc:      "", // PipelineRun doesn't have description
			Status:    pipelineRunStatusToTaskStatus(run.Status),
			CreatedAt: run.CreatedAt,
		})
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	listItems := make([]list.Item, 0, len(items))
	for _, item := range items {
		taskItem := TaskItem{
			ID:        item.ID,
			TaskTitle: item.Title,
			Desc:      item.Desc,
			Status:    item.Status,
		}

		// Create or update task status component
		if statusModel, exists := m.taskStatuses[item.ID]; exists {
			// Update existing status component
			statusModel = statusModel.SetStatus(item.Status)

			// Apply UI state based on pending/failed status
			if m.pendingTasks[item.ID] {
				statusModel = statusModel.SetUIState(taskstatus.UIStatePending)
			} else if _, isFailed := m.failedTasks[item.ID]; isFailed || item.Status == models.TaskStatusFailed {
				statusModel = statusModel.SetUIState(taskstatus.UIStateFailed)
			} else {
				statusModel = statusModel.SetUIState(taskstatus.UIStateNormal)
			}

			m.taskStatuses[item.ID] = statusModel
		} else {
			// Create new status component
			statusModel := taskstatus.New(item.Title, item.Status)

			// Apply UI state based on pending/failed status
			if m.pendingTasks[item.ID] {
				statusModel = statusModel.SetUIState(taskstatus.UIStatePending)
			} else if _, isFailed := m.failedTasks[item.ID]; isFailed || item.Status == models.TaskStatusFailed {
				statusModel = statusModel.SetUIState(taskstatus.UIStateFailed)
			} else {
				statusModel = statusModel.SetUIState(taskstatus.UIStateNormal)
			}

			m.taskStatuses[item.ID] = statusModel
		}

		listItems = append(listItems, taskItem)
	}
	m.list.SetItems(listItems)

	// Update the delegate with current task statuses
	m.delegate.SetTaskStatuses(m.taskStatuses)
	m.list.SetDelegate(m.delegate)
}

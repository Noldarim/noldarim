// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package projectcreation

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/noldarim/noldarim/internal/protocol"
	"github.com/noldarim/noldarim/internal/tui/layout"
)

// Stage represents the current stage of the form
type Stage int

const (
	DirSelection Stage = iota
	FormInput
)

// Model is the model for the project creation screen
type Model struct {
	stage        Stage
	filePicker   filepicker.Model
	selectedPath string
	form         *huh.Form
	formTitle    string
	formDesc     string
	cmdChan      chan<- protocol.Command
	width        int
	height       int
}

// NewModel creates a new project creation model
func NewModel(cmdChan chan<- protocol.Command) Model {
	// Initialize file picker
	fp := filepicker.New()
	fp.AllowedTypes = []string{} // Allow all files/dirs to be shown
	fp.DirAllowed = true
	fp.FileAllowed = false

	// Set initial directory to user's home
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	fp.CurrentDirectory = homeDir

	m := Model{
		stage:      DirSelection,
		filePicker: fp,
		cmdChan:    cmdChan,
		width:      50,
		height:     10,
	}

	m.initForm()
	return m
}

// initForm initializes the huh form for project details
func (m *Model) initForm() {
	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Key("title").
				Title("Project Title").
				Placeholder("Enter project title...").
				Value(&m.formTitle).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("title is required")
					}
					return nil
				}),

			huh.NewText().
				Key("description").
				Title("Project Description").
				Placeholder("Enter project description...").
				Value(&m.formDesc),
		),
	).WithTheme(huh.ThemeCharm())
}

func (m Model) Init() tea.Cmd {
	return m.filePicker.Init()
}

// GetLayoutInfo returns layout information for the project creation screen
func (m Model) GetLayoutInfo() layout.LayoutInfo {
	title := "Create New Project"
	breadcrumbs := []string{"Projects", "New Project"}
	status := ""

	var helpItems []layout.HelpItem

	switch m.stage {
	case DirSelection:
		status = "Select project directory"
		helpItems = []layout.HelpItem{
			{Key: "↑/↓", Description: "navigate"},
			{Key: "enter", Description: "open dir"},
			{Key: "space", Description: "select dir"},
			{Key: "esc", Description: "cancel"},
		}
	case FormInput:
		status = "Enter project details"
		helpItems = []layout.HelpItem{
			{Key: "tab", Description: "next field"},
			{Key: "enter", Description: "submit"},
			{Key: "esc", Description: "back"},
		}
	}

	return layout.LayoutInfo{
		Title:       title,
		Breadcrumbs: breadcrumbs,
		Status:      status,
		HelpItems:   helpItems,
	}
}

// SetSize updates the model's dimensions
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate content area
	layoutInfo := m.GetLayoutInfo()
	dims := layout.GetContentArea(layoutInfo, width, height)

	// Update filepicker size
	m.filePicker.Height = dims.Height - 4 // Leave room for instructions

	// Form will use default sizing
}

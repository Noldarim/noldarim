// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/noldarim/noldarim/internal/tui/components/commitgraph"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	commits        []*commitgraph.Commit
	selectedIndex  int
	hashPool       *commitgraph.StringPool
	showMergeList  bool
	mergeListIndex int
	mergeTarget    int // Index of the commit to merge with
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showMergeList {
			// Handle merge list navigation
			switch msg.String() {
			case "q", "ctrl+c", "esc":
				m.showMergeList = false
			case "up", "k":
				if m.mergeListIndex > 0 {
					m.mergeListIndex--
				}
			case "down", "j":
				if m.mergeListIndex < len(m.commits)-1 {
					m.mergeListIndex++
				}
			case "enter":
				// Create merge commit
				targetCommit := m.commits[m.mergeTarget]
				mergeCommit := m.commits[m.mergeListIndex]

				// Skip if trying to merge with self
				if m.mergeTarget != m.mergeListIndex {
					newMergeCommit := createMergeCommit(m.hashPool, *targetCommit.Hash, *mergeCommit.Hash)

					// Insert the new commit at the beginning (newest first)
					newCommits := make([]*commitgraph.Commit, len(m.commits)+1)
					newCommits[0] = newMergeCommit
					copy(newCommits[1:], m.commits)
					m.commits = newCommits

					// Keep selection on the new commit
					m.selectedIndex = 0
				}

				m.showMergeList = false
			}
		} else {
			// Handle main navigation
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				// Move to previous commit (newer in history)
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case "down", "j":
				// Move to next commit (older in history)
				if m.selectedIndex < len(m.commits)-1 {
					m.selectedIndex++
				}
			case "enter":
				// Create new commit with selected commit as parent
				selectedCommit := m.commits[m.selectedIndex]
				newCommit := createRandomCommit(m.hashPool, *selectedCommit.Hash)

				// Insert the new commit at the beginning (newest first)
				newCommits := make([]*commitgraph.Commit, len(m.commits)+1)
				newCommits[0] = newCommit
				copy(newCommits[1:], m.commits)
				m.commits = newCommits

				// Keep selection on the new commit
				m.selectedIndex = 0
			case "m":
				// Show merge list
				m.showMergeList = true
				m.mergeTarget = m.selectedIndex
				m.mergeListIndex = 0
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2)

	if m.showMergeList {
		// Show merge selection list
		var builder strings.Builder
		builder.WriteString("Select commit to merge with:\n")
		builder.WriteString("Use ↑/↓ to select, Enter to confirm, Esc to cancel\n\n")

		targetCommit := m.commits[m.mergeTarget]
		builder.WriteString(fmt.Sprintf("Merging into: %s %s\n\n",
			(*targetCommit.Hash)[:7], targetCommit.Message))

		for i, commit := range m.commits {
			listStyle := lipgloss.NewStyle()
			if i == m.mergeListIndex {
				listStyle = listStyle.Bold(true).Foreground(lipgloss.Color("15"))
			}
			if i == m.mergeTarget {
				listStyle = listStyle.Faint(true) // Dim the target commit
			}

			line := fmt.Sprintf("%s %s (%s)",
				(*commit.Hash)[:7],
				commit.Message,
				commit.Author)
			builder.WriteString(listStyle.Render(line))
			if i < len(m.commits)-1 {
				builder.WriteString("\n")
			}
		}

		return style.Render(builder.String())
	}

	// Show main commit graph
	selectedHash := m.commits[m.selectedIndex].HashPtr()

	// Create a color palette for different branches
	branchColors := []string{
		"1",  // Red
		"2",  // Green
		"3",  // Yellow
		"4",  // Blue
		"5",  // Magenta
		"6",  // Cyan
		"9",  // Bright Red
		"10", // Bright Green
		"11", // Bright Yellow
		"12", // Bright Blue
		"13", // Bright Magenta
		"14", // Bright Cyan
	}

	// Build a map of commit hash to color based on graph position
	colorMap := make(map[*string]string)

	// First pass: assign colors to commits based on their graph position
	tempGetStyle := func(c *commitgraph.Commit) *lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // Default gray
		return &s
	}

	// Get pipe sets to determine positions
	pipeSets := commitgraph.GetPipeSets(m.commits, tempGetStyle)

	// Track which color is used for each position
	positionColors := make(map[int16]string)
	nextColorIndex := 0

	for i, pipes := range pipeSets {
		commit := m.commits[i]
		var commitPos int16 = 0

		// Find the position of this commit
		for _, pipe := range pipes {
			if pipe.Kind() == commitgraph.PipeKindStarts {
				commitPos = pipe.FromPos()
				break
			}
		}

		// Assign color to position if not already assigned
		if _, exists := positionColors[commitPos]; !exists {
			positionColors[commitPos] = branchColors[nextColorIndex%len(branchColors)]
			nextColorIndex++
		}

		colorMap[commit.HashPtr()] = positionColors[commitPos]
	}

	getStyle := func(c *commitgraph.Commit) *lipgloss.Style {
		color := colorMap[c.HashPtr()]
		if color == "" {
			color = "7" // Default gray
		}
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		return &s
	}

	lines := commitgraph.RenderCommitGraph(m.commits, selectedHash, getStyle)

	var builder strings.Builder
	builder.WriteString("Commit Graph Demo\n")
	builder.WriteString("Navigation: ↑/↓ (j/k) = move through history | Enter = new commit | 'm' = merge | 'q' = quit\n\n")

	for i, line := range lines {
		if i < len(m.commits) {
			commit := m.commits[i]
			commitStyle := lipgloss.NewStyle()
			if i == m.selectedIndex {
				commitStyle = commitStyle.Bold(true).Foreground(lipgloss.Color("15"))
			}
			formattedLine := fmt.Sprintf("%s %s %s (%s)",
				line,
				commitStyle.Render((*commit.Hash)[:7]),
				commitStyle.Render(commit.Message),
				commit.Author)
			builder.WriteString(formattedLine)
		} else {
			builder.WriteString(line)
		}
		if i < len(lines)-1 {
			builder.WriteString("\n")
		}
	}

	return style.Render(builder.String())
}

func main() {
	rand.Seed(time.Now().UnixNano())
	hashPool := commitgraph.NewStringPool()
	commits := createMockCommits(hashPool)

	m := model{
		commits:        commits,
		selectedIndex:  0,
		hashPool:       hashPool,
		showMergeList:  false,
		mergeListIndex: 0,
		mergeTarget:    0,
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
	}
}

func createMockCommits(hashPool *commitgraph.StringPool) []*commitgraph.Commit {
	// Create commits in reverse chronological order (newest first) as the graph algorithm expects
	commits := []*commitgraph.Commit{

		commitgraph.NewCommit(hashPool, "a0000c3", "Merge T1I1 into main", "Alice", []string{"a0000i1", "a0000c2"}),
		commitgraph.NewCommit(hashPool, "a0000c2", "Merge T2I1 into main", "Eve", []string{"a0000i4", "a0000c1"}),
		commitgraph.NewCommit(hashPool, "a0000i4", "Task 2 - Impl 1", "Alice", []string{"a0000c1"}),
		commitgraph.NewCommit(hashPool, "a0000i3", "Task 1 - Impl 3", "Diana", []string{"a0000c1"}),
		commitgraph.NewCommit(hashPool, "a0000i2", "Task 1 - Impl 2", "Charlie", []string{"a0000c1"}),
		commitgraph.NewCommit(hashPool, "a0000i1", "Task 1 - Impl 1", "Bob", []string{""}),
		commitgraph.NewCommit(hashPool, "a0000c1", "Initial commit", "Alice", []string{}),
	}

	return commits
}

func createRandomCommit(hashPool *commitgraph.StringPool, parentHash string) *commitgraph.Commit {
	// Random commit messages
	messages := []string{
		"Fix bug in authentication",
		"Add new feature for user profiles",
		"Update dependencies",
		"Refactor code for better performance",
		"Add unit tests",
		"Update documentation",
		"Fix typo in comments",
		"Optimize database queries",
		"Add error handling",
		"Improve UI responsiveness",
		"Fix memory leak",
		"Add validation logic",
		"Update configuration",
		"Fix edge case in parser",
		"Add logging statements",
	}

	// Random authors
	authors := []string{
		"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry", "Ivy", "Jack",
	}

	// Generate random hash (simplified)
	hash := fmt.Sprintf("x%06x", rand.Intn(0xffffff))

	// Random message and author
	message := messages[rand.Intn(len(messages))]
	author := authors[rand.Intn(len(authors))]

	return commitgraph.NewCommit(hashPool, hash, message, author, []string{parentHash})
}

func createMergeCommit(hashPool *commitgraph.StringPool, parentHash1, parentHash2 string) *commitgraph.Commit {
	// Generate random hash for merge commit
	hash := fmt.Sprintf("m%06x", rand.Intn(0xffffff))

	// Create merge commit message
	message := fmt.Sprintf("Merge %s into %s", parentHash2[:7], parentHash1[:7])
	author := "Merger" // Could also randomize this

	return commitgraph.NewCommit(hashPool, hash, message, author, []string{parentHash1, parentHash2})
}

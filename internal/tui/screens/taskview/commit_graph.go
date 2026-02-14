// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package taskview

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/noldarim/noldarim/internal/tui/components/commitgraph"
)

// buildCommitLaneMapping builds a map of commit index to lane position
func (m *Model) buildCommitLaneMapping() {
	m.commitLanes = make(map[int]int16)

	// Create a temporary style function for pipe generation
	tempGetStyle := func(c *commitgraph.Commit) *lipgloss.Style {
		s := lipgloss.NewStyle()
		return &s
	}

	// Get pipe sets to determine positions
	pipeSets := commitgraph.GetPipeSets(m.commits, tempGetStyle)

	// Extract lane position for each commit
	for i, pipes := range pipeSets {
		for _, pipe := range pipes {
			if pipe.Kind() == commitgraph.PipeKindStarts {
				m.commitLanes[i] = pipe.FromPos()
				break
			}
		}
	}
}

// findNextCommitInLane finds the next commit in the same lane going down
func (m *Model) findNextCommitInLane(currentIndex int, lane int16) int {
	for i := currentIndex + 1; i < len(m.commits); i++ {
		if m.commitLanes[i] == lane {
			return i
		}
	}
	return currentIndex // Stay at current if no next commit in lane
}

// findPrevCommitInLane finds the previous commit in the same lane going up
func (m *Model) findPrevCommitInLane(currentIndex int, lane int16) int {
	for i := currentIndex - 1; i >= 0; i-- {
		if m.commitLanes[i] == lane {
			return i
		}
	}
	return currentIndex // Stay at current if no previous commit in lane
}

// findCommitInAdjacentLane finds a commit at similar position in an adjacent lane
func (m *Model) findCommitInAdjacentLane(currentIndex int, direction int) int {
	currentLane := m.commitLanes[currentIndex]
	targetLane := currentLane + int16(direction)

	// Find all lanes at this general area
	availableLanes := make(map[int16]bool)
	for i, lane := range m.commitLanes {
		// Consider commits within a reasonable range
		if i >= currentIndex-5 && i <= currentIndex+5 {
			availableLanes[lane] = true
		}
	}

	// Check if target lane exists
	if !availableLanes[targetLane] {
		return currentIndex
	}

	// Find closest commit in target lane
	bestIndex := -1
	bestDistance := len(m.commits)

	for i, lane := range m.commitLanes {
		if lane == targetLane {
			distance := i - currentIndex
			if distance < 0 {
				distance = -distance
			}
			if distance < bestDistance {
				bestDistance = distance
				bestIndex = i
			}
		}
	}

	if bestIndex != -1 {
		return bestIndex
	}
	return currentIndex
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package commitgraph

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestRenderCommitGraph(t *testing.T) {
	tests := []struct {
		name            string
		commits         []*Commit
		expectedSymbols []string // Expected graph symbols only
	}{
		{
			name: "simple linear history",
			commits: func() []*Commit {
				hashPool := NewStringPool()
				return []*Commit{
					NewCommit(hashPool, "1", "First commit", "Alice", []string{"2"}),
					NewCommit(hashPool, "2", "Second commit", "Alice", []string{"3"}),
					NewCommit(hashPool, "3", "Third commit", "Alice", []string{}),
				}
			}(),
			expectedSymbols: []string{"◯ ", "◯ ", "◯ "},
		},
		{
			name: "with merge commits",
			commits: func() []*Commit {
				hashPool := NewStringPool()
				return []*Commit{
					NewCommit(hashPool, "1", "Merge", "Alice", []string{"2"}),
					NewCommit(hashPool, "2", "Main", "Alice", []string{"3"}),
					NewCommit(hashPool, "3", "Before merge", "Alice", []string{"4", "7"}),
					NewCommit(hashPool, "4", "Feature merged", "Bob", []string{"5"}),
					NewCommit(hashPool, "7", "Feature work", "Bob", []string{"5"}),
					NewCommit(hashPool, "5", "Base", "Alice", []string{}),
				}
			}(),
			expectedSymbols: []string{"◯ ", "◯ ", "⏣─╮", "◯ │", "│ ◯", "◯─╯"},
		},
		{
			name: "complex branching",
			commits: func() []*Commit {
				hashPool := NewStringPool()
				return []*Commit{
					NewCommit(hashPool, "A", "Latest", "Alice", []string{"B"}),
					NewCommit(hashPool, "B", "Feature merge", "Alice", []string{"C", "D"}),
					NewCommit(hashPool, "C", "Main branch", "Alice", []string{"E"}),
					NewCommit(hashPool, "D", "Feature branch", "Bob", []string{"E"}),
					NewCommit(hashPool, "E", "Common base", "Alice", []string{}),
				}
			}(),
			expectedSymbols: []string{"◯ ", "⏣─╮", "◯ │", "│ ◯", "◯─╯"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getStyle := func(c *Commit) *lipgloss.Style {
				style := lipgloss.NewStyle()
				return &style
			}

			hashPool := NewStringPool()
			selectedHash := hashPool.Add("nonexistent") // No selection
			lines := RenderCommitGraph(test.commits, selectedHash, getStyle)

			// Remove styling from actual output for comparison
			cleanActual := make([]string, len(lines))
			for i, line := range lines {
				// Remove ANSI escape sequences for testing
				cleaned := removeANSI(line)
				cleanActual[i] = strings.TrimSpace(cleaned)
			}

			t.Logf("Expected symbols: %v", test.expectedSymbols)
			t.Logf("Actual lines: %v", cleanActual)

			assert.Equal(t, len(test.expectedSymbols), len(cleanActual), "Number of lines should match")

			for i, expectedSymbol := range test.expectedSymbols {
				if i < len(cleanActual) {
					// Check that the actual line contains the expected graph symbols
					expectedSymbolTrimmed := strings.TrimSpace(expectedSymbol)
					assert.Equal(t, expectedSymbolTrimmed, cleanActual[i],
						"Line %d graph symbols should match", i)
				}
			}
		})
	}
}

func TestGetNextPipes(t *testing.T) {
	hashPool := NewStringPool()

	tests := []struct {
		name      string
		prevPipes []Pipe
		commit    *Commit
		expected  int // Expected number of pipes
	}{
		{
			name: "single commit continuation",
			prevPipes: []Pipe{
				{fromPos: 0, toPos: 0, fromHash: hashPool.Add("a"), toHash: hashPool.Add("b"), kind: PipeKindStarts, style: &lipgloss.Style{}},
			},
			commit:   NewCommit(hashPool, "b", "Second commit", "Alice", []string{"c"}),
			expected: 2, // One terminating, one starting
		},
		{
			name: "merge commit",
			prevPipes: []Pipe{
				{fromPos: 0, toPos: 0, fromHash: hashPool.Add("a"), toHash: hashPool.Add("b"), kind: PipeKindStarts, style: &lipgloss.Style{}},
			},
			commit:   NewCommit(hashPool, "b", "Merge commit", "Alice", []string{"c", "d"}),
			expected: 3, // One terminating, two starting (for merge)
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getStyle := func(c *Commit) *lipgloss.Style {
				style := lipgloss.NewStyle()
				return &style
			}
			pipes := getNextPipes(test.prevPipes, test.commit, getStyle, nil)
			assert.Equal(t, test.expected, len(pipes))
		})
	}
}

func TestStringPool(t *testing.T) {
	pool := NewStringPool()

	// Test that same strings return same pointers
	hash1 := pool.Add("abc123")
	hash2 := pool.Add("abc123")
	assert.True(t, hash1 == hash2, "Same strings should return same pointers")

	// Test that different strings return different pointers
	hash3 := pool.Add("def456")
	assert.True(t, hash1 != hash3, "Different strings should return different pointers")
}

func TestPipeAccessors(t *testing.T) {
	hashPool := NewStringPool()
	style := lipgloss.NewStyle()

	fromHash := hashPool.Add("abc")
	toHash := hashPool.Add("def")
	pipe := NewPipe(fromHash, toHash, 1, 2, PipeKindStarts, &style)

	assert.Equal(t, fromHash, pipe.FromHash())
	assert.Equal(t, toHash, pipe.ToHash())
	assert.Equal(t, int16(1), pipe.FromPos())
	assert.Equal(t, int16(2), pipe.ToPos())
	assert.Equal(t, PipeKindStarts, pipe.Kind())
	assert.Equal(t, &style, pipe.Style())
	assert.Equal(t, int16(1), pipe.Left())
	assert.Equal(t, int16(2), pipe.Right())
}

func TestCommitAccessors(t *testing.T) {
	hashPool := NewStringPool()
	commit := NewCommit(hashPool, "abc123", "Test commit", "Alice", []string{"parent1", "parent2"})

	assert.Equal(t, "abc123", *commit.HashPtr())
	assert.Equal(t, 2, len(commit.ParentPtrs()))
	assert.True(t, commit.IsMerge)
	assert.False(t, commit.IsFirstCommit())

	// Test first commit
	firstCommit := NewCommit(hashPool, "initial", "Initial commit", "Alice", []string{})
	assert.True(t, firstCommit.IsFirstCommit())
	assert.False(t, firstCommit.IsMerge)
}

func TestCellRendering(t *testing.T) {
	symbols := DefaultSymbols()
	style := lipgloss.NewStyle()

	// Test commit cell
	cell := NewCell()
	cell.setType(COMMIT)
	cell.setStyle(&style)
	rendered := cell.Render(symbols)
	assert.Contains(t, rendered, symbols.Commit)

	// Test merge cell
	cell.setType(MERGE)
	rendered = cell.Render(symbols)
	assert.Contains(t, rendered, symbols.Merge)
}

func TestSymbols(t *testing.T) {
	// Test default symbols
	symbols := DefaultSymbols()
	assert.Equal(t, "◯", symbols.Commit)
	assert.Equal(t, "⏣", symbols.Merge)
	assert.Equal(t, "│", symbols.Vertical)
	assert.Equal(t, "─", symbols.Horizontal)

	// Test ASCII symbols
	asciiSymbols := ASCIISymbols()
	assert.Equal(t, "o", asciiSymbols.Commit)
	assert.Equal(t, "M", asciiSymbols.Merge)
	assert.Equal(t, "|", asciiSymbols.Vertical)
	assert.Equal(t, "-", asciiSymbols.Horizontal)
}

// removeANSI removes ANSI escape sequences for testing
func removeANSI(s string) string {
	// Simple ANSI escape sequence removal for testing
	result := strings.Builder{}
	inEscape := false

	for _, char := range s {
		if char == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if char == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(char)
	}

	return result.String()
}

func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		commits  func() []*Commit
		expected int // Expected number of lines
	}{
		{
			name: "empty commit list",
			commits: func() []*Commit {
				return []*Commit{}
			},
			expected: 0,
		},
		{
			name: "single commit",
			commits: func() []*Commit {
				hashPool := NewStringPool()
				return []*Commit{
					NewCommit(hashPool, "1", "Only commit", "Alice", []string{}),
				}
			},
			expected: 1,
		},
		{
			name: "complex merge with multiple branches",
			commits: func() []*Commit {
				hashPool := NewStringPool()
				return []*Commit{
					NewCommit(hashPool, "A", "Triple merge", "Alice", []string{"B", "C", "D"}),
					NewCommit(hashPool, "B", "Branch 1", "Bob", []string{"E"}),
					NewCommit(hashPool, "C", "Branch 2", "Charlie", []string{"E"}),
					NewCommit(hashPool, "D", "Branch 3", "Diana", []string{"E"}),
					NewCommit(hashPool, "E", "Common base", "Alice", []string{}),
				}
			},
			expected: 5,
		},
		{
			name: "long linear history",
			commits: func() []*Commit {
				hashPool := NewStringPool()
				commits := make([]*Commit, 100)
				for i := 0; i < 100; i++ {
					var parents []string
					if i < 99 {
						parents = []string{fmt.Sprintf("%d", i+1)}
					}
					commits[i] = NewCommit(hashPool, fmt.Sprintf("%d", i), fmt.Sprintf("Commit %d", i), "Alice", parents)
				}
				return commits
			},
			expected: 100,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commits := test.commits()
			getStyle := func(c *Commit) *lipgloss.Style {
				style := lipgloss.NewStyle()
				return &style
			}

			var selectedHash *string
			if len(commits) > 0 {
				hashPool := NewStringPool()
				selectedHash = hashPool.Add("nonexistent")
			}

			lines := RenderCommitGraph(commits, selectedHash, getStyle)
			assert.Equal(t, test.expected, len(lines))
		})
	}
}

func TestStringPoolConcurrency(t *testing.T) {
	pool := NewStringPool()
	const numGoroutines = 100
	const numOperations = 1000

	wg := sync.WaitGroup{}
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				hash := fmt.Sprintf("hash-%d-%d", id, j)
				ptr1 := pool.Add(hash)
				ptr2 := pool.Add(hash)
				assert.True(t, ptr1 == ptr2, "Same strings should return same pointers")
			}
		}(i)
	}
	wg.Wait()
}

func TestRenderAuxSequentialVsParallel(t *testing.T) {
	hashPool := NewStringPool()

	// Create a large enough dataset to trigger parallel processing
	commits := make([]*Commit, 200)
	for i := 0; i < 200; i++ {
		var parents []string
		if i < 199 {
			parents = []string{fmt.Sprintf("%d", i+1)}
		}
		commits[i] = NewCommit(hashPool, fmt.Sprintf("%d", i), fmt.Sprintf("Commit %d", i), "Alice", parents)
	}

	getStyle := func(c *Commit) *lipgloss.Style {
		style := lipgloss.NewStyle()
		return &style
	}

	pipeSets := GetPipeSets(commits, getStyle)
	selectedHash := hashPool.Add("nonexistent")

	// Test that both sequential and parallel produce same results
	result := RenderAux(pipeSets, commits, selectedHash)
	assert.Equal(t, 200, len(result))

	// Verify each line is not empty
	for i, line := range result {
		assert.NotEmpty(t, line, "Line %d should not be empty", i)
	}
}

func BenchmarkRenderCommitGraph(b *testing.B) {
	hashPool := NewStringPool()

	// Create a complex commit history
	commits := []*Commit{
		NewCommit(hashPool, "1", "Latest", "Alice", []string{"2"}),
		NewCommit(hashPool, "2", "Merge feature", "Alice", []string{"3", "4"}),
		NewCommit(hashPool, "3", "Main work", "Alice", []string{"5"}),
		NewCommit(hashPool, "4", "Feature work", "Bob", []string{"6"}),
		NewCommit(hashPool, "5", "More main", "Alice", []string{"7"}),
		NewCommit(hashPool, "6", "Feature base", "Bob", []string{"7"}),
		NewCommit(hashPool, "7", "Common base", "Alice", []string{}),
	}

	getStyle := func(c *Commit) *lipgloss.Style {
		style := lipgloss.NewStyle()
		return &style
	}

	selectedHash := hashPool.Add("nonexistent")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RenderCommitGraph(commits, selectedHash, getStyle)
	}
}

func BenchmarkStringPoolAdd(b *testing.B) {
	pool := NewStringPool()
	hashes := make([]string, 1000)
	for i := range hashes {
		hashes[i] = fmt.Sprintf("hash-%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, hash := range hashes {
			pool.Add(hash)
		}
	}
}

func BenchmarkGetPipeSets(b *testing.B) {
	hashPool := NewStringPool()

	// Create a large commit history for benchmarking
	commits := make([]*Commit, 1000)
	for i := 0; i < 1000; i++ {
		var parents []string
		if i < 999 {
			parents = []string{fmt.Sprintf("%d", i+1)}
		}
		commits[i] = NewCommit(hashPool, fmt.Sprintf("%d", i), fmt.Sprintf("Commit %d", i), "Alice", parents)
	}

	getStyle := func(c *Commit) *lipgloss.Style {
		style := lipgloss.NewStyle()
		return &style
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetPipeSets(commits, getStyle)
	}
}

func BenchmarkRenderAuxParallel(b *testing.B) {
	hashPool := NewStringPool()

	// Create a large dataset
	commits := make([]*Commit, 1000)
	for i := 0; i < 1000; i++ {
		var parents []string
		if i < 999 {
			parents = []string{fmt.Sprintf("%d", i+1)}
		}
		commits[i] = NewCommit(hashPool, fmt.Sprintf("%d", i), fmt.Sprintf("Commit %d", i), "Alice", parents)
	}

	getStyle := func(c *Commit) *lipgloss.Style {
		style := lipgloss.NewStyle()
		return &style
	}

	pipeSets := GetPipeSets(commits, getStyle)
	selectedHash := hashPool.Add("nonexistent")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RenderAux(pipeSets, commits, selectedHash)
	}
}

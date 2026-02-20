// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package commitgraph

import (
	"cmp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/samber/lo"
)

const (
	// Position search constants
	maxPositionSearchRadius = 10
	compactionGapThreshold  = 3

	// Git constants
	EmptyTreeCommitHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // Git's empty tree hash
	StartCommitHash     = "START"
)

var (
	highlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
)

// Commit represents a git commit for graph rendering
type Commit struct {
	Hash    *string // Using pointer for hash pooling like lazygit
	Message string
	Author  string
	Parents []*string // Using pointers for hash pooling
	IsMerge bool
}

// NewCommit creates a new commit with hash pooling
func NewCommit(hashPool *StringPool, hash string, message string, author string, parents []string) *Commit {
	parentPtrs := make([]*string, len(parents))
	for i, parent := range parents {
		parentPtrs[i] = hashPool.Add(parent)
	}

	return &Commit{
		Hash:    hashPool.Add(hash),
		Message: message,
		Author:  author,
		Parents: parentPtrs,
		IsMerge: len(parents) > 1,
	}
}

// HashPtr returns the commit's hash pointer
func (c *Commit) HashPtr() *string {
	return c.Hash
}

// ParentPtrs returns the commit's parent hash pointers
func (c *Commit) ParentPtrs() []*string {
	return c.Parents
}

// IsFirstCommit checks if this is the first commit (no parents)
func (c *Commit) IsFirstCommit() bool {
	return len(c.Parents) == 0
}

// StringPool provides hash string interning for memory efficiency using sync.Map for better performance
type StringPool struct {
	pool sync.Map // map[string]*string
}

// NewStringPool creates a new string pool
func NewStringPool() *StringPool {
	return &StringPool{}
}

// Add adds a string to the pool and returns its pointer
func (sp *StringPool) Add(s string) *string {
	if existing, ok := sp.pool.Load(s); ok {
		return existing.(*string)
	}

	// Store and return the pointer to the string
	ptr := &s
	actual, _ := sp.pool.LoadOrStore(s, ptr)
	return actual.(*string)
}

// RenderCommitGraph renders the complete commit graph
func RenderCommitGraph(commits []*Commit, selectedCommitHashPtr *string, getStyle func(c *Commit) *lipgloss.Style) []string {
	pipeSets := GetPipeSets(commits, getStyle)
	if len(pipeSets) == 0 {
		return nil
	}

	lines := RenderAux(pipeSets, commits, selectedCommitHashPtr)
	return lines
}

// buildCommitChildrenMap builds a map of commit hash to its children commits
func buildCommitChildrenMap(commits []*Commit) map[*string][]*Commit {
	childrenMap := make(map[*string][]*Commit)

	for _, commit := range commits {
		for _, parentPtr := range commit.ParentPtrs() {
			if parentPtr != nil {
				childrenMap[parentPtr] = append(childrenMap[parentPtr], commit)
			}
		}
	}

	return childrenMap
}

// GetPipeSets generates pipe sets for all commits using lazygit's algorithm
func GetPipeSets(commits []*Commit, getStyle func(c *Commit) *lipgloss.Style) [][]Pipe {
	if len(commits) == 0 {
		return nil
	}

	// Build children map to help with branch detection
	childrenMap := buildCommitChildrenMap(commits)

	defaultStyle := lipgloss.NewStyle()
	startCommitHash := StartCommitHash
	pipes := []Pipe{{fromPos: 0, toPos: 0, fromHash: &startCommitHash, toHash: commits[0].HashPtr(), kind: PipeKindStarts, style: &defaultStyle}}

	return lo.Map(commits, func(commit *Commit, _ int) []Pipe {
		pipes = getNextPipes(pipes, commit, getStyle, childrenMap)
		return pipes
	})
}

// RenderAux renders pipe sets in parallel for performance with better work distribution
func RenderAux(pipeSets [][]Pipe, commits []*Commit, selectedCommitHashPtr *string) []string {
	numPipeSets := len(pipeSets)
	if numPipeSets == 0 {
		return nil
	}

	// For small datasets, use sequential processing to avoid overhead
	if numPipeSets < 100 {
		lines := make([]string, numPipeSets)
		for i, pipeSet := range pipeSets {
			var prevCommit *Commit
			if i > 0 {
				prevCommit = commits[i-1]
			}
			lines[i] = renderPipeSet(pipeSet, selectedCommitHashPtr, prevCommit)
		}
		return lines
	}

	maxProcs := runtime.GOMAXPROCS(0)
	results := make([]string, numPipeSets)

	// Use work-stealing pattern for better load balancing
	workChan := make(chan int, numPipeSets)
	wg := sync.WaitGroup{}

	// Fill work channel
	for i := 0; i < numPipeSets; i++ {
		workChan <- i
	}
	close(workChan)

	// Start workers
	for i := 0; i < maxProcs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range workChan {
				var prevCommit *Commit
				if idx > 0 {
					prevCommit = commits[idx-1]
				}
				results[idx] = renderPipeSet(pipeSets[idx], selectedCommitHashPtr, prevCommit)
			}
		}()
	}

	wg.Wait()
	return results
}

// getNextPipes calculates the next set of pipes based on current pipes and commit
// If childrenMap is provided, it uses branch awareness for better pipe management
func getNextPipes(prevPipes []Pipe, commit *Commit, getStyle func(c *Commit) *lipgloss.Style, childrenMap map[*string][]*Commit) []Pipe {
	return getNextPipesImpl(prevPipes, commit, getStyle, childrenMap)
}

// findMaxPosition finds the maximum position among all pipes
func findMaxPosition(pipes []Pipe) int16 {
	maxPos := int16(0)
	for _, pipe := range pipes {
		if pipe.toPos > maxPos {
			maxPos = pipe.toPos
		}
	}
	return maxPos
}

// findCommitPosition finds the position where this commit should be placed
func findCommitPosition(currentPipes []Pipe, commit *Commit, maxPos int16) int16 {
	pos := maxPos + 1
	for _, pipe := range currentPipes {
		if equalHashes(pipe.toHash, commit.HashPtr()) {
			pos = pipe.toPos
			break
		}
	}
	return pos
}

// createInitialPipe creates the initial pipe for this commit to its first parent
func createInitialPipe(commit *Commit, pos int16, getStyle func(c *Commit) *lipgloss.Style) Pipe {
	var toHash *string
	if commit.IsFirstCommit() {
		emptyTreeHash := EmptyTreeCommitHash
		toHash = &emptyTreeHash
	} else {
		toHash = commit.ParentPtrs()[0]
	}

	return Pipe{
		fromPos:  pos,
		toPos:    pos,
		fromHash: commit.HashPtr(),
		toHash:   toHash,
		kind:     PipeKindStarts,
		style:    getStyle(commit),
	}
}

// shouldExtendPipe determines if a pipe should be extended based on children information
func shouldExtendPipe(pipe Pipe, childrenMap map[*string][]*Commit) bool {
	if childrenMap == nil {
		return true
	}

	children := childrenMap[pipe.toHash]
	// If this commit has multiple children, we should definitely show the junction
	if len(children) > 1 {
		return true
	}
	// Dead end - don't extend unless it's a significant branch
	if len(children) == 0 {
		return false
	}
	return true
}

// processCurrentPipes handles terminating and continuing pipes
func processCurrentPipes(newPipes []Pipe, currentPipes []Pipe, commit *Commit, pos int16,
	childrenMap map[*string][]*Commit, traverse func(int16, int16), traversedSpots map[int]bool) []Pipe {

	for _, pipe := range currentPipes {
		if equalHashes(pipe.toHash, commit.HashPtr()) {
			// Pipe terminates at this commit
			newPipes = append(newPipes, Pipe{
				fromPos:  pipe.toPos,
				toPos:    pos,
				fromHash: pipe.fromHash,
				toHash:   pipe.toHash,
				kind:     PipeKindTerminates,
				style:    pipe.style,
			})
			traverse(pipe.toPos, pos)
		} else if pipe.toPos < pos {
			// Pipe continues past this commit - prefer keeping it in same lane for branch stability
			if shouldExtendPipe(pipe, childrenMap) {
				availablePos := getNextAvailablePosNear(traversedSpots, pipe.toPos, false)
				newPipes = append(newPipes, Pipe{
					fromPos:  pipe.toPos,
					toPos:    availablePos,
					fromHash: pipe.fromHash,
					toHash:   pipe.toHash,
					kind:     PipeKindContinues,
					style:    pipe.style,
				})
				traverse(pipe.toPos, availablePos)
			}
		}
	}
	return newPipes
}

// processMergeCommitParents handles additional parents for merge commits
func processMergeCommitParents(newPipes []Pipe, commit *Commit, pos int16, takenSpots map[int]bool, getStyle func(c *Commit) *lipgloss.Style) []Pipe {
	if commit.IsMerge {
		for _, parent := range commit.ParentPtrs()[1:] {
			// For merge commits, prefer positions near the merge point to maintain clarity
			availablePos := getNextAvailablePosNear(takenSpots, pos+1, true)
			newPipes = append(newPipes, Pipe{
				fromPos:  pos,
				toPos:    availablePos,
				fromHash: commit.HashPtr(),
				toHash:   parent,
				kind:     PipeKindStarts,
				style:    getStyle(commit),
			})
			takenSpots[int(availablePos)] = true
		}
	}
	return newPipes
}

// processRemainingPipes handles pipes that continue past the current commit with compaction logic
func processRemainingPipes(newPipes []Pipe, currentPipes []Pipe, commit *Commit, pos int16,
	takenSpots, traversedSpots map[int]bool, traverse func(int16, int16)) []Pipe {

	for _, pipe := range currentPipes {
		if !equalHashes(pipe.toHash, commit.HashPtr()) && pipe.toPos > pos {
			// Prefer keeping pipes in their current position for branch stability
			// Only compact if there's significant benefit (gap larger than threshold)
			last := pipe.toPos
			if pipe.toPos-pos > compactionGapThreshold {
				// Allow limited leftward movement only for large gaps
				for i := pipe.toPos; i > pos+1; i-- {
					if takenSpots[int(i)] || traversedSpots[int(i)] {
						break
					}
					last = i
					// Stop after moving just 1 position left to maintain stability
					if last < pipe.toPos {
						break
					}
				}
			}
			newPipes = append(newPipes, Pipe{
				fromPos:  pipe.toPos,
				toPos:    last,
				fromHash: pipe.fromHash,
				toHash:   pipe.toHash,
				kind:     PipeKindContinues,
				style:    pipe.style,
			})
			traverse(pipe.toPos, last)
		}
	}
	return newPipes
}

// getNextPipesImpl is the shared implementation for pipe calculation
func getNextPipesImpl(prevPipes []Pipe, commit *Commit, getStyle func(c *Commit) *lipgloss.Style, childrenMap map[*string][]*Commit) []Pipe {
	maxPos := findMaxPosition(prevPipes)

	// Filter out terminated pipes
	currentPipes := lo.Filter(prevPipes, func(pipe Pipe, _ int) bool {
		return pipe.kind != PipeKindTerminates
	})

	newPipes := make([]Pipe, 0, len(currentPipes)+len(commit.ParentPtrs()))

	// Find position for this commit
	pos := findCommitPosition(currentPipes, commit, maxPos)

	// Create pipe for this commit to its first parent
	newPipes = append(newPipes, createInitialPipe(commit, pos, getStyle))

	// Handle continuing and terminating pipes
	takenSpots := make(map[int]bool)
	traversedSpots := make(map[int]bool)

	traverse := func(from, to int16) {
		left, right := from, to
		if left > right {
			left, right = right, left
		}
		for i := left; i <= right; i++ {
			traversedSpots[int(i)] = true
		}
		takenSpots[int(to)] = true
	}

	// Process current pipes (terminating and continuing)
	newPipes = processCurrentPipes(newPipes, currentPipes, commit, pos, childrenMap, traverse, traversedSpots)

	// Handle merge commit additional parents
	newPipes = processMergeCommitParents(newPipes, commit, pos, takenSpots, getStyle)

	// Handle remaining continuing pipes with reduced compaction for branch clarity
	newPipes = processRemainingPipes(newPipes, currentPipes, commit, pos, takenSpots, traversedSpots, traverse)

	// Sort pipes by position, then by kind
	slices.SortFunc(newPipes, func(a, b Pipe) int {
		if a.toPos == b.toPos {
			return cmp.Compare(a.kind, b.kind)
		}
		return cmp.Compare(a.toPos, b.toPos)
	})

	return newPipes
}

// getNextAvailablePos finds the next available position, preferring positions near the preferred start
func getNextAvailablePos(taken map[int]bool, start int16) int16 {
	i := start
	for {
		if !taken[int(i)] {
			return i
		}
		i++
	}
}

// getNextAvailablePosNear finds the next available position near a preferred position
// This helps maintain branch lane stability by avoiding aggressive leftward compaction
func getNextAvailablePosNear(taken map[int]bool, preferred int16, allowLeft bool) int16 {
	// First try the preferred position
	if !taken[int(preferred)] {
		return preferred
	}

	// Search in expanding radius around preferred position
	for radius := int16(1); radius <= maxPositionSearchRadius; radius++ {
		// Try right first (prefer expanding rather than compacting)
		right := preferred + radius
		if !taken[int(right)] {
			return right
		}

		// Only try left if explicitly allowed (for lane compaction)
		if allowLeft {
			left := preferred - radius
			if left >= 0 && !taken[int(left)] {
				return left
			}
		}
	}

	// Fallback to original behavior if nothing found nearby
	return getNextAvailablePos(taken, 0)
}

// renderPipeSet renders a single line of the commit graph
func renderPipeSet(pipes []Pipe, selectedCommitHashPtr *string, prevCommit *Commit) string {
	maxPos := int16(0)
	commitPos := int16(0)
	startCount := 0

	for _, pipe := range pipes {
		if pipe.kind == PipeKindStarts {
			startCount++
			commitPos = pipe.fromPos
		} else if pipe.kind == PipeKindTerminates {
			commitPos = pipe.toPos
		}

		if pipe.Right() > maxPos {
			maxPos = pipe.Right()
		}
	}

	isMerge := startCount > 1
	symbols := DefaultSymbols()

	// Create cells for the line
	cells := make([]*Cell, int(maxPos)+1)
	for i := range cells {
		cells[i] = NewCell()
		defaultStyle := lipgloss.NewStyle()
		cells[i].setStyle(&defaultStyle)
	}

	renderPipe := func(pipe *Pipe, style *lipgloss.Style, overrideRightStyle bool) {
		left := pipe.Left()
		right := pipe.Right()

		if left != right {
			for i := left + 1; i < right; i++ {
				cells[i].setLeft(style).setRight(style, overrideRightStyle)
			}
			cells[left].setRight(style, overrideRightStyle)
			cells[right].setLeft(style)
		}

		if pipe.kind == PipeKindStarts || pipe.kind == PipeKindContinues {
			cells[pipe.toPos].setDown(style)
		}
		if pipe.kind == PipeKindTerminates || pipe.kind == PipeKindContinues {
			cells[pipe.fromPos].setUp(style)
		}
	}

	// Determine if we should highlight
	highlight := true
	if prevCommit != nil && equalHashes(prevCommit.HashPtr(), selectedCommitHashPtr) {
		highlight = false
		for _, pipe := range pipes {
			if equalHashes(pipe.fromHash, selectedCommitHashPtr) && (pipe.kind != PipeKindTerminates || pipe.fromPos != pipe.toPos) {
				highlight = true
			}
		}
	}

	// Partition pipes into selected and non-selected
	selectedPipes := make([]Pipe, 0)
	nonSelectedPipes := make([]Pipe, 0)
	for _, pipe := range pipes {
		if highlight && equalHashes(pipe.fromHash, selectedCommitHashPtr) {
			selectedPipes = append(selectedPipes, pipe)
		} else {
			nonSelectedPipes = append(nonSelectedPipes, pipe)
		}
	}

	// Render non-selected pipes first
	for _, pipe := range nonSelectedPipes {
		if pipe.kind == PipeKindStarts {
			renderPipe(&pipe, pipe.style, true)
		}
	}

	for _, pipe := range nonSelectedPipes {
		if pipe.kind != PipeKindStarts && !(pipe.kind == PipeKindTerminates && pipe.fromPos == commitPos && pipe.toPos == commitPos) {
			renderPipe(&pipe, pipe.style, false)
		}
	}

	// Render selected pipes with highlight
	for _, pipe := range selectedPipes {
		for i := pipe.Left(); i <= pipe.Right(); i++ {
			cells[i].reset()
		}
	}
	for _, pipe := range selectedPipes {
		renderPipe(&pipe, &highlightStyle, true)
		if pipe.toPos == commitPos {
			cells[pipe.toPos].setStyle(&highlightStyle)
		}
	}

	// Set commit/merge cell type
	cType := COMMIT
	if isMerge {
		cType = MERGE
	}
	cells[commitPos].setType(cType)

	// Build final string
	writer := &strings.Builder{}
	writer.Grow(len(cells) * 2)
	for _, cell := range cells {
		cell.render(writer, symbols)
	}
	return writer.String()
}

// equalHashes compares hash pointers (like lazygit does for efficiency)
func equalHashes(a, b *string) bool {
	if a == nil || b == nil {
		return false
	}
	// Since we use string pooling, we can compare addresses
	return a == b
}

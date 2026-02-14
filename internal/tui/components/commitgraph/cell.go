// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package commitgraph

import (
	"io"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// cellType represents the type of content in a graph cell
type cellType int

const (
	CONNECTION cellType = iota
	COMMIT
	MERGE
)

// Cell represents a single cell in the commit graph display
type Cell struct {
	up, down, left, right bool
	cellType              cellType
	rightStyle            *lipgloss.Style
	style                 *lipgloss.Style
}

// NewCell creates a new cell with default styling
func NewCell() *Cell {
	defaultStyle := lipgloss.NewStyle()
	return &Cell{
		cellType:   CONNECTION,
		style:      &defaultStyle,
		rightStyle: nil,
	}
}

// Render writes the cell's visual representation to a string writer
func (cell *Cell) render(writer io.StringWriter, symbols GraphSymbols) {
	up, down, left, right := cell.up, cell.down, cell.left, cell.right

	first, second := getBoxDrawingChars(up, down, left, right, symbols)
	var adjustedFirst string
	switch cell.cellType {
	case CONNECTION:
		adjustedFirst = first
	case COMMIT:
		adjustedFirst = symbols.Commit
	case MERGE:
		adjustedFirst = symbols.Merge
	}

	var rightStyle *lipgloss.Style
	if cell.rightStyle == nil {
		rightStyle = cell.style
	} else {
		rightStyle = cell.rightStyle
	}

	// Handle styling for the second character
	var styledSecondChar string
	if second == " " {
		styledSecondChar = " "
	} else {
		styledSecondChar = cachedSprint(*rightStyle, second)
	}

	_, _ = writer.WriteString(cachedSprint(*cell.style, adjustedFirst))
	_, _ = writer.WriteString(styledSecondChar)
}

// Render returns the string representation of the cell
func (cell *Cell) Render(symbols GraphSymbols) string {
	var builder strings.Builder
	cell.render(&builder, symbols)
	return builder.String()
}

// Cache for styled strings to improve performance using sync.Map
type styleCacheKey struct {
	stylePtr *lipgloss.Style // Use pointer to style for better cache efficiency
	str      string
}

var styleCache sync.Map // map[styleCacheKey]string

// cachedSprint caches styled string rendering for performance
func cachedSprint(style lipgloss.Style, str string) string {
	// Use pointer to style for more efficient caching
	key := styleCacheKey{stylePtr: &style, str: str}

	if cached, ok := styleCache.Load(key); ok {
		return cached.(string)
	}

	rendered := style.Render(str)
	styleCache.Store(key, rendered)
	return rendered
}

// Reset clears all connection directions
func (cell *Cell) reset() {
	cell.up = false
	cell.down = false
	cell.left = false
	cell.right = false
}

// setUp sets the upward connection
func (cell *Cell) setUp(style *lipgloss.Style) *Cell {
	cell.up = true
	cell.style = style
	return cell
}

// setDown sets the downward connection
func (cell *Cell) setDown(style *lipgloss.Style) *Cell {
	cell.down = true
	cell.style = style
	return cell
}

// setLeft sets the leftward connection
func (cell *Cell) setLeft(style *lipgloss.Style) *Cell {
	cell.left = true
	if !cell.up && !cell.down {
		// vertical trumps left
		cell.style = style
	}
	return cell
}

// setRight sets the rightward connection
func (cell *Cell) setRight(style *lipgloss.Style, override bool) *Cell {
	cell.right = true
	if cell.rightStyle == nil || override {
		cell.rightStyle = style
	}
	return cell
}

// setStyle sets the primary style for the cell
func (cell *Cell) setStyle(style *lipgloss.Style) *Cell {
	cell.style = style
	return cell
}

// setType sets the cell type (commit, merge, connection)
func (cell *Cell) setType(cellType cellType) *Cell {
	cell.cellType = cellType
	return cell
}

// getBoxDrawingChars returns the appropriate box drawing characters for the given connections
func getBoxDrawingChars(up, down, left, right bool, symbols GraphSymbols) (string, string) {
	if up && down && left && right {
		return symbols.Vertical, symbols.Horizontal
	} else if up && down && left && !right {
		return symbols.Vertical, " "
	} else if up && down && !left && right {
		return symbols.Vertical, symbols.Horizontal
	} else if up && down && !left && !right {
		return symbols.Vertical, " "
	} else if up && !down && left && right {
		return symbols.UpMerge, symbols.Horizontal
	} else if up && !down && left && !right {
		return symbols.UpLeft, " "
	} else if up && !down && !left && right {
		return symbols.UpRight, symbols.Horizontal
	} else if up && !down && !left && !right {
		return symbols.Vertical, " " // single upward line
	} else if !up && down && left && right {
		return symbols.DownMerge, symbols.Horizontal
	} else if !up && down && left && !right {
		return symbols.DownLeft, " "
	} else if !up && down && !left && right {
		return symbols.DownRight, symbols.Horizontal
	} else if !up && down && !left && !right {
		return symbols.Vertical, " " // single downward line
	} else if !up && !down && left && right {
		return symbols.Horizontal, symbols.Horizontal
	} else if !up && !down && left && !right {
		return symbols.Horizontal, " "
	} else if !up && !down && !left && right {
		return symbols.Horizontal, symbols.Horizontal // start of horizontal line
	} else if !up && !down && !left && !right {
		return " ", " "
	}

	// Fallback (should not happen)
	return " ", " "
}

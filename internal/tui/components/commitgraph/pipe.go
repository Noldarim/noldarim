// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package commitgraph

import (
	"github.com/charmbracelet/lipgloss"
)

// PipeKind represents the type of pipe in the graph
type PipeKind uint8

const (
	PipeKindTerminates PipeKind = iota
	PipeKindStarts
	PipeKindContinues
)

// Pipe represents a connection between commits in the graph
type Pipe struct {
	fromHash *string
	toHash   *string
	style    *lipgloss.Style
	fromPos  int16
	toPos    int16
	kind     PipeKind
}

// NewPipe creates a new pipe
func NewPipe(fromHash, toHash *string, fromPos, toPos int16, kind PipeKind, style *lipgloss.Style) *Pipe {
	return &Pipe{
		fromHash: fromHash,
		toHash:   toHash,
		fromPos:  fromPos,
		toPos:    toPos,
		kind:     kind,
		style:    style,
	}
}

// Accessors for pipe fields (following lazygit pattern)
func (p *Pipe) FromHash() *string      { return p.fromHash }
func (p *Pipe) ToHash() *string        { return p.toHash }
func (p *Pipe) FromPos() int16         { return p.fromPos }
func (p *Pipe) ToPos() int16           { return p.toPos }
func (p *Pipe) Kind() PipeKind         { return p.kind }
func (p *Pipe) Style() *lipgloss.Style { return p.style }

// Left returns the leftmost position of the pipe
func (p *Pipe) Left() int16 {
	if p.fromPos < p.toPos {
		return p.fromPos
	}
	return p.toPos
}

// Right returns the rightmost position of the pipe
func (p *Pipe) Right() int16 {
	if p.fromPos > p.toPos {
		return p.fromPos
	}
	return p.toPos
}

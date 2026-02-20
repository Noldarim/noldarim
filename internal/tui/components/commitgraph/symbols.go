// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package commitgraph

// GraphSymbols defines the visual symbols used for rendering the git graph
type GraphSymbols struct {
	// Basic symbols
	Commit     string
	Merge      string
	Vertical   string
	Horizontal string

	// Connection symbols
	UpRight   string
	UpLeft    string
	DownRight string
	DownLeft  string

	// Multi-way connections
	UpMerge    string // ┴
	DownMerge  string // ┬
	LeftMerge  string // ┤
	RightMerge string // ├
	Cross      string // ┼
}

// DefaultSymbols returns the default Unicode box drawing symbols
func DefaultSymbols() GraphSymbols {
	return GraphSymbols{
		Commit:     "◯",
		Merge:      "⏣",
		Vertical:   "│",
		Horizontal: "─",
		UpRight:    "╰",
		UpLeft:     "╯",
		DownRight:  "╭",
		DownLeft:   "╮",
		UpMerge:    "┴",
		DownMerge:  "┬",
		LeftMerge:  "┤",
		RightMerge: "├",
		Cross:      "┼",
	}
}

// ASCIISymbols returns ASCII fallback symbols for terminals that don't support Unicode
func ASCIISymbols() GraphSymbols {
	return GraphSymbols{
		Commit:     "o",
		Merge:      "M",
		Vertical:   "|",
		Horizontal: "-",
		UpRight:    "\\",
		UpLeft:     "/",
		DownRight:  "/",
		DownLeft:   "\\",
		UpMerge:    "+",
		DownMerge:  "+",
		LeftMerge:  "+",
		RightMerge: "+",
		Cross:      "+",
	}
}

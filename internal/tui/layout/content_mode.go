// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package layout

// ContentDisplayMode defines how content should be rendered in available space
type ContentDisplayMode int

const (
	// ContentModeDirect renders content directly without wrapping
	// Use when content is guaranteed to fit in available space
	ContentModeDirect ContentDisplayMode = iota

	// ContentModeScrollable wraps content in a scrollable viewport
	// Use when content height may exceed available space
	ContentModeScrollable

	// ContentModeTabs uses tabs to organize multiple sections
	// Use when you have distinct sections that should be navigable
	ContentModeTabs
)

// ContentRenderer is an interface that screens can implement to provide
// flexible content rendering based on available space
type ContentRenderer interface {
	// RenderContent renders content for the given dimensions
	// dims contains the validated available space
	RenderContent(dims Dimensions) string

	// GetDisplayMode returns the preferred display mode for this content
	GetDisplayMode() ContentDisplayMode
}

// SimpleContentRenderer is a basic implementation that renders static content
type SimpleContentRenderer struct {
	content string
	mode    ContentDisplayMode
}

// NewSimpleRenderer creates a content renderer with static content
func NewSimpleRenderer(content string, mode ContentDisplayMode) *SimpleContentRenderer {
	return &SimpleContentRenderer{
		content: content,
		mode:    mode,
	}
}

func (r *SimpleContentRenderer) RenderContent(dims Dimensions) string {
	if !dims.Valid {
		return "Invalid dimensions"
	}
	return r.content
}

func (r *SimpleContentRenderer) GetDisplayMode() ContentDisplayMode {
	return r.mode
}

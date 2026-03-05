// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKindForEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		expected  ObsKind
	}{
		{name: "tool use", eventType: EventTypeToolUse, expected: KindTool},
		{name: "session start", eventType: EventTypeSessionStart, expected: KindLifecycle},
		{name: "error", eventType: EventTypeError, expected: KindError},
		{name: "unknown defaults to message", eventType: "unknown", expected: KindMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, KindForEvent(tt.eventType))
		})
	}
}

func TestLevelForEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		expected  ObsLevel
	}{
		{name: "streaming debug", eventType: EventTypeStreaming, expected: LevelDebug},
		{name: "tool blocked warn", eventType: EventTypeToolBlocked, expected: LevelWarn},
		{name: "error", eventType: EventTypeError, expected: LevelError},
		{name: "unknown defaults to info", eventType: "unknown", expected: LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, LevelForEvent(tt.eventType))
		})
	}
}

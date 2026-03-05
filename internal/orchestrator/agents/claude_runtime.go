// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"github.com/noldarim/noldarim/internal/aiobs/adapters/claude"
	"github.com/noldarim/noldarim/internal/aiobs/types"
)

type ClaudeRuntime struct {
	adapter  *ClaudeAdapter
	observer *claude.ClaudeObserver
}

func NewClaudeRuntime() *ClaudeRuntime {
	return &ClaudeRuntime{
		adapter:  NewClaudeAdapter(),
		observer: claude.NewObserver(),
	}
}

func (r *ClaudeRuntime) Name() string {
	return "claude"
}

func (r *ClaudeRuntime) PrepareCommand(config AgentConfig) ([]string, error) {
	return r.adapter.PrepareCommand(config)
}

func (r *ClaudeRuntime) Observability() types.Observer {
	return r.observer
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	opencode_obs "github.com/noldarim/noldarim/internal/aiobs/adapters/opencode"
	"github.com/noldarim/noldarim/internal/aiobs/types"
)

type OpenCodeRuntime struct {
	adapter  *OpenCodeAdapter
	observer *opencode_obs.Observer
}

func NewOpenCodeRuntime() *OpenCodeRuntime {
	return &OpenCodeRuntime{
		adapter:  NewOpenCodeAdapter(),
		observer: opencode_obs.NewObserver(),
	}
}

func (r *OpenCodeRuntime) Name() string {
	return "opencode"
}

func (r *OpenCodeRuntime) PrepareCommand(config AgentConfig) ([]string, error) {
	return r.adapter.PrepareCommand(config)
}

func (r *OpenCodeRuntime) Observability() types.Observer {
	return r.observer
}

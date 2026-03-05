// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"sort"
	"sync"

	"github.com/noldarim/noldarim/internal/aiobs/types"
)

type AgentRuntime interface {
	Name() string
	PrepareCommand(config AgentConfig) ([]string, error)
	Observability() types.Observer
}

var (
	runtimeRegistry = make(map[string]AgentRuntime)
	runtimeMu       sync.RWMutex
)

func RegisterRuntime(r AgentRuntime) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	runtimeRegistry[r.Name()] = r
}

func GetRuntime(name string) (AgentRuntime, bool) {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	r, ok := runtimeRegistry[name]
	return r, ok
}

func RegisteredRuntimes() []string {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()

	names := make([]string, 0, len(runtimeRegistry))
	for name := range runtimeRegistry {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func InitRuntimes() {
	RegisterRuntime(NewClaudeRuntime())
	RegisterRuntime(NewOpenCodeRuntime())
	RegisterRuntime(&testRuntime{adapter: NewTestAdapter()})
}

func ResetRuntimesForTesting() {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	runtimeRegistry = make(map[string]AgentRuntime)
}

type testRuntime struct {
	adapter *TestAdapter
}

func (t *testRuntime) Name() string {
	return "test"
}

func (t *testRuntime) PrepareCommand(cfg AgentConfig) ([]string, error) {
	return t.adapter.PrepareCommand(cfg)
}

func (t *testRuntime) Observability() types.Observer {
	return nil
}

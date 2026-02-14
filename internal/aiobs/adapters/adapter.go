// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package adapters provides source-specific parsers for AI event data.
package adapters

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/noldarim/noldarim/internal/aiobs/adapters/claude"
)

// registry holds registered adapters by name.
var registry = make(map[string]Adapter)
var registryMu sync.RWMutex
var initialized bool

// register adds an adapter to the registry.
func register(name string, adapter Adapter) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = adapter
}

// Get returns an adapter by name.
// Returns (nil, false) if the adapter is not registered.
// IMPORTANT: Call RegisterAll() before using Get() to ensure adapters are available.
func Get(name string) (Adapter, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	adapter, ok := registry[name]
	return adapter, ok
}

// RegisterAll explicitly registers all known adapters.
// Call this once at application startup before using Get().
func RegisterAll() {
	registryMu.Lock()
	defer registryMu.Unlock()

	if initialized {
		return // Already registered
	}

	// Register Claude adapter
	registry["claude"] = claude.New()

	// Future adapters go here:
	// registry["gemini"] = gemini.New()
	// registry["aider"] = aider.New()

	initialized = true
}

// IsInitialized returns whether RegisterAll has been called.
func IsInitialized() bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return initialized
}

// RegisteredAdapters returns the names of all registered adapters.
func RegisteredAdapters() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// DetectAndParse attempts to detect the adapter and parse the entry.
// Returns the parsed events and the adapter name used.
func DetectAndParse(raw json.RawMessage) ([]ParsedEvent, string, error) {
	// Try to detect based on common fields
	var probe struct {
		Type      string `json:"type"`
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil {
		// Claude Code transcripts have type field with specific values
		if probe.Type == "user" || probe.Type == "assistant" || probe.Type == "summary" || probe.Type == "system" {
			if a, ok := Get("claude"); ok {
				events, err := a.ParseEntry(RawEntry{Data: raw, SessionID: probe.SessionID})
				return events, "claude", err
			}
		}
	}

	// Default to claude if we can't detect
	if a, ok := Get("claude"); ok {
		events, err := a.ParseEntry(RawEntry{Data: raw})
		return events, "claude", err
	}

	return nil, "", fmt.Errorf("no adapter available for parsing (call RegisterAll() first)")
}

// ResetForTesting resets the registry for testing purposes.
// Only use in tests.
func ResetForTesting() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = make(map[string]Adapter)
	initialized = false
}

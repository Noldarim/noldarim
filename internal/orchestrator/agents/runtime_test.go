// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package agents

import (
	"reflect"
	"testing"
)

func TestInitRuntimesRegistersAll(t *testing.T) {
	ResetRuntimesForTesting()
	InitRuntimes()

	got := RegisteredRuntimes()
	want := []string{"claude", "opencode", "test"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RegisteredRuntimes() = %v, want %v", got, want)
	}
}

func TestGetRuntimeReturnsExpectedRuntime(t *testing.T) {
	ResetRuntimesForTesting()
	InitRuntimes()

	runtime, ok := GetRuntime("claude")
	if !ok {
		t.Fatalf("GetRuntime(claude) expected runtime, got not found")
	}

	if runtime.Name() != "claude" {
		t.Fatalf("runtime.Name() = %q, want %q", runtime.Name(), "claude")
	}

	if runtime.Observability() == nil {
		t.Fatalf("runtime.Observability() should not be nil for claude")
	}

	testRuntime, ok := GetRuntime("test")
	if !ok {
		t.Fatalf("GetRuntime(test) expected runtime, got not found")
	}

	if testRuntime.Observability() != nil {
		t.Fatalf("test runtime observability should be nil")
	}
}

func TestGetAdapterBackwardCompatibility(t *testing.T) {
	ResetRuntimesForTesting()

	adapter, err := GetAdapter("claude")
	if err != nil {
		t.Fatalf("GetAdapter(claude) fallback returned error: %v", err)
	}

	if _, ok := adapter.(*ClaudeAdapter); !ok {
		t.Fatalf("fallback adapter type = %T, want *ClaudeAdapter", adapter)
	}

	InitRuntimes()

	runtimeAdapter, err := GetAdapter("opencode")
	if err != nil {
		t.Fatalf("GetAdapter(opencode) returned error: %v", err)
	}

	if _, ok := runtimeAdapter.(*OpenCodeRuntime); !ok {
		t.Fatalf("runtime adapter type = %T, want *OpenCodeRuntime", runtimeAdapter)
	}
}

func TestAgentRuntimeSatisfiesAgentAdapter(t *testing.T) {
	var _ AgentAdapter = NewClaudeRuntime()
	var _ AgentAdapter = NewOpenCodeRuntime()
	var _ AgentAdapter = &testRuntime{adapter: NewTestAdapter()}
}

func TestOpenCodeAdapterPrepareCommand(t *testing.T) {
	adapter := NewOpenCodeAdapter()

	config := AgentConfig{
		ToolName:       "opencode",
		PromptTemplate: "Analyze {{.file}}",
		Variables: map[string]string{
			"file": "main.go",
		},
		ToolOptions: map[string]interface{}{
			"model":   "gpt-5",
			"timeout": 30,
			"verbose": true,
		},
		FlagFormat: "space",
	}

	got, err := adapter.PrepareCommand(config)
	if err != nil {
		t.Fatalf("PrepareCommand() unexpected error: %v", err)
	}

	want := []string{
		"opencode",
		"run",
		"--model",
		"gpt-5",
		"--timeout",
		"30",
		"--verbose",
		"--message",
		"Analyze main.go",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PrepareCommand() = %v, want %v", got, want)
	}
}

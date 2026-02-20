// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package collapsiblefeed

import (
	"testing"

	"github.com/noldarim/noldarim/internal/orchestrator/models"
)

func TestParseRecords_BasicPairing(t *testing.T) {
	// Create test records: tool_use followed by tool_result
	records := []*models.AIActivityRecord{
		{
			EventID:          "1",
			EventType:        models.AIEventToolUse,
			ToolName:         "Read",
			FilePath:         "/test/file.go",
			ToolInputSummary: `{"file_path": "/test/file.go"}`,
		},
		{
			EventID:        "2",
			EventType:      models.AIEventToolResult,
			ToolName:       "Read",
			ToolSuccess:    boolPtr(true),
			ContentPreview: "[/test/file.go] 50 lines",
		},
	}

	groups := ParseRecords(records)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if g.ToolName != "Read" {
		t.Errorf("expected tool name 'Read', got '%s'", g.ToolName)
	}
	if g.Input.FilePath != "/test/file.go" {
		t.Errorf("expected file path '/test/file.go', got '%s'", g.Input.FilePath)
	}
	if g.State != StateCompleted {
		t.Errorf("expected state Completed, got %v", g.State)
	}
	if g.Result == nil {
		t.Fatal("expected result to be set")
	}
	if !g.Result.Success {
		t.Error("expected result.Success to be true")
	}
	if g.Result.LineCount != 50 {
		t.Errorf("expected 50 lines, got %d", g.Result.LineCount)
	}
}

func TestParseRecords_MultiplePairs(t *testing.T) {
	records := []*models.AIActivityRecord{
		{EventID: "1", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/a.go"},
		{EventID: "2", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(true)},
		{EventID: "3", EventType: models.AIEventToolUse, ToolName: "Write", FilePath: "/b.go"},
		{EventID: "4", EventType: models.AIEventToolResult, ToolName: "Write", ToolSuccess: boolPtr(true)},
		{EventID: "5", EventType: models.AIEventToolUse, ToolName: "Bash", ToolInputSummary: `{"command": "ls", "description": "List files"}`},
		{EventID: "6", EventType: models.AIEventToolResult, ToolName: "Bash", ToolSuccess: boolPtr(true), ContentPreview: "file1.go\nfile2.go"},
	}

	groups := ParseRecords(records)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// Verify each group has completed state
	for i, g := range groups {
		if g.State != StateCompleted {
			t.Errorf("group %d: expected Completed state, got %v", i, g.State)
		}
		if g.Result == nil {
			t.Errorf("group %d: expected result to be set", i)
		}
	}

	// Check Bash parsing
	if groups[2].Input.Description != "List files" {
		t.Errorf("expected Bash description 'List files', got '%s'", groups[2].Input.Description)
	}
}

func TestParseRecords_PendingGroup(t *testing.T) {
	// Tool use without result
	records := []*models.AIActivityRecord{
		{EventID: "1", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/pending.go"},
	}

	groups := ParseRecords(records)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].State != StatePending {
		t.Errorf("expected Pending state, got %v", groups[0].State)
	}
	if groups[0].Result != nil {
		t.Error("expected nil result for pending group")
	}
}

func TestParseRecords_FailedGroup(t *testing.T) {
	records := []*models.AIActivityRecord{
		{EventID: "1", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/fail.go"},
		{EventID: "2", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(false), ToolError: "file not found"},
	}

	groups := ParseRecords(records)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].State != StateFailed {
		t.Errorf("expected Failed state, got %v", groups[0].State)
	}
	if groups[0].Result.Error != "file not found" {
		t.Errorf("expected error 'file not found', got '%s'", groups[0].Result.Error)
	}
}

func TestParseRecords_TodoWrite(t *testing.T) {
	records := []*models.AIActivityRecord{
		{
			EventID:          "1",
			EventType:        models.AIEventToolUse,
			ToolName:         "TodoWrite",
			ToolInputSummary: `{"todos": [{"content": "Task 1", "activeForm": "Doing task 1", "status": "in_progress"}, {"content": "Task 2", "activeForm": "Doing task 2", "status": "pending"}]}`,
		},
		{
			EventID:        "2",
			EventType:      models.AIEventToolResult,
			ToolName:       "TodoWrite",
			ToolSuccess:    boolPtr(true),
			ContentPreview: "Updated todos (2 items)",
		},
	}

	groups := ParseRecords(records)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if len(g.Input.Todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(g.Input.Todos))
	}
	if g.Input.Todos[0].Content != "Task 1" {
		t.Errorf("expected todo content 'Task 1', got '%s'", g.Input.Todos[0].Content)
	}
	if g.Input.Todos[0].Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got '%s'", g.Input.Todos[0].Status)
	}
	if g.Result.TodoCount != 2 {
		t.Errorf("expected todo count 2, got %d", g.Result.TodoCount)
	}
}

func TestParseRecords_SkipsNonToolEvents(t *testing.T) {
	records := []*models.AIActivityRecord{
		{EventID: "1", EventType: models.AIEventThinking, ContentPreview: "thinking..."},
		{EventID: "2", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/file.go"},
		{EventID: "3", EventType: models.AIEventAIOutput, ContentPreview: "output text"},
		{EventID: "4", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(true)},
	}

	groups := ParseRecords(records)

	// Should only have 1 group for the Read tool
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].ToolName != "Read" {
		t.Errorf("expected tool 'Read', got '%s'", groups[0].ToolName)
	}
}

func TestParseRecords_ConcurrentSameToolFIFO(t *testing.T) {
	// Multiple Read calls before results - FIFO matching
	records := []*models.AIActivityRecord{
		{EventID: "1", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/first.go"},
		{EventID: "2", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/second.go"},
		{EventID: "3", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/third.go"},
		{EventID: "4", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(true), ContentPreview: "first result"},
		{EventID: "5", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(true), ContentPreview: "second result"},
		{EventID: "6", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(true), ContentPreview: "third result"},
	}

	groups := ParseRecords(records)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// Verify FIFO: first tool_use gets first result
	if groups[0].Input.FilePath != "/first.go" {
		t.Errorf("group 0: expected /first.go, got %s", groups[0].Input.FilePath)
	}
	if groups[0].Result == nil || groups[0].Result.Output != "first result" {
		t.Errorf("group 0: expected 'first result'")
	}

	if groups[1].Input.FilePath != "/second.go" {
		t.Errorf("group 1: expected /second.go, got %s", groups[1].Input.FilePath)
	}
	if groups[1].Result == nil || groups[1].Result.Output != "second result" {
		t.Errorf("group 1: expected 'second result'")
	}

	if groups[2].Input.FilePath != "/third.go" {
		t.Errorf("group 2: expected /third.go, got %s", groups[2].Input.FilePath)
	}
	if groups[2].Result == nil || groups[2].Result.Output != "third result" {
		t.Errorf("group 2: expected 'third result'")
	}
}

func TestParseRecords_InterleavedTools(t *testing.T) {
	// Different tools interleaved
	records := []*models.AIActivityRecord{
		{EventID: "1", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/a.go"},
		{EventID: "2", EventType: models.AIEventToolUse, ToolName: "Write", FilePath: "/b.go"},
		{EventID: "3", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(true)},
		{EventID: "4", EventType: models.AIEventToolUse, ToolName: "Read", FilePath: "/c.go"},
		{EventID: "5", EventType: models.AIEventToolResult, ToolName: "Write", ToolSuccess: boolPtr(true)},
		{EventID: "6", EventType: models.AIEventToolResult, ToolName: "Read", ToolSuccess: boolPtr(true)},
	}

	groups := ParseRecords(records)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// All should be completed
	for i, g := range groups {
		if g.State != StateCompleted {
			t.Errorf("group %d: expected Completed, got %v", i, g.State)
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}

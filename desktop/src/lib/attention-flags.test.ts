// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, expect, it } from "vitest";

import type { AIActivityRecord } from "./types";
import { highestSeverity, scanAttentionFlags } from "./attention-flags";

function makeEvent(overrides: Partial<AIActivityRecord> = {}): AIActivityRecord {
  return {
    event_id: `evt-${Math.random().toString(36).slice(2, 8)}`,
    task_id: "task-1",
    run_id: "run-1",
    event_type: "tool_use",
    timestamp: new Date().toISOString(),
    ...overrides
  };
}

describe("scanAttentionFlags", () => {
  it("returns empty for clean events", () => {
    const events = [makeEvent({ event_type: "tool_use" }), makeEvent({ event_type: "tool_result", tool_success: true })];
    expect(scanAttentionFlags(events)).toEqual([]);
  });

  it("flags tool_blocked events", () => {
    const events = [makeEvent({ event_type: "tool_blocked", tool_name: "Write" })];
    const flags = scanAttentionFlags(events);
    expect(flags).toHaveLength(1);
    expect(flags[0].severity).toBe("warning");
    expect(flags[0].message).toContain("Write");
  });

  it("flags error events", () => {
    const events = [makeEvent({ event_type: "error", content_preview: "Out of context" })];
    const flags = scanAttentionFlags(events);
    expect(flags).toHaveLength(1);
    expect(flags[0].severity).toBe("error");
    expect(flags[0].message).toBe("Out of context");
  });

  it("flags failed tool results", () => {
    const events = [makeEvent({ event_type: "tool_result", tool_success: false, tool_name: "Bash", tool_error: "exit 1" })];
    const flags = scanAttentionFlags(events);
    expect(flags).toHaveLength(1);
    expect(flags[0].severity).toBe("warning");
    expect(flags[0].message).toContain("Bash");
  });

  it("flags subagent_stop with errors", () => {
    const events = [makeEvent({ event_type: "subagent_stop", tool_error: "Crashed" })];
    const flags = scanAttentionFlags(events);
    expect(flags).toHaveLength(1);
    expect(flags[0].severity).toBe("error");
  });

  it("does not flag tool_result with null tool_success", () => {
    const events = [makeEvent({ event_type: "tool_result", tool_success: null })];
    expect(scanAttentionFlags(events)).toEqual([]);
  });
});

describe("highestSeverity", () => {
  it("returns null for empty", () => {
    expect(highestSeverity([])).toBeNull();
  });

  it("returns error when mixed", () => {
    const flags = scanAttentionFlags([
      makeEvent({ event_type: "tool_blocked", tool_name: "X" }),
      makeEvent({ event_type: "error" })
    ]);
    expect(highestSeverity(flags)).toBe("error");
  });

  it("returns warning when no errors", () => {
    const flags = scanAttentionFlags([makeEvent({ event_type: "tool_blocked", tool_name: "X" })]);
    expect(highestSeverity(flags)).toBe("warning");
  });
});

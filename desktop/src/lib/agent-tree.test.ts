// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, expect, it } from "vitest";

import type { AIActivityRecord } from "./types";
import { buildAgentTree, collectAgentEventIds, findAgentNode } from "./agent-tree";

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

describe("buildAgentTree", () => {
  it("returns empty array for empty input", () => {
    expect(buildAgentTree([])).toEqual([]);
  });

  it("groups events by session_id into a single root", () => {
    const events = [
      makeEvent({ session_id: "s1", agent_id: "agent-main", event_type: "session_start" }),
      makeEvent({ session_id: "s1", agent_id: "agent-main", event_type: "tool_use", tool_name: "Read" }),
      makeEvent({ session_id: "s1", agent_id: "agent-main", event_type: "session_end" })
    ];
    const roots = buildAgentTree(events);
    expect(roots).toHaveLength(1);
    expect(roots[0].agentId).toBe("agent-main");
    expect(roots[0].eventCount).toBe(3);
    expect(roots[0].toolUseCount).toBe(1);
    expect(roots[0].status).toBe("done");
    expect(roots[0].children).toHaveLength(0);
  });

  it("builds parent-child hierarchy from parent_session_id", () => {
    const events = [
      makeEvent({ session_id: "s1", agent_id: "main" }),
      makeEvent({ session_id: "s2", agent_id: "sub", parent_session_id: "s1", is_sidechain: true }),
      makeEvent({ session_id: "s2", agent_id: "sub", parent_session_id: "s1", event_type: "stop" })
    ];
    const roots = buildAgentTree(events);
    expect(roots).toHaveLength(1);
    expect(roots[0].agentId).toBe("main");
    expect(roots[0].children).toHaveLength(1);
    expect(roots[0].children[0].agentId).toBe("sub");
    expect(roots[0].children[0].isSidechain).toBe(true);
    expect(roots[0].children[0].status).toBe("done");
  });

  it("aggregates token usage", () => {
    const events = [
      makeEvent({ session_id: "s1", input_tokens: 100, output_tokens: 50 }),
      makeEvent({ session_id: "s1", input_tokens: 200, output_tokens: 75 })
    ];
    const roots = buildAgentTree(events);
    expect(roots[0].inputTokens).toBe(300);
    expect(roots[0].outputTokens).toBe(125);
  });

  it("detects error status from error events", () => {
    const events = [
      makeEvent({ session_id: "s1", event_type: "error" })
    ];
    const roots = buildAgentTree(events);
    expect(roots[0].status).toBe("error");
  });

  it("falls back to agent_id when session_id is missing", () => {
    const events = [
      makeEvent({ agent_id: "a1", event_type: "tool_use" }),
      makeEvent({ agent_id: "a1", event_type: "tool_use" })
    ];
    const roots = buildAgentTree(events);
    expect(roots).toHaveLength(1);
    expect(roots[0].agentId).toBe("a1");
    expect(roots[0].eventCount).toBe(2);
  });
});

describe("collectAgentEventIds", () => {
  it("collects all event IDs from node and descendants", () => {
    const events = [
      makeEvent({ event_id: "e1", session_id: "s1" }),
      makeEvent({ event_id: "e2", session_id: "s2", parent_session_id: "s1" }),
      makeEvent({ event_id: "e3", session_id: "s2", parent_session_id: "s1" })
    ];
    const roots = buildAgentTree(events);
    const ids = collectAgentEventIds(roots[0]);
    expect(ids).toEqual(new Set(["e1", "e2", "e3"]));
  });
});

describe("findAgentNode", () => {
  it("finds nested agent node", () => {
    const events = [
      makeEvent({ session_id: "s1", agent_id: "main" }),
      makeEvent({ session_id: "s2", agent_id: "sub", parent_session_id: "s1" })
    ];
    const roots = buildAgentTree(events);
    const found = findAgentNode(roots, "sub");
    expect(found).toBeDefined();
    expect(found?.sessionId).toBe("s2");
  });

  it("returns undefined for non-existent agent", () => {
    expect(findAgentNode([], "nope")).toBeUndefined();
  });
});

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { buildAgentTree } from "../../../lib/agent-tree";
import type { AIActivityRecord } from "../../../lib/types";
import { AgentTreePanel } from "../AgentTreePanel";

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

describe("AgentTreePanel", () => {
  it("renders empty state", () => {
    const onSelect = vi.fn();
    render(<AgentTreePanel roots={[]} onSelectAgent={onSelect} />);
    expect(screen.getByText(/no agent sessions/i)).toBeDefined();
  });

  it("renders agent nodes from roots", () => {
    const onSelect = vi.fn();
    const events = [
      makeEvent({ session_id: "s1", agent_id: "main-agent", event_type: "session_start" }),
      makeEvent({ session_id: "s1", agent_id: "main-agent", event_type: "tool_use" })
    ];
    const roots = buildAgentTree(events);
    render(<AgentTreePanel roots={roots} onSelectAgent={onSelect} />);
    expect(screen.getByText(/main-agent/)).toBeDefined();
    expect(screen.getByText(/2 events/)).toBeDefined();
  });

  it("calls onSelectAgent when clicking a node", () => {
    const onSelect = vi.fn();
    const events = [
      makeEvent({ session_id: "s1", agent_id: "my-agent" })
    ];
    const roots = buildAgentTree(events);
    render(<AgentTreePanel roots={roots} onSelectAgent={onSelect} />);
    fireEvent.click(screen.getByText(/my-agent/));
    expect(onSelect).toHaveBeenCalledWith("my-agent");
  });
});

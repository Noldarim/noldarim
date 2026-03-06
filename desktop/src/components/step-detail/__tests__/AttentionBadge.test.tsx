// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { AIActivityRecord } from "../../../lib/types";
import { AttentionBadge } from "../AttentionBadge";

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

describe("AttentionBadge", () => {
  it("renders nothing for clean activities", () => {
    const { container } = render(<AttentionBadge activities={[makeEvent()]} />);
    expect(container.querySelector(".attention-badge")).toBeNull();
  });

  it("renders count for flagged activities", () => {
    const events = [
      makeEvent({ event_type: "error", content_preview: "crash" }),
      makeEvent({ event_type: "tool_blocked", tool_name: "Write" })
    ];
    render(<AttentionBadge activities={events} />);
    expect(screen.getByText("2")).toBeDefined();
  });
});

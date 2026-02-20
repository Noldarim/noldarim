// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { describe, expect, it } from "vitest";

import { mapActivitiesToSteps, groupToolEventsByName } from "./obs-mapping";
import type { AIActivityRecord, StepDraft } from "./types";

function event(id: string, stepId?: string): AIActivityRecord {
  return {
    event_id: id,
    task_id: "run-1",
    run_id: "run-1",
    step_id: stepId,
    event_type: "tool_use",
    timestamp: "2026-02-14T10:00:00Z"
  };
}

function draft(id: string, name?: string): StepDraft {
  return { id, name: name ?? id, prompt: "" };
}

describe("mapActivitiesToSteps", () => {
  it("groups events by step_id", () => {
    const steps = [draft("step-a"), draft("step-b")];
    const activities = [
      event("evt-1", "step-a"),
      event("evt-2", "step-b"),
      event("evt-3", "step-a")
    ];

    const mapped = mapActivitiesToSteps(steps, activities);
    expect(mapped["step-a"]).toHaveLength(2);
    expect(mapped["step-b"]).toHaveLength(1);
    expect(mapped["step-a"].map((e) => e.event_id)).toEqual(["evt-1", "evt-3"]);
  });

  it("excludes events without step_id (legacy data)", () => {
    const steps = [draft("step-a")];
    const activities = [
      event("evt-1", "step-a"),
      event("evt-2", undefined),
      event("evt-3", "")
    ];

    const mapped = mapActivitiesToSteps(steps, activities);
    expect(mapped["step-a"]).toHaveLength(1);
    expect(mapped["step-a"][0].event_id).toBe("evt-1");
  });

  it("gives steps without matching events empty arrays", () => {
    const steps = [draft("step-skip"), draft("step-live")];
    const activities = [event("evt-1", "step-live")];

    const mapped = mapActivitiesToSteps(steps, activities);
    expect(mapped["step-skip"]).toEqual([]);
    expect(mapped["step-live"]).toHaveLength(1);
  });

  it("ignores events with unknown step_id", () => {
    const steps = [draft("step-a")];
    const activities = [event("evt-1", "step-unknown")];

    const mapped = mapActivitiesToSteps(steps, activities);
    expect(mapped["step-a"]).toEqual([]);
  });

  it("returns empty map when no steps provided", () => {
    const mapped = mapActivitiesToSteps([], [event("evt-1", "step-a")]);
    expect(Object.keys(mapped)).toHaveLength(0);
  });

  it("initialises buckets for all steps even before results arrive", () => {
    const steps = [draft("step-a"), draft("step-b"), draft("step-c")];
    const activities = [event("evt-1", "step-a")];

    const mapped = mapActivitiesToSteps(steps, activities);
    expect(Object.keys(mapped)).toHaveLength(3);
    expect(mapped["step-a"]).toHaveLength(1);
    expect(mapped["step-b"]).toEqual([]);
    expect(mapped["step-c"]).toEqual([]);
  });

  it("preserves step-2 events even when step_results only contains step-1", () => {
    // Regression: the graph disappeared when step 1 finished because
    // buckets were derived from step_results instead of draft steps.
    const steps = [draft("step-1"), draft("step-2")];
    const activities = [
      event("evt-1", "step-1"),
      event("evt-2", "step-2"),
      event("evt-3", "step-2")
    ];

    const mapped = mapActivitiesToSteps(steps, activities);

    expect(mapped["step-1"]).toHaveLength(1);
    expect(mapped["step-2"]).toHaveLength(2);
    expect(mapped["step-2"].map((e) => e.event_id)).toEqual(["evt-2", "evt-3"]);
  });

  it("unknown step IDs are ignored while known draft step IDs always have buckets", () => {
    const steps = [draft("known-1"), draft("known-2")];
    const activities = [
      event("evt-1", "unknown-x"),
      event("evt-2", "known-1"),
      event("evt-3", "unknown-y")
    ];

    const mapped = mapActivitiesToSteps(steps, activities);

    expect(Object.keys(mapped)).toEqual(["known-1", "known-2"]);
    expect(mapped["known-1"]).toHaveLength(1);
    expect(mapped["known-2"]).toEqual([]);
    expect(mapped["unknown-x"]).toBeUndefined();
    expect(mapped["unknown-y"]).toBeUndefined();
  });
});

function toolUse(id: string, toolName: string, input?: string): AIActivityRecord {
  return {
    event_id: id,
    task_id: "run-1",
    run_id: "run-1",
    event_type: "tool_use",
    timestamp: `2026-02-14T10:00:0${id.slice(-1)}Z`,
    tool_name: toolName,
    tool_input_summary: input
  };
}

function toolResult(id: string, toolName: string, success: boolean): AIActivityRecord {
  return {
    event_id: id,
    task_id: "run-1",
    run_id: "run-1",
    event_type: "tool_result",
    timestamp: `2026-02-14T10:00:1${id.slice(-1)}Z`,
    tool_name: toolName,
    tool_success: success
  };
}

describe("groupToolEventsByName", () => {
  it("groups tool calls by tool name", () => {
    const events = [
      toolUse("e1", "bash", "ls"),
      toolUse("e2", "Read", "main.go"),
      toolUse("e3", "bash", "go build"),
      toolResult("e4", "bash", true),
      toolResult("e5", "Read", true),
      toolResult("e6", "bash", true)
    ];

    const groups = groupToolEventsByName(events);
    expect(groups).toHaveLength(2);
    expect(groups[0].toolName).toBe("bash");
    expect(groups[0].calls).toHaveLength(2);
    expect(groups[1].toolName).toBe("Read");
    expect(groups[1].calls).toHaveLength(1);
  });

  it("returns empty array for no tool events", () => {
    const events: AIActivityRecord[] = [
      { event_id: "e1", task_id: "t1", run_id: "r1", event_type: "thinking", timestamp: "2026-02-14T10:00:00Z" }
    ];
    expect(groupToolEventsByName(events)).toEqual([]);
  });

  it("preserves call order within each group", () => {
    const events = [
      toolUse("e1", "bash", "first"),
      toolUse("e2", "bash", "second"),
      toolResult("e3", "bash", true),
      toolResult("e4", "bash", false)
    ];

    const groups = groupToolEventsByName(events);
    expect(groups[0].calls[0].input).toBe("first");
    expect(groups[0].calls[0].result?.success).toBe(true);
    expect(groups[0].calls[1].input).toBe("second");
    expect(groups[0].calls[1].result?.success).toBe(false);
  });
});

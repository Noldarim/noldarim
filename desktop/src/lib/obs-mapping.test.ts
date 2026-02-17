import { describe, expect, it } from "vitest";

import { mapActivitiesToSteps } from "./obs-mapping";
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
});

import { describe, expect, it } from "vitest";

import { mapActivitiesToSteps } from "./obs-mapping";
import type { AIActivityRecord, PipelineRun } from "./types";

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

function stepResult(stepId: string, index: number, status: number = 2) {
  return {
    id: `sr-${index}`,
    pipeline_run_id: "run-1",
    step_id: stepId,
    step_index: index,
    status: status as 0 | 1 | 2 | 3 | 4,
    commit_sha: "",
    commit_message: "",
    git_diff: "",
    files_changed: 0,
    insertions: 0,
    deletions: 0,
    input_tokens: 0,
    output_tokens: 0,
    cache_read_tokens: 0,
    cache_create_tokens: 0,
    agent_output: "",
    duration: 0
  };
}

describe("mapActivitiesToSteps", () => {
  it("groups events by step_id", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 2,
      step_results: [stepResult("step-a", 0), stepResult("step-b", 1)]
    };

    const activities = [
      event("evt-1", "step-a"),
      event("evt-2", "step-b"),
      event("evt-3", "step-a")
    ];

    const mapped = mapActivitiesToSteps(run, activities);
    expect(mapped["step-a"]).toHaveLength(2);
    expect(mapped["step-b"]).toHaveLength(1);
    expect(mapped["step-a"].map((e) => e.event_id)).toEqual(["evt-1", "evt-3"]);
  });

  it("excludes events without step_id (legacy data)", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 2,
      step_results: [stepResult("step-a", 0)]
    };

    const activities = [
      event("evt-1", "step-a"),
      event("evt-2", undefined),
      event("evt-3", "")
    ];

    const mapped = mapActivitiesToSteps(run, activities);
    expect(mapped["step-a"]).toHaveLength(1);
    expect(mapped["step-a"][0].event_id).toBe("evt-1");
  });

  it("gives skipped steps empty arrays", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 2,
      step_results: [
        stepResult("step-skip", 0, 4),
        stepResult("step-live", 1, 2)
      ]
    };

    const activities = [event("evt-1", "step-live")];

    const mapped = mapActivitiesToSteps(run, activities);
    expect(mapped["step-skip"]).toEqual([]);
    expect(mapped["step-live"]).toHaveLength(1);
  });

  it("ignores events with unknown step_id", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 2,
      step_results: [stepResult("step-a", 0)]
    };

    const activities = [event("evt-1", "step-unknown")];

    const mapped = mapActivitiesToSteps(run, activities);
    expect(mapped["step-a"]).toEqual([]);
  });

  it("returns empty map when no step results", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 1,
      step_results: []
    };

    const mapped = mapActivitiesToSteps(run, [event("evt-1", "step-a")]);
    expect(Object.keys(mapped)).toHaveLength(0);
  });
});

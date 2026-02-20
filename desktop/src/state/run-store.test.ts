// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { afterEach, describe, expect, it } from "vitest";

import { useRunStore } from "./run-store";
import type { AIActivityRecord, PipelineRun, StepDraft } from "../lib/types";

const steps: StepDraft[] = [
  { id: "step-1", name: "Step 1", prompt: "do step 1" },
  { id: "step-2", name: "Step 2", prompt: "do step 2" }
];

function activity(eventId: string, stepId?: string, overrides: Partial<AIActivityRecord> = {}): AIActivityRecord {
  return {
    event_id: eventId,
    task_id: "task-1",
    run_id: "run-1",
    step_id: stepId,
    event_type: "tool_use",
    timestamp: "2026-02-14T10:00:00Z",
    ...overrides
  };
}

function makeRun(overrides?: Partial<PipelineRun>): PipelineRun {
  return {
    id: "run-1",
    project_id: "proj-1",
    name: "Pipeline",
    status: 1,
    ...overrides
  };
}

afterEach(() => {
  useRunStore.getState().reset();
});

describe("runStarted", () => {
  it("sets phase to starting and clears previous state", () => {
    const { runStarted } = useRunStore.getState();

    runStarted("proj-1", "My Pipeline", steps);

    const s = useRunStore.getState();
    expect(s.phase).toBe("starting");
    expect(s.connectionStatus).toBe("connecting");
    expect(s.projectId).toBe("proj-1");
    expect(s.runDefinition.pipelineName).toBe("My Pipeline");
    expect(s.runDefinition.steps).toEqual(steps);
    expect(s.runId).toBeNull();
    expect(s.run).toBeNull();
    expect(s.activityByEventId).toEqual({});
    expect(s.activityByStepId).toEqual({ "step-1": [], "step-2": [] });
    expect(s.error).toBeNull();
  });
});

describe("wsConnected", () => {
  it("sets phase to running and connectionStatus to streaming", () => {
    const { runStarted, wsConnected } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");

    const s = useRunStore.getState();
    expect(s.phase).toBe("running");
    expect(s.connectionStatus).toBe("streaming");
    expect(s.runId).toBe("run-1");
  });
});

describe("wsActivityReceived", () => {
  it("appends a new activity", () => {
    const { runStarted, wsActivityReceived } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsActivityReceived(activity("evt-1", "step-1"));

    const s = useRunStore.getState();
    expect(s.activityByEventId["evt-1"]).toBeDefined();
    expect(s.activityByStepId["step-1"]).toHaveLength(1);
  });

  it("deduplicates: same event_id twice yields identical state", () => {
    const { runStarted, wsActivityReceived } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsActivityReceived(activity("evt-1", "step-1"));

    const stateAfterFirst = useRunStore.getState();
    wsActivityReceived(activity("evt-1", "step-1"));

    const stateAfterDup = useRunStore.getState();
    expect(Object.keys(stateAfterDup.activityByEventId)).toHaveLength(1);
    // Zustand should return the same reference when nothing changed
    expect(stateAfterDup.activityByEventId).toBe(stateAfterFirst.activityByEventId);
  });

  it("ignores activity for unknown step_id", () => {
    const { runStarted, wsActivityReceived } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsActivityReceived(activity("evt-1", "unknown-step"));

    const s = useRunStore.getState();
    expect(s.activityByEventId["evt-1"]).toBeDefined();
    // Not assigned to any step bucket
    expect(s.activityByStepId["step-1"]).toEqual([]);
    expect(s.activityByStepId["step-2"]).toEqual([]);
  });

  it("keeps step activity sorted chronologically even when events arrive out of order", () => {
    const { runStarted, wsActivityReceived } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsActivityReceived(activity("evt-late", "step-1", { timestamp: "2026-02-14T10:00:02Z" }));
    wsActivityReceived(activity("evt-early", "step-1", { timestamp: "2026-02-14T10:00:01Z" }));

    const events = useRunStore.getState().activityByStepId["step-1"];
    expect(events.map((e) => e.event_id)).toEqual(["evt-early", "evt-late"]);
  });
});

describe("snapshotApplied", () => {
  it("merges run data and activities atomically", () => {
    const { runStarted, wsConnected, wsActivityReceived, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");
    wsActivityReceived(activity("ws-1", "step-1"));

    const run = makeRun({ status: 1 });
    snapshotApplied(run, [activity("api-1", "step-2")]);

    const s = useRunStore.getState();
    expect(s.run).toEqual(run);
    expect(s.activityByEventId["ws-1"]).toBeDefined();
    expect(s.activityByEventId["api-1"]).toBeDefined();
    expect(s.activityByStepId["step-1"]).toHaveLength(1);
    expect(s.activityByStepId["step-2"]).toHaveLength(1);
    expect(s.phase).toBe("running");
  });

  it("re-sorts merged WS and API activities by timestamp", () => {
    const { runStarted, wsConnected, wsActivityReceived, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");
    wsActivityReceived(activity("ws-late", "step-1", { timestamp: "2026-02-14T10:00:02Z" }));

    const run = makeRun({ status: 1 });
    snapshotApplied(run, [
      activity("api-early", "step-1", { timestamp: "2026-02-14T10:00:01Z" }),
      activity("api-mid", "step-1", { timestamp: "2026-02-14T10:00:01.500Z" })
    ]);

    const events = useRunStore.getState().activityByStepId["step-1"];
    expect(events.map((e) => e.event_id)).toEqual(["api-early", "api-mid", "ws-late"]);
  });

  it("builds stepExecutionById from run.step_results", () => {
    const { runStarted, wsConnected, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");

    snapshotApplied(
      makeRun({
        status: 1,
        step_results: [
          {
            id: "sr-1",
            pipeline_run_id: "run-1",
            step_id: "step-1",
            step_index: 0,
            status: 2,
            commit_sha: "abc",
            commit_message: "msg",
            git_diff: "",
            files_changed: 1,
            insertions: 2,
            deletions: 3,
            input_tokens: 100,
            output_tokens: 50,
            cache_read_tokens: 0,
            cache_create_tokens: 0,
            agent_output: "",
            duration: 10
          }
        ]
      }),
      []
    );

    const s = useRunStore.getState();
    expect(s.stepExecutionById["step-1"]).toBeDefined();
    expect(s.stepExecutionById["step-1"].status).toBe(2);
    expect(s.stepExecutionById["step-2"]).toBeUndefined();
  });

  it("does not override cancelling phase", () => {
    const { runStarted, wsConnected, runCancelling, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");
    runCancelling();

    snapshotApplied(makeRun({ status: 1 }), []);

    expect(useRunStore.getState().phase).toBe("cancelling");
  });

  it("does not override cancelled phase", () => {
    const { runStarted, wsConnected, runCancelling, runCancelled, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");
    runCancelling();
    runCancelled();

    snapshotApplied(makeRun({ status: 2 }), []);

    expect(useRunStore.getState().phase).toBe("cancelled");
  });

  it("transitions to completed when run status is completed", () => {
    const { runStarted, wsConnected, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");

    // First snapshot establishes prev.run so terminal status is accepted
    snapshotApplied(makeRun({ status: 1 }), []);
    snapshotApplied(makeRun({ status: 2 }), []);

    expect(useRunStore.getState().phase).toBe("completed");
  });

  it("skips terminal status on first snapshot (stale DB guard)", () => {
    const { runStarted, wsConnected, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");

    // First snapshot with terminal status should be ignored — DB may still
    // hold a stale record from a previous run with the same deterministic ID.
    snapshotApplied(makeRun({ status: 2 }), []);

    expect(useRunStore.getState().phase).toBe("running");
  });

  it("transitions to failed with error message", () => {
    const { runStarted, wsConnected, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");

    // First snapshot establishes prev.run so terminal status is accepted
    snapshotApplied(makeRun({ status: 1 }), []);
    snapshotApplied(makeRun({ status: 3, error_message: "something broke" }), []);

    const s = useRunStore.getState();
    expect(s.phase).toBe("failed");
    expect(s.error).toBe("something broke");
  });

  it("applying same snapshot twice yields identical activity state", () => {
    const { runStarted, wsConnected, snapshotApplied } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");

    const run = makeRun({ status: 1 });
    const acts = [activity("api-1", "step-1")];
    snapshotApplied(run, acts);
    const first = useRunStore.getState().activityByEventId;

    snapshotApplied(run, acts);
    const second = useRunStore.getState().activityByEventId;

    expect(Object.keys(first)).toEqual(Object.keys(second));
  });
});

describe("runFailed", () => {
  it("sets phase to failed and connectionStatus to terminal", () => {
    const { runStarted, wsConnected, runCancelling, runFailed } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");
    runCancelling();
    runFailed("Cancel API failed");

    const s = useRunStore.getState();
    expect(s.phase).toBe("failed");
    expect(s.error).toBe("Cancel API failed");
    expect(s.connectionStatus).toBe("terminal");
  });
});

describe("reportError", () => {
  it("sets error and connectionStatus to error", () => {
    const { reportError } = useRunStore.getState();

    reportError("Connection lost");

    const s = useRunStore.getState();
    expect(s.error).toBe("Connection lost");
    expect(s.connectionStatus).toBe("error");
  });

  it("clears error when called with null", () => {
    const { reportError } = useRunStore.getState();

    reportError("Something broke");
    reportError(null);

    const s = useRunStore.getState();
    expect(s.error).toBeNull();
    expect(s.connectionStatus).toBe("idle");
  });
});

describe("reset", () => {
  it("returns to initial state", () => {
    const { runStarted, wsConnected, wsActivityReceived, reset } = useRunStore.getState();

    runStarted("proj-1", "P", steps);
    wsConnected("run-1");
    wsActivityReceived(activity("evt-1"));

    reset();

    const s = useRunStore.getState();
    expect(s.phase).toBe("idle");
    expect(s.connectionStatus).toBe("idle");
    expect(s.runId).toBeNull();
    expect(s.runDefinition.steps).toEqual([]);
    expect(s.activityByEventId).toEqual({});
    expect(s.activityByStepId).toEqual({});
    expect(s.error).toBeNull();
  });
});

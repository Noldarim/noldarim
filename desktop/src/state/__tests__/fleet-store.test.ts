// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { afterEach, describe, expect, it } from "vitest";

import { useFleetStore, type FleetRun } from "../fleet-store";

function makeRun(overrides: Partial<FleetRun> = {}): FleetRun {
  return {
    runId: `run-${Math.random().toString(36).slice(2, 8)}`,
    projectId: "proj-1",
    projectName: "MyProject",
    name: "test-run",
    status: "running",
    stepCount: 3,
    completedSteps: 1,
    totalTokens: 5000,
    startedAt: new Date().toISOString(),
    ...overrides
  };
}

describe("fleet-store", () => {
  afterEach(() => {
    useFleetStore.getState().reset();
  });

  it("starts with empty runs", () => {
    expect(useFleetStore.getState().runs).toEqual([]);
  });

  it("updates runs via runsUpdated", () => {
    const runs = [makeRun({ runId: "r1" }), makeRun({ runId: "r2" })];
    useFleetStore.getState().runsUpdated(runs);
    expect(useFleetStore.getState().runs).toHaveLength(2);
  });

  it("changes run status", () => {
    const runs = [makeRun({ runId: "r1", status: "running" })];
    useFleetStore.getState().runsUpdated(runs);
    useFleetStore.getState().runStatusChanged("r1", "completed");
    expect(useFleetStore.getState().runs[0].status).toBe("completed");
  });

  it("reset clears runs", () => {
    useFleetStore.getState().runsUpdated([makeRun()]);
    useFleetStore.getState().reset();
    expect(useFleetStore.getState().runs).toEqual([]);
  });
});

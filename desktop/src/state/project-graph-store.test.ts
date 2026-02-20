// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { beforeEach, describe, expect, it } from "vitest";

import { PipelineRunStatus, type PipelineRun } from "../lib/types";
import { useProjectGraphStore } from "./project-graph-store";

function makeRun(id: string, projectId: string): PipelineRun {
  return {
    id,
    project_id: projectId,
    name: id,
    status: PipelineRunStatus.Completed,
    created_at: "2026-02-20T10:00:00Z",
    updated_at: "2026-02-20T10:00:00Z"
  };
}

describe("project-graph-store", () => {
  beforeEach(() => {
    useProjectGraphStore.getState().reset();
  });

  it("prunes expandedRunData entries that are no longer in the run list", () => {
    const run1 = makeRun("run-1", "proj-1");
    const run2 = makeRun("run-2", "proj-1");

    useProjectGraphStore.getState().setRuns("proj-1", [run1, run2]);
    useProjectGraphStore.getState().setExpandedRunData("run-1", run1, []);
    useProjectGraphStore.getState().setExpandedRunData("run-2", run2, []);

    useProjectGraphStore.getState().setRuns("proj-1", [run1]);

    const state = useProjectGraphStore.getState();
    expect(Object.keys(state.expandedRunData)).toEqual(["run-1"]);
  });

  it("clears expandedRunData when switching projects", () => {
    const run1 = makeRun("run-1", "proj-1");
    const run2 = makeRun("run-2", "proj-2");

    useProjectGraphStore.getState().setRuns("proj-1", [run1]);
    useProjectGraphStore.getState().setExpandedRunData("run-1", run1, []);

    useProjectGraphStore.getState().setRuns("proj-2", [run2]);

    const state = useProjectGraphStore.getState();
    expect(state.projectId).toBe("proj-2");
    expect(state.expandedRunData).toEqual({});
  });
});

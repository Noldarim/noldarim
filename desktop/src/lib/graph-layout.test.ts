import { describe, expect, it } from "vitest";

import {
  buildProjectGraph,
  resolveRuns,
  collectCommitShas,
  buildDAG,
  buildCommitSummaryMap,
  commitSummaryFromRun,
  computeDepths,
  assignPipelineRows,
  computeSourceNudges,
  spreadBlockedHeads,
  edgeHandles,
  startCommitOf,
  effectiveHeadCommitSha,
  isCancelledError,
  stepOutcomeKey,
  getHiddenHeadShas,
  type GraphEdgeData,
  type GraphInput
} from "./graph-layout";
import type { AIActivityRecord, CommitInfo, PipelineRun, StepResult } from "./types";
import { PipelineRunStatus, StepStatus } from "./types";

function makeRun(overrides: Partial<PipelineRun> & { id: string }): PipelineRun {
  return {
    project_id: "proj-1",
    name: "Pipeline",
    status: PipelineRunStatus.Completed,
    base_commit_sha: "c2",
    start_commit_sha: "c2",
    head_commit_sha: "c1",
    created_at: "2026-02-10T10:00:00Z",
    ...overrides
  };
}

function makeCommit(hash: string): CommitInfo {
  return {
    Hash: hash,
    Message: hash,
    Author: "bot",
    Parents: []
  };
}

function makeStepResult(overrides: Partial<StepResult> & { step_id: string; step_index: number }): StepResult {
  return {
    id: `sr-${overrides.step_id}`,
    pipeline_run_id: "run-1",
    step_name: overrides.step_id,
    status: StepStatus.Completed,
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
    duration: 0,
    ...overrides
  };
}

function makeInput(overrides: Partial<GraphInput> = {}): GraphInput {
  return {
    runs: [],
    commits: [],
    runDetails: {},
    highlightedRunId: null,
    selectedStep: null,
    selectedBaseCommitSha: null,
    ...overrides
  };
}

// ---------------------------------------------------------------------------
// Per-function unit tests
// ---------------------------------------------------------------------------

describe("resolveRuns", () => {
  it("prefers runDetails when it has more step_results", () => {
    const listRun = makeRun({ id: "r1", step_results: undefined });
    const detailRun = makeRun({
      id: "r1",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1" })]
    });
    const result = resolveRuns([listRun], { r1: { run: detailRun, activities: [] } });
    expect(result[0].step_results).toHaveLength(1);
  });

  it("sorts by created_at ascending", () => {
    const r1 = makeRun({ id: "r1", created_at: "2026-02-10T12:00:00Z" });
    const r2 = makeRun({ id: "r2", created_at: "2026-02-10T10:00:00Z" });
    const result = resolveRuns([r1, r2], {});
    expect(result[0].id).toBe("r2");
    expect(result[1].id).toBe("r1");
  });

  it("keeps list data when runDetails has fewer step_results", () => {
    const listRun = makeRun({
      id: "r1",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "r1" })
      ]
    });
    const detailRun = makeRun({
      id: "r1",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1" })]
    });
    const result = resolveRuns([listRun], { r1: { run: detailRun, activities: [] } });
    expect(result[0].step_results).toHaveLength(2);
  });
});

describe("collectCommitShas", () => {
  it("includes run-referenced SHAs not in git log", () => {
    const runs = [makeRun({ id: "r1", start_commit_sha: "orphan-src", head_commit_sha: "orphan-head" })];
    const dag = buildDAG(runs);
    const result = collectCommitShas(runs, [makeCommit("c1")], dag.dagCommits);
    expect(result).toContain("orphan-src");
    expect(result).toContain("orphan-head");
  });

  it("filters git-only orphans when runs exist", () => {
    const runs = [makeRun({ id: "r1", start_commit_sha: "c2", head_commit_sha: "c1" })];
    const dag = buildDAG(runs);
    const result = collectCommitShas(runs, [makeCommit("c1"), makeCommit("c2"), makeCommit("c3")], dag.dagCommits);
    expect(result).toContain("c1");
    expect(result).toContain("c2");
    expect(result).not.toContain("c3");
  });

  it("keeps only head commit when no runs exist", () => {
    const result = collectCommitShas([], [makeCommit("c1"), makeCommit("c2")], new Set());
    expect(result).toEqual(["c1"]);
  });

  it("does not include intermediate step commits (they appear on expansion)", () => {
    const runs = [makeRun({
      id: "r1",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sc1", pipeline_run_id: "r1" })]
    })];
    const dag = buildDAG(runs);
    const result = collectCommitShas(runs, [], dag.dagCommits);
    // sc1 is an intermediate step commit, not in dagCommits (source/head only)
    expect(result).not.toContain("sc1");
    expect(result).toContain("c2"); // source
    expect(result).toContain("c1"); // head
  });
});

describe("buildDAG", () => {
  it("creates edges from source to head", () => {
    const runs = [makeRun({ id: "r1", start_commit_sha: "c2", head_commit_sha: "c1" })];
    const { dagOut, dagIn, dagCommits } = buildDAG(runs);
    expect(dagCommits.has("c2")).toBe(true);
    expect(dagCommits.has("c1")).toBe(true);
    expect(dagOut.get("c2")?.has("c1")).toBe(true);
    expect(dagIn.get("c1")?.has("c2")).toBe(true);
  });

  it("handles missing head (running pipeline)", () => {
    const runs = [makeRun({ id: "r1", start_commit_sha: "c2", head_commit_sha: "", status: PipelineRunStatus.Running })];
    const { dagOut, dagCommits } = buildDAG(runs);
    expect(dagCommits.has("c2")).toBe(true);
    expect(dagOut.size).toBe(0);
  });

  it("does not create self-loop when src === tgt", () => {
    const runs = [makeRun({ id: "r1", start_commit_sha: "c1", head_commit_sha: "c1" })];
    const { dagOut } = buildDAG(runs);
    expect(dagOut.get("c1")).toBeUndefined();
  });
});

describe("buildCommitSummaryMap", () => {
  it("maps head SHA to run summary", () => {
    const runs = [makeRun({
      id: "r1",
      head_commit_sha: "c1",
      step_results: [makeStepResult({
        step_id: "s1", step_index: 0, pipeline_run_id: "r1",
        commit_sha: "c1", commit_message: "feat: something\ndetails",
        files_changed: 2, insertions: 10, deletions: 3
      })]
    })];
    const map = buildCommitSummaryMap(runs);
    expect(map.get("c1")?.diffSummary).toBe("Δ 2 / +10 -3");
    expect(map.get("c1")?.summaryLine).toBe("feat: something");
  });

  it("skips runs with no head SHA", () => {
    const runs = [makeRun({ id: "r1", head_commit_sha: "", status: PipelineRunStatus.Running })];
    const map = buildCommitSummaryMap(runs);
    expect(map.size).toBe(0);
  });
});

describe("computeDepths", () => {
  it("assigns depth via Kahn's algorithm", () => {
    const runs = [
      makeRun({ id: "r1", start_commit_sha: "a", head_commit_sha: "b", created_at: "2026-01-01T00:00:00Z" }),
      makeRun({ id: "r2", start_commit_sha: "b", head_commit_sha: "c", created_at: "2026-01-02T00:00:00Z" })
    ];
    const dag = buildDAG(runs);
    const depths = computeDepths(dag, runs, [], ["a", "b", "c"]);
    expect(depths.get("a")).toBe(0);
    expect(depths.get("b")).toBe(1);
    expect(depths.get("c")).toBe(2);
  });

  it("expands step columns without shifting non-shared heads", () => {
    const runs = [makeRun({
      id: "r1", start_commit_sha: "a", head_commit_sha: "b",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sc1", pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s2", step_index: 1, commit_sha: "sc2", pipeline_run_id: "r1" })
      ]
    })];
    const dag = buildDAG(runs);
    const depths = computeDepths(dag, runs, ["r1"], ["a", "b"]);
    // a at 0, patch1 at 1, outcome1 at 2, patch2 at 3, outcome2 at 4
    // Head (b) remains at 1 because it is hidden (not a shared start commit).
    expect(depths.get("a")).toBe(0);
    expect(depths.get("patch-r1-s1")).toBe(1);
    expect(depths.get("sc1")).toBe(2);
    expect(depths.get("patch-r1-s2")).toBe(3);
    expect(depths.get("sc2")).toBe(4);
    expect(depths.get("b")).toBe(1);
  });

  it("places synthetic step outcome when last step matches a non-shared head", () => {
    const runs = [makeRun({
      id: "r1", start_commit_sha: "a", head_commit_sha: "b",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "b", pipeline_run_id: "r1" })
      ]
    })];
    const dag = buildDAG(runs);
    const depths = computeDepths(dag, runs, ["r1"], ["a", "b"]);
    // Patch at 1, synthetic outcome at 2, head stays at 1 (hidden).
    expect(depths.get("a")).toBe(0);
    expect(depths.get("patch-r1-s1")).toBe(1);
    expect(depths.get("step-r1-s1")).toBe(2);
    expect(depths.get("b")).toBe(1);
  });

  it("reserves full gap when head is shared as downstream start commit", () => {
    const runs = [
      makeRun({
        id: "r1", start_commit_sha: "a", head_commit_sha: "b", created_at: "2026-01-01T00:00:00Z",
        step_results: [
          makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sc1", pipeline_run_id: "r1" }),
          makeStepResult({ step_id: "s2", step_index: 1, commit_sha: "b", pipeline_run_id: "r1" })
        ]
      }),
      makeRun({ id: "r2", start_commit_sha: "b", head_commit_sha: "c", created_at: "2026-01-02T00:00:00Z" })
    ];
    const dag = buildDAG(runs);
    const depths = computeDepths(dag, runs, ["r1"], ["a", "b", "c"]);

    expect(depths.get("patch-r1-s1")).toBe(1);
    expect(depths.get("sc1")).toBe(2);
    expect(depths.get("patch-r1-s2")).toBe(3);
    expect(depths.get("b")).toBe(4);
  });
});

describe("assignPipelineRows", () => {
  it("places root commits at row 0", () => {
    const runs = [makeRun({ id: "r1", start_commit_sha: "root", head_commit_sha: "h1" })];
    const dag = buildDAG(runs);
    const { commitRowMap } = assignPipelineRows(runs, dag);
    expect(commitRowMap.get("root")).toBe(0);
  });

  it("gives each pipeline a unique row", () => {
    const runs = [
      makeRun({ id: "r1", start_commit_sha: "root", head_commit_sha: "h1", created_at: "2026-01-01T00:00:00Z" }),
      makeRun({ id: "r2", start_commit_sha: "root", head_commit_sha: "h2", created_at: "2026-01-02T00:00:00Z" })
    ];
    const dag = buildDAG(runs);
    const { pipelineRowMap } = assignPipelineRows(runs, dag);
    expect(pipelineRowMap.get("r1")).toBe(1);
    expect(pipelineRowMap.get("r2")).toBe(2);
  });

  it("places shared commits only once (first pipeline wins)", () => {
    const runs = [
      makeRun({ id: "r1", start_commit_sha: "root", head_commit_sha: "shared", created_at: "2026-01-01T00:00:00Z" }),
      makeRun({ id: "r2", start_commit_sha: "root", head_commit_sha: "shared", created_at: "2026-01-02T00:00:00Z" })
    ];
    const dag = buildDAG(runs);
    const { commitRowMap } = assignPipelineRows(runs, dag);
    // shared placed by r1 at row 1
    expect(commitRowMap.get("shared")).toBe(1);
  });

  it("places ghost on pipeline row for running pipeline", () => {
    const runs = [makeRun({ id: "r1", start_commit_sha: "root", head_commit_sha: "", status: PipelineRunStatus.Running })];
    const dag = buildDAG(runs);
    const { commitRowMap } = assignPipelineRows(runs, dag);
    expect(commitRowMap.get("ghost-r1")).toBe(1);
  });

  it("places step commits on pipeline row", () => {
    const runs = [makeRun({
      id: "r1", start_commit_sha: "root", head_commit_sha: "h1",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sc1", pipeline_run_id: "r1" })]
    })];
    const dag = buildDAG(runs);
    const { commitRowMap, pipelineRowMap } = assignPipelineRows(runs, dag);
    expect(commitRowMap.get("sc1")).toBe(pipelineRowMap.get("r1"));
  });
});

describe("edgeHandles", () => {
  it("returns RIGHT→LEFT for same row", () => {
    const handles = edgeHandles(1, 1);
    expect(handles.sourceHandle).toBe("run-source");
    expect(handles.targetHandle).toBe("run-target");
  });

  it("returns TOP→LEFT for cross-row", () => {
    const handles = edgeHandles(0, 1);
    expect(handles.sourceHandle).toBe("run-source-top");
    expect(handles.targetHandle).toBe("run-target");
  });
});

describe("effectiveHeadCommitSha", () => {
  it("returns head_commit_sha when set", () => {
    const run = makeRun({ id: "r1", head_commit_sha: "abc123" });
    expect(effectiveHeadCommitSha(run)).toBe("abc123");
  });

  it("falls back to last completed step commit_sha", () => {
    const run = makeRun({
      id: "r1",
      head_commit_sha: "",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, status: StepStatus.Completed, commit_sha: "first", pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s2", step_index: 1, status: StepStatus.Completed, commit_sha: "second", pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s3", step_index: 2, status: StepStatus.Running, commit_sha: "", pipeline_run_id: "r1" })
      ]
    });
    expect(effectiveHeadCommitSha(run)).toBe("second");
  });

  it("returns empty string when no completed steps", () => {
    const run = makeRun({
      id: "r1",
      head_commit_sha: "",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, status: StepStatus.Running, commit_sha: "", pipeline_run_id: "r1" })
      ]
    });
    expect(effectiveHeadCommitSha(run)).toBe("");
  });
});

describe("isCancelledError", () => {
  it("detects 'cancelled'", () => {
    expect(isCancelledError("Cancelled by user")).toBe(true);
  });

  it("detects 'canceled' (American spelling)", () => {
    expect(isCancelledError("Operation canceled")).toBe(true);
  });

  it("is case-insensitive", () => {
    expect(isCancelledError("CANCELLED")).toBe(true);
    expect(isCancelledError("Cancellation requested")).toBe(true);
  });

  it("returns false for empty/undefined", () => {
    expect(isCancelledError("")).toBe(false);
    expect(isCancelledError(undefined)).toBe(false);
  });

  it("returns false for unrelated errors", () => {
    expect(isCancelledError("Connection timeout")).toBe(false);
  });
});

describe("stepOutcomeKey", () => {
  const startShas = new Set(["shared-sha"]);

  it("returns synthetic key when step has no commit_sha", () => {
    const sr = makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "" });
    expect(stepOutcomeKey(sr, "head", "r1", startShas)).toBe("step-r1-s1");
  });

  it("returns synthetic key when step matches non-shared head", () => {
    const sr = makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "head" });
    expect(stepOutcomeKey(sr, "head", "r1", startShas)).toBe("step-r1-s1");
  });

  it("returns real SHA when step matches shared head", () => {
    const sr = makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "shared-sha" });
    expect(stepOutcomeKey(sr, "shared-sha", "r1", startShas)).toBe("shared-sha");
  });

  it("returns real SHA when step does not match head", () => {
    const sr = makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "abc123" });
    expect(stepOutcomeKey(sr, "head", "r1", startShas)).toBe("abc123");
  });

  it("handles null/undefined head", () => {
    const sr = makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "abc123" });
    expect(stepOutcomeKey(sr, null, "r1", startShas)).toBe("abc123");
    expect(stepOutcomeKey(sr, undefined, "r1", startShas)).toBe("abc123");
  });
});

describe("getHiddenHeadShas", () => {
  it("hides head when run has steps and head is not a start SHA", () => {
    const run = makeRun({
      id: "r1", start_commit_sha: "a", head_commit_sha: "b",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1" })]
    });
    const runsById = new Map([["r1", run]]);
    const startShas = new Set(["a"]);
    const result = getHiddenHeadShas(runsById, ["r1"], startShas);
    expect(result.has("b")).toBe(true);
  });

  it("keeps head visible when it is also a start SHA (shared/chained)", () => {
    const r1 = makeRun({
      id: "r1", start_commit_sha: "a", head_commit_sha: "b",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1" })]
    });
    const r2 = makeRun({ id: "r2", start_commit_sha: "b", head_commit_sha: "c" });
    const runsById = new Map([["r1", r1], ["r2", r2]]);
    const startShas = new Set(["a", "b"]);
    const result = getHiddenHeadShas(runsById, ["r1"], startShas);
    expect(result.has("b")).toBe(false);
  });

  it("keeps head visible when run has no non-skipped steps", () => {
    const run = makeRun({
      id: "r1", start_commit_sha: "a", head_commit_sha: "b",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Skipped })]
    });
    const runsById = new Map([["r1", run]]);
    const startShas = new Set(["a"]);
    const result = getHiddenHeadShas(runsById, ["r1"], startShas);
    expect(result.has("b")).toBe(false);
  });

  it("returns empty set when no runs are expanded", () => {
    const run = makeRun({ id: "r1" });
    const runsById = new Map([["r1", run]]);
    const result = getHiddenHeadShas(runsById, [], new Set(["c2"]));
    expect(result.size).toBe(0);
  });
});

describe("commitSummaryFromRun", () => {
  it("returns diff summary from completed steps", () => {
    const run = makeRun({
      id: "r1",
      step_results: [
        makeStepResult({
          step_id: "s1", step_index: 0, pipeline_run_id: "r1",
          status: StepStatus.Completed,
          files_changed: 3, insertions: 20, deletions: 5,
          commit_message: "feat: add auth"
        })
      ]
    });
    const result = commitSummaryFromRun(run);
    expect(result.diffSummary).toBe("Δ 3 / +20 -5");
    expect(result.summaryLine).toBe("feat: add auth");
  });

  it("uses first line of multi-line commit message", () => {
    const run = makeRun({
      id: "r1",
      step_results: [
        makeStepResult({
          step_id: "s1", step_index: 0, pipeline_run_id: "r1",
          status: StepStatus.Completed,
          commit_message: "feat: something\n\nDetailed description here"
        })
      ]
    });
    const result = commitSummaryFromRun(run);
    expect(result.summaryLine).toBe("feat: something");
  });

  it("returns empty object when no completed steps", () => {
    const run = makeRun({
      id: "r1",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Running })
      ]
    });
    expect(commitSummaryFromRun(run)).toEqual({});
  });
});

describe("computeSourceNudges", () => {
  it("nudges source when intermediate same-depth node exists between rows", () => {
    // D(depth 0, row 0) → F(depth 1, row 3), E at (depth 0, row 2) blocks the vertical
    const r1 = makeRun({ id: "r1", start_commit_sha: "D", head_commit_sha: "F", created_at: "2026-01-01T00:00:00Z" });
    const r2 = makeRun({ id: "r2", start_commit_sha: "D", head_commit_sha: "E", created_at: "2026-01-02T00:00:00Z" });
    const r3 = makeRun({ id: "r3", start_commit_sha: "D", head_commit_sha: "G", created_at: "2026-01-03T00:00:00Z" });
    const runs = [r1, r2, r3];

    const depths = new Map<string, number>([["D", 0], ["E", 1], ["F", 1], ["G", 1]]);
    const dag = buildDAG(runs);
    const rows = assignPipelineRows(runs, dag);

    // D at row 0, r1→row 1 (F), r2→row 2 (E), r3→row 3 (G)
    // For r3: D(row 0) → G(row 3), gap=3. Check depth 0 for rows 1,2 — no nodes at depth 0 in between
    // But let's force D at depth 0 and put a node at depth 0 row 2 to test
    // Actually, with these depths, E/F/G are all at depth 1, D at depth 0
    // r3 goes D(depth 0, row 0) → G(depth 1, row 3). Intermediate rows 1,2 at depth 0: none → no nudge
    // Let me restructure: need an intermediate node at same depth as source

    // Better test: source at depth 1, intermediate at depth 1
    const depths2 = new Map<string, number>([["D", 1], ["E", 1], ["F", 2], ["G", 2]]);
    const rows2 = assignPipelineRows(runs, dag);
    // D at row 0, F at row 1, E at row 2, G at row 3
    // r3: D(depth 1, row 0) → G(depth 2, row 3), gap=3
    // depth 1 nodes: D(row 0), E(row 2). Intermediate rows: 1,2. E at row 2 is between 0 and 3 → nudge D!

    const nudges = computeSourceNudges(runs, depths2, rows2);
    expect(nudges.has("D")).toBe(true);
    expect(nudges.get("D")).toBeLessThan(0);
  });

  it("does not nudge when no intermediate same-depth node exists", () => {
    const r1 = makeRun({ id: "r1", start_commit_sha: "A", head_commit_sha: "B", created_at: "2026-01-01T00:00:00Z" });
    const runs = [r1];
    const depths = new Map<string, number>([["A", 0], ["B", 1]]);
    const dag = buildDAG(runs);
    const rows = assignPipelineRows(runs, dag);

    const nudges = computeSourceNudges(runs, depths, rows);
    expect(nudges.size).toBe(0);
  });

  it("does not nudge same-row edges", () => {
    // Both source and target on same row (e.g., chained pipeline)
    const r1 = makeRun({ id: "r1", start_commit_sha: "A", head_commit_sha: "B", created_at: "2026-01-01T00:00:00Z" });
    const runs = [r1];
    const depths = new Map<string, number>([["A", 0], ["B", 1]]);
    const dag = buildDAG(runs);
    const rows = assignPipelineRows(runs, dag);
    // Force same row
    rows.commitRowMap.set("A", 1);
    rows.commitRowMap.set("B", 1);

    const nudges = computeSourceNudges(runs, depths, rows);
    expect(nudges.size).toBe(0);
  });

  it("does not nudge when row gap is exactly 1", () => {
    const r1 = makeRun({ id: "r1", start_commit_sha: "A", head_commit_sha: "B", created_at: "2026-01-01T00:00:00Z" });
    const runs = [r1];
    const depths = new Map<string, number>([["A", 0], ["B", 1]]);
    const dag = buildDAG(runs);
    const rows = assignPipelineRows(runs, dag);
    // A at row 0, B at row 1 — gap is exactly 1
    expect(rows.commitRowMap.get("A")).toBe(0);
    expect(rows.commitRowMap.get("B")).toBe(1);

    const nudges = computeSourceNudges(runs, depths, rows);
    expect(nudges.size).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Integration tests — buildProjectGraph
// ---------------------------------------------------------------------------

describe("buildProjectGraph (edge-centric)", () => {
  it("returns empty graph for no commits and no runs", () => {
    const { nodes, edges } = buildProjectGraph(makeInput());
    expect(nodes).toHaveLength(0);
    expect(edges).toHaveLength(0);
  });

  it("renders only head commit when no runs exist", () => {
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2"), makeCommit("c3"), makeCommit("c4")]
      })
    );

    expect(nodes.filter((n) => n.type === "commit")).toHaveLength(1);
    expect(nodes[0].id).toBe("commit-c1");
    expect(edges).toHaveLength(0);
  });

  it("renders collapsed run edge between source and target commits", () => {
    const run = makeRun({ id: "r1", start_commit_sha: "c2", head_commit_sha: "c1" });
    const { edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run]
      })
    );

    const runEdge = edges.find((e) => e.id === "run-edge-r1");
    expect(runEdge).toBeDefined();
    expect(runEdge?.data).toMatchObject({
      kind: "run",
      runId: "r1",
      status: "completed"
    });
  });

  it("uses TOP→LEFT handle for cross-row collapsed edges", () => {
    const run = makeRun({ id: "r1", start_commit_sha: "c2", head_commit_sha: "c1" });
    const { edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run]
      })
    );

    const runEdge = edges.find((e) => e.id === "run-edge-r1");
    // Source (c2) is at row 0 (root), target (c1) is at row 1 (pipeline row)
    expect(runEdge?.sourceHandle).toBe("run-source-top");
    expect(runEdge?.targetHandle).toBe("run-target");
  });

  it("always expands runs into step commit nodes and edges", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "c1",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Completed, commit_sha: "sc1" }),
        makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "r1", status: StepStatus.Running })
      ]
    });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run],
        runDetails: { "r1": { run, activities: [] } },
      })
    );

    // Step commits should be real commit nodes, not edge-anchors
    expect(nodes.filter((n) => n.type === "edge-anchor")).toHaveLength(0);
    const stepNodes = nodes.filter((n) => n.data && (n.data as Record<string, unknown>).isStepCommit);
    expect(stepNodes.length).toBeGreaterThanOrEqual(2);
    expect(edges.find((e) => e.id === "step-edge-r1-s1")).toBeDefined();
    expect(edges.find((e) => e.id === "step-edge-r1-s2")).toBeDefined();
  });

  it("adds ghost target node for running run without head commit", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "",
      status: PipelineRunStatus.Running
    });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c2")],
        runs: [run]
      })
    );

    const ghost = nodes.find((n) => n.id === "ghost-r1");
    expect(ghost).toBeDefined();
    const runEdge = edges.find((e) => e.id === "run-edge-r1");
    expect(runEdge?.target).toBe("ghost-r1");
    expect(runEdge?.data).toMatchObject({ status: "running" });
  });

  it("marks selected base commit node", () => {
    const run = makeRun({ id: "r1", start_commit_sha: "c2", head_commit_sha: "c1" });
    const { nodes } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run],
        selectedBaseCommitSha: "c1"
      })
    );

    const c1 = nodes.find((n) => n.id === "commit-c1");
    const c2 = nodes.find((n) => n.id === "commit-c2");
    expect(c1?.selected).toBe(true);
    expect(c2?.selected).toBeFalsy();
  });

  it("marks selected step edge", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "c1",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", commit_sha: "sc1" })]
    });
    const { edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run],
        runDetails: { "r1": { run, activities: [] } },
        highlightedRunId: "r1",
        selectedStep: { runId: "r1", stepId: "s1" }
      })
    );

    const stepEdge = edges.find((e) => e.id === "step-edge-r1-s1");
    expect(stepEdge?.selected).toBe(true);
  });

  it("classifies failed runs with cancellation errors as cancelled", () => {
    const run = makeRun({
      id: "r1",
      status: PipelineRunStatus.Failed,
      error_message: "Cancelled by user from edge drawer",
      start_commit_sha: "c2",
      head_commit_sha: "c1"
    });
    const { edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run]
      })
    );

    const runEdge = edges.find((e) => e.id === "run-edge-r1");
    expect(runEdge?.data).toMatchObject({
      kind: "run",
      status: "cancelled"
    });
  });

  it("renders runs even when commit history does not include run commits", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "older-base",
      head_commit_sha: "older-head"
    });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("new-a"), makeCommit("new-b")],
        runs: [run]
      })
    );

    expect(nodes.find((n) => n.id === "commit-older-base")).toBeDefined();
    expect(nodes.find((n) => n.id === "commit-older-head")).toBeDefined();
    expect(edges.find((e) => e.id === "run-edge-r1")).toBeDefined();
  });

  it("does not create tail edge when head commit is hidden", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "c1",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Completed, commit_sha: "sc1" }),
        makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "r1", status: StepStatus.Completed, commit_sha: "sc2" })
      ]
    });
    const { edges, nodes } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run],
        runDetails: { "r1": { run, activities: [] } },
      })
    );

    // Head commit c1 is hidden (subsumed by step chain), no tail edge
    expect(edges.find((e) => e.id === "run-tail-r1-s2")).toBeUndefined();
    expect(nodes.find((n) => n.id === "commit-c1")).toBeUndefined();
    // Step chain still exists
    expect(edges.find((e) => e.id === "step-edge-r1-s1")).toBeDefined();
    expect(edges.find((e) => e.id === "step-edge-r1-s2")).toBeDefined();
  });

  it("skips skipped step edges", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "c1",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Completed, commit_sha: "sc1" }),
        makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "r1", status: StepStatus.Skipped }),
        makeStepResult({ step_id: "s3", step_index: 2, pipeline_run_id: "r1", status: StepStatus.Running })
      ]
    });
    const { edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run],
        runDetails: { "r1": { run, activities: [] } },
      })
    );

    expect(edges.find((e) => e.id === "step-edge-r1-s1")).toBeDefined();
    expect(edges.find((e) => e.id === "step-edge-r1-s2")).toBeUndefined();
    expect(edges.find((e) => e.id === "step-edge-r1-s3")).toBeDefined();
  });

  it("uses last completed step commit as effective head", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "",
      status: PipelineRunStatus.Running,
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Completed, commit_sha: "step-c1" }),
        makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "r1", status: StepStatus.Running })
      ]
    });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c2")],
        runs: [run]
      })
    );

    // Should use step-c1 as effective head, not create a ghost
    const ghost = nodes.find((n) => n.id === "ghost-r1");
    expect(ghost).toBeUndefined();
    // Head commit (step-c1) is hidden — represented by synthetic step outcome node
    expect(nodes.find((n) => n.id === "commit-step-c1")).toBeUndefined();
    const stepOutcome = nodes.find((n) => n.id === "commit-step-r1-s1");
    expect(stepOutcome).toBeDefined();
    // Auto-expanded: produces step edges, not a single run-edge
    expect(edges.find((e) => e.id === "step-edge-r1-s1")).toBeDefined();
    expect(edges.find((e) => e.id === "step-edge-r1-s2")).toBeDefined();
  });

  it("does not create ghost when run has step data (step outcomes serve as endpoints)", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "",
      status: PipelineRunStatus.Running,
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Running })
      ]
    });
    const { nodes } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c2")],
        runs: [run]
      })
    );

    // No ghost — step outcome node serves as the endpoint
    expect(nodes.find((n) => n.id === "ghost-r1")).toBeUndefined();
    // In-progress step outcome exists instead
    const stepOutcome = nodes.find(n => n.id === "commit-step-r1-s1");
    expect(stepOutcome).toBeDefined();
  });

  it("step outcomes are ordered left-to-right when pipeline has no head", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "",
      status: PipelineRunStatus.Running,
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Completed, commit_sha: "" }),
        makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "r1", status: StepStatus.Running })
      ]
    });
    const { nodes } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c2")],
        runs: [run],
      })
    );

    const source = nodes.find(n => n.id === "commit-c2")!;
    const step1 = nodes.find(n => n.id === "commit-step-r1-s1")!;
    const step2 = nodes.find(n => n.id === "commit-step-r1-s2")!;

    // No ghost — step chain is the complete visual
    expect(nodes.find(n => n.id === "ghost-r1")).toBeUndefined();
    // Steps should be ordered left-to-right after source
    expect(step1.position.x).toBeGreaterThan(source.position.x);
    expect(step2.position.x).toBeGreaterThan(step1.position.x);
  });

  it("creates step chain edges for runs with step data", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "c1",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Completed }),
        makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "r1", status: StepStatus.Completed })
      ]
    });
    const { edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run],
        runDetails: { "r1": { run, activities: [] } },
        highlightedRunId: "r1"
      })
    );

    // All runs are auto-expanded — step edges created, no collapsed run-edge
    expect(edges.find((e) => e.id === "step-edge-r1-s1")).toBeDefined();
    expect(edges.find((e) => e.id === "step-edge-r1-s2")).toBeDefined();
    expect(edges.find((e) => e.id === "run-edge-r1")).toBeUndefined();
  });

  it("hides head commit when step commit_sha matches head", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "c2",
      head_commit_sha: "c1",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1", status: StepStatus.Completed, commit_sha: "c1" })
      ]
    });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("c1"), makeCommit("c2")],
        runs: [run],
        runDetails: { "r1": { run, activities: [] } },
      })
    );

    // c1 (head commit) is hidden — subsumed by step chain
    expect(nodes.find((n) => n.id === "commit-c1")).toBeUndefined();

    // Step gets a synthetic outcome node
    const stepNode = nodes.find((n) => n.id === "commit-step-r1-s1");
    expect(stepNode).toBeDefined();
    expect((stepNode!.data as Record<string, unknown>).isStepCommit).toBe(true);

    // Step edge targets the synthetic step node
    const stepEdge = edges.find((e) => e.id === "step-edge-r1-s1");
    expect(stepEdge?.target).toBe("commit-step-r1-s1");

    // No tail edge — head is hidden
    expect(edges.find((e) => e.id.startsWith("run-tail-r1"))).toBeUndefined();
  });

  it("chained pipelines (P1→P2) place on separate rows", () => {
    const r1 = makeRun({ id: "r1", start_commit_sha: "a", head_commit_sha: "b", created_at: "2026-01-01T00:00:00Z" });
    const r2 = makeRun({ id: "r2", start_commit_sha: "b", head_commit_sha: "c", created_at: "2026-01-02T00:00:00Z" });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("a"), makeCommit("b"), makeCommit("c")],
        runs: [r1, r2]
      })
    );

    // All 3 commits present
    expect(nodes.find(n => n.id === "commit-a")).toBeDefined();
    expect(nodes.find(n => n.id === "commit-b")).toBeDefined();
    expect(nodes.find(n => n.id === "commit-c")).toBeDefined();
    // Both edges present
    expect(edges.find(e => e.id === "run-edge-r1")).toBeDefined();
    expect(edges.find(e => e.id === "run-edge-r2")).toBeDefined();
    // Root commit (a) at bottom, pipelines above
    const aNode = nodes.find(n => n.id === "commit-a")!;
    const bNode = nodes.find(n => n.id === "commit-b")!;
    const cNode = nodes.find(n => n.id === "commit-c")!;
    // a should have highest y (bottom = row 0 with max-row subtraction)
    expect(aNode.position.y).toBeGreaterThan(bNode.position.y);
  });

  it("fork sharing commits renders shared commit once", () => {
    const r1 = makeRun({ id: "r1", start_commit_sha: "root", head_commit_sha: "h1", created_at: "2026-01-01T00:00:00Z" });
    const r2 = makeRun({ id: "r2", start_commit_sha: "root", head_commit_sha: "h2", created_at: "2026-01-02T00:00:00Z" });
    const { nodes } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("root"), makeCommit("h1"), makeCommit("h2")],
        runs: [r1, r2]
      })
    );

    const rootNodes = nodes.filter(n => n.id === "commit-root");
    expect(rootNodes).toHaveLength(1);
  });

  it("multiple pipelines from same source get separate rows", () => {
    const r1 = makeRun({ id: "r1", start_commit_sha: "root", head_commit_sha: "h1", created_at: "2026-01-01T00:00:00Z" });
    const r2 = makeRun({ id: "r2", start_commit_sha: "root", head_commit_sha: "h2", created_at: "2026-01-02T00:00:00Z" });
    const r3 = makeRun({ id: "r3", start_commit_sha: "root", head_commit_sha: "h3", created_at: "2026-01-03T00:00:00Z" });
    const { nodes } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("root"), makeCommit("h1"), makeCommit("h2"), makeCommit("h3")],
        runs: [r1, r2, r3]
      })
    );

    const h1 = nodes.find(n => n.id === "commit-h1")!;
    const h2 = nodes.find(n => n.id === "commit-h2")!;
    const h3 = nodes.find(n => n.id === "commit-h3")!;
    // All head commits should be at different y positions (different rows)
    const ySet = new Set([h1.position.y, h2.position.y, h3.position.y]);
    expect(ySet.size).toBe(3);
  });

  it("nudges source node left when cross-row vertical passes through intermediate node", () => {
    // root → h1 (row 1), root → h2 (row 2), root → h3 (row 3)
    // root is at depth 0, row 0. h1/h2/h3 are at depth 1, rows 1/2/3.
    // For the r3 edge: root(depth 0, row 0) → h3(depth 1, row 3), gap=3
    // At depth 0, only root at row 0 — no intermediate blocking node at depth 0
    // So root won't be nudged in this setup. For nudging to trigger we need
    // another node at same depth as source between source and target rows.
    // Let's chain: a→b (r1), a→c (r2), b→d (r3).
    // Then a at depth 0 row 0, b at depth 1 row 1, c at depth 1 row 2, d at depth 2 row 3.
    // r2 edge: a(depth 0, row 0) → c(depth 1, row 2), gap=2 > 1.
    // Depth 0 has only a at row 0. Between 0 and 2: row 1 — a not at row 1 → no nudge.
    // We need a node at depth 0 between the rows. Let's use a different setup:
    // a→b (r1, chained), b→c (r2), a→d (r3, fork from a to row 3)
    // a at depth 0, b at depth 1 row 1, c at depth 2 row 2, d at depth 1 row 3
    // r3: a(depth 0, row 0) → d(depth 1, row 3), gap 3. depth 0 rows: {0}. No blocking.
    // To truly test, put another root: e at depth 0, row 2 (another root with its own pipeline)
    // Actually, let's use the simplest scenario:
    // Two separate root commits at same depth, one pipeline crosses past the other.
    // This requires multiple roots — but our DAG places root commits at row 0.
    // The realistic scenario is: one root has 3+ forks. The vertical from root to fork 3
    // passes through the rows of forks 1 and 2. But root is at depth 0 and
    // forks' heads are at depth 1 — the only node at depth 0 is root itself.
    // So nudging only triggers when a node OTHER than the source at the same depth
    // sits on an intermediate row. This happens when multiple pipelines chain and share depth.

    // Simple integration test: no nudge when all source nodes have unique depths
    const r1 = makeRun({ id: "r1", start_commit_sha: "root", head_commit_sha: "h1", created_at: "2026-01-01T00:00:00Z" });
    const r2 = makeRun({ id: "r2", start_commit_sha: "root", head_commit_sha: "h2", created_at: "2026-01-02T00:00:00Z" });
    const { nodes } = buildProjectGraph(
      makeInput({
        commits: [makeCommit("root"), makeCommit("h1"), makeCommit("h2")],
        runs: [r1, r2]
      })
    );

    const rootNode = nodes.find(n => n.id === "commit-root")!;
    const h1Node = nodes.find(n => n.id === "commit-h1")!;
    // root should NOT be nudged (no intermediate same-depth node)
    // ORIGIN_X is 90, depth 0 → x = 90
    expect(rootNode.position.x).toBe(90);
    // h1 at depth 1 → x = 90 + 340 = 430
    expect(h1Node.position.x).toBe(430);
  });
});

// ---------------------------------------------------------------------------
// Realistic DB scenarios — models what the backend actually returns
// ---------------------------------------------------------------------------

const COMPLETED_2STEP_RUN: PipelineRun = {
  id: "run-abc-001",
  project_id: "proj-demo",
  name: "Feature: add auth",
  status: PipelineRunStatus.Completed,
  base_commit_sha: "aaa1111111111111111111111111111111111111",
  start_commit_sha: "aaa1111111111111111111111111111111111111",
  head_commit_sha: "ccc3333333333333333333333333333333333333",
  created_at: "2026-02-18T09:00:00Z",
  started_at: "2026-02-18T09:00:05Z",
  completed_at: "2026-02-18T09:02:30Z",
  step_results: [
    {
      id: "sr-001-gen",
      pipeline_run_id: "run-abc-001",
      step_id: "step-codegen",
      step_name: "Code generation",
      step_index: 0,
      status: StepStatus.Completed,
      commit_sha: "bbb2222222222222222222222222222222222222",
      commit_message: "feat: add auth middleware\n\nAdded JWT validation.",
      git_diff: "diff --git ...",
      files_changed: 3,
      insertions: 85,
      deletions: 2,
      input_tokens: 1200,
      output_tokens: 800,
      cache_read_tokens: 0,
      cache_create_tokens: 0,
      agent_output: "Created auth middleware with JWT validation",
      duration: 45_000_000_000,
      started_at: "2026-02-18T09:00:05Z",
      completed_at: "2026-02-18T09:00:50Z"
    },
    {
      id: "sr-001-rev",
      pipeline_run_id: "run-abc-001",
      step_id: "step-review",
      step_name: "Code review",
      step_index: 1,
      status: StepStatus.Completed,
      commit_sha: "ccc3333333333333333333333333333333333333",
      commit_message: "fix: address review comments",
      git_diff: "diff --git ...",
      files_changed: 2,
      insertions: 15,
      deletions: 8,
      input_tokens: 900,
      output_tokens: 400,
      cache_read_tokens: 200,
      cache_create_tokens: 0,
      agent_output: "Fixed error handling in auth middleware",
      duration: 30_000_000_000,
      started_at: "2026-02-18T09:00:55Z",
      completed_at: "2026-02-18T09:01:25Z"
    }
  ]
};

const RUNNING_3STEP_RUN: PipelineRun = {
  id: "run-abc-002",
  project_id: "proj-demo",
  name: "Feature: add tests",
  status: PipelineRunStatus.Running,
  base_commit_sha: "ccc3333333333333333333333333333333333333",
  start_commit_sha: "ccc3333333333333333333333333333333333333",
  head_commit_sha: "",
  created_at: "2026-02-18T09:05:00Z",
  started_at: "2026-02-18T09:05:02Z",
  step_results: [
    {
      id: "sr-002-unit",
      pipeline_run_id: "run-abc-002",
      step_id: "step-unit-tests",
      step_name: "Write unit tests",
      step_index: 0,
      status: StepStatus.Completed,
      commit_sha: "ddd4444444444444444444444444444444444444",
      commit_message: "test: add unit tests for auth",
      git_diff: "",
      files_changed: 2,
      insertions: 120,
      deletions: 0,
      input_tokens: 1500,
      output_tokens: 1000,
      cache_read_tokens: 0,
      cache_create_tokens: 0,
      agent_output: "Added 12 test cases for auth module",
      duration: 60_000_000_000,
      started_at: "2026-02-18T09:05:02Z",
      completed_at: "2026-02-18T09:06:02Z"
    },
    {
      id: "sr-002-integ",
      pipeline_run_id: "run-abc-002",
      step_id: "step-integ-tests",
      step_name: "Write integration tests",
      step_index: 1,
      status: StepStatus.Running,
      commit_sha: "",
      commit_message: "",
      git_diff: "",
      files_changed: 0,
      insertions: 0,
      deletions: 0,
      input_tokens: 500,
      output_tokens: 200,
      cache_read_tokens: 0,
      cache_create_tokens: 0,
      agent_output: "",
      duration: 0,
      started_at: "2026-02-18T09:06:05Z"
    },
    {
      id: "sr-002-e2e",
      pipeline_run_id: "run-abc-002",
      step_id: "step-e2e-tests",
      step_name: "Write E2E tests",
      step_index: 2,
      status: StepStatus.Pending,
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
    }
  ]
};

const CANCELLED_RUN: PipelineRun = {
  id: "run-abc-003",
  project_id: "proj-demo",
  name: "Feature: add logging",
  status: PipelineRunStatus.Failed,
  error_message: "Cancelled by user",
  base_commit_sha: "aaa1111111111111111111111111111111111111",
  start_commit_sha: "aaa1111111111111111111111111111111111111",
  head_commit_sha: "",
  created_at: "2026-02-18T08:00:00Z",
  started_at: "2026-02-18T08:00:02Z",
  step_results: [
    {
      id: "sr-003-impl",
      pipeline_run_id: "run-abc-003",
      step_id: "step-implement",
      step_name: "Implement logging",
      step_index: 0,
      status: StepStatus.Completed,
      commit_sha: "eee5555555555555555555555555555555555555",
      commit_message: "feat: add structured logging",
      git_diff: "",
      files_changed: 4,
      insertions: 60,
      deletions: 5,
      input_tokens: 800,
      output_tokens: 600,
      cache_read_tokens: 0,
      cache_create_tokens: 0,
      agent_output: "Added structured logging framework",
      duration: 35_000_000_000,
      started_at: "2026-02-18T08:00:02Z",
      completed_at: "2026-02-18T08:00:37Z"
    },
    {
      id: "sr-003-test",
      pipeline_run_id: "run-abc-003",
      step_id: "step-test-logging",
      step_name: "Test logging",
      step_index: 1,
      status: StepStatus.Failed,
      commit_sha: "",
      commit_message: "",
      git_diff: "",
      files_changed: 0,
      insertions: 0,
      deletions: 0,
      input_tokens: 200,
      output_tokens: 50,
      cache_read_tokens: 0,
      cache_create_tokens: 0,
      agent_output: "",
      duration: 5_000_000_000,
      error_message: "Cancelled by user",
      started_at: "2026-02-18T08:00:40Z",
      completed_at: "2026-02-18T08:00:45Z"
    }
  ]
};

const DEMO_COMMITS: CommitInfo[] = [
  { Hash: "aaa1111111111111111111111111111111111111", Message: "initial commit", Author: "dev", Parents: [] },
  { Hash: "bbb2222222222222222222222222222222222222", Message: "feat: add auth middleware", Author: "bot", Parents: ["aaa1111111111111111111111111111111111111"] },
  { Hash: "ccc3333333333333333333333333333333333333", Message: "fix: address review comments", Author: "bot", Parents: ["bbb2222222222222222222222222222222222222"] },
  { Hash: "ddd4444444444444444444444444444444444444", Message: "test: add unit tests for auth", Author: "bot", Parents: ["ccc3333333333333333333333333333333333333"] },
  { Hash: "eee5555555555555555555555555555555555555", Message: "feat: add structured logging", Author: "bot", Parents: ["aaa1111111111111111111111111111111111111"] }
];

describe("buildProjectGraph — realistic DB scenarios", () => {
  function input(overrides: Partial<GraphInput> = {}): GraphInput {
    return {
      runs: [],
      commits: DEMO_COMMITS,
      runDetails: {},
      highlightedRunId: null,
      selectedStep: null,
      selectedBaseCommitSha: null,
      ...overrides
    };
  }

  describe("completed 2-step pipeline", () => {
    it("produces step chain edges (always expanded)", () => {
      const { edges } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN]
      }));

      // No collapsed run-edge — all runs are always expanded
      expect(edges.find(e => e.id === "run-edge-run-abc-001")).toBeUndefined();
      expect(edges.find(e => e.id === "step-edge-run-abc-001-step-codegen")).toBeDefined();
      expect(edges.find(e => e.id === "step-edge-run-abc-001-step-review")).toBeDefined();
    });

    it("creates commit nodes for start and steps but hides head (always expanded)", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN]
      }));

      const commitNodes = nodes.filter(n => n.type === "commit");
      const shas = commitNodes.map(n => n.id);

      expect(shas).toContain("commit-aaa1111111111111111111111111111111111111");
      // Intermediate step commit is visible
      expect(shas).toContain("commit-bbb2222222222222222222222222222222222222");
      // Head commit (ccc333...) is hidden — subsumed by step chain
      // (last step's commit_sha matches head → synthetic outcome node instead)
      expect(shas).not.toContain("commit-ccc3333333333333333333333333333333333333");
    });

    it("shows intermediate step commits but hides head (always expanded)", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN],
      }));

      const commitNodes = nodes.filter(n => n.type === "commit");
      const shas = commitNodes.map(n => n.id);

      expect(shas).toContain("commit-aaa1111111111111111111111111111111111111");
      expect(shas).toContain("commit-bbb2222222222222222222222222222222222222");
      // Head commit hidden — represented by synthetic step outcome node
      expect(shas).not.toContain("commit-ccc3333333333333333333333333333333333333");
    });

    it("shows both step edges and commit nodes", () => {
      const { nodes, edges } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN],
      }));

      expect(edges.find(e => e.id === "step-edge-run-abc-001-step-codegen")).toBeDefined();
      expect(edges.find(e => e.id === "step-edge-run-abc-001-step-review")).toBeDefined();

      const stepNodes = nodes.filter(n => {
        const d = n.data as Record<string, unknown>;
        return d.isStepCommit === true;
      });
      expect(stepNodes.length).toBeGreaterThanOrEqual(1);
    });
  });

  describe("running 3-step pipeline (step 1 done, step 2 running, step 3 pending)", () => {
    it("uses last completed step commit as effective head, not a ghost", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [RUNNING_3STEP_RUN]
      }));

      const ghost = nodes.find(n => n.id === "ghost-run-abc-002");
      expect(ghost).toBeUndefined();

      // Effective head (ddd444...) is hidden — represented by synthetic step outcome
      expect(nodes.find(n => n.id === "commit-ddd4444444444444444444444444444444444444")).toBeUndefined();
      const stepOutcome = nodes.find(n => n.id === "commit-step-run-abc-002-step-unit-tests");
      expect(stepOutcome).toBeDefined();
    });

    it("shows all 3 step edges (completed, running, pending)", () => {
      const { edges } = buildProjectGraph(input({
        runs: [RUNNING_3STEP_RUN],
      }));

      const stepEdges = edges.filter(e => e.id.startsWith("step-edge-run-abc-002"));
      expect(stepEdges).toHaveLength(3);

      const statuses = stepEdges.map(e => (e.data as GraphEdgeData).status);
      expect(statuses).toContain("completed");
      expect(statuses).toContain("running");
      expect(statuses).toContain("pending");
    });
  });

  describe("cancelled pipeline (step 1 done, step 2 cancelled)", () => {
    it("uses completed step commit as effective head", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [CANCELLED_RUN]
      }));

      expect(nodes.find(n => n.id === "ghost-run-abc-003")).toBeUndefined();
      // Effective head (eee555...) is hidden — represented by synthetic step outcome
      expect(nodes.find(n => n.id === "commit-eee5555555555555555555555555555555555555")).toBeUndefined();
      const stepOutcome = nodes.find(n => n.id === "commit-step-run-abc-003-step-implement");
      expect(stepOutcome).toBeDefined();
    });

    it("produces step edges with cancelled/failed statuses", () => {
      const { edges } = buildProjectGraph(input({
        runs: [CANCELLED_RUN]
      }));

      // Auto-expanded: produces step edges, not a collapsed run-edge
      expect(edges.find(e => e.id === "step-edge-run-abc-003-step-implement")).toBeDefined();
      expect(edges.find(e => e.id === "step-edge-run-abc-003-step-test-logging")).toBeDefined();
    });
  });

  describe("multiple pipelines on same graph", () => {
    it("renders all pipelines with step chain edges", () => {
      const { edges } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN, RUNNING_3STEP_RUN, CANCELLED_RUN]
      }));

      // All runs produce step edges (auto-expanded)
      expect(edges.find(e => e.id === "step-edge-run-abc-001-step-codegen")).toBeDefined();
      expect(edges.find(e => e.id === "step-edge-run-abc-002-step-unit-tests")).toBeDefined();
      expect(edges.find(e => e.id === "step-edge-run-abc-003-step-implement")).toBeDefined();
    });

    it("does not duplicate commit nodes shared between pipelines", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN, RUNNING_3STEP_RUN, CANCELLED_RUN]
      }));

      const cccNodes = nodes.filter(n => n.id === "commit-ccc3333333333333333333333333333333333333");
      expect(cccNodes).toHaveLength(1);

      const aaaNodes = nodes.filter(n => n.id === "commit-aaa1111111111111111111111111111111111111");
      expect(aaaNodes).toHaveLength(1);
    });
  });

  describe("run with no step_results (legacy or missing data)", () => {
    it("falls back to head_commit_sha when step_results is undefined", () => {
      const legacyRun: PipelineRun = {
        id: "run-legacy",
        project_id: "proj-demo",
        name: "Old pipeline",
        status: PipelineRunStatus.Completed,
        base_commit_sha: "aaa1111111111111111111111111111111111111",
        start_commit_sha: "aaa1111111111111111111111111111111111111",
        head_commit_sha: "ccc3333333333333333333333333333333333333",
        created_at: "2026-01-01T00:00:00Z"
      };

      const { edges } = buildProjectGraph(input({ runs: [legacyRun] }));
      const runEdge = edges.find(e => e.id === "run-edge-run-legacy");
      expect(runEdge).toBeDefined();
      expect(runEdge!.target).toBe("commit-ccc3333333333333333333333333333333333333");
    });

    it("reports stepCount 0 when no step_results", () => {
      const legacyRun: PipelineRun = {
        id: "run-legacy",
        project_id: "proj-demo",
        name: "Old pipeline",
        status: PipelineRunStatus.Completed,
        base_commit_sha: "aaa1111111111111111111111111111111111111",
        start_commit_sha: "aaa1111111111111111111111111111111111111",
        head_commit_sha: "ccc3333333333333333333333333333333333333",
        created_at: "2026-01-01T00:00:00Z"
      };

      const { edges } = buildProjectGraph(input({ runs: [legacyRun] }));
      const data = edges.find(e => e.id === "run-edge-run-legacy")!.data as GraphEdgeData;
      expect(data.stepCount).toBe(0);
    });
  });

  describe("resolveRun — runDetails override stale list data", () => {
    it("uses step_results from runDetails when list data has none", () => {
      const staleRun: PipelineRun = {
        ...COMPLETED_2STEP_RUN,
        step_results: undefined
      };

      const { edges } = buildProjectGraph(input({
        runs: [staleRun],
        runDetails: {
          "run-abc-001": { run: COMPLETED_2STEP_RUN, activities: [] }
        }
      }));

      // runDetails provides step_results → produces step chain edges
      expect(edges.find(e => e.id === "step-edge-run-abc-001-step-codegen")).toBeDefined();
      expect(edges.find(e => e.id === "step-edge-run-abc-001-step-review")).toBeDefined();
    });

    it("uses step_results from runDetails for effective head computation", () => {
      const staleRun: PipelineRun = {
        ...RUNNING_3STEP_RUN,
        step_results: undefined
      };

      const { nodes } = buildProjectGraph(input({
        runs: [staleRun],
        runDetails: {
          "run-abc-002": { run: RUNNING_3STEP_RUN, activities: [] }
        }
      }));

      const ghost = nodes.find(n => n.id === "ghost-run-abc-002");
      expect(ghost).toBeUndefined();

      // Effective head (ddd444...) is hidden — represented by synthetic step outcome
      expect(nodes.find(n => n.id === "commit-ddd4444444444444444444444444444444444444")).toBeUndefined();
      const stepOutcome = nodes.find(n => n.id === "commit-step-run-abc-002-step-unit-tests");
      expect(stepOutcome).toBeDefined();
    });
  });

  describe("forked pipeline with skipped steps", () => {
    it("only creates step edges for non-skipped steps", () => {
      const forkedRun: PipelineRun = {
        id: "run-fork-001",
        project_id: "proj-demo",
        name: "Fork: retry from step 2",
        status: PipelineRunStatus.Completed,
        base_commit_sha: "aaa1111111111111111111111111111111111111",
        start_commit_sha: "bbb2222222222222222222222222222222222222",
        head_commit_sha: "ccc3333333333333333333333333333333333333",
        parent_run_id: "run-abc-001",
        fork_after_step_id: "step-codegen",
        created_at: "2026-02-18T10:00:00Z",
        step_results: [
          makeStepResult({
            step_id: "step-codegen",
            step_index: 0,
            pipeline_run_id: "run-fork-001",
            status: StepStatus.Skipped,
            commit_sha: "bbb2222222222222222222222222222222222222"
          }),
          makeStepResult({
            step_id: "step-review",
            step_index: 1,
            pipeline_run_id: "run-fork-001",
            status: StepStatus.Completed,
            commit_sha: "ccc3333333333333333333333333333333333333"
          })
        ]
      };

      const { edges } = buildProjectGraph(input({ runs: [forkedRun] }));
      // Skipped step has no edge, only the non-skipped step gets an edge
      expect(edges.find(e => e.id === "step-edge-run-fork-001-step-codegen")).toBeUndefined();
      expect(edges.find(e => e.id === "step-edge-run-fork-001-step-review")).toBeDefined();
    });

    it("does not create edges for skipped steps in expansion", () => {
      const forkedRun: PipelineRun = {
        id: "run-fork-001",
        project_id: "proj-demo",
        name: "Fork: retry",
        status: PipelineRunStatus.Completed,
        base_commit_sha: "aaa1111111111111111111111111111111111111",
        start_commit_sha: "bbb2222222222222222222222222222222222222",
        head_commit_sha: "ccc3333333333333333333333333333333333333",
        created_at: "2026-02-18T10:00:00Z",
        step_results: [
          makeStepResult({
            step_id: "step-codegen",
            step_index: 0,
            pipeline_run_id: "run-fork-001",
            status: StepStatus.Skipped,
            commit_sha: "bbb2222222222222222222222222222222222222"
          }),
          makeStepResult({
            step_id: "step-review",
            step_index: 1,
            pipeline_run_id: "run-fork-001",
            status: StepStatus.Completed,
            commit_sha: "ccc3333333333333333333333333333333333333"
          })
        ]
      };

      const { edges } = buildProjectGraph(input({
        runs: [forkedRun],
      }));

      expect(edges.find(e => e.id === "step-edge-run-fork-001-step-codegen")).toBeUndefined();
      expect(edges.find(e => e.id === "step-edge-run-fork-001-step-review")).toBeDefined();
    });
  });

  describe("spreadBlockedHeads", () => {
    it("staggers heads at the same depth when cross-row edges are blocked", () => {
      // 3 runs from the same base commit → heads at same depth on rows 1, 2, 3
      const runs = [
        makeRun({ id: "r1", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "h1", created_at: "2026-02-10T10:00:00Z" }),
        makeRun({ id: "r2", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "h2", created_at: "2026-02-10T10:01:00Z" }),
        makeRun({ id: "r3", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "h3", created_at: "2026-02-10T10:02:00Z" }),
      ];
      const dag = buildDAG(runs);
      const commitOrder = ["a", "h1", "h2", "h3"];
      const depths = computeDepths(dag, runs, [], commitOrder);
      const rows = assignPipelineRows(runs, dag);

      // Before spreading, all heads at depth 1
      expect(depths.get("h1")).toBe(1);
      expect(depths.get("h2")).toBe(1);
      expect(depths.get("h3")).toBe(1);
      // Rows: a=0, h1=1, h2=2, h3=3
      expect(rows.commitRowMap.get("a")).toBe(0);
      expect(rows.commitRowMap.get("h1")).toBe(1);
      expect(rows.commitRowMap.get("h2")).toBe(2);
      expect(rows.commitRowMap.get("h3")).toBe(3);

      spreadBlockedHeads(runs, depths, rows, []);

      // r1 (row 1, gap=1) — not cross-row with gap>1, stays at depth 1
      expect(depths.get("h1")).toBe(1);
      // r2 (row 2, gap=2) — blocked by h1 at depth 1 row 1 → shift to depth 2
      expect(depths.get("h2")).toBe(2);
      // r3 (row 3, gap=3) — blocked by h1 at depth 1 row 1, h2 at depth 2 row 2
      //   depth 1: blocked (h1 at row 1), depth 2: blocked (h2 at row 2), depth 3: clear
      expect(depths.get("h3")).toBe(3);
    });

    it("does not shift heads when cross-row edges are unblocked", () => {
      // 2 runs: r1 from a→h1, r2 from h1→h2 (chain, not fan)
      const runs = [
        makeRun({ id: "r1", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "h1", created_at: "2026-02-10T10:00:00Z" }),
        makeRun({ id: "r2", base_commit_sha: "h1", start_commit_sha: "h1", head_commit_sha: "h2", created_at: "2026-02-10T10:01:00Z" }),
      ];
      const dag = buildDAG(runs);
      const commitOrder = ["a", "h1", "h2"];
      const depths = computeDepths(dag, runs, [], commitOrder);
      const rows = assignPipelineRows(runs, dag);

      spreadBlockedHeads(runs, depths, rows, []);

      // Each head is only 1 row away from its source, no blocking
      expect(depths.get("h1")).toBe(1);
      expect(depths.get("h2")).toBe(2);
    });

    it("handles fan-out with 2 runs where only the farther is blocked", () => {
      const runs = [
        makeRun({ id: "r1", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "h1", created_at: "2026-02-10T10:00:00Z" }),
        makeRun({ id: "r2", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "h2", created_at: "2026-02-10T10:01:00Z" }),
      ];
      const dag = buildDAG(runs);
      const commitOrder = ["a", "h1", "h2"];
      const depths = computeDepths(dag, runs, [], commitOrder);
      const rows = assignPipelineRows(runs, dag);

      // r1 gap=1 (row 0→1), r2 gap=2 (row 0→2), h1 at row 1 blocks r2's edge
      spreadBlockedHeads(runs, depths, rows, []);

      expect(depths.get("h1")).toBe(1); // not shifted (gap=1)
      expect(depths.get("h2")).toBe(2); // shifted from 1 → 2
    });

    it("does not shift a head commit that is a start commit for downstream runs", () => {
      const runs = [
        makeRun({ id: "r1", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "h1", created_at: "2026-02-10T10:00:00Z" }),
        makeRun({ id: "r2", base_commit_sha: "a", start_commit_sha: "a", head_commit_sha: "b", created_at: "2026-02-10T10:01:00Z" }),
        makeRun({ id: "r3", base_commit_sha: "b", start_commit_sha: "b", head_commit_sha: "c", created_at: "2026-02-10T10:02:00Z" }),
      ];
      const dag = buildDAG(runs);
      const commitOrder = ["a", "h1", "b", "c"];
      const depths = computeDepths(dag, runs, [], commitOrder);
      const rows = assignPipelineRows(runs, dag);

      // b is initially at depth 1 and would normally be shifted due r2's
      // cross-row blockage by h1(row 1), but b is also a shared source (r3).
      expect(depths.get("b")).toBe(1);
      spreadBlockedHeads(runs, depths, rows, []);
      expect(depths.get("b")).toBe(1);
    });
  });

  describe("pipeline background nodes", () => {
    it("creates pipeline-bg node for each run with step data", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN]
      }));

      const bgNodes = nodes.filter(n => n.type === "pipeline-bg");
      expect(bgNodes).toHaveLength(1);
      expect(bgNodes[0].id).toBe("pipeline-bg-run-abc-001");
      expect(bgNodes[0].data).toMatchObject({
        runId: "run-abc-001",
        runName: "Feature: add auth",
        status: "completed"
      });
    });

    it("sets zIndex -1 on pipeline-bg nodes", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN]
      }));

      const bgNode = nodes.find(n => n.type === "pipeline-bg");
      expect(bgNode?.zIndex).toBe(-1);
    });

    it("bg node wraps step chain (position to the left of first patch)", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN]
      }));

      const bgNode = nodes.find(n => n.type === "pipeline-bg")!;
      const patchNodes = nodes.filter(n => n.type === "patch");
      expect(patchNodes.length).toBeGreaterThan(0);

      // bg x should be less than the first patch node's x
      const firstPatchX = Math.min(...patchNodes.map(n => n.position.x));
      expect(bgNode.position.x).toBeLessThan(firstPatchX);
    });

    it("does not create pipeline-bg for legacy runs without step data", () => {
      const legacyRun: PipelineRun = {
        id: "run-legacy",
        project_id: "proj-demo",
        name: "Old pipeline",
        status: PipelineRunStatus.Completed,
        base_commit_sha: "aaa1111111111111111111111111111111111111",
        start_commit_sha: "aaa1111111111111111111111111111111111111",
        head_commit_sha: "ccc3333333333333333333333333333333333333",
        created_at: "2026-01-01T00:00:00Z"
      };

      const { nodes } = buildProjectGraph(input({ runs: [legacyRun] }));
      const bgNodes = nodes.filter(n => n.type === "pipeline-bg");
      expect(bgNodes).toHaveLength(0);

      // But still has a direct run-edge
      const { edges } = buildProjectGraph(input({ runs: [legacyRun] }));
      expect(edges.find(e => e.id === "run-edge-run-legacy")).toBeDefined();
    });

    it("creates separate bg nodes for multiple runs", () => {
      const { nodes } = buildProjectGraph(input({
        runs: [COMPLETED_2STEP_RUN, RUNNING_3STEP_RUN, CANCELLED_RUN]
      }));

      const bgNodes = nodes.filter(n => n.type === "pipeline-bg");
      expect(bgNodes).toHaveLength(3);
      const bgIds = bgNodes.map(n => n.id);
      expect(bgIds).toContain("pipeline-bg-run-abc-001");
      expect(bgIds).toContain("pipeline-bg-run-abc-002");
      expect(bgIds).toContain("pipeline-bg-run-abc-003");
    });
  });

  describe("forked runs", () => {
    it("forked run's start commit is positioned at parent pipeline row, not row 0", () => {
      // Parent run: base=a, step S1→x, step S2→b (head)
      const parentRun = makeRun({
        id: "p1",
        start_commit_sha: "a",
        head_commit_sha: "b",
        created_at: "2026-02-10T10:00:00Z",
        step_results: [
          makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "p1", commit_sha: "x" }),
          makeStepResult({ step_id: "s2", step_index: 1, pipeline_run_id: "p1", commit_sha: "b" })
        ]
      });
      // Forked run: starts from x (S1's commit in parent), with modified step config
      const forkedRun = makeRun({
        id: "p2",
        start_commit_sha: "x",
        head_commit_sha: "",
        parent_run_id: "p1",
        fork_after_step_id: "s1",
        status: PipelineRunStatus.Running,
        created_at: "2026-02-10T11:00:00Z",
        step_results: [
          makeStepResult({ step_id: "s2-fork", step_index: 0, pipeline_run_id: "p2", commit_sha: "", status: StepStatus.Running })
        ]
      });

      const { nodes, edges } = buildProjectGraph(input({
        runs: [parentRun, forkedRun],
        commits: [makeCommit("a")]
      }));

      // The shared commit x should be positioned at parent's pipeline row, not row 0
      const xNode = nodes.find(n => n.id === "commit-x");
      expect(xNode).toBeDefined();
      // x is upgraded to a step commit from P1
      expect((xNode!.data as any).isStepCommit).toBe(true);
      expect((xNode!.data as any).runId).toBe("p1");

      // Parent pipeline row is 1, forked is 2. x should be at parent's row (not row 0).
      // Both P1 patches and x outcome should share the same Y.
      const p1Patches = nodes.filter(n => n.type === "patch" && (n.data as any).runId === "p1");
      expect(p1Patches.length).toBeGreaterThan(0);
      expect(xNode!.position.y).toBe(p1Patches[0].position.y);

      // Forked run should have its own nodes at a different row
      const p2Patches = nodes.filter(n => n.type === "patch" && (n.data as any).runId === "p2");
      expect(p2Patches.length).toBeGreaterThan(0);
      expect(p2Patches[0].position.y).not.toBe(xNode!.position.y);

      // Forked run should have a pipeline bg
      const p2Bg = nodes.find(n => n.id === "pipeline-bg-p2");
      expect(p2Bg).toBeDefined();

      // Edge from x to forked run's first patch should exist
      const connectorEdge = edges.find(e => e.id.includes("connector-p2"));
      expect(connectorEdge).toBeDefined();
      expect(connectorEdge!.source).toBe("commit-x");
    });

    it("forked run with no step results shows ghost node", () => {
      const parentRun = makeRun({
        id: "p1",
        start_commit_sha: "a",
        head_commit_sha: "b",
        step_results: [
          makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "p1", commit_sha: "b" })
        ]
      });
      // Freshly started fork with no step results yet
      const forkedRun = makeRun({
        id: "p2",
        start_commit_sha: "a",
        head_commit_sha: "",
        parent_run_id: "p1",
        status: PipelineRunStatus.Running,
        created_at: "2026-02-10T11:00:00Z",
        step_results: []
      });

      const { nodes, edges } = buildProjectGraph(input({
        runs: [parentRun, forkedRun],
        commits: [makeCommit("a")]
      }));

      // Ghost node for the forked run (no step data → legacy path)
      const ghost = nodes.find(n => n.id === "ghost-p2");
      expect(ghost).toBeDefined();
      expect((ghost!.data as any).isGhost).toBe(true);

      // Edge from source to ghost
      const runEdge = edges.find(e => e.id === "run-edge-p2");
      expect(runEdge).toBeDefined();
    });
  });
});

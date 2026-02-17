import { describe, expect, it } from "vitest";

import { buildProjectGraph, type GraphInput } from "./graph-layout";
import type { PipelineRun, StepResult } from "./types";
import { PipelineRunStatus, StepStatus } from "./types";

function makeRun(overrides: Partial<PipelineRun> & { id: string }): PipelineRun {
  return {
    project_id: "proj-1",
    name: "Pipeline",
    status: PipelineRunStatus.Completed,
    base_commit_sha: "abc123",
    start_commit_sha: "abc123",
    head_commit_sha: "def456",
    created_at: "2026-02-10T10:00:00Z",
    ...overrides
  };
}

function makeInput(overrides: Partial<GraphInput> = {}): GraphInput {
  return {
    runs: [],
    expandedRunIds: new Set(),
    expandedRunData: {},
    selectedStep: null,
    ...overrides
  };
}

function makeStepResult(overrides: Partial<StepResult> & { step_id: string; step_index: number }): StepResult {
  return {
    id: `sr-${overrides.step_id}`,
    pipeline_run_id: "run-1",
    step_name: overrides.step_id,
    status: StepStatus.Completed,
    commit_sha: `sha-after-${overrides.step_id}`,
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

describe("buildProjectGraph", () => {
  it("returns empty nodes and edges for empty runs", () => {
    const { nodes, edges } = buildProjectGraph(makeInput());
    expect(nodes).toEqual([]);
    expect(edges).toEqual([]);
  });

  it("collapsed completed run: start_commit → run → end_commit", () => {
    const run = makeRun({ id: "r1", start_commit_sha: "aaa", head_commit_sha: "bbb" });
    const { nodes, edges } = buildProjectGraph(makeInput({ runs: [run] }));

    expect(nodes).toHaveLength(3);
    expect(nodes.filter((n) => n.type === "commit")).toHaveLength(2);
    expect(nodes.filter((n) => n.type === "run")).toHaveLength(1);

    expect(edges).toHaveLength(2);
    expect(edges[0]).toMatchObject({ source: "commit-aaa", target: "run-r1" });
    expect(edges[1]).toMatchObject({ source: "run-r1", target: "commit-bbb" });
  });

  it("collapsed running run: start_commit → run (no end commit)", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "aaa",
      head_commit_sha: "",
      status: PipelineRunStatus.Running
    });
    const { nodes, edges } = buildProjectGraph(makeInput({ runs: [run] }));

    expect(nodes).toHaveLength(2); // start commit + run
    expect(nodes.filter((n) => n.type === "commit")).toHaveLength(1);
    expect(edges).toHaveLength(1);
    expect(edges[0].animated).toBe(true);
  });

  it("expanded run with 3 completed steps: alternating step/commit chain", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "aaa",
      head_commit_sha: "sha-after-s3",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sha-s1", pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s2", step_index: 1, commit_sha: "sha-s2", pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s3", step_index: 2, commit_sha: "sha-s3", pipeline_run_id: "r1" })
      ]
    });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        runs: [run],
        expandedRunIds: new Set(["r1"]),
        expandedRunData: { "r1": { run, activities: [] } }
      })
    );

    // start_commit(aaa) + 3 steps + 3 intermediate commits = 7 nodes
    expect(nodes.filter((n) => n.type === "commit")).toHaveLength(4);
    expect(nodes.filter((n) => n.type === "step")).toHaveLength(3);
    expect(nodes).toHaveLength(7);

    // Chain: commit-aaa → step-s1 → commit-sha-s1 → step-s2 → commit-sha-s2 → step-s3 → commit-sha-s3
    expect(edges).toHaveLength(6);
    expect(edges[0]).toMatchObject({ source: "commit-aaa", target: "step-r1-s1" });
    expect(edges[1]).toMatchObject({ source: "step-r1-s1", target: "commit-sha-s1" });
    expect(edges[2]).toMatchObject({ source: "commit-sha-s1", target: "step-r1-s2" });
    expect(edges[3]).toMatchObject({ source: "step-r1-s2", target: "commit-sha-s2" });
    expect(edges[4]).toMatchObject({ source: "commit-sha-s2", target: "step-r1-s3" });
    expect(edges[5]).toMatchObject({ source: "step-r1-s3", target: "commit-sha-s3" });
  });

  it("expanded run: skipped steps produce no nodes", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "aaa",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, status: StepStatus.Skipped, commit_sha: "", pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s2", step_index: 1, commit_sha: "sha-s2", pipeline_run_id: "r1" })
      ]
    });
    const { nodes } = buildProjectGraph(
      makeInput({
        runs: [run],
        expandedRunIds: new Set(["r1"]),
        expandedRunData: { "r1": { run, activities: [] } }
      })
    );

    const stepNodes = nodes.filter((n) => n.type === "step");
    expect(stepNodes).toHaveLength(1);
    expect(stepNodes[0].id).toBe("step-r1-s2");
  });

  it("two runs from same start_commit share one commit node", () => {
    const runs = [
      makeRun({ id: "r1", start_commit_sha: "aaa", head_commit_sha: "bbb" }),
      makeRun({ id: "r2", start_commit_sha: "aaa", head_commit_sha: "ccc", created_at: "2026-02-10T11:00:00Z" })
    ];
    const { nodes } = buildProjectGraph(makeInput({ runs }));

    const aaaNodes = nodes.filter((n) => n.id === "commit-aaa");
    expect(aaaNodes).toHaveLength(1); // deduplicated

    // r2 is below r1 vertically
    const r1Node = nodes.find((n) => n.id === "run-r1")!;
    const r2Node = nodes.find((n) => n.id === "run-r2")!;
    expect(r2Node.position.y).toBeGreaterThan(r1Node.position.y);
  });

  it("collapsed parent: fork child branches from parent run node via child's start_commit", () => {
    const parent = makeRun({ id: "p1", start_commit_sha: "aaa", head_commit_sha: "ppp" });
    const child = makeRun({
      id: "c1",
      start_commit_sha: "intermediate-sha",
      head_commit_sha: "ccc",
      parent_run_id: "p1",
      created_at: "2026-02-10T11:00:00Z"
    });
    const { nodes, edges } = buildProjectGraph(makeInput({ runs: [parent, child] }));

    // Parent is collapsed → intermediate-sha was not placed by parent's chain.
    // Fork edge: run-p1 → commit-intermediate-sha
    const forkEdge = edges.find((e) => e.target === "commit-intermediate-sha");
    expect(forkEdge).toBeDefined();
    expect(forkEdge!.source).toBe("run-p1");

    // Child chain continues: commit-intermediate-sha → run-c1 → commit-ccc
    const childRunEdge = edges.find((e) => e.source === "commit-intermediate-sha" && e.target === "run-c1");
    expect(childRunEdge).toBeDefined();

    // Child is below parent vertically
    const parentNode = nodes.find((n) => n.id === "run-p1")!;
    const childNode = nodes.find((n) => n.id === "run-c1")!;
    expect(childNode.position.y).toBeGreaterThan(parentNode.position.y);
  });

  it("expanded parent: fork child branches from intermediate commit node", () => {
    const parent = makeRun({
      id: "p1",
      start_commit_sha: "aaa",
      head_commit_sha: "sha-s3",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sha-s1", pipeline_run_id: "p1" }),
        makeStepResult({ step_id: "s2", step_index: 1, commit_sha: "sha-s2", pipeline_run_id: "p1" }),
        makeStepResult({ step_id: "s3", step_index: 2, commit_sha: "sha-s3", pipeline_run_id: "p1" })
      ]
    });
    const child = makeRun({
      id: "c1",
      start_commit_sha: "sha-s2",
      head_commit_sha: "child-end",
      parent_run_id: "p1",
      created_at: "2026-02-10T11:00:00Z"
    });
    const { nodes, edges } = buildProjectGraph(
      makeInput({
        runs: [parent, child],
        expandedRunIds: new Set(["p1"]),
        expandedRunData: { "p1": { run: parent, activities: [] } }
      })
    );

    // commit-sha-s2 was placed by parent's expansion.
    // Child reuses it (deduped), so no self-edge.
    // Child chain: commit-sha-s2 → run-c1 → commit-child-end
    const childRunEdge = edges.find((e) => e.source === "commit-sha-s2" && e.target === "run-c1");
    expect(childRunEdge).toBeDefined();

    // commit-sha-s2 also connects to step-p1-s3 in the parent chain
    const parentContinuation = edges.find((e) => e.source === "commit-sha-s2" && e.target === "step-p1-s3");
    expect(parentContinuation).toBeDefined();

    // Only one commit-sha-s2 node (deduplicated)
    const shaS2Nodes = nodes.filter((n) => n.id === "commit-sha-s2");
    expect(shaS2Nodes).toHaveLength(1);
  });

  it("multiple forks from different intermediate commits of same parent", () => {
    const parent = makeRun({
      id: "p1",
      start_commit_sha: "aaa",
      head_commit_sha: "sha-s3",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sha-s1", pipeline_run_id: "p1" }),
        makeStepResult({ step_id: "s2", step_index: 1, commit_sha: "sha-s2", pipeline_run_id: "p1" }),
        makeStepResult({ step_id: "s3", step_index: 2, commit_sha: "sha-s3", pipeline_run_id: "p1" })
      ]
    });
    const child1 = makeRun({
      id: "c1",
      start_commit_sha: "sha-s1",
      head_commit_sha: "c1-end",
      parent_run_id: "p1",
      created_at: "2026-02-10T11:00:00Z"
    });
    const child2 = makeRun({
      id: "c2",
      start_commit_sha: "sha-s2",
      head_commit_sha: "c2-end",
      parent_run_id: "p1",
      created_at: "2026-02-10T12:00:00Z"
    });
    const { nodes } = buildProjectGraph(
      makeInput({
        runs: [parent, child1, child2],
        expandedRunIds: new Set(["p1"]),
        expandedRunData: { "p1": { run: parent, activities: [] } }
      })
    );

    const c1RunNode = nodes.find((n) => n.id === "run-c1")!;
    const c2RunNode = nodes.find((n) => n.id === "run-c2")!;
    const parentStepNode = nodes.find((n) => n.id === "step-p1-s1")!;

    // Both children are below the parent's lane
    expect(c1RunNode.position.y).toBeGreaterThan(parentStepNode.position.y);
    expect(c2RunNode.position.y).toBeGreaterThan(parentStepNode.position.y);
    // c2 is further down than c1
    expect(c2RunNode.position.y).toBeGreaterThan(c1RunNode.position.y);
  });

  it("orphaned child (parent not in dataset) is still laid out", () => {
    const orphan = makeRun({
      id: "c1",
      start_commit_sha: "orphan-start",
      head_commit_sha: "orphan-end",
      parent_run_id: "missing-parent"
    });
    const { nodes } = buildProjectGraph(makeInput({ runs: [orphan] }));

    const runNode = nodes.find((n) => n.id === "run-c1");
    expect(runNode).toBeDefined();
    const commitNodes = nodes.filter((n) => n.type === "commit");
    expect(commitNodes).toHaveLength(2); // start + end
  });

  it("marks the matching step node as selected when both selected step and run match", () => {
    const run = makeRun({
      id: "r1",
      start_commit_sha: "aaa",
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, commit_sha: "sha-s1", pipeline_run_id: "r1" }),
        makeStepResult({ step_id: "s2", step_index: 1, commit_sha: "sha-s2", pipeline_run_id: "r1" })
      ]
    });
    const { nodes } = buildProjectGraph(
      makeInput({
        runs: [run],
        expandedRunIds: new Set(["r1"]),
        expandedRunData: { "r1": { run, activities: [] } },
        selectedStep: { stepId: "s2", runId: "r1" }
      })
    );

    const s1 = nodes.find((n) => n.id === "step-r1-s1");
    const s2 = nodes.find((n) => n.id === "step-r1-s2");
    expect(s1?.selected).toBeFalsy();
    expect(s2?.selected).toBe(true);
  });

  it("does not select steps from other runs when step IDs overlap", () => {
    const runA = makeRun({
      id: "r1",
      start_commit_sha: "aaa",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r1" })]
    });
    const runB = makeRun({
      id: "r2",
      start_commit_sha: "bbb",
      created_at: "2026-02-10T11:00:00Z",
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, pipeline_run_id: "r2" })]
    });
    const { nodes } = buildProjectGraph(
      makeInput({
        runs: [runA, runB],
        expandedRunIds: new Set(["r1", "r2"]),
        expandedRunData: {
          "r1": { run: runA, activities: [] },
          "r2": { run: runB, activities: [] }
        },
        selectedStep: { stepId: "s1", runId: "r2" }
      })
    );

    const r1Step = nodes.find((n) => n.id === "step-r1-s1");
    const r2Step = nodes.find((n) => n.id === "step-r2-s1");
    expect(r1Step?.selected).toBeFalsy();
    expect(r2Step?.selected).toBe(true);
  });
});

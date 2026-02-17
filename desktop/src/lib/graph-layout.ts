import type { Edge, Node } from "@xyflow/react";

import type { PipelineRun, AIActivityRecord } from "./types";
import { stepStatusToView, StepStatus, PipelineRunStatus } from "./types";
import { summarizeStepObservability } from "./obs-mapping";
import type { StepNodeData } from "../components/nodes/StepNode";
import type { CommitNodeData } from "../components/nodes/CommitNode";
import type { RunNodeData } from "../components/nodes/RunNode";

// Layout constants
const COMMIT_NODE_W = 140;
const RUN_NODE_W = 260;
const STEP_NODE_W = 260;
const H_GAP = 40;

// Estimated node heights (used for vertical spacing)
const STEP_NODE_H = 185;
const RUN_NODE_H = 120;
const COMMIT_NODE_H = 55;
const V_PAD = 40;

export type GraphInput = {
  runs: PipelineRun[];
  expandedRunIds: Set<string>;
  expandedRunData: Record<string, { run: PipelineRun; activities: AIActivityRecord[] }>;
  selectedStep: { stepId: string; runId: string | null } | null;
};

function runStatusLabel(status: PipelineRun["status"]): string {
  switch (status) {
    case PipelineRunStatus.Pending:
      return "pending";
    case PipelineRunStatus.Running:
      return "running";
    case PipelineRunStatus.Completed:
      return "completed";
    case PipelineRunStatus.Failed:
      return "failed";
    default:
      return "pending";
  }
}

function startCommitOf(run: PipelineRun): string {
  return run.start_commit_sha || run.base_commit_sha || "unknown";
}

export function buildProjectGraph(input: GraphInput): { nodes: Node[]; edges: Edge[] } {
  const { runs, expandedRunIds, expandedRunData, selectedStep } = input;
  const nodes: Node[] = [];
  const edges: Edge[] = [];

  if (runs.length === 0) {
    return { nodes, edges };
  }

  // --- Data structures ---
  const childrenByParent = new Map<string, PipelineRun[]>();
  const placedCommits = new Map<string, string>(); // sha -> nodeId
  const nodeXPositions = new Map<string, number>(); // nodeId -> x
  const laidOutRunIds = new Set<string>();

  for (const run of runs) {
    if (run.parent_run_id) {
      const siblings = childrenByParent.get(run.parent_run_id) ?? [];
      siblings.push(run);
      childrenByParent.set(run.parent_run_id, siblings);
    }
  }

  // --- Helper: place a commit node (deduped by SHA) ---
  function placeCommit(sha: string, x: number, y: number, label?: string): string {
    const existing = placedCommits.get(sha);
    if (existing) return existing;

    const nodeId = `commit-${sha}`;
    nodes.push({
      id: nodeId,
      type: "commit",
      position: { x, y },
      data: { sha, label } satisfies CommitNodeData,
      draggable: false
    });
    placedCommits.set(sha, nodeId);
    nodeXPositions.set(nodeId, x);
    return nodeId;
  }

  // --- Helper: place an edge ---
  function placeEdge(source: string, target: string, animated = false): void {
    edges.push({
      id: `edge-${source}-${target}`,
      source,
      target,
      animated
    });
  }

  // --- Core: layout a single run chain and its fork children ---
  // Returns the bottom edge (lowest pixel Y occupied) by this chain and its descendants.
  function layoutRunChain(run: PipelineRun, startX: number, laneY: number, incomingNodeId: string | null): number {
    if (laidOutRunIds.has(run.id)) return laneY; // guard: cycle or already processed
    laidOutRunIds.add(run.id);

    const isExpanded = expandedRunIds.has(run.id);
    const status = runStatusLabel(run.status);
    const startSha = startCommitOf(run);
    let cursorX = startX;

    // Track the tallest node placed on this lane
    let laneMaxH = COMMIT_NODE_H;

    // 1. Start commit node (deduped)
    const startCommitNodeId = placeCommit(startSha, cursorX, laneY);

    // Edge from incoming node to this chain's start commit (skip if same node — avoids self-edges)
    if (incomingNodeId && incomingNodeId !== startCommitNodeId) {
      placeEdge(incomingNodeId, startCommitNodeId);
    }
    cursorX += COMMIT_NODE_W + H_GAP;

    // Tracks the last node in this chain (for connecting fork children when collapsed)
    let lastNodeId = startCommitNodeId;

    if (!isExpanded) {
      // --- COLLAPSED: start_commit → run_node → end_commit ---
      const runNodeId = `run-${run.id}`;
      nodes.push({
        id: runNodeId,
        type: "run",
        position: { x: cursorX, y: laneY },
        data: {
          runId: run.id,
          name: run.name,
          status,
          createdAt: run.created_at,
          isExpanded: false,
          errorMessage: run.error_message
        } satisfies RunNodeData,
        draggable: false
      });
      placeEdge(startCommitNodeId, runNodeId, status === "running");
      cursorX += RUN_NODE_W + H_GAP;
      lastNodeId = runNodeId;
      laneMaxH = Math.max(laneMaxH, RUN_NODE_H);

      // End commit (only if run completed with a head_commit_sha)
      if (run.head_commit_sha) {
        const endCommitNodeId = placeCommit(run.head_commit_sha, cursorX, laneY);
        placeEdge(runNodeId, endCommitNodeId);
        cursorX += COMMIT_NODE_W + H_GAP;
        lastNodeId = endCommitNodeId;
      }
    } else {
      // --- EXPANDED: start_commit → step → commit → step → commit → ... ---
      const expandedData = expandedRunData[run.id];
      const stepResults = expandedData?.run.step_results ?? run.step_results ?? [];
      const activities = expandedData?.activities ?? [];

      for (const sr of stepResults) {
        // Skip steps that didn't produce new state
        if (sr.status === StepStatus.Skipped || sr.status === StepStatus.Pending) {
          continue;
        }

        // Place step node
        const stepNodeId = `step-${run.id}-${sr.step_id}`;
        const stepEvents = activities.filter((a) => a.step_id === sr.step_id);
        const summary = summarizeStepObservability(stepEvents);
        const stepStatus = stepStatusToView(sr.status);

        nodes.push({
          id: stepNodeId,
          type: "step",
          position: { x: cursorX, y: laneY },
          data: {
            runId: run.id,
            stepId: sr.step_id,
            stepName: sr.step_name || sr.step_id,
            index: sr.step_index,
            status: stepStatus,
            inputTokens: sr.input_tokens || summary.inputTokens,
            outputTokens: sr.output_tokens || summary.outputTokens,
            filesChanged: sr.files_changed ?? 0,
            insertions: sr.insertions ?? 0,
            deletions: sr.deletions ?? 0,
            eventCount: summary.eventCount,
            toolUseCount: summary.toolUseCount,
            errorMessage: sr.error_message
          } satisfies StepNodeData,
          selected: selectedStep?.stepId === sr.step_id && selectedStep.runId === run.id,
          draggable: false
        });
        placeEdge(lastNodeId, stepNodeId, stepStatus === "running");
        cursorX += STEP_NODE_W + H_GAP;
        lastNodeId = stepNodeId;
        laneMaxH = Math.max(laneMaxH, STEP_NODE_H);

        // Place commit node after completed step (if it produced a commit)
        if (sr.commit_sha && sr.status === StepStatus.Completed) {
          const commitNodeId = placeCommit(sr.commit_sha, cursorX, laneY);
          placeEdge(stepNodeId, commitNodeId);
          cursorX += COMMIT_NODE_W + H_GAP;
          lastNodeId = commitNodeId;
        }
      }
    }

    // Bottom edge of this lane's own nodes
    let maxBottom = laneY + laneMaxH;

    // --- Layout fork children below this chain ---
    const children = childrenByParent.get(run.id) ?? [];
    let childY = maxBottom + V_PAD;

    for (const child of children) {
      // Determine where the fork branches from:
      // If the child's start_commit is already placed (expanded parent), fork from that commit.
      // Otherwise (collapsed parent), fork from the parent's run node.
      const childStartSha = startCommitOf(child);
      let forkSourceNodeId: string;
      let forkX: number;

      if (placedCommits.has(childStartSha)) {
        // Expanded parent: the intermediate commit is already in the chain
        forkSourceNodeId = placedCommits.get(childStartSha)!;
        forkX = nodeXPositions.get(forkSourceNodeId) ?? startX;
      } else {
        // Collapsed parent: fork from the run node
        forkSourceNodeId = `run-${run.id}`;
        forkX = startX;
      }

      const childBottom = layoutRunChain(child, forkX, childY, forkSourceNodeId);
      childY = childBottom + V_PAD;
      maxBottom = Math.max(maxBottom, childBottom);
    }

    return maxBottom;
  }

  // --- Group root runs by start commit, lay out each group ---
  const rootRuns = runs
    .filter((r) => !r.parent_run_id)
    .sort((a, b) => {
      const aTime = a.created_at ? Date.parse(a.created_at) : 0;
      const bTime = b.created_at ? Date.parse(b.created_at) : 0;
      return aTime - bTime;
    });

  const rootsByStartCommit = new Map<string, PipelineRun[]>();
  for (const run of rootRuns) {
    const key = startCommitOf(run);
    const group = rootsByStartCommit.get(key) ?? [];
    group.push(run);
    rootsByStartCommit.set(key, group);
  }

  let globalY = 0;

  for (const [, groupRuns] of rootsByStartCommit) {
    let laneY = globalY;

    for (const run of groupRuns) {
      const bottom = layoutRunChain(run, 0, laneY, null);
      laneY = bottom + V_PAD;
    }

    globalY = laneY;
  }

  // --- Fallback: lay out orphaned runs (parent not in dataset) ---
  const orphanedRuns = runs.filter((r) => !laidOutRunIds.has(r.id));
  for (const run of orphanedRuns) {
    const bottom = layoutRunChain(run, 0, globalY, null);
    globalY = bottom + V_PAD;
  }

  return { nodes, edges };
}

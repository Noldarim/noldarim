// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { Edge, Node } from "@xyflow/react";

import type { AgentConfigInput, AIActivityRecord, CommitInfo, PipelineRun, RunStepSnapshot, StepResult } from "./types";
import { PipelineRunStatus, StepStatus, stepStatusToView, type StepStatusView } from "./types";
import { summarizeStepObservability } from "./obs-mapping";
import { durationToMs } from "./duration";
import type { CommitNodeData, StepMetrics } from "../components/nodes/CommitNode";
import type { PatchNodeData } from "../components/nodes/PatchNode";
import type { PipelineBgNodeData } from "../components/nodes/PipelineBgNode";

const COLUMN_GAP = 340;
const ROW_GAP = 200;
const ORIGIN_X = 90;

// Pipeline background node padding
const BG_PADDING_X = 20;
const BG_PADDING_TOP = 36;
const BG_PADDING_BOTTOM = 16;
const PATCH_NODE_WIDTH = 190;
const OUTCOME_NODE_WIDTH = 240;

// Each expanded step occupies 2 depth columns: patch node + outcome node
const STEP_DEPTH_SPACING = 2;

// Edge style constants — centralized to avoid scattered magic color values
const EDGE_STYLES = {
  timeline:  { stroke: "#334155", strokeWidth: 1.6 },
  running:   { stroke: "#3ec6e0", strokeWidth: 2.6 },
  completed: { stroke: "#34d399", strokeWidth: 2.3 },
  failed:    { stroke: "#f87171", strokeWidth: 2.3 },
  cancelled: { stroke: "#fb923c", strokeWidth: 2.3, strokeDasharray: "8 4" },
  pending:   { stroke: "#94a3b8", strokeWidth: 2 },
  skipped:   { stroke: "#fbbf24", strokeWidth: 2, strokeDasharray: "4 4" },
  stale:     { stroke: "#475569", strokeWidth: 1, strokeDasharray: "4 4", opacity: 0.35 },
} as const;

export type GraphStatus = "pending" | "running" | "completed" | "failed" | "cancelled" | "skipped";

export type GraphEdgeMetrics = {
  tokens: number;
  durationMs: number;
  eventCount: number;
  toolUseCount: number;
  filesChanged: number;
  insertions: number;
  deletions: number;
};

export type GraphEdgeData = {
  kind: "timeline" | "run" | "step" | "connector";
  clickable: boolean;
  status: GraphStatus;
  highlighted?: boolean;
  stepCount?: number;
  runId?: string;
  runName?: string;
  stepId?: string;
  stepName?: string;
  stepIndex?: number;
  sourceCommitSha?: string;
  targetCommitSha?: string;
  metrics?: GraphEdgeMetrics;
  metricsLabel?: string;
  currentStepName?: string;
  currentStepIndex?: number;
  totalStepCount?: number;
  isFork?: boolean;
  reusedStepCount?: number;
  isStale?: boolean;
};

export type GraphInput = {
  runs: PipelineRun[];
  commits: CommitInfo[];
  runDetails: Record<string, { run: PipelineRun; activities: AIActivityRecord[] }>;
  highlightedRunId: string | null;
  selectedStep: { runId: string; stepId: string } | null;
  selectedBaseCommitSha: string | null;
};

// ---------------------------------------------------------------------------
// Pure helper functions (unchanged)
// ---------------------------------------------------------------------------

export function runStatusLabel(run: PipelineRun): GraphStatus {
  switch (run.status) {
    case PipelineRunStatus.Pending:
      return "pending";
    case PipelineRunStatus.Running:
      return "running";
    case PipelineRunStatus.Completed:
      return "completed";
    case PipelineRunStatus.Failed:
      if (isCancelledError(run.error_message)) {
        return "cancelled";
      }
      return "failed";
    default:
      return "pending";
  }
}

export function isCancelledError(errorMessage?: string): boolean {
  if (!errorMessage) return false;
  const normalized = errorMessage.toLowerCase();
  return /\bcancelled\b|\bcanceled\b|\bcancellation\b/.test(normalized);
}

function edgeStyleForStatus(status: GraphStatus, kind: GraphEdgeData["kind"]): Edge["style"] {
  if (kind === "timeline") return EDGE_STYLES.timeline;
  switch (status) {
    case "running":   return EDGE_STYLES.running;
    case "completed": return EDGE_STYLES.completed;
    case "failed":    return EDGE_STYLES.failed;
    case "cancelled": return EDGE_STYLES.cancelled;
    case "pending":   return EDGE_STYLES.pending;
    case "skipped":   return EDGE_STYLES.skipped;
    default:          return undefined;
  }
}

export function startCommitOf(run: PipelineRun): string {
  return run.start_commit_sha || run.base_commit_sha || "unknown";
}

export function effectiveHeadCommitSha(run: PipelineRun): string {
  if (run.head_commit_sha) return run.head_commit_sha;
  const completed = (run.step_results ?? [])
    .filter(sr => sr.status === StepStatus.Completed && sr.commit_sha)
    .sort((a, b) => a.step_index - b.step_index);
  return completed.length > 0 ? completed[completed.length - 1].commit_sha : "";
}

function metricsFromStepResult(sr: StepResult): Pick<GraphEdgeMetrics, "tokens" | "durationMs" | "filesChanged" | "insertions" | "deletions"> {
  return {
    tokens: (sr.input_tokens ?? 0) + (sr.output_tokens ?? 0),
    durationMs: durationToMs(sr.duration),
    filesChanged: sr.files_changed ?? 0,
    insertions: sr.insertions ?? 0,
    deletions: sr.deletions ?? 0
  };
}

function summarizeRunMetrics(run: PipelineRun, activities: AIActivityRecord[]): GraphEdgeMetrics {
  const stepResults = run.step_results ?? [];
  const fallbackObs = summarizeStepObservability(activities);

  let tokens = 0;
  let durationMs = 0;
  let filesChanged = 0;
  let insertions = 0;
  let deletions = 0;
  for (const sr of stepResults) {
    const m = metricsFromStepResult(sr);
    tokens += m.tokens;
    durationMs += m.durationMs;
    filesChanged += m.filesChanged;
    insertions += m.insertions;
    deletions += m.deletions;
  }

  return {
    tokens: tokens || fallbackObs.inputTokens + fallbackObs.outputTokens,
    durationMs,
    eventCount: activities.length,
    toolUseCount: fallbackObs.toolUseCount,
    filesChanged,
    insertions,
    deletions
  };
}

function summarizeStepMetrics(step: StepResult, stepEvents: AIActivityRecord[]): GraphEdgeMetrics {
  const obs = summarizeStepObservability(stepEvents);
  const m = metricsFromStepResult(step);
  return {
    tokens: m.tokens || obs.inputTokens + obs.outputTokens,
    durationMs: m.durationMs,
    eventCount: stepEvents.length,
    toolUseCount: obs.toolUseCount,
    filesChanged: m.filesChanged,
    insertions: m.insertions,
    deletions: m.deletions
  };
}

export function commitSummaryFromRun(run: PipelineRun): { diffSummary?: string; summaryLine?: string } {
  const steps = [...(run.step_results ?? [])]
    .filter((sr) => sr.status === StepStatus.Completed)
    .sort((a, b) => a.step_index - b.step_index);

  if (steps.length === 0) {
    return {};
  }

  const filesChanged = steps.reduce((sum, sr) => sum + (sr.files_changed ?? 0), 0);
  const insertions = steps.reduce((sum, sr) => sum + (sr.insertions ?? 0), 0);
  const deletions = steps.reduce((sum, sr) => sum + (sr.deletions ?? 0), 0);
  const lastStep = steps[steps.length - 1];
  const summaryLine = (lastStep.commit_message || "").split("\n")[0]?.trim() || undefined;

  return {
    diffSummary: `Δ ${filesChanged} / +${insertions} -${deletions}`,
    summaryLine
  };
}

/**
 * Compute the outcome key for a step result.
 * When step SHA matches head SHA and head is NOT shared with another run,
 * use a synthetic key. When head IS shared (chained pipeline), use real SHA
 * so the commit serves as both this run's outcome and the next run's source.
 */
export function stepOutcomeKey(
  sr: StepResult,
  headSha: string | null | undefined,
  runId: string,
  startShas: Set<string>
): string {
  const matchesHead = !!(sr.commit_sha && headSha && sr.commit_sha === headSha);
  const headIsShared = matchesHead && startShas.has(headSha!);
  return (!sr.commit_sha || (matchesHead && !headIsShared))
    ? `step-${runId}-${sr.step_id}`
    : sr.commit_sha;
}

/**
 * Compute the set of head commit SHAs that should be hidden because their
 * runs have step data (the step chain subsumes the head commit node).
 * Heads that also serve as start commits for other runs are NOT hidden.
 */
export function getHiddenHeadShas(
  runsById: Map<string, PipelineRun>,
  expandedRunIds: string[],
  startShas: Set<string>,
  expandedStepsMap?: Map<string, StepResult[]>
): Set<string> {
  const hiddenHeadShas = new Set<string>();
  for (const runId of expandedRunIds) {
    const run = runsById.get(runId);
    if (!run) continue;
    const headSha = effectiveHeadCommitSha(run);
    if (!headSha) continue;
    const steps = expandedStepsMap?.get(runId) ?? (run.step_results ?? [])
      .filter(sr => sr.status !== StepStatus.Skipped);
    if (steps.length > 0 && !startShas.has(headSha)) {
      hiddenHeadShas.add(headSha);
    }
  }
  return hiddenHeadShas;
}

// ---------------------------------------------------------------------------
// Decomposed layout functions
// ---------------------------------------------------------------------------

export type DAG = {
  dagOut: Map<string, Set<string>>;
  dagIn: Map<string, Set<string>>;
  dagCommits: Set<string>;
};

export type RowAssignment = {
  pipelineRowMap: Map<string, number>;
  commitRowMap: Map<string, number>;
  maxRow: number;
};

type CommitPlacement = {
  id: string;
  x: number;
  y: number;
  depth: number;
  row: number;
  data: CommitNodeData;
};

/**
 * Merge list-endpoint runs with runDetails for freshest step_results.
 * Returns sorted by created_at ascending.
 */
export function resolveRuns(
  runs: PipelineRun[],
  runDetails: Record<string, { run: PipelineRun; activities: AIActivityRecord[] }>
): PipelineRun[] {
  const runVersionTime = (run?: PipelineRun | null): number => {
    if (!run) return 0;
    const updated = run.updated_at ? Date.parse(run.updated_at) : Number.NaN;
    if (!Number.isNaN(updated)) return updated;
    const created = run.created_at ? Date.parse(run.created_at) : Number.NaN;
    return Number.isNaN(created) ? 0 : created;
  };

  function resolveRun(run: PipelineRun): PipelineRun {
    const details = runDetails[run.id];
    if (!details) return run;
    const detailsRun = details.run;

    const detailsTime = runVersionTime(detailsRun);
    const listTime = runVersionTime(run);
    if (detailsTime > listTime) {
      return { ...run, ...detailsRun };
    }
    if (detailsTime < listTime) {
      return run;
    }

    const detailsStepCount = detailsRun.step_results?.length ?? 0;
    const listStepCount = run.step_results?.length ?? 0;
    const detailsSnapshotCount = detailsRun.step_snapshots?.length ?? 0;
    const listSnapshotCount = run.step_snapshots?.length ?? 0;

    if (detailsStepCount >= listStepCount && detailsSnapshotCount >= listSnapshotCount) {
      return { ...run, ...detailsRun };
    }

    return run;
  }

  return [...runs].map(resolveRun).sort((a, b) => {
    const aTime = a.created_at ? Date.parse(a.created_at) : 0;
    const bTime = b.created_at ? Date.parse(b.created_at) : 0;
    return aTime - bTime;
  });
}

/**
 * Builds ordered list of all commit SHAs (git log + run-referenced SHAs).
 * Filters orphans — only commits in dagCommits survive (unless no runs exist).
 */
export function collectCommitShas(
  runs: PipelineRun[],
  commits: CommitInfo[],
  dagCommits: Set<string>
): string[] {
  const commitOrder: string[] = [];
  const seen = new Set<string>();

  for (const commit of commits) {
    if (commit.Hash && !seen.has(commit.Hash)) {
      seen.add(commit.Hash);
      commitOrder.push(commit.Hash);
    }
  }

  for (const run of runs) {
    const start = startCommitOf(run);
    if (start && !seen.has(start)) {
      seen.add(start);
      commitOrder.push(start);
    }
    const headSha = effectiveHeadCommitSha(run);
    if (headSha && !seen.has(headSha)) {
      seen.add(headSha);
      commitOrder.push(headSha);
    }
    for (const sr of run.step_results ?? []) {
      if (sr.commit_sha && !seen.has(sr.commit_sha)) {
        seen.add(sr.commit_sha);
        commitOrder.push(sr.commit_sha);
      }
    }
  }

  // Filter out git-only orphans; if no runs exist, keep only the head commit
  if (dagCommits.size > 0) {
    return commitOrder.filter((sha) => dagCommits.has(sha));
  } else if (commitOrder.length > 0) {
    return [commitOrder[0]];
  }
  return [];
}

/**
 * Creates adjacency maps from startCommitOf(run) → effectiveHeadCommitSha(run).
 */
export function buildDAG(runs: PipelineRun[]): DAG {
  const dagOut = new Map<string, Set<string>>();
  const dagIn = new Map<string, Set<string>>();
  const dagCommits = new Set<string>();

  for (const run of runs) {
    const src = startCommitOf(run);
    const tgt = effectiveHeadCommitSha(run);
    if (!tgt) {
      dagCommits.add(src);
      continue;
    }
    dagCommits.add(src);
    dagCommits.add(tgt);
    if (src === tgt) continue;
    if (!dagOut.has(src)) dagOut.set(src, new Set());
    dagOut.get(src)!.add(tgt);
    if (!dagIn.has(tgt)) dagIn.set(tgt, new Set());
    dagIn.get(tgt)!.add(src);
  }

  return { dagOut, dagIn, dagCommits };
}

/**
 * Extracts the commitSummaryBySha building loop.
 */
export function buildCommitSummaryMap(
  runs: PipelineRun[]
): Map<string, { diffSummary?: string; summaryLine?: string }> {
  const map = new Map<string, { diffSummary?: string; summaryLine?: string }>();
  for (const run of runs) {
    const headSha = effectiveHeadCommitSha(run);
    if (!headSha) continue;
    const summary = commitSummaryFromRun(run);
    if (summary.diffSummary || summary.summaryLine) {
      map.set(headSha, summary);
    }
  }
  return map;
}

/**
 * Compute depth via Kahn's algorithm.
 * If expandedRunIds is non-empty, shift depths to make room for step commits.
 * Processes parent runs first (topological order by parent_run_id).
 */
export function computeDepths(
  dag: DAG,
  runs: PipelineRun[],
  expandedRunIds: string[],
  commitOrder: string[],
  expandedStepsMap?: Map<string, StepResult[]>,
  precomputedStartShas?: Set<string>
): Map<string, number> {
  const { dagOut, dagIn, dagCommits } = dag;
  const depth = new Map<string, number>();
  const inDegree = new Map<string, number>();

  for (const sha of dagCommits) {
    depth.set(sha, 0);
    inDegree.set(sha, dagIn.get(sha)?.size ?? 0);
  }

  const queue: string[] = [];
  for (const sha of dagCommits) {
    if (inDegree.get(sha) === 0) queue.push(sha);
  }

  while (queue.length > 0) {
    const current = queue.shift()!;
    const targets = dagOut.get(current);
    if (!targets) continue;
    for (const tgt of targets) {
      const newDepth = depth.get(current)! + 1;
      if (newDepth > depth.get(tgt)!) {
        depth.set(tgt, newDepth);
      }
      const remaining = inDegree.get(tgt)! - 1;
      inDegree.set(tgt, remaining);
      if (remaining === 0) queue.push(tgt);
    }
  }

  // Assign depth 0 to any commit not yet covered
  for (const sha of commitOrder) {
    if (!depth.has(sha)) depth.set(sha, 0);
  }

  // Expansion: shift depths to make room for step commits.
  // Process parent runs before fork runs so fork steps land after shared nodes.
  if (expandedRunIds.length > 0) {
    const expandedSet = new Set(expandedRunIds);
    const runsById = new Map(runs.map(r => [r.id, r]));
    const startShas = precomputedStartShas ?? new Set(runs.map(r => startCommitOf(r)));

    // Topological order: parents first
    const sorted = [...expandedRunIds].sort((a, b) => {
      const ra = runsById.get(a);
      const rb = runsById.get(b);
      const aIsChild = ra?.parent_run_id && expandedSet.has(ra.parent_run_id) ? 1 : 0;
      const bIsChild = rb?.parent_run_id && expandedSet.has(rb.parent_run_id) ? 1 : 0;
      return aIsChild - bIsChild;
    });

    for (const expandedRunId of sorted) {
      const expandedRun = runsById.get(expandedRunId);
      if (!expandedRun) {
        console.warn(`expandedRunId "${expandedRunId}" not found in runs`);
        continue;
      }

      const src = startCommitOf(expandedRun);
      const headSha = effectiveHeadCommitSha(expandedRun);
      const srcDepth = depth.get(src) ?? 0;

      const nonSkippedSteps = expandedStepsMap?.get(expandedRunId) ?? (expandedRun.step_results ?? [])
        .filter(sr => sr.status !== StepStatus.Skipped)
        .sort((a, b) => a.step_index - b.step_index);
      const N = nonSkippedSteps.length;
      const headIsShared = !!(headSha && startShas.has(headSha));

      if (N > 0) {
        // Reserve extra columns only when the run head remains visible as a
        // shared start commit. Non-shared heads are hidden by the step chain,
        // so shifting the whole DAG would create artificial horizontal bloat.
        if (headIsShared) {
          const headDepth = depth.get(headSha!) ?? (srcDepth + 1);
          const neededGap = STEP_DEPTH_SPACING * N;
          const currentGap = Math.max(0, headDepth - srcDepth);

          if (neededGap > currentGap) {
            const shift = neededGap - currentGap;
            const insertionDepth = srcDepth + 1;
            for (const [sha, d] of depth) {
              // Insert columns strictly to the right of the source commit.
              // Shifting from headDepth can fail when headDepth < srcDepth.
              if (d >= insertionDepth) {
                depth.set(sha, d + shift);
              }
            }
          }

          const minHeadDepth = srcDepth + neededGap;
          const placedHeadDepth = depth.get(headSha!) ?? headDepth;
          if (placedHeadDepth < minHeadDepth) {
            depth.set(headSha!, minHeadDepth);
          }
        }

        for (let i = 0; i < N; i++) {
          const sr = nonSkippedSteps[i];
          const patchKey = `patch-${expandedRun.id}-${sr.step_id}`;
          depth.set(patchKey, srcDepth + 1 + STEP_DEPTH_SPACING * i);

          const stepKey = stepOutcomeKey(sr, headSha, expandedRun.id, startShas);
          depth.set(stepKey, srcDepth + STEP_DEPTH_SPACING + STEP_DEPTH_SPACING * i);
        }

        if (!headSha) {
          depth.set(`ghost-${expandedRun.id}`, srcDepth + STEP_DEPTH_SPACING * N + 1);
        }
      }
    }
  }

  return depth;
}

/**
 * Assign rows: each pipeline run gets a dedicated row.
 * Root commits sit at row 0 (bottom).
 */
export function assignPipelineRows(runs: PipelineRun[], dag: DAG): RowAssignment {
  const { dagIn, dagCommits } = dag;
  const pipelineRowMap = new Map<string, number>();
  const commitRowMap = new Map<string, number>();

  // Root commits = dagCommits that have no incoming edges
  for (const sha of dagCommits) {
    if (!dagIn.has(sha) || dagIn.get(sha)!.size === 0) {
      commitRowMap.set(sha, 0);
    }
  }

  let nextRow = 1;

  for (const run of runs) {
    pipelineRowMap.set(run.id, nextRow);

    const headSha = effectiveHeadCommitSha(run);
    if (headSha && !commitRowMap.has(headSha)) {
      commitRowMap.set(headSha, nextRow);
    }

    if (!headSha) {
      // Running pipeline with no completed steps — ghost gets pipeline row
      commitRowMap.set(`ghost-${run.id}`, nextRow);
    }

    nextRow++;
  }

  // Place step commits on their pipeline's row
  for (const run of runs) {
    const pipelineRow = pipelineRowMap.get(run.id)!;
    for (const sr of run.step_results ?? []) {
      if (sr.commit_sha && !commitRowMap.has(sr.commit_sha)) {
        commitRowMap.set(sr.commit_sha, pipelineRow);
      }
    }
  }

  const maxRow = Math.max(0, nextRow - 1);
  return { pipelineRowMap, commitRowMap, maxRow };
}

/**
 * Decide edge handle positions based on source/target row.
 * Same row → RIGHT→LEFT, cross-row → TOP→LEFT.
 */
export function edgeHandles(
  sourceRow: number,
  targetRow: number
): { sourceHandle: string; targetHandle: string } {
  if (sourceRow === targetRow) {
    return { sourceHandle: "run-source", targetHandle: "run-target" }; // RIGHT → LEFT
  }
  return { sourceHandle: "run-source-top", targetHandle: "run-target" }; // TOP → LEFT
}

/**
 * Spread head commits that sit at the same depth column across different rows.
 * When a cross-row edge from source to a distant head would visually pass
 * through intermediate-row nodes at the head's depth, shift that head right
 * to the next unblocked column. Processes closest heads first so they keep
 * their original position; farther heads cascade outward.
 *
 * Mutates `depths` in place.
 */
export function spreadBlockedHeads(
  runsSorted: PipelineRun[],
  depths: Map<string, number>,
  rows: RowAssignment,
  _expandedRunIds: string[]
): void {
  const { commitRowMap } = rows;

  // Build depth → Set<row> grid for commit nodes only.
  // Using expanded step/patch nodes here can over-constrain placement and
  // push heads arbitrarily far right in dense graphs.
  const depthRows = new Map<number, Set<number>>();
  const addToGrid = (d: number, r: number) => {
    if (!depthRows.has(d)) depthRows.set(d, new Set());
    depthRows.get(d)!.add(r);
  };

  for (const [key, d] of depths) {
    const r = commitRowMap.get(key);
    if (r !== undefined) {
      addToGrid(d, r);
    }
  }

  const startShas = new Set(runsSorted.map((run) => startCommitOf(run)));

  // Collect cross-row runs whose edges span more than 1 row
  type CrossRowRun = { tgtKey: string; srcRow: number; tgtRow: number };
  const crossRowRuns: CrossRowRun[] = [];

  for (const run of runsSorted) {
    const srcSha = startCommitOf(run);
    const headSha = effectiveHeadCommitSha(run);
    const tgtKey = headSha || `ghost-${run.id}`;

    const srcRow = commitRowMap.get(srcSha);
    const tgtRow = commitRowMap.get(tgtKey);
    if (srcRow === undefined || tgtRow === undefined) continue;
    if (Math.abs(tgtRow - srcRow) <= 1) continue;

    crossRowRuns.push({ tgtKey, srcRow, tgtRow });
  }

  // Process closest heads first so they keep their original column
  crossRowRuns.sort((a, b) => Math.abs(a.tgtRow - a.srcRow) - Math.abs(b.tgtRow - b.srcRow));

  for (const { tgtKey, srcRow, tgtRow } of crossRowRuns) {
    // Keep shared source anchors stable. Moving a head that is also the start
    // of other runs causes downstream step chains to stretch dramatically.
    if (startShas.has(tgtKey)) continue;

    const tgtDepth = depths.get(tgtKey);
    if (tgtDepth === undefined) continue;

    const minR = Math.min(srcRow, tgtRow);
    const maxR = Math.max(srcRow, tgtRow);

    const isBlocked = (d: number): boolean => {
      const rowsAtD = depthRows.get(d);
      if (!rowsAtD) return false;
      for (const rm of rowsAtD) {
        if (rm > minR && rm < maxR) return true;
      }
      return false;
    };

    if (!isBlocked(tgtDepth)) continue;

    // Bounded search: allow enough staggering for nearby overlaps, but avoid
    // runaway depth growth in dense histories.
    const maxShiftColumns = Math.max(1, Math.abs(tgtRow - srcRow) - 1);
    let newDepth = tgtDepth + 1;
    const depthLimit = tgtDepth + maxShiftColumns;
    while (newDepth <= depthLimit && isBlocked(newDepth)) newDepth++;
    if (newDepth > depthLimit) continue;

    // Remove from old position in grid
    depthRows.get(tgtDepth)?.delete(tgtRow);

    // Update depth and grid
    depths.set(tgtKey, newDepth);
    addToGrid(newDepth, tgtRow);
  }
}

/**
 * Detect source nodes whose cross-row vertical segment would pass through
 * intermediate same-depth nodes, and return a leftward pixel nudge for each.
 */
export function computeSourceNudges(
  runsSorted: PipelineRun[],
  depths: Map<string, number>,
  rows: RowAssignment
): Map<string, number> {
  const { commitRowMap } = rows;
  const nudges = new Map<string, number>();

  // Build index: depth → Set<row> for all placed nodes
  const depthRows = new Map<number, Set<number>>();
  for (const [sha, d] of depths) {
    const r = commitRowMap.get(sha);
    if (r === undefined) continue;
    if (!depthRows.has(d)) depthRows.set(d, new Set());
    depthRows.get(d)!.add(r);
  }

  for (const run of runsSorted) {
    const srcSha = startCommitOf(run);
    if (nudges.has(srcSha)) continue;

    const srcDepth = depths.get(srcSha);
    const srcRow = commitRowMap.get(srcSha);
    if (srcDepth === undefined || srcRow === undefined) continue;

    const headSha = effectiveHeadCommitSha(run);
    const tgtKey = headSha || `ghost-${run.id}`;
    const tgtRow = commitRowMap.get(tgtKey);
    if (tgtRow === undefined || tgtRow === srcRow) continue;

    const gap = Math.abs(tgtRow - srcRow);
    if (gap <= 1) continue;

    const minR = Math.min(srcRow, tgtRow);
    const maxR = Math.max(srcRow, tgtRow);
    const rowsAtDepth = depthRows.get(srcDepth);
    if (!rowsAtDepth) continue;

    let blocked = false;
    for (const rm of rowsAtDepth) {
      if (rm > minR && rm < maxR) {
        blocked = true;
        break;
      }
    }

    if (blocked) {
      nudges.set(srcSha, -COLUMN_GAP * 0.4);
    }
  }

  return nudges;
}

// ---------------------------------------------------------------------------
// computeNodePositions sub-functions
// ---------------------------------------------------------------------------

/** Shared mutable state for node layout sub-functions. */
type NodeLayoutCtx = {
  depths: Map<string, number>;
  rows: RowAssignment;
  maxRow: number;
  startShas: Set<string>;
  runsById: Map<string, PipelineRun>;
  expandedStepsMap: Map<string, StepResult[]> | undefined;
  commitBySha: Map<string, CommitPlacement>;
  nodeById: Map<string, Node>;
};

/** Create commit nodes from commitOrder, recording placements and skipping hidden heads. */
function createCommitNodes(
  commitOrder: string[],
  ctx: NodeLayoutCtx,
  commitSummaryBySha: Map<string, { diffSummary?: string; summaryLine?: string }>,
  commitMessageBySha: Map<string, string> | undefined,
  selectedBaseCommitSha: string | null,
  sourceNudges: Map<string, number> | undefined,
  expandedHeadShas: Set<string>,
  hiddenHeadShas: Set<string>
): Node[] {
  const { depths, rows, maxRow, commitBySha } = ctx;
  const { commitRowMap } = rows;
  const nodes: Node[] = [];

  for (const sha of commitOrder) {
    const nodeId = `commit-${sha}`;
    const nodeData: CommitNodeData = {
      sha,
      ...commitSummaryBySha.get(sha),
      commitMessage: commitMessageBySha?.get(sha),
    };
    const d = depths.get(sha) ?? 0;
    const r = commitRowMap.get(sha) ?? 0;
    const nudge = expandedHeadShas.has(sha) ? 0 : (sourceNudges?.get(sha) ?? 0);
    const x = ORIGIN_X + d * COLUMN_GAP + nudge;
    const y = (maxRow - r) * ROW_GAP;

    commitBySha.set(sha, { id: nodeId, x, y, depth: d, row: r, data: nodeData });

    if (hiddenHeadShas.has(sha)) continue;

    nodes.push({
      id: nodeId,
      type: "commit",
      position: { x, y },
      data: nodeData,
      selected: sha === selectedBaseCommitSha,
      draggable: false
    });
  }

  return nodes;
}

/**
 * Create ghost endpoint nodes for legacy runs (no step data) without a head commit.
 * Runs with step data don't need ghosts — the step chain handles the endpoint.
 */
function createGhostNodes(runsSorted: PipelineRun[], ctx: NodeLayoutCtx): Node[] {
  const { depths, rows, maxRow, expandedStepsMap, commitBySha } = ctx;
  const { commitRowMap } = rows;
  const nodes: Node[] = [];
  const ghostNodeIds = new Set<string>();

  for (const run of runsSorted) {
    if (effectiveHeadCommitSha(run)) continue;
    const nonSkippedSteps = expandedStepsMap?.get(run.id) ?? (run.step_results ?? [])
      .filter(sr => sr.status !== StepStatus.Skipped);
    if (nonSkippedSteps.length > 0) continue;

    const src = startCommitOf(run);
    const ghostKey = `ghost-${run.id}`;
    if (ghostNodeIds.has(ghostKey)) continue;
    ghostNodeIds.add(ghostKey);

    const gDepth = depths.get(ghostKey) ?? ((depths.get(src) ?? 0) + 1);
    const gRow = commitRowMap.get(ghostKey) ?? (rows.pipelineRowMap.get(run.id) ?? 0);
    const targetX = ORIGIN_X + gDepth * COLUMN_GAP;
    const targetY = (maxRow - gRow) * ROW_GAP;

    commitBySha.set(ghostKey, {
      id: ghostKey, x: targetX, y: targetY, depth: gDepth, row: gRow,
      data: { sha: ghostKey, isGhost: true, label: "running" }
    });

    nodes.push({
      id: ghostKey,
      type: "commit",
      position: { x: targetX, y: targetY },
      data: { sha: ghostKey, isGhost: true, label: "running" } satisfies CommitNodeData,
      draggable: false
    });
  }

  return nodes;
}

/** Create patch (step config) + outcome (step result) nodes for expanded runs. */
function createStepChainNodes(expandedRunIds: string[], ctx: NodeLayoutCtx): Node[] {
  const { depths, rows, maxRow, startShas, runsById, expandedStepsMap, commitBySha, nodeById } = ctx;
  const nodes: Node[] = [];

  for (const expandedRunId of expandedRunIds) {
    const expandedRun = runsById.get(expandedRunId);
    if (!expandedRun) continue;

    const nonSkippedSteps = expandedStepsMap?.get(expandedRunId) ?? (expandedRun.step_results ?? [])
      .filter(sr => sr.status !== StepStatus.Skipped)
      .sort((a, b) => a.step_index - b.step_index);
    const pipelineRow = rows.pipelineRowMap.get(expandedRun.id) ?? 0;
    const runHeadSha = effectiveHeadCommitSha(expandedRun);

    // Build snapshot lookup for patch node data
    const snapshotByStepId = new Map<string, RunStepSnapshot>();
    for (const snap of expandedRun.step_snapshots ?? []) {
      snapshotByStepId.set(snap.step_id, snap);
    }

    for (const sr of nonSkippedSteps) {
      const stepStatus: StepStatusView = stepStatusToView(sr.status);

      // ── Patch node (step config / input) ──
      const patchKey = `patch-${expandedRun.id}-${sr.step_id}`;
      const patchNodeId = `patch-${patchKey}`;
      const patchDepth = depths.get(patchKey) ?? 0;
      const patchX = ORIGIN_X + patchDepth * COLUMN_GAP;
      const patchY = (maxRow - pipelineRow) * ROW_GAP;

      const snapshot = snapshotByStepId.get(sr.step_id);
      let parsedConfig: AgentConfigInput | null = null;
      if (snapshot?.agent_config_json) {
        try {
          parsedConfig = JSON.parse(snapshot.agent_config_json) as AgentConfigInput;
        } catch { /* ignore parse errors */ }
      }

      const patchNodeData: PatchNodeData = {
        runId: expandedRun.id,
        stepId: sr.step_id,
        stepIndex: sr.step_index,
        stepName: sr.step_name || sr.step_id,
        stepStatus,
        toolName: parsedConfig?.tool_name,
        toolVersion: parsedConfig?.tool_version,
        promptPreview: parsedConfig?.prompt_template
          ? parsedConfig.prompt_template.slice(0, 60) + (parsedConfig.prompt_template.length > 60 ? "..." : "")
          : undefined,
        promptFull: parsedConfig?.prompt_template,
        variables: parsedConfig?.variables,
        definitionHash: snapshot?.definition_hash,
        configAvailable: !!parsedConfig,
      };

      commitBySha.set(patchKey, {
        id: patchNodeId, x: patchX, y: patchY, depth: patchDepth, row: pipelineRow,
        data: { sha: patchKey } as CommitNodeData
      });

      const patchNode: Node = {
        id: patchNodeId,
        type: "patch",
        position: { x: patchX, y: patchY },
        data: patchNodeData,
        draggable: false,
      };
      nodes.push(patchNode);
      nodeById.set(patchNodeId, patchNode);

      // ── Outcome node (step result / output) ──
      const stepSha = stepOutcomeKey(sr, runHeadSha, expandedRun.id, startShas);
      const stepNodeId = `commit-${stepSha}`;
      const existingNode = nodeById.get(stepNodeId);

      const srMetrics: StepMetrics = {
        tokens: (sr.input_tokens ?? 0) + (sr.output_tokens ?? 0),
        durationMs: durationToMs(sr.duration),
        filesChanged: sr.files_changed ?? 0,
        insertions: sr.insertions ?? 0,
        deletions: sr.deletions ?? 0,
        commitMessage: sr.commit_message ? sr.commit_message.split("\n")[0]?.trim() : undefined,
        errorMessage: sr.error_message
      };

      if (existingNode) {
        const existing = existingNode.data as CommitNodeData;
        existingNode.data = {
          ...existing,
          isStepCommit: true,
          runId: expandedRun.id,
          stepId: sr.step_id,
          stepName: sr.step_name || sr.step_id,
          stepStatus,
          stepIndex: sr.step_index,
          stepMetrics: srMetrics
        } satisfies CommitNodeData;
        // Reposition to the pipeline's row — the commit loop may have placed this
        // node at row 0 when a forked run added the SHA to dagCommits/commitOrder.
        const stepDepth = depths.get(stepSha) ?? 0;
        const stepX = ORIGIN_X + stepDepth * COLUMN_GAP;
        const stepY = (maxRow - pipelineRow) * ROW_GAP;
        existingNode.position = { x: stepX, y: stepY };
        const existingPlacement = commitBySha.get(stepSha);
        if (existingPlacement) {
          existingPlacement.x = stepX;
          existingPlacement.y = stepY;
          existingPlacement.row = pipelineRow;
        }
      } else {
        const stepDepth = depths.get(stepSha) ?? 0;
        const stepX = ORIGIN_X + stepDepth * COLUMN_GAP;
        const stepY = (maxRow - pipelineRow) * ROW_GAP;

        const stepNodeData: CommitNodeData = {
          sha: sr.commit_sha || stepSha,
          isGhost: !sr.commit_sha,
          isStepCommit: true,
          runId: expandedRun.id,
          stepId: sr.step_id,
          stepName: sr.step_name || sr.step_id,
          stepStatus,
          stepIndex: sr.step_index,
          stepMetrics: srMetrics
        };

        commitBySha.set(stepSha, {
          id: stepNodeId, x: stepX, y: stepY, depth: stepDepth, row: pipelineRow,
          data: stepNodeData
        });

        const stepNode: Node = {
          id: stepNodeId,
          type: "commit",
          position: { x: stepX, y: stepY },
          data: stepNodeData,
          draggable: false
        };
        nodes.push(stepNode);
        nodeById.set(stepNodeId, stepNode);
      }
    }
  }

  return nodes;
}

/** Create pipeline background container nodes that wrap step chains. */
function createPipelineBgNodes(expandedRunIds: string[], ctx: NodeLayoutCtx): Node[] {
  const { depths, rows, maxRow, startShas, runsById, expandedStepsMap, commitBySha } = ctx;
  const nodes: Node[] = [];

  for (const expandedRunId of expandedRunIds) {
    const expandedRun = runsById.get(expandedRunId);
    if (!expandedRun) continue;

    const nonSkippedSteps = expandedStepsMap?.get(expandedRunId) ?? (expandedRun.step_results ?? [])
      .filter(sr => sr.status !== StepStatus.Skipped)
      .sort((a, b) => a.step_index - b.step_index);
    if (nonSkippedSteps.length === 0) continue;

    const pipelineRow = rows.pipelineRowMap.get(expandedRun.id) ?? 0;
    const runHeadSha = effectiveHeadCommitSha(expandedRun);

    let minX = Infinity;
    let maxX = -Infinity;
    let stepY = (maxRow - pipelineRow) * ROW_GAP;

    for (const sr of nonSkippedSteps) {
      const patchKey = `patch-${expandedRun.id}-${sr.step_id}`;
      const patchPlacement = commitBySha.get(patchKey);
      if (patchPlacement) {
        minX = Math.min(minX, patchPlacement.x);
        maxX = Math.max(maxX, patchPlacement.x + PATCH_NODE_WIDTH);
        stepY = patchPlacement.y;
      }

      const stepSha = stepOutcomeKey(sr, runHeadSha, expandedRun.id, startShas);
      const stepPlacement = commitBySha.get(stepSha);
      if (stepPlacement) {
        minX = Math.min(minX, stepPlacement.x);
        maxX = Math.max(maxX, stepPlacement.x + OUTCOME_NODE_WIDTH);
      }
    }

    if (minX === Infinity) continue;

    const bgWidth = maxX - minX + 2 * BG_PADDING_X;
    const bgHeight = BG_PADDING_TOP + 60 + BG_PADDING_BOTTOM; // 60 ≈ node height
    const status = runStatusLabel(expandedRun);

    nodes.push({
      id: `pipeline-bg-${expandedRun.id}`,
      type: "pipeline-bg",
      position: { x: minX - BG_PADDING_X, y: stepY - BG_PADDING_TOP },
      data: {
        runId: expandedRun.id,
        runName: expandedRun.name,
        status,
        width: bgWidth,
        height: bgHeight,
      } satisfies PipelineBgNodeData,
      zIndex: -1,
      selectable: false,
      draggable: false,
    });
  }

  return nodes;
}

/** Annotate fork point commits (where a fork run diverges from its parent). */
function markForkPoints(
  expandedRunIds: string[],
  runsSorted: PipelineRun[],
  nodeById: Map<string, Node>
): void {
  if (expandedRunIds.length <= 1) return;
  const expandedSet = new Set(expandedRunIds);
  for (const run of runsSorted) {
    if (!run.parent_run_id || !expandedSet.has(run.id) || !expandedSet.has(run.parent_run_id)) continue;
    const forkNodeId = `commit-${startCommitOf(run)}`;
    const forkNode = nodeById.get(forkNodeId);
    if (forkNode) {
      const existing = forkNode.data as CommitNodeData;
      forkNode.data = { ...existing, isForkPoint: true } satisfies CommitNodeData;
    }
  }
}

/**
 * Compute pixel positions for all nodes. Orchestrates the sub-functions above.
 */
export function computeNodePositions(
  commitOrder: string[],
  depths: Map<string, number>,
  rows: RowAssignment,
  runsSorted: PipelineRun[],
  expandedRunIds: string[],
  commitSummaryBySha: Map<string, { diffSummary?: string; summaryLine?: string }>,
  selectedBaseCommitSha: string | null,
  sourceNudges?: Map<string, number>,
  expandedStepsMap?: Map<string, StepResult[]>,
  commitMessageBySha?: Map<string, string>,
  runsById?: Map<string, PipelineRun>,
  startShas?: Set<string>,
  hiddenHeadShas?: Set<string>
): { commitBySha: Map<string, CommitPlacement>; nodes: Node[] } {
  const effectiveRunsById = runsById ?? new Map(runsSorted.map(r => [r.id, r]));
  const effectiveStartShas = startShas ?? new Set(runsSorted.map(r => startCommitOf(r)));
  const effectiveHiddenHeads = hiddenHeadShas ?? getHiddenHeadShas(effectiveRunsById, expandedRunIds, effectiveStartShas, expandedStepsMap);

  // Head commits of expanded runs must not be nudged
  const expandedHeadShas = new Set<string>();
  for (const runId of expandedRunIds) {
    const run = effectiveRunsById.get(runId);
    if (run) {
      const headSha = effectiveHeadCommitSha(run);
      if (headSha) expandedHeadShas.add(headSha);
    }
  }

  const ctx: NodeLayoutCtx = {
    depths, rows, maxRow: rows.maxRow,
    startShas: effectiveStartShas,
    runsById: effectiveRunsById,
    expandedStepsMap,
    commitBySha: new Map(),
    nodeById: new Map(),
  };

  const commitNodes = createCommitNodes(
    commitOrder, ctx, commitSummaryBySha, commitMessageBySha,
    selectedBaseCommitSha, sourceNudges, expandedHeadShas, effectiveHiddenHeads
  );
  const ghostNodes = createGhostNodes(runsSorted, ctx);

  // Build nodeById from initial nodes before step chain creation
  for (const node of [...commitNodes, ...ghostNodes]) {
    ctx.nodeById.set(node.id, node);
  }

  const stepChainNodes = createStepChainNodes(expandedRunIds, ctx);
  const bgNodes = createPipelineBgNodes(expandedRunIds, ctx);
  markForkPoints(expandedRunIds, runsSorted, ctx.nodeById);

  const nodes = [...commitNodes, ...ghostNodes, ...stepChainNodes, ...bgNodes];
  return { commitBySha: ctx.commitBySha, nodes };
}

// ---------------------------------------------------------------------------
// createEdges sub-functions
// ---------------------------------------------------------------------------

/** Build connector→patch→outcome edges for a single run's step chain. */
function buildStepChainEdges(args: {
  run: PipelineRun;
  nonSkippedSteps: StepResult[];
  sourceCommit: CommitPlacement;
  commitBySha: Map<string, CommitPlacement>;
  rows: RowAssignment;
  activities: AIActivityRecord[];
  selectedStep: { runId: string; stepId: string } | null;
  startShas: Set<string>;
  targetCommitSha: string | null;
  targetNodeId: string | null;
  status: GraphStatus;
  startSha: string;
  runMetrics: GraphEdgeMetrics;
  hiddenHeadShas: Set<string>;
}): Edge[] {
  const {
    run, nonSkippedSteps, sourceCommit, commitBySha, rows, activities,
    selectedStep, startShas, targetCommitSha, targetNodeId, status,
    startSha, runMetrics, hiddenHeadShas,
  } = args;
  const edges: Edge[] = [];

  let previousNodeId = sourceCommit.id;
  let previousRow = sourceCommit.row;
  let previousStepId = "";
  const pipelineRow = rows.pipelineRowMap.get(run.id) ?? 0;

  for (const sr of nonSkippedSteps) {
    const stepStatus: StepStatusView = stepStatusToView(sr.status);
    const stepSha = stepOutcomeKey(sr, targetCommitSha, run.id, startShas);
    const stepNodeId = `commit-${stepSha}`;

    const patchKey = `patch-${run.id}-${sr.step_id}`;
    const patchNodeId = `patch-${patchKey}`;
    const patchPlacement = commitBySha.get(patchKey);
    const patchRow = patchPlacement?.row ?? pipelineRow;
    const stepPlacement = commitBySha.get(stepSha);
    const stepRow = stepPlacement?.row ?? pipelineRow;

    // Pre-edge: previous node → patch node (connector, no label)
    const preHandles = edgeHandles(previousRow, patchRow);
    edges.push({
      id: `connector-${run.id}-${sr.step_id}-pre`,
      type: "pipeline",
      source: previousNodeId,
      sourceHandle: preHandles.sourceHandle,
      target: patchNodeId,
      targetHandle: preHandles.targetHandle,
      animated: stepStatus === "running",
      style: edgeStyleForStatus(stepStatus, "step"),
      data: {
        kind: "connector",
        clickable: false,
        status: stepStatus,
        runId: run.id,
      } satisfies GraphEdgeData,
    });

    // Post-edge: patch node → outcome node (step edge, clickable)
    const postHandles = edgeHandles(patchRow, stepRow);
    const stepEvents = activities.filter((a) => a.step_id === sr.step_id);
    const selectedStepEdge = selectedStep?.runId === run.id && selectedStep?.stepId === sr.step_id;

    edges.push({
      id: `step-edge-${run.id}-${sr.step_id}`,
      type: "pipeline",
      source: patchNodeId,
      sourceHandle: postHandles.sourceHandle,
      target: stepNodeId,
      targetHandle: postHandles.targetHandle,
      animated: stepStatus === "running",
      selected: selectedStepEdge,
      style: edgeStyleForStatus(stepStatus, "step"),
      data: {
        kind: "step",
        clickable: true,
        status: stepStatus,
        highlighted: selectedStepEdge,
        runId: run.id,
        runName: run.name,
        stepId: sr.step_id,
        stepName: sr.step_name || sr.step_id,
        stepIndex: sr.step_index,
        sourceCommitSha: startSha,
        targetCommitSha: targetCommitSha ?? undefined,
        metrics: summarizeStepMetrics(sr, stepEvents)
      } satisfies GraphEdgeData,
      className: "pipeline-edge--clickable"
    });

    previousNodeId = stepNodeId;
    previousRow = stepRow;
    previousStepId = sr.step_id;
  }

  // Tail edge from last step node to target (only if target exists and is rendered)
  const targetRow = targetCommitSha ? (commitBySha.get(targetCommitSha)?.row ?? sourceCommit.row) : sourceCommit.row;
  if (targetNodeId && previousNodeId !== targetNodeId && !hiddenHeadShas.has(targetCommitSha ?? "")) {
    const handles = edgeHandles(previousRow, targetRow);
    edges.push({
      id: `run-tail-${run.id}-${previousStepId || "start"}`,
      type: "pipeline",
      source: previousNodeId,
      sourceHandle: handles.sourceHandle,
      target: targetNodeId,
      targetHandle: handles.targetHandle,
      animated: status === "running",
      style: edgeStyleForStatus(status, "run"),
      data: {
        kind: "run",
        clickable: true,
        status,
        runId: run.id,
        runName: run.name,
        sourceCommitSha: startSha,
        targetCommitSha: targetCommitSha ?? undefined,
        metrics: runMetrics,
        metricsLabel: "run total"
      } satisfies GraphEdgeData,
      className: "pipeline-edge--clickable"
    });
  }

  return edges;
}

/**
 * Create all edges (collapsed run edges, expanded step edges).
 */
export function createEdges(
  runsSorted: PipelineRun[],
  commitBySha: Map<string, CommitPlacement>,
  rows: RowAssignment,
  runDetails: Record<string, { run: PipelineRun; activities: AIActivityRecord[] }>,
  highlightedRunId: string | null,
  expandedRunIds: string[],
  selectedStep: { runId: string; stepId: string } | null,
  expandedStepsMap?: Map<string, StepResult[]>,
  startShas?: Set<string>,
  hiddenHeadShas?: Set<string>
): Edge[] {
  const edges: Edge[] = [];

  const effectiveStartShas = startShas ?? new Set(runsSorted.map(r => startCommitOf(r)));
  const effectiveHiddenHeads = hiddenHeadShas ?? getHiddenHeadShas(
    new Map(runsSorted.map(r => [r.id, r])),
    expandedRunIds,
    effectiveStartShas,
    expandedStepsMap
  );

  for (const run of runsSorted) {
    const status = runStatusLabel(run);
    const startSha = startCommitOf(run);
    const sourceCommit = commitBySha.get(startSha);
    if (!sourceCommit) continue;

    const activities = runDetails[run.id]?.activities ?? [];
    const runMetrics = summarizeRunMetrics(run, activities);

    let targetNodeId: string | null;
    let targetCommitSha: string | null = effectiveHeadCommitSha(run) || null;

    if (targetCommitSha) {
      const targetCommit = commitBySha.get(targetCommitSha);
      if (!targetCommit) continue;
      targetNodeId = targetCommit.id;
    } else {
      const ghostKey = `ghost-${run.id}`;
      if (commitBySha.has(ghostKey)) {
        targetCommitSha = ghostKey;
        targetNodeId = ghostKey;
      } else {
        // No head and no ghost (run has step data) — step chain is the endpoint
        targetCommitSha = null;
        targetNodeId = null;
      }
    }

    const isHighlighted = highlightedRunId === run.id;
    const allSteps = run.step_results ?? [];
    const nonSkippedSteps = expandedStepsMap?.get(run.id)
      ?? [...allSteps]
        .filter((sr) => sr.status !== StepStatus.Skipped)
        .sort((a, b) => a.step_index - b.step_index);

    const isStale = status === "cancelled" && (runMetrics.tokens === 0);

    const sourceRow = sourceCommit.row;
    const targetPlacement = targetCommitSha ? commitBySha.get(targetCommitSha) : undefined;
    const targetRow = targetPlacement?.row ?? sourceRow;

    // Legacy runs with no step data: direct edge (requires a target node)
    if (nonSkippedSteps.length === 0) {
      if (!targetNodeId) continue; // no target to connect to
      const handles = edgeHandles(sourceRow, targetRow);
      const highlighted = selectedStep === null && isHighlighted;
      const staleStyle = isStale ? EDGE_STYLES.stale : undefined;
      edges.push({
        id: `run-edge-${run.id}`,
        type: "pipeline",
        source: sourceCommit.id,
        target: targetNodeId,
        sourceHandle: handles.sourceHandle,
        targetHandle: handles.targetHandle,
        animated: status === "running",
        selected: highlighted,
        style: staleStyle ?? edgeStyleForStatus(status, "run"),
        data: {
          kind: "run",
          clickable: true,
          status,
          highlighted,
          stepCount: 0,
          runId: run.id,
          runName: run.name,
          sourceCommitSha: startSha,
          targetCommitSha: targetCommitSha!,
          metrics: runMetrics,
          isStale
        } satisfies GraphEdgeData,
        className: "pipeline-edge--clickable"
      });
      continue;
    }

    // Step chain: connector → patch → outcome edges + optional tail edge
    edges.push(...buildStepChainEdges({
      run, nonSkippedSteps, sourceCommit, commitBySha, rows, activities,
      selectedStep, startShas: effectiveStartShas,
      targetCommitSha, targetNodeId, status, startSha, runMetrics,
      hiddenHeadShas: effectiveHiddenHeads,
    }));
  }

  // Z-order: stale edges first (behind), running edges last (on top)
  edges.sort((a, b) => {
    const aData = a.data as GraphEdgeData | undefined;
    const bData = b.data as GraphEdgeData | undefined;
    const aStale = aData?.isStale ? 0 : 1;
    const bStale = bData?.isStale ? 0 : 1;
    if (aStale !== bStale) return aStale - bStale;
    const aRunning = aData?.status === "running" ? 1 : 0;
    const bRunning = bData?.status === "running" ? 1 : 0;
    return aRunning - bRunning;
  });

  return edges;
}

// ---------------------------------------------------------------------------
// Main entry point — composes all the above
// ---------------------------------------------------------------------------

export function buildProjectGraph(input: GraphInput): { nodes: Node[]; edges: Edge[] } {
  const { runs, commits, runDetails, highlightedRunId, selectedStep, selectedBaseCommitSha } = input;

  const runsSorted = resolveRuns(runs, runDetails);

  // All runs are always expanded — no collapse toggle
  const expandedRunIds = runsSorted.map(r => r.id);

  const dag = buildDAG(runsSorted);

  const commitOrder = collectCommitShas(runsSorted, commits, dag.dagCommits);

  if (commitOrder.length === 0 && runs.length === 0) {
    return { nodes: [], edges: [] };
  }

  // Shared lookups — computed once and threaded through all layout functions
  const runsById = new Map(runsSorted.map(r => [r.id, r]));
  const startShas = new Set(runsSorted.map(r => startCommitOf(r)));

  // Pre-compute non-skipped sorted steps for all expanded runs (used by multiple functions)
  const expandedStepsMap = new Map<string, StepResult[]>();
  for (const expandedRunId of expandedRunIds) {
    const expandedRun = runsById.get(expandedRunId);
    if (expandedRun) {
      const nonSkipped = (expandedRun.step_results ?? [])
        .filter(sr => sr.status !== StepStatus.Skipped)
        .sort((a, b) => a.step_index - b.step_index);
      expandedStepsMap.set(expandedRunId, nonSkipped);
    }
  }

  const hiddenHeadShas = getHiddenHeadShas(runsById, expandedRunIds, startShas, expandedStepsMap);
  const commitSummaryBySha = buildCommitSummaryMap(runsSorted);

  // Build commit message map from git log data
  const commitMessageBySha = new Map<string, string>();
  for (const c of commits) {
    if (c.Hash && c.Message) {
      const firstLine = c.Message.split("\n")[0]?.trim() || "";
      commitMessageBySha.set(c.Hash, firstLine.length > 40 ? firstLine.slice(0, 40) + "..." : firstLine);
    }
  }

  const depths = computeDepths(dag, runsSorted, expandedRunIds, commitOrder, expandedStepsMap, startShas);
  const rows = assignPipelineRows(runsSorted, dag);
  spreadBlockedHeads(runsSorted, depths, rows, expandedRunIds);
  const sourceNudges = computeSourceNudges(runsSorted, depths, rows);

  const { commitBySha, nodes } = computeNodePositions(
    commitOrder,
    depths,
    rows,
    runsSorted,
    expandedRunIds,
    commitSummaryBySha,
    selectedBaseCommitSha,
    sourceNudges,
    expandedStepsMap,
    commitMessageBySha,
    runsById,
    startShas,
    hiddenHeadShas
  );

  const edges = createEdges(
    runsSorted,
    commitBySha,
    rows,
    runDetails,
    highlightedRunId,
    expandedRunIds,
    selectedStep,
    expandedStepsMap,
    startShas,
    hiddenHeadShas
  );

  return { nodes, edges };
}

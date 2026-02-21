// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Background, Controls, ReactFlow, applyNodeChanges, type Edge, type EdgeTypes, type Node, type NodeChange, type NodeTypes } from "@xyflow/react";

import { getCommits, getPipelineRun, getPipelineRunActivity, listPipelineRuns } from "../lib/api";
import { messageFromError } from "../lib/formatting";
import type { GraphSelection } from "../lib/graph-selection";
import { buildProjectGraph, type GraphEdgeData, type GraphInput } from "../lib/graph-layout";
import { PipelineRunStatus, StepStatus, type AIActivityRecord, type CommitInfo, type PipelineRun, type StepResult } from "../lib/types";
import { theme } from "../lib/theme";
import { isLiveRun } from "../lib/run-phase";
import { useActivitiesByStep, useRunSteps, useStepExecutionMap } from "../state/selectors";
import { useProjectGraphStore } from "../state/project-graph-store";
import { useRunStore, type RunPhase } from "../state/run-store";
import { CommitNode } from "./nodes/CommitNode";
import { PatchNode } from "./nodes/PatchNode";
import { PipelineBgNode } from "./nodes/PipelineBgNode";
import { PipelineEdge } from "./edges/PipelineEdge";
import { EdgeDetailsDrawer } from "./EdgeDetailsDrawer";
import { PatchExpandProvider, usePatchExpand } from "./nodes/PatchExpandContext";
import { ForkFromCommitProvider } from "./nodes/ForkFromCommitContext";

type NoldarimGraphViewProps = {
  projectId: string;
  serverUrl: string;
  selectedBaseCommitSha: string | null;
  onSelectBaseCommit: (sha: string) => void;
  onForkFromCommit: (sha: string) => void;
};

function unresolvedRunningStepIndex(
  steps: { id: string }[],
  stepExecutionById: Record<string, StepResult>,
  activitiesByStep: Record<string, AIActivityRecord[]>
): number {
  for (let index = 0; index < steps.length; index += 1) {
    const result = stepExecutionById[steps[index].id];
    if (!result) {
      break;
    }
    if (result.status !== StepStatus.Completed && result.status !== StepStatus.Skipped) {
      return index;
    }
  }

  let lastActiveIndex = -1;
  for (let index = 0; index < steps.length; index += 1) {
    const events = activitiesByStep[steps[index].id];
    if (events && events.length > 0) {
      lastActiveIndex = index;
    }
  }
  return lastActiveIndex >= 0 ? lastActiveIndex : 0;
}

function phaseToPipelineRunStatus(phase: RunPhase, snapshotStatus?: PipelineRunStatus): PipelineRunStatus {
  if (typeof snapshotStatus === "number") {
    return snapshotStatus;
  }
  if (phase === "completed") return PipelineRunStatus.Completed;
  if (phase === "failed" || phase === "cancelled") return PipelineRunStatus.Failed;
  if (phase === "running" || phase === "starting" || phase === "cancelling") return PipelineRunStatus.Running;
  return PipelineRunStatus.Pending;
}

function buildLiveStepResults(args: {
  runId: string;
  phase: RunPhase;
  runStatus?: PipelineRunStatus;
  steps: { id: string; name: string }[];
  stepExecutionById: Record<string, StepResult>;
  activitiesByStep: Record<string, AIActivityRecord[]>;
}): StepResult[] {
  const { runId, phase, runStatus, steps, stepExecutionById, activitiesByStep } = args;
  const effectiveStatus = phaseToPipelineRunStatus(phase, runStatus);
  const fallbackRunningIndex = unresolvedRunningStepIndex(steps, stepExecutionById, activitiesByStep);

  return steps.map((step, index) => {
    const existing = stepExecutionById[step.id];
    if (existing) {
      return {
        ...existing,
        pipeline_run_id: existing.pipeline_run_id || runId,
        step_name: existing.step_name || step.name,
        step_index: existing.step_index ?? index
      };
    }

    let status: StepResult["status"] = StepStatus.Pending;
    if (effectiveStatus === PipelineRunStatus.Running) {
      if (index < fallbackRunningIndex) {
        status = StepStatus.Completed;
      } else if (index === fallbackRunningIndex) {
        status = StepStatus.Running;
      }
    }

    return {
      id: `live-${runId}-${step.id}`,
      pipeline_run_id: runId,
      step_id: step.id,
      step_name: step.name,
      step_index: index,
      status,
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
  });
}

const EDGE_ANIM_DURATION = 300;

function runVersionTime(run?: PipelineRun | null): number {
  if (!run) return 0;
  const updated = run.updated_at ? Date.parse(run.updated_at) : Number.NaN;
  if (!Number.isNaN(updated)) return updated;
  const created = run.created_at ? Date.parse(run.created_at) : Number.NaN;
  return Number.isNaN(created) ? 0 : created;
}

function pickPreferredRun(listRun: PipelineRun | null, detailsRun: PipelineRun | null): PipelineRun | null {
  if (!listRun) return detailsRun;
  if (!detailsRun) return listRun;
  return runVersionTime(detailsRun) >= runVersionTime(listRun)
    ? { ...listRun, ...detailsRun }
    : listRun;
}

export function mergeAnimatedEdges(animatedEdges: Edge[], exitingEdges: Edge[]): Edge[] {
  if (exitingEdges.length === 0) return animatedEdges;
  const activeIds = new Set(animatedEdges.map((edge) => edge.id));
  return [...animatedEdges, ...exitingEdges.filter((edge) => !activeIds.has(edge.id))];
}

/**
 * Diffs edges by ID between renders and tags new edges with "edge-entering"
 * and keeps removed edges temporarily with "edge-exiting" so they can animate out.
 * Unchanged edges pass through with stable references (no re-render).
 */
function useAnimatedEdges(targetEdges: Edge[]): Edge[] {
  const prevMapRef = useRef<Map<string, Edge>>(new Map());
  const [exitingEdges, setExitingEdges] = useState<Edge[]>([]);
  const cleanupTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  const result = useMemo(() => {
    const prevMap = prevMapRef.current;
    const nextMap = new Map<string, Edge>();
    const animated: Edge[] = [];

    for (const edge of targetEdges) {
      nextMap.set(edge.id, edge);
      if (!prevMap.has(edge.id)) {
        // New edge — tag with entering class
        animated.push({
          ...edge,
          className: [edge.className, "edge-entering"].filter(Boolean).join(" "),
        });
      } else {
        // Existing edge — pass through as-is
        animated.push(edge);
      }
    }

    // Find removed edges
    const removed: Edge[] = [];
    for (const [id, old] of prevMap) {
      if (!nextMap.has(id)) {
        removed.push({
          ...old,
          className: [old.className?.replace("edge-entering", "").trim(), "edge-exiting"]
            .filter(Boolean)
            .join(" "),
          data: { ...(old.data as object), clickable: false },
        });
      }
    }

    prevMapRef.current = nextMap;
    return { animated, removed };
  }, [targetEdges]);

  // Schedule removal of exiting edges after animation
  useEffect(() => {
    if (result.removed.length > 0) {
      setExitingEdges(result.removed);
      clearTimeout(cleanupTimer.current);
      cleanupTimer.current = setTimeout(() => setExitingEdges([]), EDGE_ANIM_DURATION);
    } else {
      setExitingEdges([]);
    }
    return () => clearTimeout(cleanupTimer.current);
  }, [result.removed]);

  return useMemo(
    () => mergeAnimatedEdges(result.animated, exitingEdges),
    [result.animated, exitingEdges]
  );
}

export const projectNodeTypes: NodeTypes = {
  commit: CommitNode,
  patch: PatchNode,
  "pipeline-bg": PipelineBgNode,
};

export const projectEdgeTypes: EdgeTypes = {
  pipeline: PipelineEdge
};

function ProjectGraphInner({
  graphInput,
  isLoading,
  onSelectRunEdge,
  onSelectStepEdge,
  onSelectBaseCommit,
  onClearSelection
}: {
  graphInput: GraphInput;
  isLoading: boolean;
  onSelectRunEdge: (runId: string) => void;
  onSelectStepEdge: (runId: string, stepId: string) => void;
  onSelectBaseCommit: (sha: string) => void;
  onClearSelection: () => void;
}) {
  const { expandedPatchId, setExpandedPatchId } = usePatchExpand();
  const graphResult = useMemo(
    () => buildProjectGraph(graphInput),
    [graphInput]
  );

  // Lift expanded patch node's z-index so overlay renders above other nodes
  const nodesWithZIndex = useMemo(() => {
    if (!expandedPatchId) return graphResult.nodes;
    return graphResult.nodes.map(n =>
      n.id === expandedPatchId ? { ...n, zIndex: 1000 } : n
    );
  }, [graphResult.nodes, expandedPatchId]);

  const [nodes, setNodes] = useState<Node[]>(nodesWithZIndex);
  const edges = useAnimatedEdges(graphResult.edges);

  useEffect(() => {
    setNodes(nodesWithZIndex);
  }, [nodesWithZIndex]);

  const onNodesChange = useCallback(
    (changes: NodeChange[]) => setNodes((nds) => applyNodeChanges(changes, nds)),
    []
  );

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: { type?: string; data: Record<string, unknown> }) => {
      if (node.type === "pipeline-bg" && node.data.runId) {
        onSelectRunEdge(node.data.runId as string);
        return;
      }
      if (node.type !== "commit") {
        return;
      }
      // Step commit (outcome) nodes open the step details drawer
      if (node.data.isStepCommit && node.data.runId && node.data.stepId) {
        onSelectStepEdge(node.data.runId as string, node.data.stepId as string);
        return;
      }
      const sha = node.data.sha as string;
      const isGhost = Boolean(node.data.isGhost);
      if (!isGhost && sha && sha !== "unknown") {
        onSelectBaseCommit(sha);
      }
    },
    [onSelectBaseCommit, onSelectRunEdge, onSelectStepEdge]
  );

  const handleEdgeClick = useCallback(
    (_: React.MouseEvent, edge: { data?: unknown }) => {
      const data = edge.data as GraphEdgeData | undefined;
      if (!data || !data.clickable) return;
      if (data.kind === "run" && data.runId) {
        onSelectRunEdge(data.runId);
        return;
      }
      if (data.kind === "step" && data.runId && data.stepId) {
        onSelectStepEdge(data.runId, data.stepId);
      }
    },
    [onSelectRunEdge, onSelectStepEdge]
  );

  const handlePaneClick = useCallback(() => {
    setExpandedPatchId(null);
    onClearSelection();
  }, [setExpandedPatchId, onClearSelection]);

  if (isLoading && graphInput.runs.length === 0 && graphInput.commits.length === 0) {
    return (
      <div className="run-graph-empty">
        <p className="run-graph-empty__title">Loading commit and workflow history...</p>
        <p className="muted-text">
          The graph will show commit states on nodes and Temporal execution history on edges.
        </p>
      </div>
    );
  }

  if (graphInput.runs.length === 0 && graphInput.commits.length === 0) {
    return (
      <div className="run-graph-empty">
        <p className="run-graph-empty__title">No commit or run data yet.</p>
        <ol className="run-graph-empty__steps muted-text">
          <li>Start a pipeline to create the first workflow branch.</li>
          <li>Click any commit node to choose a base commit for new runs.</li>
          <li>Click run or step edges to inspect events, replay, or fork deterministically.</li>
        </ol>
      </div>
    );
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={projectNodeTypes}
      edgeTypes={projectEdgeTypes}
      fitView
      minZoom={0.1}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable
      onNodesChange={onNodesChange}
      onNodeClick={handleNodeClick}
      onEdgeClick={handleEdgeClick}
      onPaneClick={handlePaneClick}
    >
      <Background gap={18} size={1} color={theme.canvasDotColor} />
      <Controls showInteractive={false} />
    </ReactFlow>
  );
}

export function ProjectGraph(props: {
  graphInput: GraphInput;
  isLoading: boolean;
  onSelectRunEdge: (runId: string) => void;
  onSelectStepEdge: (runId: string, stepId: string) => void;
  onSelectBaseCommit: (sha: string) => void;
  onClearSelection: () => void;
  onForkFromCommit?: (sha: string) => void;
}) {
  const { onForkFromCommit, ...innerProps } = props;
  return (
    <ForkFromCommitProvider value={onForkFromCommit ?? null}>
      <PatchExpandProvider>
        <ProjectGraphInner {...innerProps} />
      </PatchExpandProvider>
    </ForkFromCommitProvider>
  );
}

export function NoldarimGraphView({
  projectId,
  serverUrl,
  selectedBaseCommitSha,
  onSelectBaseCommit,
  onForkFromCommit
}: NoldarimGraphViewProps) {
  const phase = useRunStore((s) => s.phase);
  const runId = useRunStore((s) => s.runId);
  const liveRunSnapshot = useRunStore((s) => s.run);
  const liveRunProjectId = useRunStore((s) => s.projectId);
  const liveSteps = useRunSteps();
  const liveStepExecutionById = useStepExecutionMap();
  const liveActivitiesByStep = useActivitiesByStep();

  const refreshToken = useProjectGraphStore((s) => s.refreshToken);
  const projectError = useProjectGraphStore((s) => s.error);
  const runs = useProjectGraphStore((s) => s.runs);
  const runDetails = useProjectGraphStore((s) => s.expandedRunData);
  const isLoading = useProjectGraphStore((s) => s.isLoading);

  const [commits, setCommits] = useState<CommitInfo[]>([]);
  const [commitsError, setCommitsError] = useState<string | null>(null);
  const [selection, setSelection] = useState<GraphSelection | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const fetchRunsRequestIdRef = useRef(0);
  const fetchRunsAbortRef = useRef<AbortController | null>(null);
  const fetchCommitsRequestIdRef = useRef(0);
  const fetchCommitsAbortRef = useRef<AbortController | null>(null);
  const loadingRunDetailsRef = useRef(new Set<string>());

  const liveActivities = useMemo(
    () => Object.values(liveActivitiesByStep).flat(),
    [liveActivitiesByStep]
  );

  const liveRunOverlay = useMemo<PipelineRun | null>(() => {
    if (!isLiveRun(phase) || !runId || liveRunProjectId !== projectId) {
      return null;
    }

    const stepResults = buildLiveStepResults({
      runId,
      phase,
      runStatus: liveRunSnapshot?.status,
      steps: liveSteps,
      stepExecutionById: liveStepExecutionById,
      activitiesByStep: liveActivitiesByStep
    });

    return {
      id: runId,
      project_id: liveRunSnapshot?.project_id || liveRunProjectId || projectId,
      name: liveRunSnapshot?.name || "Live run",
      status: phaseToPipelineRunStatus(phase, liveRunSnapshot?.status),
      base_commit_sha: liveRunSnapshot?.base_commit_sha,
      start_commit_sha: liveRunSnapshot?.start_commit_sha,
      head_commit_sha: liveRunSnapshot?.head_commit_sha,
      parent_run_id: liveRunSnapshot?.parent_run_id,
      fork_after_step_id: liveRunSnapshot?.fork_after_step_id,
      created_at: liveRunSnapshot?.created_at,
      updated_at: liveRunSnapshot?.updated_at,
      started_at: liveRunSnapshot?.started_at,
      completed_at: liveRunSnapshot?.completed_at,
      error_message: liveRunSnapshot?.error_message,
      step_results: stepResults
    };
  }, [
    phase,
    runId,
    liveRunProjectId,
    liveRunSnapshot,
    projectId,
    liveSteps,
    liveStepExecutionById,
    liveActivitiesByStep
  ]);

  const mergedRuns = useMemo(() => {
    const result = [...runs];
    if (liveRunOverlay) {
      const existingIndex = result.findIndex((run) => run.id === liveRunOverlay.id);
      if (existingIndex >= 0) {
        // Explicitly set step_results after spread — the spread may carry stale
        // step_results from the list-endpoint snapshot, so the live overlay's
        // freshly-built results must win.
        result[existingIndex] = {
          ...result[existingIndex],
          ...liveRunOverlay,
          step_results: liveRunOverlay.step_results
        };
      } else {
        result.push(liveRunOverlay);
      }
    }
    return result;
  }, [runs, liveRunOverlay]);

  const mergedRunDetails = useMemo(() => {
    const details = { ...runDetails };
    if (liveRunOverlay) {
      details[liveRunOverlay.id] = {
        run: liveRunOverlay,
        activities: liveActivities
      };
    }
    return details;
  }, [runDetails, liveRunOverlay, liveActivities]);

  const highlightedRunId = selection?.runId ?? null;

  const graphInput = useMemo<GraphInput>(() => {
    return {
      runs: mergedRuns,
      commits,
      runDetails: mergedRunDetails,
      highlightedRunId,
      selectedStep: selection?.kind === "step-edge"
        ? { runId: selection.runId, stepId: selection.stepId }
        : null,
      selectedBaseCommitSha
    };
  }, [mergedRuns, commits, mergedRunDetails, highlightedRunId, selection, selectedBaseCommitSha]);

  const selectedRunId = selection?.runId ?? null;

  const selectedRun = useMemo(() => {
    if (!selectedRunId) return null;
    const detailsRun = mergedRunDetails[selectedRunId]?.run ?? null;
    const listRun = mergedRuns.find((run) => run.id === selectedRunId) ?? null;
    return pickPreferredRun(listRun, detailsRun);
  }, [selectedRunId, mergedRunDetails, mergedRuns]);

  const selectedActivities = useMemo(() => {
    if (!selectedRunId) return [];
    return mergedRunDetails[selectedRunId]?.activities ?? [];
  }, [selectedRunId, mergedRunDetails]);

  const fetchProjectRuns = useCallback(async (pid: string) => {
    fetchRunsAbortRef.current?.abort();
    const controller = new AbortController();
    fetchRunsAbortRef.current = controller;
    const requestId = ++fetchRunsRequestIdRef.current;

    useProjectGraphStore.getState().setLoading(true);
    try {
      const result = await listPipelineRuns(serverUrl, pid, { signal: controller.signal });
      if (controller.signal.aborted || fetchRunsRequestIdRef.current !== requestId) {
        return;
      }
      const runsArray = Object.values(result.Runs).sort((a, b) => {
        const aTime = a.created_at ? Date.parse(a.created_at) : 0;
        const bTime = b.created_at ? Date.parse(b.created_at) : 0;
        return bTime - aTime;
      });
      useProjectGraphStore.getState().setRuns(pid, runsArray);
    } catch (err) {
      if (controller.signal.aborted || fetchRunsRequestIdRef.current !== requestId) {
        return;
      }
      useProjectGraphStore.getState().setError(messageFromError(err));
    } finally {
      if (fetchRunsAbortRef.current === controller) {
        fetchRunsAbortRef.current = null;
      }
    }
  }, [serverUrl]);

  const fetchProjectCommits = useCallback(async (pid: string, limit: number) => {
    fetchCommitsAbortRef.current?.abort();
    const controller = new AbortController();
    fetchCommitsAbortRef.current = controller;
    const requestId = ++fetchCommitsRequestIdRef.current;

    setCommitsError(null);
    try {
      const result = await getCommits(serverUrl, pid, limit, { signal: controller.signal });
      if (controller.signal.aborted || fetchCommitsRequestIdRef.current !== requestId) {
        return;
      }
      setCommits(result.Commits ?? []);
    } catch (err) {
      if (controller.signal.aborted || fetchCommitsRequestIdRef.current !== requestId) {
        return;
      }
      setCommits([]);
      setCommitsError(messageFromError(err));
    } finally {
      if (fetchCommitsAbortRef.current === controller) {
        fetchCommitsAbortRef.current = null;
      }
    }
  }, [serverUrl]);

  const ensureRunDetails = useCallback(async (targetRunId: string) => {
    if (loadingRunDetailsRef.current.has(targetRunId)) return;

    const storeState = useProjectGraphStore.getState();
    const cached = storeState.expandedRunData[targetRunId];
    const listRun = storeState.runs.find((run) => run.id === targetRunId) ?? null;
    const cachedIsFresh = cached && (!listRun || runVersionTime(cached.run) >= runVersionTime(listRun));
    if (cachedIsFresh) return;

    loadingRunDetailsRef.current.add(targetRunId);
    try {
      const [run, activityBatch] = await Promise.all([
        getPipelineRun(serverUrl, targetRunId),
        getPipelineRunActivity(serverUrl, targetRunId)
      ]);
      useProjectGraphStore.getState().setExpandedRunData(targetRunId, run, activityBatch.Activities ?? []);
    } catch (err) {
      useProjectGraphStore.getState().setError(messageFromError(err));
    } finally {
      loadingRunDetailsRef.current.delete(targetRunId);
    }
  }, [serverUrl]);

  useEffect(() => {
    if (!projectId) {
      fetchRunsAbortRef.current?.abort();
      fetchCommitsAbortRef.current?.abort();
      fetchRunsAbortRef.current = null;
      fetchCommitsAbortRef.current = null;
      useProjectGraphStore.getState().reset();
      loadingRunDetailsRef.current.clear();
      setCommits([]);
      setCommitsError(null);
      setSelection(null);
      setDrawerOpen(false);
      return;
    }
    void fetchProjectRuns(projectId);
  }, [projectId, refreshToken, fetchProjectRuns]);

  useEffect(() => {
    if (!projectId) return;
    const limit = runs.length === 0 ? 4 : 200;
    void fetchProjectCommits(projectId, limit);
  }, [projectId, runs.length, fetchProjectCommits]);

  useEffect(() => {
    return () => {
      fetchRunsAbortRef.current?.abort();
      fetchCommitsAbortRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    if (selection) {
      void ensureRunDetails(selection.runId);
    }
  }, [selection, ensureRunDetails]);

  useEffect(() => {
    if (phase === "completed" || phase === "failed" || phase === "cancelled") {
      const timer = setTimeout(() => {
        useProjectGraphStore.getState().requestRefresh();
      }, 2_000);
      return () => clearTimeout(timer);
    }
  }, [phase]);

  const handleSelectRunEdge = useCallback((targetRunId: string) => {
    setSelection({ kind: "run-edge", runId: targetRunId });
    setDrawerOpen(true);
  }, []);

  const handleSelectStepEdge = useCallback((targetRunId: string, stepId: string) => {
    setSelection({ kind: "step-edge", runId: targetRunId, stepId });
    setDrawerOpen(true);
  }, []);

  const handleClearSelection = useCallback(() => {
    setSelection(null);
    setDrawerOpen(false);
  }, []);

  const handleCloseDrawer = useCallback(() => {
    setDrawerOpen(false);
  }, []);

  return (
    <>
      <section className="run-graph">
        <div className="run-graph-canvas">
          <ProjectGraph
            graphInput={graphInput}
            isLoading={isLoading}
            onSelectRunEdge={handleSelectRunEdge}
            onSelectStepEdge={handleSelectStepEdge}
            onSelectBaseCommit={onSelectBaseCommit}
            onClearSelection={handleClearSelection}
            onForkFromCommit={onForkFromCommit}
          />
          <div className="floating-legend" aria-label="Graph capability legend">
            <span className="run-graph-legend__item">
              <strong>Commit node:</strong> choose base state
            </span>
            <span className="run-graph-legend__item">
              <strong>Run edge:</strong> inspect workflow + replay
            </span>
            <span className="run-graph-legend__item">
              <strong>Step edge:</strong> inspect events + deterministic fork
            </span>
          </div>
        </div>
      </section>
      {projectError && <p className="error-text panel">{projectError}</p>}
      {commitsError && <p className="error-text panel">{commitsError}</p>}
      <EdgeDetailsDrawer
        isOpen={drawerOpen}
        selection={selection}
        run={selectedRun}
        activities={selectedActivities}
        projectId={projectId}
        serverUrl={serverUrl}
        onClose={handleCloseDrawer}
        onSelectBaseCommit={onSelectBaseCommit}
        onRefreshed={() => useProjectGraphStore.getState().requestRefresh()}
      />
    </>
  );
}

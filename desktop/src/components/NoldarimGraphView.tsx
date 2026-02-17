import { useCallback, useEffect, useMemo, useRef } from "react";
import { ReactFlow, Background, Controls, type Edge, type NodeTypes } from "@xyflow/react";

import { PipelineRunStatus, StepStatus, stepStatusToView } from "../lib/types";
import type { StepDraft, StepResult } from "../lib/types";
import { summarizeStepObservability } from "../lib/obs-mapping";
import { buildProjectGraph } from "../lib/graph-layout";
import { isLiveRun } from "../lib/run-phase";
import { getPipelineRun, getPipelineRunActivity, listPipelineRuns } from "../lib/api";
import { StepNode, type StepNodeType } from "./nodes/StepNode";
import { CommitNode } from "./nodes/CommitNode";
import { RunNode } from "./nodes/RunNode";
import { useRunSteps, useStepExecutionMap, useActivitiesByStep } from "../state/selectors";
import { useRunStore } from "../state/run-store";
import { useProjectGraphStore } from "../state/project-graph-store";
import { messageFromError } from "../lib/formatting";

type NoldarimGraphViewProps = {
  projectId: string;
  serverUrl: string;
  selectedStep: StepSelection | null;
  onSelectStep: (selection: StepSelection) => void;
  onDeselectStep: () => void;
};

export type StepSelection = {
  stepId: string;
  runId: string | null;
};

// --- Live run graph ---

const liveNodeTypes: NodeTypes = {
  step: StepNode
};

function createLinearEdges(steps: StepDraft[]): Edge[] {
  const edges: Edge[] = [];
  for (let index = 0; index < steps.length - 1; index += 1) {
    edges.push({
      id: `edge-${steps[index].id}-${steps[index + 1].id}`,
      source: `step-${steps[index].id}`,
      target: `step-${steps[index + 1].id}`,
      animated: true
    });
  }
  return edges;
}

function unresolvedRunningStepIndex(steps: StepDraft[], stepExecutionById: Record<string, StepResult>): number {
  for (let index = 0; index < steps.length; index += 1) {
    const result = stepExecutionById[steps[index].id];
    if (!result) {
      return index;
    }
    if (result.status !== StepStatus.Completed && result.status !== StepStatus.Skipped) {
      return index;
    }
  }
  return -1;
}

function LiveRunGraph({ selectedStep, onSelectStep, onDeselectStep }: { selectedStep: StepSelection | null; onSelectStep: (selection: StepSelection) => void; onDeselectStep: () => void }) {
  const steps = useRunSteps();
  const stepExecutionById = useStepExecutionMap();
  const activitiesByStep = useActivitiesByStep();
  const runStatus = useRunStore((s) => s.run?.status);
  const runId = useRunStore((s) => s.runId);

  const fallbackRunningIndex = useMemo(() => unresolvedRunningStepIndex(steps, stepExecutionById), [steps, stepExecutionById]);

  const nodes = useMemo<StepNodeType[]>(() => {
    return steps.map((step, index) => {
      const result = stepExecutionById[step.id];
      const stepEvents = activitiesByStep[step.id] ?? [];
      const summary = summarizeStepObservability(stepEvents);

      const status = result
        ? stepStatusToView(result.status)
        : runStatus === PipelineRunStatus.Running && index === fallbackRunningIndex
          ? "running"
          : "pending";

      return {
        id: `step-${step.id}`,
        type: "step",
        position: {
          x: index * 320,
          y: 80
        },
        data: {
          runId,
          stepId: step.id,
          stepName: step.name,
          index,
          status,
          inputTokens: result?.input_tokens || summary.inputTokens,
          outputTokens: result?.output_tokens || summary.outputTokens,
          filesChanged: result?.files_changed ?? 0,
          insertions: result?.insertions ?? 0,
          deletions: result?.deletions ?? 0,
          eventCount: summary.eventCount,
          toolUseCount: summary.toolUseCount,
          errorMessage: result?.error_message
        },
        selected: selectedStep?.stepId === step.id && (selectedStep.runId === null || selectedStep.runId === runId),
        draggable: false
      };
    });
  }, [steps, stepExecutionById, activitiesByStep, selectedStep, runStatus, fallbackRunningIndex, runId]);

  const edges = useMemo(() => createLinearEdges(steps), [steps]);

  if (steps.length === 0) {
    return <p className="muted-text">Start a run to visualize steps.</p>;
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={liveNodeTypes}
      fitView
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable
      onNodeClick={(_, node) => onSelectStep({ stepId: node.data.stepId, runId: runId ?? null })}
      onPaneClick={onDeselectStep}
    >
      <Background gap={18} size={1} />
      <Controls showInteractive={false} />
    </ReactFlow>
  );
}

// --- Project graph ---

const projectNodeTypes: NodeTypes = {
  commit: CommitNode,
  run: RunNode,
  step: StepNode
};

function ProjectGraph({ selectedStep, onSelectStep, onExpandRun, onDeselectStep }: {
  selectedStep: StepSelection | null;
  onSelectStep: (selection: StepSelection) => void;
  onExpandRun: (runId: string) => void;
  onDeselectStep: () => void;
}) {
  const runs = useProjectGraphStore((s) => s.runs);
  const expandedRunIds = useProjectGraphStore((s) => s.expandedRunIds);
  const expandedRunData = useProjectGraphStore((s) => s.expandedRunData);
  const isLoading = useProjectGraphStore((s) => s.isLoading);

  const { nodes, edges } = useMemo(
    () => buildProjectGraph({ runs, expandedRunIds, expandedRunData, selectedStep }),
    [runs, expandedRunIds, expandedRunData, selectedStep]
  );

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: { type?: string; data: Record<string, unknown> }) => {
      if (node.type === "run") {
        onExpandRun(node.data.runId as string);
      } else if (node.type === "step") {
        onSelectStep({
          stepId: node.data.stepId as string,
          runId: (node.data.runId as string | null) ?? null
        });
      }
    },
    [onExpandRun, onSelectStep]
  );

  if (runs.length === 0 && !isLoading) {
    return (
      <p className="muted-text">No pipeline runs yet. Start a run to see the project graph.</p>
    );
  }

  if (isLoading && runs.length === 0) {
    return <p className="muted-text">Loading runs...</p>;
  }

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={projectNodeTypes}
      fitView
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable
      onNodeClick={handleNodeClick}
      onPaneClick={onDeselectStep}
    >
      <Background gap={18} size={1} />
      <Controls showInteractive={false} />
    </ReactFlow>
  );
}

// --- NoldarimGraphView (dual-mode + orchestration) ---

export function NoldarimGraphView({ projectId, serverUrl, selectedStep, onSelectStep, onDeselectStep }: NoldarimGraphViewProps) {
  const phase = useRunStore((s) => s.phase);
  const refreshToken = useProjectGraphStore((s) => s.refreshToken);
  const projectError = useProjectGraphStore((s) => s.error);
  const fetchRunsRequestIdRef = useRef(0);
  const fetchRunsAbortRef = useRef<AbortController | null>(null);

  // --- Fetch project runs ---
  const fetchProjectRuns = useCallback(
    async (pid: string) => {
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
    },
    [serverUrl]
  );

  // Fetch runs when project changes or refresh is requested
  useEffect(() => {
    if (projectId) {
      void fetchProjectRuns(projectId);
    } else {
      fetchRunsAbortRef.current?.abort();
      fetchRunsAbortRef.current = null;
      fetchRunsRequestIdRef.current += 1;
      useProjectGraphStore.getState().reset();
    }
  }, [projectId, refreshToken, fetchProjectRuns]);

  useEffect(() => {
    return () => {
      fetchRunsAbortRef.current?.abort();
      fetchRunsAbortRef.current = null;
    };
  }, []);

  // Refresh project graph when a run completes
  useEffect(() => {
    if (phase === "completed" || phase === "failed") {
      const timer = setTimeout(() => {
        useProjectGraphStore.getState().requestRefresh();
      }, 2_000);
      return () => clearTimeout(timer);
    }
  }, [phase]);

  // --- Run expansion ---
  const loadingRunIds = useRef(new Set<string>());

  const onExpandRun = useCallback(
    async (expandRunId: string) => {
      if (loadingRunIds.current.has(expandRunId)) return;

      useProjectGraphStore.getState().toggleRunExpanded(expandRunId);
      const { expandedRunIds } = useProjectGraphStore.getState();

      if (!expandedRunIds.has(expandRunId)) return;

      if (useProjectGraphStore.getState().expandedRunData[expandRunId]) return;

      loadingRunIds.current.add(expandRunId);
      try {
        const [fetchedRun, activityBatch] = await Promise.all([
          getPipelineRun(serverUrl, expandRunId),
          getPipelineRunActivity(serverUrl, expandRunId)
        ]);
        if (useProjectGraphStore.getState().expandedRunIds.has(expandRunId)) {
          useProjectGraphStore.getState().setExpandedRunData(expandRunId, fetchedRun, activityBatch.Activities ?? []);
        }
      } catch (err) {
        if (useProjectGraphStore.getState().expandedRunIds.has(expandRunId)) {
          useProjectGraphStore.getState().toggleRunExpanded(expandRunId);
        }
        useProjectGraphStore.getState().setError(messageFromError(err));
      } finally {
        loadingRunIds.current.delete(expandRunId);
      }
    },
    [serverUrl]
  );

  // --- Step click: load historical run into run-store for drawer ---
  const handleSelectStep = useCallback(
    (selection: StepSelection) => {
      onSelectStep(selection);

      const currentPhase = useRunStore.getState().phase;
      if (!isLiveRun(currentPhase)) {
        const { expandedRunData: erd } = useProjectGraphStore.getState();
        if (selection.runId && erd[selection.runId]) {
          const data = erd[selection.runId];
          if (data.run.step_results?.some((sr) => sr.step_id === selection.stepId)) {
            useRunStore.getState().viewHistoricalRun(data.run, data.activities);
            return;
          }
        }

        // Fallback for legacy data where runId may be missing.
        for (const [, data] of Object.entries(erd)) {
          if (data.run.step_results?.some((sr) => sr.step_id === selection.stepId)) {
            useRunStore.getState().viewHistoricalRun(data.run, data.activities);
            break;
          }
        }
      }
    },
    [onSelectStep]
  );

  const live = isLiveRun(phase);

  return (
    <>
      <section className="panel run-graph">
        <h2>Pipeline Graph</h2>
        <div className="run-graph-canvas">
          {live ? (
            <LiveRunGraph selectedStep={selectedStep} onSelectStep={handleSelectStep} onDeselectStep={onDeselectStep} />
          ) : (
            <ProjectGraph
              selectedStep={selectedStep}
              onSelectStep={handleSelectStep}
              onExpandRun={onExpandRun}
              onDeselectStep={onDeselectStep}
            />
          )}
        </div>
      </section>
      {projectError && <p className="error-text panel">{projectError}</p>}
    </>
  );
}

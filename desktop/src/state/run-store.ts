import { create } from "zustand";

import { PipelineRunStatus } from "../lib/types";
import type { AIActivityRecord, PipelineRun, StepDraft, StepResult } from "../lib/types";

export type RunPhase = "idle" | "starting" | "running" | "cancelling" | "completed" | "failed" | "cancelled";

export type ConnectionStatus = "idle" | "connecting" | "streaming" | "terminal" | "error";

export type RunDefinition = {
  steps: StepDraft[];
  pipelineName: string;
};

export type RunState = {
  phase: RunPhase;
  connectionStatus: ConnectionStatus;
  runId: string | null;
  projectId: string | null;
  runDefinition: RunDefinition;
  run: PipelineRun | null;
  stepExecutionById: Record<string, StepResult>;
  activityByEventId: Record<string, AIActivityRecord>;
  activityByStepId: Record<string, AIActivityRecord[]>;
  error: string | null;
};

export type RunActions = {
  runStarted: (projectId: string, pipelineName: string, steps: StepDraft[]) => void;
  wsConnected: (runId: string) => void;
  wsActivityReceived: (activity: AIActivityRecord) => void;
  snapshotApplied: (run: PipelineRun, activities: AIActivityRecord[]) => void;
  viewHistoricalRun: (run: PipelineRun, activities: AIActivityRecord[]) => void;
  runCancelling: () => void;
  runCancelled: () => void;
  runFailed: (message: string) => void;
  reportError: (message: string | null) => void;
  reset: () => void;
};

const emptyDefinition: RunDefinition = { steps: [], pipelineName: "" };

const initialState: RunState = {
  phase: "idle",
  connectionStatus: "idle",
  runId: null,
  projectId: null,
  runDefinition: emptyDefinition,
  run: null,
  stepExecutionById: {},
  activityByEventId: {},
  activityByStepId: {},
  error: null
};

function stepsFromStepResults(stepResults: StepResult[]): StepDraft[] {
  return [...stepResults]
    .sort((a, b) => a.step_index - b.step_index)
    .map((sr) => ({
      id: sr.step_id,
      name: sr.step_name || sr.step_id,
      prompt: ""
    }));
}

function phaseFromStatus(status: PipelineRunStatus): RunPhase {
  if (status === PipelineRunStatus.Completed) {
    return "completed";
  }
  if (status === PipelineRunStatus.Failed) {
    return "failed";
  }
  return "running";
}

function buildStepExecutionMap(run: PipelineRun): Record<string, StepResult> {
  const map: Record<string, StepResult> = {};
  for (const sr of run.step_results ?? []) {
    map[sr.step_id] = sr;
  }
  return map;
}

function compareActivitiesByTimestamp(a: AIActivityRecord, b: AIActivityRecord): number {
  const aTime = Date.parse(a.timestamp);
  const bTime = Date.parse(b.timestamp);
  const aValid = !Number.isNaN(aTime);
  const bValid = !Number.isNaN(bTime);

  if (aValid && bValid && aTime !== bTime) {
    return aTime - bTime;
  }
  if (aValid && !bValid) {
    return -1;
  }
  if (!aValid && bValid) {
    return 1;
  }
  return a.event_id.localeCompare(b.event_id);
}

function rebuildActivityByStepId(
  activityByEventId: Record<string, AIActivityRecord>,
  draftStepIds: Set<string>
): Record<string, AIActivityRecord[]> {
  const result: Record<string, AIActivityRecord[]> = {};
  for (const stepId of draftStepIds) {
    result[stepId] = [];
  }
  for (const activity of Object.values(activityByEventId)) {
    const stepId = activity.step_id;
    if (stepId && result[stepId]) {
      result[stepId].push(activity);
    }
  }
  for (const events of Object.values(result)) {
    events.sort(compareActivitiesByTimestamp);
  }
  return result;
}

function mergeActivities(
  existing: Record<string, AIActivityRecord>,
  incoming: AIActivityRecord[]
): Record<string, AIActivityRecord> {
  if (incoming.length === 0) {
    return existing;
  }
  const merged = { ...existing };
  for (const a of incoming) {
    merged[a.event_id] = a;
  }
  return merged;
}

export const useRunStore = create<RunState & RunActions>()((set) => ({
  ...initialState,

  runStarted: (projectId, pipelineName, steps) =>
    set({
      phase: "starting",
      connectionStatus: "connecting",
      projectId,
      runDefinition: { steps, pipelineName },
      runId: null,
      run: null,
      stepExecutionById: {},
      activityByEventId: {},
      activityByStepId: Object.fromEntries(steps.map((s) => [s.id, []])),
      error: null
    }),

  wsConnected: (runId) =>
    set({
      phase: "running",
      connectionStatus: "streaming",
      runId
    }),

  wsActivityReceived: (activity) =>
    set((prev) => {
      if (prev.activityByEventId[activity.event_id]) {
        return prev;
      }
      const activityByEventId = { ...prev.activityByEventId, [activity.event_id]: activity };
      const stepId = activity.step_id;
      let activityByStepId = prev.activityByStepId;
      if (stepId && activityByStepId[stepId]) {
        const sortedEvents = [...activityByStepId[stepId], activity].sort(compareActivitiesByTimestamp);
        activityByStepId = {
          ...activityByStepId,
          [stepId]: sortedEvents
        };
      }
      return { activityByEventId, activityByStepId };
    }),

  snapshotApplied: (run, activities) =>
    set((prev) => {
      // Guard: if no prior snapshot has been applied (prev.run is null), skip
      // terminal status — the DB may still hold a stale record from a previous
      // run that shared the same deterministic run ID.
      const isFirstSnapshot = prev.run === null;
      const incomingTerminal =
        run.status === PipelineRunStatus.Completed || run.status === PipelineRunStatus.Failed;

      const phase =
        prev.phase === "cancelling" || prev.phase === "cancelled"
          ? prev.phase
          : isFirstSnapshot && incomingTerminal
            ? prev.phase
            : phaseFromStatus(run.status);

      const error =
        phase === "failed" && run.error_message ? run.error_message : prev.error;

      const stepExecutionById = buildStepExecutionMap(run);
      const activityByEventId = mergeActivities(prev.activityByEventId, activities);
      const draftStepIds = new Set(prev.runDefinition.steps.map((s) => s.id));
      const activityByStepId = rebuildActivityByStepId(activityByEventId, draftStepIds);

      console.debug(
        "[snapshotApplied] phase=%s stepResults=%d stepExecKeys=%o",
        phase,
        run.step_results?.length ?? 0,
        Object.keys(stepExecutionById)
      );

      return {
        run,
        phase,
        error,
        stepExecutionById,
        activityByEventId,
        activityByStepId
      };
    }),

  viewHistoricalRun: (run, activities) =>
    set(() => {
      const steps = stepsFromStepResults(run.step_results ?? []);
      const stepExecutionById = buildStepExecutionMap(run);
      const draftStepIds = new Set(steps.map((s) => s.id));
      const activityByEventId = mergeActivities({}, activities);
      const activityByStepId = rebuildActivityByStepId(activityByEventId, draftStepIds);
      const phase = phaseFromStatus(run.status);

      return {
        phase,
        connectionStatus: "terminal",
        runId: run.id,
        projectId: run.project_id,
        runDefinition: { steps, pipelineName: run.name },
        run,
        stepExecutionById,
        activityByEventId,
        activityByStepId,
        error: run.error_message || null
      };
    }),

  runCancelling: () =>
    set({ phase: "cancelling" }),

  runCancelled: () =>
    set({ phase: "cancelled", connectionStatus: "terminal" }),

  runFailed: (message) =>
    set({ phase: "failed", error: message, connectionStatus: "terminal" }),

  reportError: (message) =>
    set({ error: message, connectionStatus: message ? "error" : "idle" }),

  reset: () => set(initialState)
}));

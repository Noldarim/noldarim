import { useMemo, useState } from "react";
import { PipelineRunStatus } from "../lib/types";
import type { AIActivityRecord, PipelineRun, StepDraft } from "../lib/types";

export type RunPhase = "idle" | "starting" | "running" | "cancelling" | "completed" | "failed" | "cancelled";

export type RunState = {
  phase: RunPhase;
  runId: string | null;
  projectId: string | null;
  pipelineName: string;
  steps: StepDraft[];
  run: PipelineRun | null;
  activities: AIActivityRecord[];
  activityIds: Set<string>;
  error: string | null;
};

const initialState: RunState = {
  phase: "idle",
  runId: null,
  projectId: null,
  pipelineName: "",
  steps: [],
  run: null,
  activities: [],
  activityIds: new Set(),
  error: null
};

function phaseFromStatus(status: PipelineRunStatus): RunPhase {
  if (status === PipelineRunStatus.Completed) {
    return "completed";
  }
  if (status === PipelineRunStatus.Failed) {
    return "failed";
  }
  return "running";
}

export function useRunStore() {
  const [state, setState] = useState<RunState>(initialState);

  // Empty deps: actions are stable across renders because setState is stable.
  const actions = useMemo(
    () => ({
      reset: () => setState(initialState),
      setError: (error: string | null) => {
        setState((prev) => ({ ...prev, error }));
      },
      startRun: (projectId: string, pipelineName: string, steps: StepDraft[]) => {
        setState((prev) => ({
          ...prev,
          phase: "starting",
          projectId,
          pipelineName,
          steps,
          runId: null,
          run: null,
          activities: [],
          activityIds: new Set(),
          error: null
        }));
      },
      setRunStarted: (runId: string) => {
        setState((prev) => ({
          ...prev,
          phase: "running",
          runId
        }));
      },
      setRunData: (run: PipelineRun) => {
        setState((prev) => {
          const phase =
            prev.phase === "cancelling" || prev.phase === "cancelled"
              ? prev.phase
              : phaseFromStatus(run.status);
          const error =
            phase === "failed" && run.error_message ? run.error_message : prev.error;
          return { ...prev, run, phase, error };
        });
      },
      setActivities: (newActivities: AIActivityRecord[]) => {
        setState((prev) => {
          // Merge instead of replace: keep any WS-delivered or previously
          // fetched activities that might not yet appear in the API response
          // (e.g. async activity flush after step completion).
          const merged = new Map<string, AIActivityRecord>();
          for (const a of prev.activities) {
            merged.set(a.event_id, a);
          }
          for (const a of newActivities) {
            merged.set(a.event_id, a);
          }
          return {
            ...prev,
            activities: [...merged.values()],
            activityIds: new Set(merged.keys())
          };
        });
      },
      appendActivity: (activity: AIActivityRecord) => {
        setState((prev) => {
          if (prev.activityIds.has(activity.event_id)) {
            return prev;
          }
          const nextIds = new Set(prev.activityIds);
          nextIds.add(activity.event_id);
          return { ...prev, activities: [...prev.activities, activity], activityIds: nextIds };
        });
      },
      markCancelling: () => {
        setState((prev) => ({ ...prev, phase: "cancelling" }));
      },
      markCancelled: () => {
        setState((prev) => ({ ...prev, phase: "cancelled" }));
      },
      markFailed: (message: string) => {
        setState((prev) => ({ ...prev, phase: "failed", error: message }));
      }
    }),
    []
  );

  return { state, actions };
}

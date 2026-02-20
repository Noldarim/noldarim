import { create } from "zustand";

import type { AIActivityRecord, PipelineRun } from "../lib/types";

export type ExpandedRunData = {
  run: PipelineRun;
  activities: AIActivityRecord[];
};

export type ProjectGraphState = {
  runs: PipelineRun[];
  projectId: string | null;
  isLoading: boolean;
  error: string | null;
  expandedRunData: Record<string, ExpandedRunData>;
  refreshToken: number;
};

export type ProjectGraphActions = {
  setRuns(projectId: string, runs: PipelineRun[]): void;
  setLoading(isLoading: boolean): void;
  setError(error: string | null): void;
  setExpandedRunData(runId: string, run: PipelineRun, activities: AIActivityRecord[]): void;
  requestRefresh(): void;
  reset(): void;
};

const initialState: ProjectGraphState = {
  runs: [],
  projectId: null,
  isLoading: false,
  error: null,
  expandedRunData: {},
  refreshToken: 0
};

export const useProjectGraphStore = create<ProjectGraphState & ProjectGraphActions>()((set) => ({
  ...initialState,

  setRuns: (projectId, runs) =>
    set({
      projectId,
      runs,
      isLoading: false,
      error: null
    }),

  setLoading: (isLoading) => set({ isLoading }),

  setError: (error) => set({ error, isLoading: false }),

  setExpandedRunData: (runId, run, activities) =>
    set((prev) => ({
      expandedRunData: {
        ...prev.expandedRunData,
        [runId]: { run, activities }
      }
    })),

  requestRefresh: () =>
    set((prev) => ({ refreshToken: prev.refreshToken + 1 })),

  reset: () => set(initialState)
}));

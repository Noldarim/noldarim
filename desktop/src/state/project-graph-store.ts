// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

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
    set((prev) => {
      const sameProject = prev.projectId === projectId;
      const nextExpandedRunData: Record<string, ExpandedRunData> = {};

      if (sameProject) {
        const runIds = new Set(runs.map((run) => run.id));
        for (const [runId, detail] of Object.entries(prev.expandedRunData)) {
          if (runIds.has(runId)) {
            nextExpandedRunData[runId] = detail;
          }
        }
      }

      return {
        projectId,
        runs,
        isLoading: false,
        error: null,
        expandedRunData: nextExpandedRunData
      };
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

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { create } from "zustand";

import type { PipelineRunStatus } from "../lib/types";

export type FleetRunStatus = "pending" | "running" | "completed" | "failed";

export type FleetRun = {
  runId: string;
  projectId: string;
  projectName: string;
  name: string;
  status: FleetRunStatus;
  stepCount: number;
  completedSteps: number;
  totalTokens: number;
  startedAt: string | null;
};

export type FleetState = {
  runs: FleetRun[];
};

export type FleetActions = {
  runsUpdated: (runs: FleetRun[]) => void;
  runStatusChanged: (runId: string, status: FleetRunStatus) => void;
  reset: () => void;
};

const initialState: FleetState = {
  runs: []
};

function statusFromNumeric(status: PipelineRunStatus): FleetRunStatus {
  switch (status) {
    case 0:
      return "pending";
    case 1:
      return "running";
    case 2:
      return "completed";
    case 3:
      return "failed";
    default:
      return "pending";
  }
}

export { statusFromNumeric };

export const useFleetStore = create<FleetState & FleetActions>()((set) => ({
  ...initialState,

  runsUpdated: (runs) => set({ runs }),

  runStatusChanged: (runId, status) =>
    set((prev) => ({
      runs: prev.runs.map((r) => (r.runId === runId ? { ...r, status } : r))
    })),

  reset: () => set(initialState)
}));

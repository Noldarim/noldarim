// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useRunStore } from "./run-store";

export function useRunSteps() {
  return useRunStore((s) => s.runDefinition.steps);
}

export function useStepExecutionMap() {
  return useRunStore((s) => s.stepExecutionById);
}

export function useActivitiesByStep() {
  return useRunStore((s) => s.activityByStepId);
}

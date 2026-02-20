// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { RunPhase } from "../state/run-store";

export function isLiveRun(phase: RunPhase): boolean {
  return phase === "starting" || phase === "running" || phase === "cancelling";
}

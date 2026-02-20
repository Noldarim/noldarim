// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

export type GraphSelection =
  | { kind: "run-edge"; runId: string }
  | { kind: "step-edge"; runId: string; stepId: string };

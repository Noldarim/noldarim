// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { AIActivityRecord } from "./types";

export type AttentionSeverity = "error" | "warning" | "info";

export type AttentionFlag = {
  severity: AttentionSeverity;
  message: string;
  eventId: string;
};

/**
 * Scans AI activity records for items that need human attention.
 */
export function scanAttentionFlags(activities: AIActivityRecord[]): AttentionFlag[] {
  const flags: AttentionFlag[] = [];

  for (const event of activities) {
    if (event.event_type === "tool_blocked") {
      flags.push({
        severity: "warning",
        message: `Tool blocked: ${event.tool_name || "unknown"}`,
        eventId: event.event_id
      });
    }

    if (event.event_type === "error") {
      flags.push({
        severity: "error",
        message: event.content_preview || "Agent error",
        eventId: event.event_id
      });
    }

    if (event.event_type === "tool_result" && event.tool_success === false) {
      flags.push({
        severity: "warning",
        message: `Tool failed: ${event.tool_name || "unknown"}${event.tool_error ? ` — ${event.tool_error}` : ""}`,
        eventId: event.event_id
      });
    }

    if (event.event_type === "subagent_stop" && event.tool_error) {
      flags.push({
        severity: "error",
        message: `Sub-agent error: ${event.tool_error}`,
        eventId: event.event_id
      });
    }
  }

  return flags;
}

/**
 * Returns the highest severity from a set of flags.
 */
export function highestSeverity(flags: AttentionFlag[]): AttentionSeverity | null {
  if (flags.length === 0) return null;
  if (flags.some((f) => f.severity === "error")) return "error";
  if (flags.some((f) => f.severity === "warning")) return "warning";
  return "info";
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

/**
 * Observability category mapping.
 * Maps event kinds and types to visual properties for the UI.
 * Adding a new event type requires ZERO frontend changes — it automatically
 * gets the default style for its kind. Optionally add an override for a custom icon.
 */

export type ObsKind = "message" | "tool" | "lifecycle" | "error" | "metric";
export type ObsLevel = "debug" | "info" | "warn" | "error";

export type CategoryStyle = {
  icon: string;
  color: string;
  borderColor: string;
  defaultCollapsed: boolean;
  label: string;
};

/** Default style per ObsKind — the UI fallback for any event type in this kind. */
const KIND_DEFAULTS: Record<ObsKind, CategoryStyle> = {
  message: {
    icon: "💬",
    color: "var(--text-primary)",
    borderColor: "var(--accent-cyan)",
    defaultCollapsed: false,
    label: "Message",
  },
  tool: {
    icon: "⚙",
    color: "var(--accent-gold-text)",
    borderColor: "var(--accent-gold)",
    defaultCollapsed: false,
    label: "Tool",
  },
  lifecycle: {
    icon: "⚡",
    color: "var(--text-tertiary)",
    borderColor: "var(--border-emphasis)",
    defaultCollapsed: true,
    label: "Lifecycle",
  },
  error: {
    icon: "✗",
    color: "var(--accent-rose)",
    borderColor: "var(--accent-rose)",
    defaultCollapsed: false,
    label: "Error",
  },
  metric: {
    icon: "◈",
    color: "var(--accent-cyan)",
    borderColor: "var(--accent-cyan)",
    defaultCollapsed: true,
    label: "Metric",
  },
};

/** Per-event-type overrides — only for types that need special visual treatment. */
const TYPE_OVERRIDES: Partial<Record<string, Partial<CategoryStyle>>> = {
  thinking: { icon: "🧠", color: "var(--accent-violet)", borderColor: "var(--accent-violet)", label: "Thinking" },
  user_prompt: { icon: "▸", color: "var(--status-success)", borderColor: "var(--status-success)", label: "User" },
  ai_output: { icon: "◆", label: "Output" },
  tool_use: { icon: "⚙", label: "Tool Call" },
  tool_result: { icon: "↩", label: "Tool Result" },
  tool_blocked: { icon: "⊘", color: "var(--status-warning)", borderColor: "var(--status-warning)", label: "Blocked" },
  session_start: { icon: "▶", label: "Session Start" },
  session_end: { icon: "■", label: "Session End" },
  subagent_start: { icon: "⑂", color: "var(--accent-cyan)", borderColor: "var(--accent-cyan)", label: "Sub-agent Start" },
  subagent_stop: { icon: "⑂", label: "Sub-agent End" },
  error: { icon: "✗", label: "Error" },
  stop: { icon: "■", label: "Stopped" },
  streaming: { icon: "…", label: "Streaming" },
};

/**
 * Get the visual style for an event.
 * Looks up override first, falls back to kind defaults.
 * Unknown kinds fall back to "message".
 */
export function getEventStyle(kind: ObsKind | string | undefined, eventType: string): CategoryStyle {
  const resolvedKind = (kind && kind in KIND_DEFAULTS ? kind : kindForEventType(eventType)) as ObsKind;
  const base = KIND_DEFAULTS[resolvedKind] ?? KIND_DEFAULTS.message;
  const override = TYPE_OVERRIDES[eventType];
  if (!override) return base;
  return { ...base, ...override };
}

/** Fallback: infer kind from event_type when kind field is not present (backward compat). */
function kindForEventType(eventType: string): ObsKind {
  switch (eventType) {
    case "user_prompt":
    case "ai_output":
    case "thinking":
    case "streaming":
      return "message";
    case "tool_use":
    case "tool_result":
    case "tool_blocked":
      return "tool";
    case "session_start":
    case "session_end":
    case "subagent_start":
    case "subagent_stop":
    case "stop":
      return "lifecycle";
    case "error":
      return "error";
    default:
      return "message";
  }
}

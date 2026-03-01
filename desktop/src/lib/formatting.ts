// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { durationToMs, formatDuration } from "./duration";

/** Shared formatting utilities used across components. */

export function formatTimestamp(value?: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString();
}

export function formatRunTimestamp(ts?: string): string {
  if (!ts) return "";
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return ts;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" }) +
    " " +
    d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

export function formatTokens(n: number | undefined): string {
  if (!n) {
    return "0";
  }
  return n.toLocaleString();
}

export function messageFromError(error: unknown): string {
  return error instanceof Error ? error.message : "Unknown error";
}

export function formatDurationFromNanos(ns: number): string {
  if (!ns || ns <= 0) return "0s";
  return formatDuration(durationToMs(ns));
}

export function extractToolOutputFromPayload(rawPayload?: string): string | null {
  if (!rawPayload) return null;
  try {
    const parsed = JSON.parse(rawPayload);

    if (parsed.toolUseResult) {
      if (parsed.toolUseResult.stdout) return parsed.toolUseResult.stdout;
      if (parsed.toolUseResult.stderr) return parsed.toolUseResult.stderr;
      if (parsed.toolUseResult.file?.content) return parsed.toolUseResult.file.content;
    }

    if (parsed.type === "user" && Array.isArray(parsed.message?.content)) {
      for (const item of parsed.message.content) {
        if (item.type === "tool_result" && item.content) {
          return typeof item.content === "string" ? item.content : JSON.stringify(item.content);
        }
      }
    }

    if (parsed.type === "assistant" && Array.isArray(parsed.message?.content)) {
      for (const item of parsed.message.content) {
        if (item.type === "text" && item.text) {
          return item.text;
        }
      }
    }

    return null;
  } catch (e) {
    if (import.meta.env.DEV) console.warn("extractToolOutputFromPayload: failed to parse raw_payload", e);
    return null;
  }
}


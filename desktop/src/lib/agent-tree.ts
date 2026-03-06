// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { summarizeStepObservability } from "./obs-mapping";
import type { AIActivityRecord } from "./types";

export type AgentStatus = "active" | "done" | "error";

export type AgentNode = {
  agentId: string;
  sessionId: string;
  parentSessionId: string | undefined;
  isSidechain: boolean;
  status: AgentStatus;
  eventCount: number;
  toolUseCount: number;
  inputTokens: number;
  outputTokens: number;
  children: AgentNode[];
  events: AIActivityRecord[];
};

function deriveStatus(events: AIActivityRecord[]): AgentStatus {
  for (const e of events) {
    if (e.event_type === "error") return "error";
    if (e.event_type === "subagent_stop" && e.tool_error) return "error";
  }
  const hasEnd = events.some(
    (e) => e.event_type === "session_end" || e.event_type === "stop" || e.event_type === "subagent_stop"
  );
  return hasEnd ? "done" : "active";
}

/**
 * Transforms a flat list of AIActivityRecord into a tree of AgentNode objects
 * grouped by session_id, using parent_session_id to build hierarchy.
 */
export function buildAgentTree(activities: AIActivityRecord[]): AgentNode[] {
  // Group events by session_id (or agent_id as fallback key)
  const sessionMap = new Map<string, AIActivityRecord[]>();

  for (const event of activities) {
    const key = event.session_id || event.agent_id || "__root__";
    const existing = sessionMap.get(key);
    if (existing) {
      existing.push(event);
    } else {
      sessionMap.set(key, [event]);
    }
  }

  // Build AgentNode for each session
  const nodeMap = new Map<string, AgentNode>();

  for (const [sessionId, events] of sessionMap) {
    const summary = summarizeStepObservability(events);
    const first = events[0];
    nodeMap.set(sessionId, {
      agentId: first.agent_id || sessionId,
      sessionId,
      parentSessionId: first.parent_session_id || undefined,
      isSidechain: first.is_sidechain ?? false,
      status: deriveStatus(events),
      eventCount: summary.eventCount,
      toolUseCount: summary.toolUseCount,
      inputTokens: summary.inputTokens,
      outputTokens: summary.outputTokens,
      children: [],
      events
    });
  }

  // Wire parent-child relationships
  const roots: AgentNode[] = [];

  for (const node of nodeMap.values()) {
    if (node.parentSessionId && nodeMap.has(node.parentSessionId)) {
      nodeMap.get(node.parentSessionId)!.children.push(node);
    } else {
      roots.push(node);
    }
  }

  return roots;
}

/**
 * Collects all event_ids reachable from an agent node (including descendants).
 */
export function collectAgentEventIds(node: AgentNode): Set<string> {
  const ids = new Set<string>();
  const stack: AgentNode[] = [node];
  while (stack.length > 0) {
    const current = stack.pop()!;
    for (const event of current.events) {
      ids.add(event.event_id);
    }
    for (const child of current.children) {
      stack.push(child);
    }
  }
  return ids;
}

/**
 * Finds a node by agentId anywhere in the tree.
 */
export function findAgentNode(roots: AgentNode[], agentId: string): AgentNode | undefined {
  const stack = [...roots];
  while (stack.length > 0) {
    const node = stack.pop()!;
    if (node.agentId === agentId) return node;
    for (const child of node.children) {
      stack.push(child);
    }
  }
  return undefined;
}

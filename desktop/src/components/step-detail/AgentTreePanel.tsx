// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useEffect, useMemo, useState } from "react";

import type { AgentNode } from "../../lib/agent-tree";
import { formatTokens } from "../../lib/formatting";

type Props = {
  roots: AgentNode[];
  onSelectAgent: (agentId: string) => void;
  selectedAgentId?: string | null;
};

function collectParentSessionIds(roots: AgentNode[]): Set<string> {
  const set = new Set<string>();
  const stack = [...roots];
  while (stack.length > 0) {
    const node = stack.pop()!;
    if (node.children.length > 0) {
      set.add(node.sessionId);
      stack.push(...node.children);
    }
  }
  return set;
}

function AgentNodeRow({
  node,
  depth,
  selectedAgentId,
  expandedSet,
  onToggle,
  onSelect
}: {
  node: AgentNode;
  depth: number;
  selectedAgentId?: string | null;
  expandedSet: Set<string>;
  onToggle: (id: string) => void;
  onSelect: (id: string) => void;
}) {
  const hasChildren = node.children.length > 0;
  const isExpanded = expandedSet.has(node.sessionId);
  const isSelected = selectedAgentId === node.agentId;

  const statusClass =
    node.status === "active"
      ? "agent-node--active"
      : node.status === "error"
        ? "agent-node--error"
        : "agent-node--done";

  return (
    <>
      <div
        className={`agent-node ${statusClass} ${isSelected ? "agent-node--selected" : ""}`}
        style={{ paddingLeft: `${depth * 1.2 + 0.5}rem` }}
        role="treeitem"
        aria-expanded={hasChildren ? isExpanded : undefined}
        aria-selected={isSelected}
      >
        {hasChildren && (
          <button
            type="button"
            className="agent-node__toggle"
            onClick={() => onToggle(node.sessionId)}
            aria-label={isExpanded ? "Collapse" : "Expand"}
          >
            {isExpanded ? "\u25BE" : "\u25B8"}
          </button>
        )}
        <button type="button" className="agent-node__label" onClick={() => onSelect(node.agentId)}>
          <span className="agent-node__id">
            {node.isSidechain ? "\u2937 " : ""}
            {node.agentId.length > 16 ? `${node.agentId.slice(0, 16)}...` : node.agentId}
          </span>
          <span className={`agent-node__status agent-node__status--${node.status}`}>
            {node.status}
          </span>
          <span className="agent-node__meta">
            {node.eventCount} events &middot; {node.toolUseCount} tools &middot;{" "}
            {formatTokens(node.inputTokens + node.outputTokens)}
          </span>
        </button>
      </div>
      {hasChildren &&
        isExpanded &&
        node.children.map((child) => (
          <AgentNodeRow
            key={child.sessionId}
            node={child}
            depth={depth + 1}
            selectedAgentId={selectedAgentId}
            expandedSet={expandedSet}
            onToggle={onToggle}
            onSelect={onSelect}
          />
        ))}
    </>
  );
}

export function AgentTreePanel({ roots, onSelectAgent, selectedAgentId }: Props) {
  const [expandedSet, setExpandedSet] = useState<Set<string>>(() => collectParentSessionIds(roots));

  // Auto-expand new parent nodes when roots change (e.g., new agents appear during live streaming)
  const parentIds = useMemo(() => collectParentSessionIds(roots), [roots]);
  useEffect(() => {
    setExpandedSet((prev) => {
      let changed = false;
      const next = new Set(prev);
      for (const id of parentIds) {
        if (!next.has(id)) {
          next.add(id);
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [parentIds]);

  function handleToggle(sessionId: string) {
    setExpandedSet((prev) => {
      const next = new Set(prev);
      if (next.has(sessionId)) {
        next.delete(sessionId);
      } else {
        next.add(sessionId);
      }
      return next;
    });
  }

  if (roots.length === 0) {
    return <p className="muted-text">No agent sessions detected.</p>;
  }

  return (
    <div className="agent-tree" role="tree">
      {roots.map((root) => (
        <AgentNodeRow
          key={root.sessionId}
          node={root}
          depth={0}
          selectedAgentId={selectedAgentId}
          expandedSet={expandedSet}
          onToggle={handleToggle}
          onSelect={onSelectAgent}
        />
      ))}
      {selectedAgentId && (
        <button
          type="button"
          className="agent-tree__clear"
          onClick={() => onSelectAgent("")}
        >
          Clear agent filter
        </button>
      )}
    </div>
  );
}

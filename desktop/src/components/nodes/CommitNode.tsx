// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { memo, useCallback } from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { formatDuration } from "../../lib/duration";
import { formatTokens } from "../../lib/formatting";
import { useForkFromCommit } from "./ForkFromCommitContext";

export type StepMetrics = {
  tokens: number;
  durationMs: number;
  filesChanged: number;
  insertions: number;
  deletions: number;
  commitMessage?: string;
  errorMessage?: string;
};

export type CommitNodeData = {
  sha: string;
  label?: string;
  isGhost?: boolean;
  isStepCommit?: boolean;
  stepName?: string;
  stepStatus?: string;
  stepIndex?: number;
  diffSummary?: string;
  summaryLine?: string;
  isForkPoint?: boolean;
  commitMessage?: string;
  runId?: string;
  stepId?: string;
  stepMetrics?: StepMetrics;
};

export type CommitNodeType = Node<CommitNodeData, "commit">;

export const CommitNode = memo(function CommitNode({ data }: NodeProps<CommitNodeType>) {
  const forkFromCommit = useForkFromCommit();
  const canFork = forkFromCommit && !data.isGhost && data.sha && data.sha !== "unknown";

  const handleFork = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      if (canFork) forkFromCommit(data.sha);
    },
    [canFork, forkFromCommit, data.sha]
  );

  // Ghost nodes that are NOT step commits: render as a tiny pulsing dot
  if (data.isGhost && !data.isStepCommit) {
    return (
      <div className="commit-node--ghost-indicator" title={data.sha}>
        <Handle id="run-target" type="target" position={Position.Left} />
        <Handle id="run-source" type="source" position={Position.Right} />
        <Handle id="run-source-top" type="source" position={Position.Top} />
        <span className="commit-node__pulse-dot" />
      </div>
    );
  }

  const shortSha = data.sha === "unknown" ? "no commit" : data.sha.slice(0, 8);
  const m = data.stepMetrics;

  // Step commit: card layout with index badge, metrics grid, commit message
  // For in-progress steps (isGhost + isStepCommit), renders as step card with status styling
  if (data.isStepCommit) {
    const classNames = ["commit-node", "commit-node--step-card"];
    if (data.stepStatus) classNames.push(`commit-node--step-${data.stepStatus}`);
    if (data.isForkPoint) classNames.push("commit-node--fork-point");

    return (
      <div className={classNames.join(" ")} title={data.sha}>
        <Handle id="run-target" type="target" position={Position.Left} />
        <Handle id="run-source" type="source" position={Position.Right} />
        <Handle id="run-source-top" type="source" position={Position.Top} />
        {data.isForkPoint && <span className="commit-node__fork-indicator" />}
        <div className="commit-node__header">
          <span className="commit-node__index">{(data.stepIndex ?? 0) + 1}</span>
          <span className="commit-node__step-name">{data.stepName}</span>
        </div>
        {m && (
          <dl className="commit-node__stats">
            <div>
              <dt className="commit-node__stats-label">Tokens</dt>
              <dd className="commit-node__stats-value">{formatTokens(m.tokens)}</dd>
            </div>
            <div>
              <dt className="commit-node__stats-label">Duration</dt>
              <dd className="commit-node__stats-value">{formatDuration(m.durationMs)}</dd>
            </div>
            <div>
              <dt className="commit-node__stats-label">Files</dt>
              <dd className="commit-node__stats-value">&Delta;{m.filesChanged} +{m.insertions} -{m.deletions}</dd>
            </div>
          </dl>
        )}
        {m?.commitMessage && (
          <span className="commit-node__commit-msg">{m.commitMessage.slice(0, 50)}</span>
        )}
        {canFork && (
          <button type="button" className="commit-node__fork-btn" onClick={handleFork}>
            Fork
          </button>
        )}
      </div>
    );
  }

  // Regular commit
  const classNames = ["commit-node"];
  if (data.isForkPoint) classNames.push("commit-node--fork-point");

  return (
    <div className={classNames.join(" ")} title={data.sha}>
      <Handle id="run-target" type="target" position={Position.Left} />
      <Handle id="run-source" type="source" position={Position.Right} />
      <Handle id="run-source-top" type="source" position={Position.Top} />
      {data.isForkPoint && <span className="commit-node__fork-indicator" />}
      {data.commitMessage && <span className="commit-node__message">{data.commitMessage}</span>}
      <span className={data.commitMessage ? "commit-node__sha commit-node__sha--secondary" : "commit-node__sha"}>
        {shortSha}
      </span>
      {!data.commitMessage && data.label && <span className="commit-node__label">{data.label}</span>}
      {data.diffSummary && <span className="commit-node__meta">{data.diffSummary}</span>}
    </div>
  );
});

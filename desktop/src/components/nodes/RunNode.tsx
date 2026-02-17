import { memo } from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";

export type RunNodeData = {
  runId: string;
  name: string;
  status: string;
  createdAt?: string;
  isExpanded: boolean;
  errorMessage?: string;
};

export type RunNodeType = Node<RunNodeData, "run">;

import { formatRunTimestamp } from "../../lib/formatting";

export const RunNode = memo(function RunNode({ data }: NodeProps<RunNodeType>) {
  const expandClass = data.isExpanded ? " run-node--expanded" : "";
  const statusClass = data.status === "running" ? " run-node--running" : "";

  return (
    <div className={`run-node${expandClass}${statusClass}`}>
      <Handle type="target" position={Position.Left} />
      <header className="run-node__header">
        <h4 className="run-node__name">{data.name}</h4>
        <span className="run-node__chevron">{data.isExpanded ? "\u25BC" : "\u25B6"}</span>
      </header>
      <div className="run-node__meta">
        <span className={`status-pill status-pill--${data.status}`}>{data.status}</span>
        <span className="run-node__id">{data.runId.slice(0, 8)}</span>
      </div>
      {data.createdAt && (
        <p className="run-node__time muted-text">{formatRunTimestamp(data.createdAt)}</p>
      )}
      {data.errorMessage && (
        <p className="run-node__error">{data.errorMessage}</p>
      )}
      <Handle type="source" position={Position.Right} />
    </div>
  );
});

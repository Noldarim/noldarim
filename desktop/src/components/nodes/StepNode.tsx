import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";

import type { StepStatusView } from "../../lib/types";

export type StepNodeData = {
  stepId: string;
  stepName: string;
  index: number;
  status: StepStatusView;
  inputTokens: number;
  outputTokens: number;
  filesChanged: number;
  insertions: number;
  deletions: number;
  eventCount: number;
  toolUseCount: number;
  errorMessage?: string;
};

export type StepNodeType = Node<StepNodeData, "step">;

const statusLabel: Record<StepStatusView, string> = {
  pending: "Pending",
  running: "Running",
  completed: "Completed",
  failed: "Failed",
  skipped: "Skipped"
};

export function StepNode({ data }: NodeProps<StepNodeType>) {
  return (
    <div className={`step-node step-node--${data.status}`}>
      <Handle type="target" position={Position.Left} />
      <header className="step-node__header">
        <span className="step-node__index">{data.index + 1}</span>
        <div>
          <h4>{data.stepName}</h4>
          <p className="muted-text">{data.stepId}</p>
        </div>
      </header>

      <p className={`status-pill status-pill--${data.status}`}>{statusLabel[data.status]}</p>

      <dl className="step-node__stats">
        <div>
          <dt>Tokens</dt>
          <dd>{data.inputTokens + data.outputTokens}</dd>
        </div>
        <div>
          <dt>Diff</dt>
          <dd>
            {data.filesChanged} files / +{data.insertions} -{data.deletions}
          </dd>
        </div>
        <div>
          <dt>Obs</dt>
          <dd>
            {data.eventCount} events / {data.toolUseCount} tools
          </dd>
        </div>
      </dl>

      {data.errorMessage && (
        <p className="step-node__error">{data.errorMessage}</p>
      )}

      <Handle type="source" position={Position.Right} />
    </div>
  );
}

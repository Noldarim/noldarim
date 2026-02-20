import { memo } from "react";
import type { Node, NodeProps } from "@xyflow/react";
import type { GraphStatus } from "../../lib/graph-layout";

export type PipelineBgNodeData = {
  runId: string;
  runName: string;
  status: GraphStatus;
  width: number;
  height: number;
};

export type PipelineBgNodeType = Node<PipelineBgNodeData, "pipeline-bg">;

const STATUS_DOT_CLASS: Record<GraphStatus, string> = {
  pending: "pipeline-bg-node__dot--pending",
  running: "pipeline-bg-node__dot--running",
  completed: "pipeline-bg-node__dot--completed",
  failed: "pipeline-bg-node__dot--failed",
  cancelled: "pipeline-bg-node__dot--cancelled",
  skipped: "pipeline-bg-node__dot--skipped",
};

export const PipelineBgNode = memo(function PipelineBgNode({ data }: NodeProps<PipelineBgNodeType>) {
  const statusMod = `pipeline-bg-node--${data.status}`;

  return (
    <div
      className={`pipeline-bg-node ${statusMod}`}
      style={{ width: data.width, height: data.height }}
    >
      <div className="pipeline-bg-node__header">
        <span className={`pipeline-bg-node__dot ${STATUS_DOT_CLASS[data.status] ?? ""}`} />
        <span className="pipeline-bg-node__name">{data.runName}</span>
      </div>
    </div>
  );
});

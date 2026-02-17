import { memo } from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";

export type CommitNodeData = {
  sha: string;
  label?: string;
};

export type CommitNodeType = Node<CommitNodeData, "commit">;

export const CommitNode = memo(function CommitNode({ data }: NodeProps<CommitNodeType>) {
  const shortSha = data.sha === "unknown" ? "no commit" : data.sha.slice(0, 8);

  return (
    <div className="commit-node">
      <Handle type="target" position={Position.Left} />
      <span className="commit-node__sha">{shortSha}</span>
      {data.label && <span className="commit-node__label">{data.label}</span>}
      <Handle type="source" position={Position.Right} />
    </div>
  );
});

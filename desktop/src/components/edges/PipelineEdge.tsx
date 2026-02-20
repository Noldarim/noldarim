import { BaseEdge, getSmoothStepPath, Position, type EdgeProps } from "@xyflow/react";

import type { GraphEdgeData } from "../../lib/graph-layout";

export function statusDotColor(status: string): string {
  switch (status) {
    case "running": return "#3ec6e0";
    case "completed": return "#34d399";
    case "failed": return "#f87171";
    case "cancelled": return "#fb923c";
    case "pending": return "#94a3b8";
    case "skipped": return "#fbbf24";
    default: return "#94a3b8";
  }
}

export function PipelineEdge(props: EdgeProps) {
  const {
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    style,
    data: rawData,
    markerEnd
  } = props;

  const data = rawData as GraphEdgeData | undefined;
  const isCrossRow = sourcePosition === Position.Top;
  const [edgePath] = getSmoothStepPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    borderRadius: 8
  });

  // Cross-row edges: neutral styling unless it's a fork edge (which keeps its status color)
  const effectiveStyle = isCrossRow && !data?.isFork
    ? { ...style, stroke: "#475569", strokeWidth: 1.2, opacity: 0.4 }
    : style;

  return <BaseEdge path={edgePath} markerEnd={markerEnd} style={effectiveStyle} />;
}

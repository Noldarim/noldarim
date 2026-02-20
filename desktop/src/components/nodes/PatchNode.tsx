import { memo, useCallback } from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { usePatchExpand } from "./PatchExpandContext";

export type PatchNodeData = {
  runId: string;
  stepId: string;
  stepIndex: number;
  stepName: string;
  stepStatus: string; // StepStatusView

  toolName?: string;
  toolVersion?: string;
  promptPreview?: string;
  promptFull?: string;
  variables?: Record<string, string>;
  definitionHash?: string;
  configAvailable: boolean;
};

export type PatchNodeType = Node<PatchNodeData, "patch">;

export const PatchNode = memo(function PatchNode({ id, data }: NodeProps<PatchNodeType>) {
  const { expandedPatchId, setExpandedPatchId } = usePatchExpand();
  const isExpanded = expandedPatchId === id;

  const handleSeeMore = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      setExpandedPatchId(isExpanded ? null : id);
    },
    [id, isExpanded, setExpandedPatchId]
  );

  const handleCloseOverlay = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      setExpandedPatchId(null);
    },
    [setExpandedPatchId]
  );

  const statusClass = data.stepStatus ? `patch-node--${data.stepStatus}` : "";

  return (
    <div className={`patch-node ${statusClass}`}>
      <Handle id="run-target" type="target" position={Position.Left} />
      <Handle id="run-source" type="source" position={Position.Right} />
      <Handle id="run-source-top" type="source" position={Position.Top} />

      <div className="patch-node__header">
        <span className="patch-node__index">{data.stepIndex + 1}</span>
        <span className="patch-node__step-name">{data.stepName}</span>
      </div>

      {!data.configAvailable && (
        <span className="patch-node__placeholder">Config not loaded</span>
      )}

      {data.configAvailable && (
        <>
          {data.toolName && (
            <span className="patch-node__tool-pill">{data.toolName}</span>
          )}
          {data.promptPreview && (
            <span className="patch-node__prompt-preview">{data.promptPreview}</span>
          )}
          <button className="patch-node__see-more" onClick={handleSeeMore}>
            {isExpanded ? "close" : "see more"}
          </button>
        </>
      )}

      {isExpanded && data.configAvailable && (
        <div className="patch-node__overlay" onClick={(e) => e.stopPropagation()}>
          <div className="patch-node__overlay-header">
            <span className="patch-node__overlay-title">Step Config</span>
            <button className="patch-node__overlay-close" onClick={handleCloseOverlay}>
              &times;
            </button>
          </div>

          {data.toolName && (
            <div className="patch-node__overlay-row">
              <span className="patch-node__overlay-label">Tool</span>
              <span className="patch-node__overlay-value">
                {data.toolName}
                {data.toolVersion && <span className="patch-node__overlay-version"> v{data.toolVersion}</span>}
              </span>
            </div>
          )}

          {data.promptFull && (
            <div className="patch-node__overlay-section">
              <span className="patch-node__overlay-label">Prompt Template</span>
              <pre className="patch-node__overlay-prompt">{data.promptFull}</pre>
            </div>
          )}

          {data.variables && Object.keys(data.variables).length > 0 && (
            <div className="patch-node__overlay-section">
              <span className="patch-node__overlay-label">Variables</span>
              <dl className="patch-node__overlay-vars">
                {Object.entries(data.variables).map(([key, value]) => (
                  <div key={key}>
                    <dt>{key}</dt>
                    <dd>{value}</dd>
                  </div>
                ))}
              </dl>
            </div>
          )}

          {data.definitionHash && (
            <div className="patch-node__overlay-row">
              <span className="patch-node__overlay-label">Hash</span>
              <span className="patch-node__overlay-value patch-node__overlay-hash">
                {data.definitionHash.slice(0, 12)}
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
});

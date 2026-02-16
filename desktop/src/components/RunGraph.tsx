import { useMemo } from "react";
import { ReactFlow, Background, Controls, type Edge, type NodeTypes } from "@xyflow/react";

import { PipelineRunStatus, StepStatus, stepStatusToView } from "../lib/types";
import type { PipelineRun, StepDraft, StepResult } from "../lib/types";
import { summarizeStepObservability, type StepActivityMap } from "../lib/obs-mapping";
import { StepNode, type StepNodeData, type StepNodeType } from "./nodes/StepNode";

type Props = {
  steps: StepDraft[];
  run: PipelineRun | null;
  activitiesByStep: StepActivityMap;
  selectedStepId: string | null;
  onSelectStep: (stepId: string) => void;
};

const nodeTypes: NodeTypes = {
  step: StepNode
};

function createLinearEdges(steps: StepDraft[]): Edge[] {
  const edges: Edge[] = [];
  for (let index = 0; index < steps.length - 1; index += 1) {
    edges.push({
      id: `edge-${steps[index].id}-${steps[index + 1].id}`,
      source: `step-${steps[index].id}`,
      target: `step-${steps[index + 1].id}`,
      animated: true
    });
  }
  return edges;
}

function unresolvedRunningStepIndex(steps: StepDraft[], stepResults: Map<string, StepResult>): number {
  for (let index = 0; index < steps.length; index += 1) {
    const result = stepResults.get(steps[index].id);
    if (!result) {
      return index;
    }
    if (result.status !== StepStatus.Completed && result.status !== StepStatus.Skipped) {
      return index;
    }
  }
  return -1;
}

export function RunGraph({ steps, run, activitiesByStep, selectedStepId, onSelectStep }: Props) {
  const stepResults = useMemo(() => {
    const result = new Map<string, StepResult>();
    for (const step of run?.step_results ?? []) {
      result.set(step.step_id, step);
    }
    return result;
  }, [run]);

  const fallbackRunningIndex = useMemo(() => unresolvedRunningStepIndex(steps, stepResults), [steps, stepResults]);

  const nodes = useMemo<StepNodeType[]>(() => {
    return steps.map((step, index) => {
      const result = stepResults.get(step.id);
      const stepEvents = activitiesByStep[step.id] ?? [];
      const summary = summarizeStepObservability(stepEvents);

      const status = result
        ? stepStatusToView(result.status)
        : run?.status === PipelineRunStatus.Running && index === fallbackRunningIndex
          ? "running"
          : "pending";

      return {
        id: `step-${step.id}`,
        type: "step",
        position: {
          x: index * 320,
          y: 80
        },
        data: {
          stepId: step.id,
          stepName: step.name,
          index,
          status,
          inputTokens: result?.input_tokens || summary.inputTokens,
          outputTokens: result?.output_tokens || summary.outputTokens,
          filesChanged: result?.files_changed ?? 0,
          insertions: result?.insertions ?? 0,
          deletions: result?.deletions ?? 0,
          eventCount: summary.eventCount,
          toolUseCount: summary.toolUseCount,
          errorMessage: result?.error_message
        },
        selected: selectedStepId === step.id,
        draggable: false
      };
    });
  }, [steps, stepResults, activitiesByStep, selectedStepId, run?.status, fallbackRunningIndex]);

  const edges = useMemo(() => createLinearEdges(steps), [steps]);

  if (steps.length === 0) {
    return (
      <section className="panel run-graph run-graph--empty">
        <h2>Pipeline Graph</h2>
        <p className="muted-text">Start a run to visualize steps.</p>
      </section>
    );
  }

  return (
    <section className="panel run-graph">
      <h2>Pipeline Graph</h2>
      <div className="run-graph-canvas">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          fitView
          nodesDraggable={false}
          nodesConnectable={false}
          elementsSelectable
          onNodeClick={(_, node) => onSelectStep(node.data.stepId)}
        >
          <Background gap={18} size={1} />
          <Controls showInteractive={false} />
        </ReactFlow>
      </div>
    </section>
  );
}

import { useMemo } from "react";
import { ReactFlow, Background, Controls, type Edge, type NodeTypes } from "@xyflow/react";

import { PipelineRunStatus, StepStatus, stepStatusToView } from "../lib/types";
import type { StepDraft, StepResult } from "../lib/types";
import { summarizeStepObservability } from "../lib/obs-mapping";
import type { StepActivityMap } from "../lib/obs-mapping";
import { StepNode, type StepNodeType } from "./nodes/StepNode";
import { useRunSteps, useStepExecutionMap, useActivitiesByStep } from "../state/selectors";
import { useRunStore } from "../state/run-store";

type Props = {
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

function unresolvedRunningStepIndex(steps: StepDraft[], stepExecutionById: Record<string, StepResult>): number {
  for (let index = 0; index < steps.length; index += 1) {
    const result = stepExecutionById[steps[index].id];
    if (!result) {
      return index;
    }
    if (result.status !== StepStatus.Completed && result.status !== StepStatus.Skipped) {
      return index;
    }
  }
  return -1;
}

export function RunGraph({ selectedStepId, onSelectStep }: Props) {
  const steps = useRunSteps();
  const stepExecutionById = useStepExecutionMap();
  const activitiesByStep = useActivitiesByStep();
  const runStatus = useRunStore((s) => s.run?.status);

  const fallbackRunningIndex = useMemo(() => unresolvedRunningStepIndex(steps, stepExecutionById), [steps, stepExecutionById]);

  const nodes = useMemo<StepNodeType[]>(() => {
    return steps.map((step, index) => {
      const result = stepExecutionById[step.id];
      const stepEvents = activitiesByStep[step.id] ?? [];
      const summary = summarizeStepObservability(stepEvents);

      const status = result
        ? stepStatusToView(result.status)
        : runStatus === PipelineRunStatus.Running && index === fallbackRunningIndex
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
  }, [steps, stepExecutionById, activitiesByStep, selectedStepId, runStatus, fallbackRunningIndex]);

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

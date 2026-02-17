import type { AIActivityRecord, StepDraft } from "./types";

export type StepActivityMap = Record<string, AIActivityRecord[]>;

export type ToolGroup = {
  toolName: string;
  input: string;
  result?: {
    success: boolean;
    output: string;
    error: string;
  };
};

/**
 * Maps AI activity events to their corresponding pipeline steps using the step_id field.
 * Buckets are always initialised from `steps` (the draft definitions) so that every
 * step has an entry even before its StepResult appears in `run.step_results`.
 * Events without a step_id (legacy data) are excluded gracefully.
 */
export function mapActivitiesToSteps(
  steps: StepDraft[],
  activities: AIActivityRecord[]
): StepActivityMap {
  const mapped: StepActivityMap = {};

  for (const step of steps) {
    mapped[step.id] = [];
  }

  for (const event of activities) {
    const stepId = event.step_id;
    if (stepId && mapped[stepId]) {
      mapped[stepId].push(event);
    }
  }

  return mapped;
}

export function groupToolEvents(events: AIActivityRecord[]): ToolGroup[] {
  const groups: ToolGroup[] = [];
  const pendingIndexes: Record<string, number[]> = {};
  const sorted = [...events].sort((a, b) => Date.parse(a.timestamp) - Date.parse(b.timestamp));

  for (const event of sorted) {
    if (event.event_type === "tool_use") {
      const toolName = event.tool_name || "Unknown";
      groups.push({
        toolName,
        input: event.tool_input_summary || event.content_preview || ""
      });
      if (!pendingIndexes[toolName]) {
        pendingIndexes[toolName] = [];
      }
      pendingIndexes[toolName].push(groups.length - 1);
      continue;
    }

    if (event.event_type === "tool_result") {
      const toolName = event.tool_name || "Unknown";
      const pending = pendingIndexes[toolName];
      if (!pending || pending.length === 0) {
        continue;
      }
      const index = pending.shift();
      if (index === undefined) {
        continue;
      }
      groups[index].result = {
        success: Boolean(event.tool_success),
        output: event.content_preview || "",
        error: event.tool_error || ""
      };
    }
  }

  return groups;
}

export type ToolNameGroup = { toolName: string; calls: ToolGroup[] };

export function groupToolEventsByName(events: AIActivityRecord[]): ToolNameGroup[] {
  const toolGroups = groupToolEvents(events);
  const byName = new Map<string, ToolGroup[]>();
  for (const group of toolGroups) {
    const existing = byName.get(group.toolName);
    existing ? existing.push(group) : byName.set(group.toolName, [group]);
  }
  return Array.from(byName.entries()).map(([toolName, calls]) => ({ toolName, calls }));
}

export function summarizeStepObservability(events: AIActivityRecord[]): {
  eventCount: number;
  toolUseCount: number;
  toolNames: string[];
  inputTokens: number;
  outputTokens: number;
} {
  const toolNames = new Set<string>();
  let toolUseCount = 0;
  let inputTokens = 0;
  let outputTokens = 0;

  for (const event of events) {
    inputTokens += event.input_tokens ?? 0;
    outputTokens += event.output_tokens ?? 0;

    if (event.event_type === "tool_use") {
      toolUseCount += 1;
      if (event.tool_name) {
        toolNames.add(event.tool_name);
      }
    }
  }

  return {
    eventCount: events.length,
    toolUseCount,
    toolNames: [...toolNames],
    inputTokens,
    outputTokens
  };
}

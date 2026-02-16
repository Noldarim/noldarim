import { StepStatus } from "./types";
import type { AIActivityRecord, PipelineRun, StepResult } from "./types";

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

function timestampMs(value?: string): number | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? undefined : parsed;
}

function sortedStepResults(run: PipelineRun): StepResult[] {
  return [...(run.step_results ?? [])].sort((a, b) => a.step_index - b.step_index);
}

/**
 * Maps AI activity events to their corresponding pipeline steps based on time windows.
 * @param now - Injectable clock for deterministic tests. Defaults to current time.
 */
export function mapActivitiesToSteps(
  run: PipelineRun,
  activities: AIActivityRecord[],
  now: Date = new Date()
): StepActivityMap {
  const results = sortedStepResults(run).filter((step) => step.status !== StepStatus.Skipped);
  const mapped: StepActivityMap = {};

  for (const step of results) {
    mapped[step.step_id] = [];
  }
  if (results.length === 0) {
    return mapped;
  }

  const nowMs = now.getTime();
  const runStart = timestampMs(run.started_at);

  const windows = results.map((step, index) => {
    const previous = index > 0 ? results[index - 1] : undefined;
    const previousEnd = previous ? timestampMs(previous.completed_at) : undefined;

    let start = timestampMs(step.started_at);
    if (start === undefined) {
      start = previousEnd ?? runStart ?? nowMs;
    }

    let end = timestampMs(step.completed_at);
    if (end === undefined) {
      end = nowMs;
    }

    if (end < start) {
      end = start;
    }

    return {
      stepId: step.step_id,
      start,
      end,
      hasStartedAt: timestampMs(step.started_at) !== undefined
    };
  });

  const firstStepId = windows[0].stepId;
  const lastStarted = [...windows].reverse().find((window) => window.hasStartedAt);
  const lastStartedStepId = lastStarted?.stepId ?? windows[windows.length - 1].stepId;

  for (const event of activities) {
    const ts = timestampMs(event.timestamp);

    if (ts === undefined) {
      mapped[lastStartedStepId].push(event);
      continue;
    }

    if (ts < windows[0].start) {
      mapped[firstStepId].push(event);
      continue;
    }

    const match = windows.find((window) => ts >= window.start && ts <= window.end);
    if (match) {
      mapped[match.stepId].push(event);
      continue;
    }

    const lastWindow = windows[windows.length - 1];
    if (ts > lastWindow.end) {
      mapped[lastStartedStepId].push(event);
      continue;
    }

    mapped[firstStepId].push(event);
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

import type { z } from "zod/v4";

import type {
  AgentDefaults,
  AIActivityBatchEvent,
  CancelPipelineResult,
  PipelineRun,
  PipelineRunResult,
  PipelineRunsLoadedEvent,
  ProjectsLoadedEvent,
  StartPipelineRequest
} from "./types";
import {
  AgentDefaultsSchema,
  AIActivityBatchEventSchema,
  CancelPipelineResultSchema,
  PipelineRunResultSchema,
  PipelineRunSchema,
  PipelineRunsLoadedEventSchema,
  ProjectsLoadedEventSchema
} from "./schemas";

const DEFAULT_TIMEOUT_MS = 30_000;

async function requestJson<T>(baseUrl: string, path: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T> {
  // Use caller-provided signal if present, otherwise apply a default timeout.
  const signal = init?.signal ?? AbortSignal.timeout(DEFAULT_TIMEOUT_MS);

  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    signal,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const body = (await response.json()) as { error?: string; context?: string };
      if (body.error) {
        message = body.context ? `${body.error}: ${body.context}` : body.error;
      }
    } catch {
      // Keep default status message.
    }
    throw new Error(message);
  }

  const raw: unknown = await response.json();
  return schema.parse(raw);
}

export async function getProjects(baseUrl: string, init?: RequestInit): Promise<ProjectsLoadedEvent> {
  return requestJson(baseUrl, "/api/v1/projects", ProjectsLoadedEventSchema, init);
}

export async function getAgentDefaults(baseUrl: string, init?: RequestInit): Promise<AgentDefaults> {
  return requestJson(baseUrl, "/api/v1/agent/defaults", AgentDefaultsSchema, init);
}

export async function startPipeline(
  baseUrl: string,
  projectId: string,
  payload: StartPipelineRequest
): Promise<PipelineRunResult> {
  return requestJson(baseUrl, `/api/v1/projects/${encodeURIComponent(projectId)}/pipelines`, PipelineRunResultSchema, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function listPipelineRuns(baseUrl: string, projectId: string, init?: RequestInit): Promise<PipelineRunsLoadedEvent> {
  return requestJson(baseUrl, `/api/v1/projects/${encodeURIComponent(projectId)}/pipelines`, PipelineRunsLoadedEventSchema, init);
}

export async function getPipelineRun(baseUrl: string, runId: string): Promise<PipelineRun> {
  return requestJson(baseUrl, `/api/v1/pipelines/${encodeURIComponent(runId)}`, PipelineRunSchema);
}

export async function getPipelineRunActivity(baseUrl: string, runId: string): Promise<AIActivityBatchEvent> {
  return requestJson(baseUrl, `/api/v1/pipelines/${encodeURIComponent(runId)}/activity`, AIActivityBatchEventSchema);
}

export async function cancelPipeline(baseUrl: string, runId: string, reason = "Cancelled from desktop UI"): Promise<CancelPipelineResult> {
  return requestJson(baseUrl, `/api/v1/pipelines/${encodeURIComponent(runId)}/cancel`, CancelPipelineResultSchema, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

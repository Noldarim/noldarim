import type {
  AgentDefaults,
  AIActivityBatchEvent,
  CancelPipelineResult,
  PipelineRun,
  PipelineRunResult,
  ProjectsLoadedEvent,
  StartPipelineRequest
} from "./types";

async function requestJson<T>(baseUrl: string, path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
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

  return (await response.json()) as T;
}

export async function getProjects(baseUrl: string, init?: RequestInit): Promise<ProjectsLoadedEvent> {
  return requestJson<ProjectsLoadedEvent>(baseUrl, "/api/v1/projects", init);
}

export async function getAgentDefaults(baseUrl: string, init?: RequestInit): Promise<AgentDefaults> {
  return requestJson<AgentDefaults>(baseUrl, "/api/v1/agent/defaults", init);
}

export async function startPipeline(
  baseUrl: string,
  projectId: string,
  payload: StartPipelineRequest
): Promise<PipelineRunResult> {
  return requestJson<PipelineRunResult>(baseUrl, `/api/v1/projects/${encodeURIComponent(projectId)}/pipelines`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getPipelineRun(baseUrl: string, runId: string): Promise<PipelineRun> {
  return requestJson<PipelineRun>(baseUrl, `/api/v1/pipelines/${encodeURIComponent(runId)}`);
}

export async function getPipelineRunActivity(baseUrl: string, runId: string): Promise<AIActivityBatchEvent> {
  return requestJson<AIActivityBatchEvent>(baseUrl, `/api/v1/pipelines/${encodeURIComponent(runId)}/activity`);
}

export async function cancelPipeline(baseUrl: string, runId: string, reason = "Cancelled from desktop UI"): Promise<CancelPipelineResult> {
  return requestJson<CancelPipelineResult>(baseUrl, `/api/v1/pipelines/${encodeURIComponent(runId)}/cancel`, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

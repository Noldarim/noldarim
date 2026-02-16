import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  cancelPipeline,
  getAgentDefaults,
  getPipelineRun,
  getPipelineRunActivity,
  getProjects,
  startPipeline
} from "./lib/api";
import { mapActivitiesToSteps } from "./lib/obs-mapping";
import { renderPipelineDraft } from "./lib/pipeline-templating";
import { pipelineTemplates } from "./lib/templates";
import { PipelineRunStatus } from "./lib/types";
import type { AgentDefaults, AIActivityRecord, PipelineDraft, Project, WsEnvelope } from "./lib/types";
import { connectPipelineStream, type WsConnection } from "./lib/ws";
import { StepDetailsDrawer } from "./components/StepDetailsDrawer";
import { PipelineForm } from "./components/PipelineForm";
import { RunGraph } from "./components/RunGraph";
import { RunToolbar } from "./components/RunToolbar";
import { ServerSettings } from "./components/ServerSettings";
import { useRunStore } from "./state/run-store";

const serverUrlStorageKey = "noldarim.desktop.serverUrl";
const defaultServerUrl = "http://127.0.0.1:8080";

function normalizeProjects(projectMap: Record<string, Project>): Project[] {
  return Object.values(projectMap).sort((a, b) => a.name.localeCompare(b.name));
}

function messageFromError(error: unknown): string {
  return error instanceof Error ? error.message : "Unknown error";
}

export default function App() {
  const [serverUrl, setServerUrl] = useState<string>(() => localStorage.getItem(serverUrlStorageKey) || defaultServerUrl);
  const [isConnecting, setIsConnecting] = useState<boolean>(false);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>("");
  const [agentDefaults, setAgentDefaults] = useState<AgentDefaults | null>(null);
  const [selectedStepId, setSelectedStepId] = useState<string | null>(null);

  const { state, actions } = useRunStore();

  const wsRef = useRef<WsConnection | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const hydrateTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hasAutoConnectedRef = useRef<boolean>(false);
  const connectAbortRef = useRef<AbortController | null>(null);
  // Tracks the current run ID for guarding tail hydrations against stale fetches.
  const currentRunIdRef = useRef<string | null>(null);
  currentRunIdRef.current = state.runId;

  const selectedStep = useMemo(
    () => state.steps.find((step) => step.id === selectedStepId) ?? null,
    [state.steps, selectedStepId]
  );

  const activitiesByStep = useMemo(() => {
    if (!state.run) {
      return {};
    }
    return mapActivitiesToSteps(state.run, state.activities);
  }, [state.run, state.activities]);

  const closeRealtime = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    if (hydrateTimerRef.current) {
      clearTimeout(hydrateTimerRef.current);
      hydrateTimerRef.current = null;
    }
  }, []);

  const hydrateRun = useCallback(
    async (runId: string) => {
      const [run, activityBatch] = await Promise.all([
        getPipelineRun(serverUrl, runId),
        getPipelineRunActivity(serverUrl, runId)
      ]);
      actions.setRunData(run);
      actions.setActivities(activityBatch.Activities ?? []);

      if (run.status === PipelineRunStatus.Completed || run.status === PipelineRunStatus.Failed) {
        closeRealtime();
      }
    },
    [actions, closeRealtime, serverUrl]
  );

  // Post-completion tail hydrations: after the run enters a terminal phase,
  // schedule a couple more fetches at short intervals to pick up any
  // activity records that were still being flushed to the database when the
  // main polling loop was torn down.
  useEffect(() => {
    if (
      (state.phase === "completed" || state.phase === "failed") &&
      state.runId
    ) {
      const runId = state.runId;
      const timers = [2_000, 5_000].map((delay) =>
        setTimeout(async () => {
          // Guard: if a new run was started in the meantime, skip the stale fetch.
          if (currentRunIdRef.current !== runId) {
            return;
          }
          try {
            const [run, activityBatch] = await Promise.all([
              getPipelineRun(serverUrl, runId),
              getPipelineRunActivity(serverUrl, runId)
            ]);
            if (currentRunIdRef.current !== runId) {
              return;
            }
            actions.setRunData(run);
            actions.setActivities(activityBatch.Activities ?? []);
          } catch {
            // Ignore errors during tail hydrations — run is already done.
          }
        }, delay)
      );

      return () => {
        for (const timer of timers) {
          clearTimeout(timer);
        }
      };
    }
  }, [state.phase, state.runId, actions, serverUrl]);

  const scheduleHydrate = useCallback(
    (runId: string) => {
      if (hydrateTimerRef.current) {
        return;
      }
      hydrateTimerRef.current = setTimeout(async () => {
        hydrateTimerRef.current = null;
        try {
          await hydrateRun(runId);
        } catch (error) {
          actions.setError(messageFromError(error));
        }
      }, 250);
    },
    [actions, hydrateRun]
  );

  const startRealtime = useCallback(
    (projectId: string, runId: string) => {
      closeRealtime();

      wsRef.current = connectPipelineStream(
        serverUrl,
        projectId,
        runId,
        (message: WsEnvelope) => {
          if (message.type === "error") {
            actions.setError(message.message ?? "WebSocket error");
            return;
          }

          if (message.event_type === "*models.AIActivityRecord" && message.payload) {
            actions.appendActivity(message.payload as AIActivityRecord);
          }

          scheduleHydrate(runId);
        },
        (errorMessage) => {
          actions.setError(errorMessage);
        }
      );

      // Background reconciliation poll — WS is the primary channel for real-time updates.
      pollRef.current = setInterval(() => {
        void hydrateRun(runId).catch((error) => {
          actions.setError(messageFromError(error));
        });
      }, 10_000);
    },
    [actions, closeRealtime, hydrateRun, scheduleHydrate, serverUrl]
  );

  const connect = useCallback(async () => {
    connectAbortRef.current?.abort();
    const controller = new AbortController();
    connectAbortRef.current = controller;

    setIsConnecting(true);
    setConnectionError(null);
    actions.setError(null);
    try {
      const { signal } = controller;
      const [projectsPayload, defaults] = await Promise.all([
        getProjects(serverUrl, { signal }),
        getAgentDefaults(serverUrl, { signal })
      ]);
      const normalizedProjects = normalizeProjects(projectsPayload.Projects ?? {});
      setProjects(normalizedProjects);
      setSelectedProjectId((prev) => prev || normalizedProjects[0]?.id || "");
      setAgentDefaults(defaults);
      localStorage.setItem(serverUrlStorageKey, serverUrl);
    } catch (error) {
      if (controller.signal.aborted) {
        return;
      }
      setConnectionError(messageFromError(error));
    } finally {
      if (!controller.signal.aborted) {
        setIsConnecting(false);
      }
    }
  }, [actions, serverUrl]);

  useEffect(() => {
    if (!hasAutoConnectedRef.current) {
      hasAutoConnectedRef.current = true;
      void connect();
    }
  }, [connect]);

  useEffect(
    () => () => {
      closeRealtime();
    },
    [closeRealtime]
  );

  const onStart = useCallback(
    async (draft: PipelineDraft) => {
      if (!selectedProjectId) {
        throw new Error("Select a project before starting a run.");
      }
      if (!agentDefaults) {
        throw new Error("Connect to server and load agent defaults first.");
      }

      const rendered = renderPipelineDraft(draft);
      closeRealtime(); // Clear any tail timers / WS from a previous run.
      actions.startRun(selectedProjectId, rendered.name, rendered.steps);

      let pipelineCreated = false;
      try {
        const result = await startPipeline(serverUrl, selectedProjectId, {
          name: rendered.name,
          steps: rendered.steps.map((step) => ({
            step_id: step.id,
            name: step.name,
            agent_config: {
              tool_name: agentDefaults.tool_name,
              tool_version: agentDefaults.tool_version,
              prompt_template: step.prompt,
              variables: {},
              tool_options: agentDefaults.tool_options,
              flag_format: agentDefaults.flag_format
            }
          }))
        });

        pipelineCreated = true;
        actions.setRunStarted(result.RunID);
        setSelectedStepId(rendered.steps[0]?.id ?? null);

        // Initial hydration — errors here are non-fatal because the poll will retry.
        try {
          await hydrateRun(result.RunID);
        } catch (hydrateError) {
          actions.setError(messageFromError(hydrateError));
        }

        startRealtime(selectedProjectId, result.RunID);
      } catch (error) {
        if (!pipelineCreated) {
          // Pipeline was never created on the server — safe to reset to idle.
          actions.reset();
        }
        throw error;
      }
    },
    [actions, agentDefaults, closeRealtime, hydrateRun, selectedProjectId, serverUrl, startRealtime]
  );

  const onCancel = useCallback(async () => {
    if (!state.runId) {
      return;
    }

    actions.markCancelling();
    try {
      await cancelPipeline(serverUrl, state.runId);
      actions.markCancelled();
      closeRealtime();
      await hydrateRun(state.runId);
    } catch (error) {
      actions.markFailed(messageFromError(error));
    }
  }, [actions, closeRealtime, hydrateRun, serverUrl, state.runId]);

  const runLocked = state.phase === "starting" || state.phase === "running" || state.phase === "cancelling";

  return (
    <main className="app-shell">
      <header className="app-header">
        <h1>Noldarim Desktop Pipeline Runner</h1>
      </header>

      <div className="app-grid">
        <aside className="left-column">
          <ServerSettings
            serverUrl={serverUrl}
            onServerUrlChange={setServerUrl}
            onConnect={connect}
            isConnecting={isConnecting}
            connectionError={connectionError}
          />

          <section className="panel">
            <h2>Project</h2>
            <select
              value={selectedProjectId}
              onChange={(event) => setSelectedProjectId(event.target.value)}
              disabled={projects.length === 0 || runLocked}
            >
              {projects.length === 0 && <option value="">No projects</option>}
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </section>

          <PipelineForm
            templates={pipelineTemplates}
            onStart={onStart}
            disabled={isConnecting || !selectedProjectId || !agentDefaults || runLocked}
          />
        </aside>

        <section className="center-column">
          <RunToolbar runId={state.runId} phase={state.phase} onCancel={onCancel} />
          <RunGraph
            steps={state.steps}
            run={state.run}
            activitiesByStep={activitiesByStep}
            selectedStepId={selectedStepId}
            onSelectStep={setSelectedStepId}
          />
          {state.error && <p className="error-text panel">{state.error}</p>}
        </section>
      </div>

      <StepDetailsDrawer
        isOpen={selectedStep !== null}
        step={selectedStep}
        result={
          selectedStep
            ? state.run?.step_results?.find((r) => r.step_id === selectedStep.id) ?? null
            : null
        }
        events={selectedStep ? activitiesByStep[selectedStep.id] ?? [] : []}
        onClose={() => setSelectedStepId(null)}
      />
    </main>
  );
}

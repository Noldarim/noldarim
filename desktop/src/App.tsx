import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  cancelPipeline,
  getAgentDefaults,
  getProjects,
  startPipeline
} from "./lib/api";
import { renderPipelineDraft } from "./lib/pipeline-templating";
import { pipelineTemplates } from "./lib/templates";
import type { AgentDefaults, PipelineDraft, Project } from "./lib/types";
import { StepDetailsDrawer } from "./components/StepDetailsDrawer";
import { PipelineForm } from "./components/PipelineForm";
import { RunGraph } from "./components/RunGraph";
import { RunToolbar } from "./components/RunToolbar";
import { ServerSettings } from "./components/ServerSettings";
import { useRunStore } from "./state/run-store";
import { useRunConnection } from "./hooks/useRunConnection";
import { useActivitiesByStep } from "./state/selectors";

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

  // Zustand store selectors
  const phase = useRunStore((s) => s.phase);
  const runId = useRunStore((s) => s.runId);
  const steps = useRunStore((s) => s.runDefinition.steps);
  const run = useRunStore((s) => s.run);
  const error = useRunStore((s) => s.error);
  const stepExecutionById = useRunStore((s) => s.stepExecutionById);
  const activitiesByStep = useActivitiesByStep();

  // Actions from getState() are stable references — safe to destructure outside
  // a selector hook. This avoids subscribing to action identity changes.
  const { runStarted, wsConnected, runCancelling, runCancelled, runFailed, reset } = useRunStore.getState();

  // Connection lifecycle hook — handles WS, polling, hydrations
  const { startRealtime, closeRealtime, hydrateRun } = useRunConnection(serverUrl);

  const hasAutoConnectedRef = useRef<boolean>(false);
  const connectAbortRef = useRef<AbortController | null>(null);

  const selectedStep = useMemo(
    () => steps.find((step) => step.id === selectedStepId) ?? null,
    [steps, selectedStepId]
  );

  const connect = useCallback(async () => {
    connectAbortRef.current?.abort();
    const controller = new AbortController();
    connectAbortRef.current = controller;

    setIsConnecting(true);
    setConnectionError(null);
    useRunStore.getState().reportError(null);
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
    } catch (err) {
      if (controller.signal.aborted) {
        return;
      }
      setConnectionError(messageFromError(err));
    } finally {
      if (!controller.signal.aborted) {
        setIsConnecting(false);
      }
    }
  }, [serverUrl]);

  useEffect(() => {
    if (!hasAutoConnectedRef.current) {
      hasAutoConnectedRef.current = true;
      void connect();
    }
  }, [connect]);

  const onStart = useCallback(
    async (draft: PipelineDraft) => {
      if (!selectedProjectId) {
        throw new Error("Select a project before starting a run.");
      }
      if (!agentDefaults) {
        throw new Error("Connect to server and load agent defaults first.");
      }

      const rendered = renderPipelineDraft(draft);
      closeRealtime();
      runStarted(selectedProjectId, rendered.name, rendered.steps);

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
        wsConnected(result.RunID);
        setSelectedStepId(rendered.steps[0]?.id ?? null);

        startRealtime(selectedProjectId, result.RunID);

        const maxAttempts = 6;
        for (let attempt = 0; attempt < maxAttempts; attempt++) {
          try {
            await hydrateRun(result.RunID);
            break;
          } catch (err) {
            console.warn(`[onStart] initial hydration attempt ${attempt + 1}/${maxAttempts} failed:`, err);
            if (attempt < maxAttempts - 1) {
              await new Promise((r) => setTimeout(r, 1_000 * (attempt + 1)));
            }
          }
        }
      } catch (err) {
        if (!pipelineCreated) {
          reset();
        }
        throw err;
      }
    },
    [agentDefaults, closeRealtime, hydrateRun, runStarted, wsConnected, reset, selectedProjectId, serverUrl, startRealtime]
  );

  const onCancel = useCallback(async () => {
    if (!runId) {
      return;
    }

    runCancelling();
    try {
      await cancelPipeline(serverUrl, runId);
      runCancelled();
      closeRealtime();
      await hydrateRun(runId);
    } catch (err) {
      runFailed(messageFromError(err));
    }
  }, [closeRealtime, hydrateRun, runCancelling, runCancelled, runFailed, serverUrl, runId]);

  const runLocked = phase === "starting" || phase === "running" || phase === "cancelling";

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
          <RunToolbar runId={runId} phase={phase} onCancel={onCancel} />
          <RunGraph
            selectedStepId={selectedStepId}
            onSelectStep={setSelectedStepId}
          />
          {error && <p className="error-text panel">{error}</p>}
        </section>
      </div>

      <StepDetailsDrawer
        isOpen={selectedStep !== null}
        step={selectedStep}
        result={
          selectedStep
            ? stepExecutionById[selectedStep.id] ?? null
            : null
        }
        events={selectedStep ? activitiesByStep[selectedStep.id] ?? [] : []}
        onClose={() => setSelectedStepId(null)}
      />
    </main>
  );
}

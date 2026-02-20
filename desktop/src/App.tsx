// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { lazy, Suspense, useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import {
  cancelPipeline,
  getAgentDefaults,
  getProjects,
  startPipeline
} from "./lib/api";
import { renderPipelineDraft } from "./lib/pipeline-templating";
import { pipelineTemplates } from "./lib/templates";
import { isLiveRun } from "./lib/run-phase";
import { messageFromError } from "./lib/formatting";
import type { AgentDefaults, PipelineDraft, Project } from "./lib/types";
import { NoldarimGraphView } from "./components/NoldarimGraphView";
import { FloatingProjectSelector } from "./components/FloatingProjectSelector";
import { FloatingRunStatus } from "./components/FloatingRunStatus";
import { PipelineFormDialog } from "./components/PipelineFormDialog";
import { useRunStore } from "./state/run-store";
import { useRunConnection } from "./hooks/useRunConnection";

import { DevNavToggle } from "./sandbox/DevNavToggle";

const LazyGraphSandboxPage = import.meta.env.DEV
  ? lazy(() => import("./sandbox/GraphSandboxPage").then((m) => ({ default: m.GraphSandboxPage })))
  : null;

const serverUrlStorageKey = "noldarim.desktop.serverUrl";
const defaultServerUrl = "http://127.0.0.1:8080";

function normalizeProjects(projectMap: Record<string, Project>): Project[] {
  return Object.values(projectMap).sort((a, b) => a.name.localeCompare(b.name));
}

export default function App() {
  const [isSandbox, setIsSandbox] = useState(() => window.location.hash === "#sandbox");

  useEffect(() => {
    const onHashChange = () => setIsSandbox(window.location.hash === "#sandbox");
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  if (import.meta.env.DEV && isSandbox && LazyGraphSandboxPage) {
    return (
      <main className="app-shell">
        <Suspense fallback={<p className="muted-text">Loading sandbox...</p>}>
          <LazyGraphSandboxPage />
        </Suspense>
        <div className="floating-dev-toggle">
          <DevNavToggle isSandbox={isSandbox} />
        </div>
      </main>
    );
  }

  return <AppMain isSandbox={isSandbox} />;
}

function AppMain({ isSandbox }: { isSandbox: boolean }) {
  const [serverUrl] = useState<string>(() => localStorage.getItem(serverUrlStorageKey) || defaultServerUrl);
  const [isConnecting, setIsConnecting] = useState<boolean>(false);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProjectId, setSelectedProjectId] = useState<string>("");
  const [agentDefaults, setAgentDefaults] = useState<AgentDefaults | null>(null);
  const [selectedBaseCommitSha, setSelectedBaseCommitSha] = useState<string | null>(null);

  const [isPipelineDialogOpen, setIsPipelineDialogOpen] = useState(false);

  // Zustand store selectors
  const phase = useRunStore((s) => s.phase);
  const runId = useRunStore((s) => s.runId);
  const error = useRunStore((s) => s.error);

  // Actions from getState() are stable references — safe to destructure outside
  // a selector hook. This avoids subscribing to action identity changes.
  const { runStarted, wsConnected, runCancelling, runCancelled, runFailed, reset } = useRunStore.getState();

  // Connection lifecycle hook — handles WS, polling, hydrations
  const { startRealtime, closeRealtime, hydrateRun } = useRunConnection(serverUrl);

  const hasAutoConnectedRef = useRef<boolean>(false);
  const connectAbortRef = useRef<AbortController | null>(null);

  const connect = useCallback(async () => {
    connectAbortRef.current?.abort();
    const controller = new AbortController();
    connectAbortRef.current = controller;

    setIsConnecting(true);
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
      toast.error(messageFromError(err));
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

  useEffect(() => {
    setSelectedBaseCommitSha(null);
  }, [selectedProjectId]);

  const handleForkFromCommit = useCallback((sha: string) => {
    setSelectedBaseCommitSha(sha);
    setIsPipelineDialogOpen(true);
  }, []);

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
          base_commit_sha: selectedBaseCommitSha || undefined,
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
    [agentDefaults, closeRealtime, hydrateRun, runStarted, wsConnected, reset, selectedProjectId, selectedBaseCommitSha, serverUrl, startRealtime]
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

  const runLocked = isLiveRun(phase);

  return (
    <main className="app-shell">
      <div className="graph-fullscreen">
        <NoldarimGraphView
          projectId={selectedProjectId}
          serverUrl={serverUrl}
          selectedBaseCommitSha={selectedBaseCommitSha}
          onSelectBaseCommit={setSelectedBaseCommitSha}
          onForkFromCommit={handleForkFromCommit}
        />

        <FloatingProjectSelector
          projects={projects}
          selectedProjectId={selectedProjectId}
          onSelectProject={setSelectedProjectId}
          disabled={runLocked}
        />

        {runId && runLocked && (
          <FloatingRunStatus
            runId={runId}
            phase={phase}
            onCancel={onCancel}
          />
        )}

        <PipelineFormDialog
          isOpen={isPipelineDialogOpen}
          onClose={() => setIsPipelineDialogOpen(false)}
          templates={pipelineTemplates}
          onStart={onStart}
          disabled={isConnecting || !selectedProjectId || !agentDefaults || runLocked}
          baseCommitSha={selectedBaseCommitSha}
        />

        {import.meta.env.DEV && (
          <div className="floating-dev-toggle">
            <DevNavToggle isSandbox={isSandbox} />
          </div>
        )}
      </div>
      {error && <p className="error-text panel">{error}</p>}
    </main>
  );
}

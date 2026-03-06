// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useEffect, useMemo, useRef, useState } from "react";

import { cancelPipeline, promotePipeline, startPipeline } from "../lib/api";
import { durationToMs } from "../lib/duration";
import { formatDurationFromNanos, formatRunTimestamp, formatTokens, messageFromError } from "../lib/formatting";
import type { GraphSelection } from "../lib/graph-selection";
import { groupToolEventsByName } from "../lib/obs-mapping";
import { PipelineRunStatus, StepStatus, type AIActivityRecord, type AgentConfigInput, type PipelineRun, type StepResult } from "../lib/types";
import { buildAgentTree, collectAgentEventIds, findAgentNode } from "../lib/agent-tree";
import { StepResultSummary, AgentOutput, GitDiffView, ToolActivityPanel, EventTimeline, AgentTreePanel, AttentionBadge } from "./step-detail";

type TabKey = "overview" | "metrics" | "diff" | "logs" | "agents" | "config";

type SnapshotStep = {
  step_id: string;
  step_name: string;
  step_index: number;
  agent_config: AgentConfigInput | null;
};

type Props = {
  projectId: string;
  serverUrl: string;
  isOpen: boolean;
  selection: GraphSelection | null;
  run: PipelineRun | null;
  activities: AIActivityRecord[];
  onClose: () => void;
  onSelectBaseCommit: (sha: string) => void;
  onRefreshed: () => void;
  onForkCreated?: (runId: string) => void;
};

function parseSnapshots(run: PipelineRun | null): SnapshotStep[] {
  if (!run?.step_snapshots || run.step_snapshots.length === 0) return [];
  const ordered = [...run.step_snapshots].sort((a, b) => a.step_index - b.step_index);
  return ordered.map((snapshot) => {
    let parsed: AgentConfigInput | null = null;
    try {
      parsed = JSON.parse(snapshot.agent_config_json) as AgentConfigInput;
    } catch {
      parsed = null;
    }
    return {
      step_id: snapshot.step_id,
      step_name: snapshot.step_name || snapshot.step_id,
      step_index: snapshot.step_index,
      agent_config: parsed
    };
  });
}

type ParsedConfigJson =
  | { ok: true; variables: Record<string, string>; toolOptions: Record<string, unknown> }
  | { ok: false; error: string };

function parseJsonObject(raw: string, label: string): { ok: true; value: Record<string, unknown> } | { ok: false; error: string } {
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return { ok: false, error: `${label} must be a JSON object.` };
    }
    return { ok: true, value: parsed as Record<string, unknown> };
  } catch (err) {
    const reason = err instanceof Error ? err.message : "Invalid JSON syntax.";
    return { ok: false, error: `${label} is invalid JSON: ${reason}` };
  }
}

function parseEditedConfigJson(variablesJson: string, toolOptionsJson: string): ParsedConfigJson {
  const variablesResult = parseJsonObject(variablesJson, "Variables");
  if (!variablesResult.ok) {
    return variablesResult;
  }

  const variables: Record<string, string> = {};
  for (const [key, value] of Object.entries(variablesResult.value)) {
    if (typeof value !== "string") {
      return { ok: false, error: `Variables JSON values must be strings (invalid key: "${key}").` };
    }
    variables[key] = value;
  }

  const toolOptionsResult = parseJsonObject(toolOptionsJson, "Tool options");
  if (!toolOptionsResult.ok) {
    return toolOptionsResult;
  }

  return {
    ok: true,
    variables,
    toolOptions: toolOptionsResult.value
  };
}

function resolveForkAfterStepId(selectedStepId: string, orderedStepResults: StepResult[]): string | undefined {
  const selectedStep = orderedStepResults.find((result) => result.step_id === selectedStepId);
  if (!selectedStep) {
    return undefined;
  }

  let previousCompleted: StepResult | null = null;
  for (const result of orderedStepResults) {
    if (result.step_index >= selectedStep.step_index) {
      break;
    }
    if (result.status === StepStatus.Completed) {
      previousCompleted = result;
    }
  }
  return previousCompleted?.step_id;
}

export function EdgeDetailsDrawer({
  projectId,
  serverUrl,
  isOpen,
  selection,
  run,
  activities,
  onClose,
  onSelectBaseCommit,
  onRefreshed,
  onForkCreated
}: Props) {
  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionInfo, setActionInfo] = useState<string | null>(null);
  const [promptTemplate, setPromptTemplate] = useState("");
  const [toolName, setToolName] = useState("");
  const [toolVersion, setToolVersion] = useState("");
  const [flagFormat, setFlagFormat] = useState("space");
  const [variablesJson, setVariablesJson] = useState("{}");
  const [toolOptionsJson, setToolOptionsJson] = useState("{}");
  const pendingTimers = useRef<ReturnType<typeof setTimeout>[]>([]);

  const selectedStepId = selection?.kind === "step-edge" ? selection.stepId : null;

  // Reset agent filter when switching steps
  useEffect(() => {
    setSelectedAgentId(null);
  }, [selectedStepId]);

  const snapshots = useMemo(() => parseSnapshots(run), [run]);
  const selectedSnapshot = useMemo(
    () => snapshots.find((snapshot) => snapshot.step_id === selectedStepId) ?? null,
    [snapshots, selectedStepId]
  );
  const selectedStepResult = useMemo(() => {
    if (!run || !selectedStepId) return null;
    return run.step_results?.find((sr) => sr.step_id === selectedStepId) ?? null;
  }, [run, selectedStepId]);

  const orderedStepResults = useMemo(
    () => [...(run?.step_results ?? [])].sort((a, b) => a.step_index - b.step_index),
    [run]
  );

  const runTokens = useMemo(
    () =>
      orderedStepResults.reduce(
        (sum, step) => sum + (step.input_tokens ?? 0) + (step.output_tokens ?? 0),
        0
      ),
    [orderedStepResults]
  );

  const runDurationMs = useMemo(
    () => orderedStepResults.reduce((sum, step) => sum + durationToMs(step.duration), 0),
    [orderedStepResults]
  );

  const stepEvents = useMemo(() => {
    if (!selectedStepId) return activities;
    return activities.filter((event) => event.step_id === selectedStepId);
  }, [activities, selectedStepId]);

  const agentRoots = useMemo(() => buildAgentTree(stepEvents), [stepEvents]);

  const filteredStepEvents = useMemo(() => {
    if (!selectedAgentId) return stepEvents;
    const node = findAgentNode(agentRoots, selectedAgentId);
    if (!node) return stepEvents;
    const ids = collectAgentEventIds(node);
    return stepEvents.filter((e) => ids.has(e.event_id));
  }, [stepEvents, selectedAgentId, agentRoots]);

  const toolGroups = useMemo(
    () => groupToolEventsByName(filteredStepEvents),
    [filteredStepEvents]
  );

  const lastAgentOutput = useMemo(
    () => orderedStepResults.slice().reverse().find((step) => step.agent_output?.trim())?.agent_output || "",
    [orderedStepResults]
  );

  const canEditConfig = selection?.kind === "step-edge" && selectedSnapshot?.agent_config;

  useEffect(() => {
    if (!selection) return;
    setActionError(null);
    setActionInfo(null);
    setActiveTab(selection.kind === "step-edge" ? "config" : "overview");
  }, [selection]);

  useEffect(() => {
    const cfg = selectedSnapshot?.agent_config;
    if (!cfg) return;
    setPromptTemplate(cfg.prompt_template || "");
    setToolName(cfg.tool_name || "");
    setToolVersion(cfg.tool_version || "");
    setFlagFormat(cfg.flag_format || "space");
    setVariablesJson(JSON.stringify(cfg.variables ?? {}, null, 2));
    setToolOptionsJson(JSON.stringify(cfg.tool_options ?? {}, null, 2));
  }, [selectedSnapshot]);

  useEffect(() => {
    if (!isOpen) return;
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      onClose();
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [isOpen, onClose]);

  useEffect(() => {
    return () => {
      for (const id of pendingTimers.current) clearTimeout(id);
      pendingTimers.current = [];
    };
  }, []);

  if (!isOpen || !selection || !run) {
    return null;
  }
  const currentRun = run;
  const currentSelection = selection;

  async function handleCancelRun() {
    if (!currentRun.id) return;
    setIsSubmitting(true);
    setActionError(null);
    setActionInfo(null);
    try {
      await cancelPipeline(serverUrl, currentRun.id, "Cancelled from edge drawer");
      setActionInfo("Cancellation requested.");
      onRefreshed();
    } catch (err) {
      setActionError(messageFromError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handlePromoteToMain() {
    if (!currentRun.id) return;
    setIsSubmitting(true);
    setActionError(null);
    setActionInfo(null);
    try {
      await promotePipeline(serverUrl, currentRun.id);
      setActionInfo("Promote queued.");
      onRefreshed();
      pendingTimers.current.push(setTimeout(onRefreshed, 2_000));
      pendingTimers.current.push(setTimeout(onRefreshed, 5_000));
    } catch (err) {
      setActionError(messageFromError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleRerunFromSource() {
    const steps = snapshots
      .filter((snapshot) => snapshot.agent_config)
      .map((snapshot) => ({
        step_id: snapshot.step_id,
        name: snapshot.step_name,
        agent_config: snapshot.agent_config as AgentConfigInput
      }));
    if (steps.length === 0) {
      setActionError("No stored step config snapshots for this run.");
      return;
    }

    setIsSubmitting(true);
    setActionError(null);
    setActionInfo(null);
    try {
      await startPipeline(serverUrl, projectId, {
        name: `${currentRun.name} rerun`,
        base_commit_sha: currentRun.start_commit_sha || currentRun.base_commit_sha,
        no_auto_fork: true,
        steps
      });
      setActionInfo("Rerun started.");
      onRefreshed();
      // Temporal workflow needs time to persist the run record.
      pendingTimers.current.push(setTimeout(onRefreshed, 2_000));
      pendingTimers.current.push(setTimeout(onRefreshed, 5_000));
    } catch (err) {
      setActionError(messageFromError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  function updatedStepsFromConfigEdit(
    parsedVariables: Record<string, string>,
    parsedToolOptions: Record<string, unknown>
  ): { step_id: string; name: string; agent_config: AgentConfigInput }[] {
    return snapshots.map((snapshot) => {
      if (snapshot.step_id !== selectedStepId) {
        return {
          step_id: snapshot.step_id,
          name: snapshot.step_name,
          agent_config: (snapshot.agent_config ?? {
            tool_name: "claude",
            prompt_template: ""
          }) as AgentConfigInput
        };
      }
      return {
        step_id: snapshot.step_id,
        name: snapshot.step_name,
        agent_config: {
          tool_name: toolName,
          tool_version: toolVersion || undefined,
          prompt_template: promptTemplate,
          flag_format: flagFormat || undefined,
          variables: parsedVariables,
          tool_options: parsedToolOptions
        }
      };
    });
  }

  async function handleForkFromStep() {
    if (currentSelection.kind !== "step-edge") return;
    if (!selectedSnapshot) {
      setActionError("No stored config snapshot for this step.");
      return;
    }
    const parsedConfig = parseEditedConfigJson(variablesJson, toolOptionsJson);
    if (!parsedConfig.ok) {
      setActionError(parsedConfig.error);
      return;
    }

    setIsSubmitting(true);
    setActionError(null);
    setActionInfo(null);
    try {
      const steps = updatedStepsFromConfigEdit(parsedConfig.variables, parsedConfig.toolOptions);
      const forkAfterStepId = resolveForkAfterStepId(currentSelection.stepId, orderedStepResults);
      const sourceCommitSha = currentRun.start_commit_sha || currentRun.base_commit_sha;

      const result = await startPipeline(serverUrl, projectId, {
        name: `${currentRun.name} fork ${currentSelection.stepId}`,
        base_commit_sha: sourceCommitSha || undefined,
        fork_from_run_id: currentRun.id,
        fork_after_step_id: forkAfterStepId,
        no_auto_fork: true,
        steps
      });
      setActionInfo("Fork run started.");
      onRefreshed();
      onForkCreated?.(result.RunID);
      onClose();
    } catch (err) {
      setActionError(messageFromError(err));
    } finally {
      setIsSubmitting(false);
    }
  }

  const statusText = currentRun.status === PipelineRunStatus.Running
    ? "running"
    : currentRun.status === PipelineRunStatus.Completed
      ? "completed"
      : currentRun.status === PipelineRunStatus.Failed
        ? "failed"
        : "pending";

  return (
    <>
      <div className="details-drawer-backdrop" onClick={onClose} />
      <aside className="details-drawer details-drawer--wide" role="complementary" aria-label="Workflow edge details">
        <header className="details-drawer__header">
          <div>
            <h3>{currentSelection.kind === "step-edge" ? selectedSnapshot?.step_name || selectedStepId : currentRun.name}</h3>
            <p className="muted-text">{currentSelection.kind === "step-edge" ? currentSelection.stepId : currentRun.id}</p>
          </div>
          <button type="button" onClick={onClose}>Close</button>
        </header>

        <div className="edge-tabs">
          <button type="button" onClick={() => setActiveTab("overview")} className={activeTab === "overview" ? "active" : ""}>Overview</button>
          <button type="button" onClick={() => setActiveTab("metrics")} className={activeTab === "metrics" ? "active" : ""}>Metrics</button>
          <button type="button" onClick={() => setActiveTab("diff")} className={activeTab === "diff" ? "active" : ""}>Diff</button>
          <button type="button" onClick={() => setActiveTab("logs")} className={activeTab === "logs" ? "active" : ""}>Event Timeline</button>
          <button type="button" onClick={() => setActiveTab("agents")} className={activeTab === "agents" ? "active" : ""}>
            Agents <AttentionBadge activities={stepEvents} />
          </button>
          <button type="button" onClick={() => setActiveTab("config")} className={activeTab === "config" ? "active" : ""}>Fork &amp; Replay Config</button>
        </div>

        {activeTab === "overview" && (
          <section className="drawer-section">
            {currentSelection.kind === "step-edge" && selectedStepResult ? (
              <>
                <StepResultSummary result={selectedStepResult} />
                {selectedStepResult.agent_output?.trim() && (
                  <AgentOutput output={selectedStepResult.agent_output} defaultOpen />
                )}
              </>
            ) : (
              <>
                <dl className="drawer-metrics-grid">
                  <dt>Status</dt>
                  <dd>{statusText}</dd>
                  <dt>Created</dt>
                  <dd>{formatRunTimestamp(currentRun.created_at)}</dd>
                  <dt>Base commit</dt>
                  <dd><code>{(currentRun.base_commit_sha || "").slice(0, 8) || "-"}</code></dd>
                  <dt>Start commit</dt>
                  <dd><code>{(currentRun.start_commit_sha || "").slice(0, 8) || "-"}</code></dd>
                  <dt>Head commit</dt>
                  <dd><code>{(currentRun.head_commit_sha || "").slice(0, 8) || "running"}</code></dd>
                </dl>
                {currentRun.error_message && <p className="error-text"><strong>Error:</strong> {currentRun.error_message}</p>}
                {(currentRun.start_commit_sha || currentRun.base_commit_sha) && (
                  <button
                    type="button"
                    onClick={() => onSelectBaseCommit(currentRun.start_commit_sha || currentRun.base_commit_sha || "")}
                  >
                    Select Source Commit
                  </button>
                )}
                {lastAgentOutput && <AgentOutput output={lastAgentOutput} defaultOpen />}
              </>
            )}
          </section>
        )}

        {activeTab === "metrics" && (
          <section className="drawer-section">
            {currentSelection.kind === "step-edge" && selectedStepResult ? (
              <dl className="drawer-metrics-grid">
                <dt>Input tokens</dt>
                <dd>{formatTokens(selectedStepResult.input_tokens)}</dd>
                <dt>Output tokens</dt>
                <dd>{formatTokens(selectedStepResult.output_tokens)}</dd>
                {(selectedStepResult.cache_read_tokens > 0 || selectedStepResult.cache_create_tokens > 0) && (
                  <>
                    <dt>Cache read</dt>
                    <dd>{formatTokens(selectedStepResult.cache_read_tokens)}</dd>
                    <dt>Cache create</dt>
                    <dd>{formatTokens(selectedStepResult.cache_create_tokens)}</dd>
                  </>
                )}
                <dt>Duration</dt>
                <dd>{formatDurationFromNanos(selectedStepResult.duration)}</dd>
                <dt>Files changed</dt>
                <dd>{selectedStepResult.files_changed} (+{selectedStepResult.insertions} -{selectedStepResult.deletions})</dd>
                <dt>Events</dt>
                <dd>{stepEvents.length}</dd>
              </dl>
            ) : (
              <>
                <dl className="drawer-metrics-grid">
                  <dt>Total tokens</dt>
                  <dd>{formatTokens(runTokens)}</dd>
                  <dt>Duration</dt>
                  <dd title={`${runDurationMs.toFixed(1)}ms`}>{formatDurationFromNanos(orderedStepResults.reduce((sum, s) => sum + s.duration, 0))}</dd>
                  <dt>Steps</dt>
                  <dd>{orderedStepResults.length}</dd>
                  <dt>Events</dt>
                  <dd>{activities.length}</dd>
                </dl>
                {orderedStepResults.length > 0 && (
                  <>
                    <h4 style={{ marginTop: "0.5rem" }}>Per-step breakdown</h4>
                    <table className="drawer-step-table">
                      <thead>
                        <tr>
                          <th>Step</th>
                          <th>Tokens</th>
                          <th>Duration</th>
                          <th>Files</th>
                        </tr>
                      </thead>
                      <tbody>
                        {orderedStepResults.map((step) => (
                          <tr key={step.step_id}>
                            <td>{step.step_name || step.step_id}</td>
                            <td>{formatTokens((step.input_tokens ?? 0) + (step.output_tokens ?? 0))}</td>
                            <td>{formatDurationFromNanos(step.duration)}</td>
                            <td>{step.files_changed} (+{step.insertions} -{step.deletions})</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </>
                )}
              </>
            )}
          </section>
        )}

        {activeTab === "diff" && (
          <section className="drawer-section">
            {currentSelection.kind === "step-edge"
              ? (selectedStepResult?.git_diff
                ? <GitDiffView diff={selectedStepResult.git_diff} inline />
                : <p className="muted-text">No diff for this step.</p>)
              : (orderedStepResults.filter((s) => s.git_diff).length > 0
                ? orderedStepResults
                    .filter((step) => step.git_diff)
                    .map((step) => (
                      <div key={step.step_id}>
                        <h4>{step.step_name || step.step_id}</h4>
                        <GitDiffView diff={step.git_diff} inline />
                      </div>
                    ))
                : <p className="muted-text">No diff for this run.</p>)}
          </section>
        )}

        {activeTab === "logs" && (
          <section className="drawer-section">
            <ToolActivityPanel groups={toolGroups} events={filteredStepEvents} />
            <EventTimeline events={filteredStepEvents} />
          </section>
        )}

        {activeTab === "agents" && (
          <section className="drawer-section">
            <AgentTreePanel
              roots={agentRoots}
              onSelectAgent={(id) => setSelectedAgentId(id || null)}
              selectedAgentId={selectedAgentId}
            />
          </section>
        )}

        {activeTab === "config" && (
          <section className="drawer-section">
            {currentSelection.kind !== "step-edge" && (
              <p className="muted-text">Select a step edge to inspect and edit its fork/replay config.</p>
            )}
            {currentSelection.kind === "step-edge" && !canEditConfig && (
              <p className="muted-text">Replay config snapshot unavailable for this historical step.</p>
            )}
            {currentSelection.kind === "step-edge" && canEditConfig && (
              <div className="edge-config-form">
                <label>
                  Tool
                  <input value={toolName} onChange={(event) => setToolName(event.target.value)} />
                </label>
                <label>
                  Tool Version
                  <input value={toolVersion} onChange={(event) => setToolVersion(event.target.value)} />
                </label>
                <label>
                  Flag Format
                  <select value={flagFormat} onChange={(event) => setFlagFormat(event.target.value)}>
                    <option value="space">space</option>
                    <option value="equals">equals</option>
                  </select>
                </label>
                <label>
                  Prompt Template
                  <textarea rows={8} value={promptTemplate} onChange={(event) => setPromptTemplate(event.target.value)} />
                </label>
                <label>
                  Variables (JSON)
                  <textarea rows={6} value={variablesJson} onChange={(event) => setVariablesJson(event.target.value)} />
                </label>
                <label>
                  Tool Options (JSON)
                  <textarea rows={6} value={toolOptionsJson} onChange={(event) => setToolOptionsJson(event.target.value)} />
                </label>
              </div>
            )}
          </section>
        )}

        <footer className="edge-drawer-actions">
          {currentSelection.kind === "run-edge" && currentRun.status === PipelineRunStatus.Running && (
            <button type="button" onClick={handleCancelRun} disabled={isSubmitting} className="danger-button">
              Cancel Run
            </button>
          )}
          {currentSelection.kind === "run-edge" && (
            <button type="button" onClick={handleRerunFromSource} disabled={isSubmitting}>
              Replay From Source Commit
            </button>
          )}
          {currentSelection.kind === "run-edge" &&
            currentRun.status === PipelineRunStatus.Completed &&
            currentRun.run_type !== "promote" && (
              <button type="button" onClick={handlePromoteToMain} disabled={isSubmitting} className="promote-button">
                Promote to Main
              </button>
            )}
          {currentSelection.kind === "step-edge" && (
            <button type="button" onClick={handleForkFromStep} disabled={isSubmitting || !canEditConfig}>
              Fork Deterministically From Here
            </button>
          )}
          {actionInfo && <p className="success-text">{actionInfo}</p>}
          {actionError && <p className="error-text">{actionError}</p>}
        </footer>
      </aside>
    </>
  );
}

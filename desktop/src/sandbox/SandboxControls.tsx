// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useCallback, useState } from "react";

import { PipelineRunStatus } from "../lib/types";
import type { PipelineRun } from "../lib/types";
import { SCENARIO_NAMES } from "./mock-data";

type SandboxControlsProps = {
  runs: PipelineRun[];
  onLoadScenario: (name: string) => void;
  onAddRun: (stepCount: number, status: "pending" | "running" | "completed" | "failed") => void;
  onAddFork: (parentRunId: string) => void;
  onCycleRunStatus: (runId: string) => void;
  onRemoveRun: (runId: string) => void;
  onClearAll: () => void;
};

const STATUS_CYCLE: PipelineRunStatus[] = [
  PipelineRunStatus.Pending,
  PipelineRunStatus.Running,
  PipelineRunStatus.Completed,
  PipelineRunStatus.Failed,
];

function statusLabel(status: PipelineRunStatus): string {
  switch (status) {
    case PipelineRunStatus.Pending: return "pending";
    case PipelineRunStatus.Running: return "running";
    case PipelineRunStatus.Completed: return "completed";
    case PipelineRunStatus.Failed: return "failed";
    default: return "unknown";
  }
}

export function SandboxControls({
  runs,
  onLoadScenario,
  onAddRun,
  onAddFork,
  onCycleRunStatus,
  onRemoveRun,
  onClearAll,
}: SandboxControlsProps) {
  const [scenario, setScenario] = useState<string>(SCENARIO_NAMES[0]);
  const [stepCount, setStepCount] = useState(2);
  const [addStatus, setAddStatus] = useState<"pending" | "running" | "completed" | "failed">("completed");
  const [forkParent, setForkParent] = useState<string>("");

  const handleLoadScenario = useCallback(() => {
    onLoadScenario(scenario);
  }, [scenario, onLoadScenario]);

  const handleAddRun = useCallback(() => {
    onAddRun(stepCount, addStatus);
  }, [stepCount, addStatus, onAddRun]);

  const handleAddFork = useCallback(() => {
    if (forkParent) {
      onAddFork(forkParent);
    }
  }, [forkParent, onAddFork]);

  return (
    <aside className="sandbox-controls panel">
      <h2>Sandbox Controls</h2>

      <div className="field-group">
        <label>Preset Scenario</label>
        <select value={scenario} onChange={(e) => setScenario(e.target.value)}>
          {SCENARIO_NAMES.map((name) => (
            <option key={name} value={name}>{name}</option>
          ))}
        </select>
        <button type="button" className="primary-button" onClick={handleLoadScenario}>
          Load
        </button>
      </div>

      <div className="field-group">
        <label>Add Run</label>
        <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
          <select value={stepCount} onChange={(e) => setStepCount(Number(e.target.value))}>
            {[1, 2, 3, 4, 5].map((n) => (
              <option key={n} value={n}>{n} step{n > 1 ? "s" : ""}</option>
            ))}
          </select>
          <select value={addStatus} onChange={(e) => setAddStatus(e.target.value as typeof addStatus)}>
            <option value="completed">completed</option>
            <option value="running">running</option>
            <option value="failed">failed</option>
            <option value="pending">pending</option>
          </select>
          <button type="button" className="primary-button" onClick={handleAddRun}>
            Add
          </button>
        </div>
      </div>

      {runs.length > 0 && (
        <div className="field-group">
          <label>Add Fork</label>
          <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
            <select value={forkParent} onChange={(e) => setForkParent(e.target.value)}>
              <option value="">Select parent...</option>
              {runs.map((r) => (
                <option key={r.id} value={r.id}>{r.name}</option>
              ))}
            </select>
            <button type="button" className="primary-button" onClick={handleAddFork} disabled={!forkParent}>
              Fork
            </button>
          </div>
        </div>
      )}

      {runs.length > 0 && (
        <div className="field-group">
          <label>Runs ({runs.length})</label>
          <ul className="sandbox-run-list">
            {runs.map((run) => (
              <li key={run.id} className="sandbox-run-item">
                <span className={`status-pill status-pill--${statusLabel(run.status)}`}>
                  {statusLabel(run.status)}
                </span>
                <span className="sandbox-run-name" title={run.id}>
                  {run.name}
                </span>
                <span className="sandbox-run-steps">
                  {run.step_results?.length ?? 0}s
                </span>
                <button
                  type="button"
                  className="sandbox-btn-small"
                  onClick={() => onCycleRunStatus(run.id)}
                  title="Cycle status"
                >
                  &circlearrowright;
                </button>
                <button
                  type="button"
                  className="sandbox-btn-small sandbox-btn-danger"
                  onClick={() => onRemoveRun(run.id)}
                  title="Remove"
                >
                  &times;
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      <button type="button" onClick={onClearAll} disabled={runs.length === 0}>
        Clear All
      </button>
    </aside>
  );
}

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useMemo } from "react";

import { useFleetStore, type FleetRun } from "../state/fleet-store";
import { formatTokens } from "../lib/formatting";

type Props = {
  onNavigateToRun?: (runId: string, projectId: string) => void;
};

const statusColors: Record<string, string> = {
  running: "var(--status-running)",
  completed: "var(--status-success)",
  failed: "var(--status-danger)",
  pending: "var(--text-tertiary)"
};

function FleetRow({ run, onClick }: { run: FleetRun; onClick?: () => void }) {
  return (
    <div className="fleet-panel__row" onClick={onClick} role="button" tabIndex={0}>
      <span className="fleet-panel__name" title={run.name}>
        {run.name}
      </span>
      <span className="fleet-panel__project">{run.projectName}</span>
      <span
        className="fleet-panel__status"
        style={{ color: statusColors[run.status] ?? "var(--text-secondary)" }}
      >
        {run.status}
      </span>
      <span className="fleet-panel__progress">
        {run.completedSteps}/{run.stepCount}
      </span>
      <span className="fleet-panel__tokens">{formatTokens(run.totalTokens)}</span>
    </div>
  );
}

export function FleetOverviewPanel({ onNavigateToRun }: Props) {
  const runs = useFleetStore((s) => s.runs);

  const [activeRuns, otherRuns] = useMemo(() => {
    const active: FleetRun[] = [];
    const other: FleetRun[] = [];
    for (const r of runs) {
      (r.status === "running" ? active : other).push(r);
    }
    return [active, other] as const;
  }, [runs]);

  if (runs.length === 0) {
    return null;
  }

  return (
    <aside className="fleet-panel" aria-label="Fleet overview">
      <header className="fleet-panel__header">
        <h4>Fleet ({runs.length})</h4>
        {activeRuns.length > 0 && (
          <span className="fleet-panel__active-count" style={{ color: "var(--status-running)" }}>
            {activeRuns.length} active
          </span>
        )}
      </header>
      <div className="fleet-panel__list">
        {activeRuns.map((run) => (
          <FleetRow
            key={run.runId}
            run={run}
            onClick={() => onNavigateToRun?.(run.runId, run.projectId)}
          />
        ))}
        {otherRuns.slice(0, 5).map((run) => (
          <FleetRow
            key={run.runId}
            run={run}
            onClick={() => onNavigateToRun?.(run.runId, run.projectId)}
          />
        ))}
      </div>
    </aside>
  );
}

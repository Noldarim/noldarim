import { useCallback, useMemo, useReducer, useState } from "react";
import "@xyflow/react/dist/style.css";

import { PipelineRunStatus } from "../lib/types";
import type { AIActivityRecord, CommitInfo, PipelineRun } from "../lib/types";
import { generateRun, generateScenario } from "./mock-data";
import { SandboxControls } from "./SandboxControls";
import { SandboxGraphTab, type TabConfig } from "./SandboxGraphTab";

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

type SandboxState = {
  runs: PipelineRun[];
  commits: CommitInfo[];
  runDetails: Record<string, { run: PipelineRun; activities: AIActivityRecord[] }>;
};

type SandboxAction =
  | { type: "load-scenario"; name: string }
  | { type: "add-run"; stepCount: number; status: "pending" | "running" | "completed" | "failed" }
  | { type: "add-fork"; parentRunId: string }
  | { type: "cycle-status"; runId: string }
  | { type: "remove-run"; runId: string }
  | { type: "clear" };

const STATUS_CYCLE: PipelineRunStatus[] = [
  PipelineRunStatus.Pending,
  PipelineRunStatus.Running,
  PipelineRunStatus.Completed,
  PipelineRunStatus.Failed,
];

function nextStatus(current: PipelineRunStatus): PipelineRunStatus {
  const idx = STATUS_CYCLE.indexOf(current);
  return STATUS_CYCLE[(idx + 1) % STATUS_CYCLE.length];
}

function getBaseCommitSha(state: SandboxState): string {
  // Use the last run's head if available, otherwise first commit
  for (let i = state.runs.length - 1; i >= 0; i--) {
    if (state.runs[i].head_commit_sha) return state.runs[i].head_commit_sha!;
  }
  if (state.commits.length > 0) return state.commits[state.commits.length - 1].Hash;
  return "0000000000000000000000000000000000000000";
}

const INITIAL_STATE: SandboxState = { runs: [], commits: [], runDetails: {} };

function reducer(state: SandboxState, action: SandboxAction): SandboxState {
  switch (action.type) {
    case "load-scenario": {
      const scenario = generateScenario(action.name);
      return { runs: scenario.runs, commits: scenario.commits, runDetails: scenario.runDetails };
    }

    case "add-run": {
      const baseSha = getBaseCommitSha(state);
      const result = generateRun({
        stepCount: action.stepCount,
        status: action.status,
        baseCommitSha: baseSha,
      });
      const newDetails = { ...state.runDetails };
      newDetails[result.run.id] = { run: result.run, activities: result.activities };
      return {
        runs: [...state.runs, result.run],
        commits: [...state.commits, ...result.commits],
        runDetails: newDetails,
      };
    }

    case "add-fork": {
      const parent = state.runs.find((r) => r.id === action.parentRunId);
      if (!parent) return state;
      const baseSha = parent.start_commit_sha || getBaseCommitSha(state);
      const forkStepId = parent.step_results?.[0]?.step_id;
      const result = generateRun({
        stepCount: parent.step_results?.length || 2,
        status: "completed",
        baseCommitSha: baseSha,
        parentRunId: parent.id,
        forkAfterStepId: forkStepId,
      });
      const newDetails = { ...state.runDetails };
      newDetails[result.run.id] = { run: result.run, activities: result.activities };
      return {
        runs: [...state.runs, result.run],
        commits: [...state.commits, ...result.commits],
        runDetails: newDetails,
      };
    }

    case "cycle-status": {
      const newRuns = state.runs.map((r) => {
        if (r.id !== action.runId) return r;
        const newSt = nextStatus(r.status);
        return { ...r, status: newSt };
      });
      const newDetails = { ...state.runDetails };
      const detail = newDetails[action.runId];
      if (detail) {
        const updated = newRuns.find((r) => r.id === action.runId)!;
        newDetails[action.runId] = { ...detail, run: updated };
      }
      return { ...state, runs: newRuns, runDetails: newDetails };
    }

    case "remove-run": {
      const newDetails = { ...state.runDetails };
      delete newDetails[action.runId];
      return {
        ...state,
        runs: state.runs.filter((r) => r.id !== action.runId),
        runDetails: newDetails,
      };
    }

    case "clear":
      return INITIAL_STATE;
  }
}

// ---------------------------------------------------------------------------
// Tab definitions
// ---------------------------------------------------------------------------

type TabDef = {
  id: string;
  label: string;
  config: TabConfig;
};

function buildTabs(state: SandboxState): TabDef[] {
  const firstRunId = state.runs[0]?.id ?? null;
  const firstCommitSha = state.commits[0]?.Hash ?? null;
  return [
    { id: "default", label: "Default", config: {} },
    {
      id: "first-highlighted",
      label: "First Highlighted",
      config: { highlightRunId: firstRunId },
    },
    {
      id: "base-selected",
      label: "Base Commit Selected",
      config: { selectedBaseCommitSha: firstCommitSha },
    },
  ];
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function GraphSandboxPage() {
  const [state, dispatch] = useReducer(reducer, INITIAL_STATE);
  const [activeTabId, setActiveTabId] = useState("default");

  const tabs = useMemo(() => buildTabs(state), [state]);
  const activeTab = tabs.find((t) => t.id === activeTabId) ?? tabs[0];

  const baseInput = useMemo(
    () => ({
      runs: state.runs,
      commits: state.commits,
      runDetails: state.runDetails,
    }),
    [state.runs, state.commits, state.runDetails]
  );

  const handleLoadScenario = useCallback((name: string) => dispatch({ type: "load-scenario", name }), []);
  const handleAddRun = useCallback(
    (stepCount: number, status: "pending" | "running" | "completed" | "failed") =>
      dispatch({ type: "add-run", stepCount, status }),
    []
  );
  const handleAddFork = useCallback((parentRunId: string) => dispatch({ type: "add-fork", parentRunId }), []);
  const handleCycleStatus = useCallback((runId: string) => dispatch({ type: "cycle-status", runId }), []);
  const handleRemoveRun = useCallback((runId: string) => dispatch({ type: "remove-run", runId }), []);
  const handleClearAll = useCallback(() => dispatch({ type: "clear" }), []);

  return (
    <div className="sandbox-page">
      <SandboxControls
        runs={state.runs}
        onLoadScenario={handleLoadScenario}
        onAddRun={handleAddRun}
        onAddFork={handleAddFork}
        onCycleRunStatus={handleCycleStatus}
        onRemoveRun={handleRemoveRun}
        onClearAll={handleClearAll}
      />

      <div className="sandbox-main">
        <nav className="sandbox-tab-bar">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              className={`sandbox-tab ${tab.id === activeTabId ? "sandbox-tab--active" : ""}`}
              onClick={() => setActiveTabId(tab.id)}
            >
              {tab.label}
            </button>
          ))}
        </nav>

        <div className="sandbox-graph-area">
          <SandboxGraphTab
            key={activeTabId}
            baseInput={baseInput}
            config={activeTab.config}
          />
        </div>
      </div>
    </div>
  );
}

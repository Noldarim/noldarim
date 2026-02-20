// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useCallback, useMemo, useState } from "react";

import { ProjectGraph } from "../components/NoldarimGraphView";
import type { GraphInput } from "../lib/graph-layout";

export type TabConfig = {
  /** Override highlightedRunId */
  highlightRunId?: string | null;
  /** Override selectedBaseCommitSha */
  selectedBaseCommitSha?: string | null;
};

type SandboxGraphTabProps = {
  /** Base graph input from sandbox state (runs, commits, runDetails) */
  baseInput: Omit<GraphInput, "highlightedRunId" | "selectedStep" | "selectedBaseCommitSha">;
  config: TabConfig;
};

export function SandboxGraphTab({ baseInput, config }: SandboxGraphTabProps) {
  const [highlightedRunId, setHighlightedRunId] = useState<string | null>(
    config.highlightRunId ?? null
  );
  const [selectedStep, setSelectedStep] = useState<{ runId: string; stepId: string } | null>(null);
  const [selectedBaseCommitSha, setSelectedBaseCommitSha] = useState<string | null>(
    config.selectedBaseCommitSha ?? null
  );

  const graphInput = useMemo<GraphInput>(
    () => ({
      ...baseInput,
      highlightedRunId,
      selectedStep,
      selectedBaseCommitSha,
    }),
    [baseInput, highlightedRunId, selectedStep, selectedBaseCommitSha]
  );

  const handleSelectRunEdge = useCallback(
    (runId: string) => {
      setHighlightedRunId(runId);
    },
    []
  );

  const handleSelectStepEdge = useCallback((runId: string, stepId: string) => {
    setSelectedStep({ runId, stepId });
  }, []);

  const handleSelectBaseCommit = useCallback((sha: string) => {
    setSelectedBaseCommitSha(sha);
  }, []);

  const handleClearSelection = useCallback(() => {
    setHighlightedRunId(null);
    setSelectedStep(null);
  }, []);

  return (
    <div className="sandbox-graph-tab">
      <div className="run-graph-canvas">
        <ProjectGraph
          graphInput={graphInput}
          isLoading={false}
          onSelectRunEdge={handleSelectRunEdge}
          onSelectStepEdge={handleSelectStepEdge}
          onSelectBaseCommit={handleSelectBaseCommit}
          onClearSelection={handleClearSelection}
        />
      </div>
    </div>
  );
}

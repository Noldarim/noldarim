// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";

import { EdgeDetailsDrawer } from "./EdgeDetailsDrawer";
import { startPipeline } from "../lib/api";
import { PipelineRunStatus, StepStatus, type PipelineRun, type StepResult } from "../lib/types";
import type { GraphSelection } from "../lib/graph-selection";

vi.mock("../lib/api", () => ({
  cancelPipeline: vi.fn(),
  startPipeline: vi.fn()
}));

const mockedStartPipeline = vi.mocked(startPipeline);

function makeStepResult(
  overrides: Partial<StepResult> & { step_id: string; step_index: number; status?: StepStatus }
): StepResult {
  const { step_id, step_index, status, ...rest } = overrides;
  return {
    id: `sr-${step_id}`,
    pipeline_run_id: "run-1",
    step_id,
    step_name: step_id,
    step_index,
    status: status ?? StepStatus.Completed,
    commit_sha: "",
    commit_message: "",
    git_diff: "",
    files_changed: 0,
    insertions: 0,
    deletions: 0,
    input_tokens: 0,
    output_tokens: 0,
    cache_read_tokens: 0,
    cache_create_tokens: 0,
    agent_output: "",
    duration: 0,
    ...rest
  };
}

function makeRun(overrides: Partial<PipelineRun> = {}): PipelineRun {
  return {
    id: "run-1",
    project_id: "proj-1",
    name: "Pipeline",
    status: PipelineRunStatus.Completed,
    base_commit_sha: "base0000",
    start_commit_sha: "start1111",
    step_results: [],
    step_snapshots: [],
    ...overrides
  };
}

function renderDrawer(options?: {
  selection?: GraphSelection;
  run?: PipelineRun;
  onClose?: () => void;
  onRefreshed?: () => void;
}) {
  const selection = options?.selection ?? { kind: "run-edge", runId: "run-1" };
  render(
    <EdgeDetailsDrawer
      projectId="proj-1"
      serverUrl="http://localhost:8080"
      isOpen
      selection={selection}
      run={options?.run ?? makeRun()}
      activities={[]}
      onClose={options?.onClose ?? vi.fn()}
      onSelectBaseCommit={vi.fn()}
      onRefreshed={options?.onRefreshed ?? vi.fn()}
    />
  );
}

describe("EdgeDetailsDrawer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedStartPipeline.mockResolvedValue({
      RunID: "run-new",
      ProjectID: "proj-1",
      Name: "Pipeline",
      WorkflowID: "wf-1"
    });
  });

  it("closes when Escape is pressed", () => {
    const onClose = vi.fn();
    renderDrawer({ onClose });

    fireEvent.keyDown(window, { key: "Escape" });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("uses start_commit_sha for rerun payload when available", async () => {
    const run = makeRun({
      step_snapshots: [
        {
          run_id: "run-1",
          step_id: "s1",
          step_index: 0,
          step_name: "S1",
          agent_config_json: JSON.stringify({ tool_name: "claude", prompt_template: "do work" }),
          definition_hash: "hash-s1"
        }
      ]
    });
    renderDrawer({ run });

    fireEvent.click(screen.getByRole("button", { name: "Replay From Source Commit" }));

    await waitFor(() => expect(mockedStartPipeline).toHaveBeenCalledTimes(1));
    expect(mockedStartPipeline).toHaveBeenCalledWith(
      "http://localhost:8080",
      "proj-1",
      expect.objectContaining({
        base_commit_sha: "start1111"
      })
    );
  });

  it("uses previous completed step for fork anchor when skipped steps exist", async () => {
    const run = makeRun({
      step_results: [
        makeStepResult({ step_id: "s1", step_index: 0, status: StepStatus.Completed }),
        makeStepResult({ step_id: "s2", step_index: 1, status: StepStatus.Skipped }),
        makeStepResult({ step_id: "s3", step_index: 2, status: StepStatus.Completed })
      ],
      step_snapshots: [
        {
          run_id: "run-1",
          step_id: "s1",
          step_index: 0,
          step_name: "S1",
          agent_config_json: JSON.stringify({ tool_name: "claude", prompt_template: "s1" }),
          definition_hash: "hash-s1"
        },
        {
          run_id: "run-1",
          step_id: "s2",
          step_index: 1,
          step_name: "S2",
          agent_config_json: JSON.stringify({ tool_name: "claude", prompt_template: "s2" }),
          definition_hash: "hash-s2"
        },
        {
          run_id: "run-1",
          step_id: "s3",
          step_index: 2,
          step_name: "S3",
          agent_config_json: JSON.stringify({ tool_name: "claude", prompt_template: "s3" }),
          definition_hash: "hash-s3"
        }
      ]
    });

    renderDrawer({
      run,
      selection: { kind: "step-edge", runId: "run-1", stepId: "s3" }
    });

    await screen.findByLabelText("Variables (JSON)");
    fireEvent.click(screen.getByRole("button", { name: "Fork Deterministically From Here" }));

    await waitFor(() => expect(mockedStartPipeline).toHaveBeenCalledTimes(1));
    expect(mockedStartPipeline).toHaveBeenCalledWith(
      "http://localhost:8080",
      "proj-1",
      expect.objectContaining({
        base_commit_sha: "start1111",
        fork_after_step_id: "s1"
      })
    );
  });

  it("shows friendly validation error when variables JSON is invalid", async () => {
    const run = makeRun({
      step_results: [makeStepResult({ step_id: "s1", step_index: 0, status: StepStatus.Completed })],
      step_snapshots: [
        {
          run_id: "run-1",
          step_id: "s1",
          step_index: 0,
          step_name: "S1",
          agent_config_json: JSON.stringify({ tool_name: "claude", prompt_template: "s1", variables: {} }),
          definition_hash: "hash-s1"
        }
      ]
    });

    renderDrawer({
      run,
      selection: { kind: "step-edge", runId: "run-1", stepId: "s1" }
    });

    const variablesInput = await screen.findByLabelText("Variables (JSON)");
    fireEvent.change(variablesInput, { target: { value: "{ invalid" } });
    fireEvent.click(screen.getByRole("button", { name: "Fork Deterministically From Here" }));

    expect(await screen.findByText(/Variables is invalid JSON/i)).toBeInTheDocument();
    expect(mockedStartPipeline).not.toHaveBeenCalled();
  });
});

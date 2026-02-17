import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { RunGraph } from "./RunGraph";
import { StepDetailsDrawer } from "./StepDetailsDrawer";
import type { PipelineRun, StepDraft } from "../lib/types";
import { useRunStore } from "../state/run-store";

const steps: StepDraft[] = [
  { id: "analyze", name: "Analyze", prompt: "Analyze changes" },
  { id: "implement", name: "Implement", prompt: "Implement changes" }
];

function setupStore(storeSteps: StepDraft[], run: PipelineRun | null) {
  act(() => {
    const { runStarted, wsConnected, snapshotApplied } = useRunStore.getState();
    runStarted("proj-1", "Pipeline", storeSteps);
    wsConnected("run-1");
    if (run) {
      snapshotApplied(run, []);
    }
  });
}

function renderGraph(run: PipelineRun | null, overrides?: { steps?: StepDraft[] }) {
  const s = overrides?.steps ?? steps;
  setupStore(s, run);
  render(
    <div style={{ width: "1200px", height: "700px" }}>
      <RunGraph selectedStepId={null} onSelectStep={() => {}} />
    </div>
  );
}

afterEach(() => {
  useRunStore.getState().reset();
});

describe("RunGraph", () => {
  it("renders pending nodes before run data arrives", () => {
    renderGraph(null);

    expect(screen.getByText("Analyze")).toBeInTheDocument();
    expect(screen.getByText("Implement")).toBeInTheDocument();
    expect(screen.getAllByText("Pending").length).toBeGreaterThan(0);
  });

  it("reflects step status transitions from run data", () => {
    renderGraph({
      id: "run-1",
      project_id: "project-1",
      name: "Pipeline",
      status: 1,
      step_results: [
        {
          id: "step-result-1",
          pipeline_run_id: "run-1",
          step_id: "analyze",
          step_index: 0,
          status: 2,
          commit_sha: "",
          commit_message: "",
          git_diff: "",
          files_changed: 1,
          insertions: 2,
          deletions: 3,
          input_tokens: 20,
          output_tokens: 30,
          cache_read_tokens: 0,
          cache_create_tokens: 0,
          agent_output: "",
          duration: 0
        },
        {
          id: "step-result-2",
          pipeline_run_id: "run-1",
          step_id: "implement",
          step_index: 1,
          status: 1,
          commit_sha: "",
          commit_message: "",
          git_diff: "",
          files_changed: 0,
          insertions: 0,
          deletions: 0,
          input_tokens: 5,
          output_tokens: 5,
          cache_read_tokens: 0,
          cache_create_tokens: 0,
          agent_output: "",
          duration: 0
        }
      ]
    });

    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("renders all draft nodes after a snapshot with partial step_results", () => {
    const threeSteps: StepDraft[] = [
      { id: "step-1", name: "Step 1", prompt: "" },
      { id: "step-2", name: "Step 2", prompt: "" },
      { id: "step-3", name: "Step 3", prompt: "" }
    ];

    renderGraph(
      {
        id: "run-1",
        project_id: "proj-1",
        name: "Pipeline",
        status: 1,
        step_results: [
          {
            id: "sr-1",
            pipeline_run_id: "run-1",
            step_id: "step-1",
            step_index: 0,
            status: 2,
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
            duration: 0
          }
        ]
      },
      { steps: threeSteps }
    );

    expect(screen.getByText("Step 1")).toBeInTheDocument();
    expect(screen.getByText("Step 2")).toBeInTheDocument();
    expect(screen.getByText("Step 3")).toBeInTheDocument();
  });

  it("step-1 completed + step-2 running transition keeps both nodes visible", () => {
    renderGraph({
      id: "run-1",
      project_id: "proj-1",
      name: "Pipeline",
      status: 1,
      step_results: [
        {
          id: "sr-1",
          pipeline_run_id: "run-1",
          step_id: "analyze",
          step_index: 0,
          status: 2,
          commit_sha: "",
          commit_message: "",
          git_diff: "",
          files_changed: 1,
          insertions: 5,
          deletions: 2,
          input_tokens: 100,
          output_tokens: 50,
          cache_read_tokens: 0,
          cache_create_tokens: 0,
          agent_output: "",
          duration: 10
        },
        {
          id: "sr-2",
          pipeline_run_id: "run-1",
          step_id: "implement",
          step_index: 1,
          status: 1,
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
          duration: 0
        }
      ]
    });

    expect(screen.getByText("Analyze")).toBeInTheDocument();
    expect(screen.getByText("Implement")).toBeInTheDocument();
    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("renders details drawer with event timeline", () => {
    render(
      <StepDetailsDrawer
        isOpen
        onClose={() => {}}
        step={steps[0]}
        result={null}
        events={[
          {
            event_id: "evt-1",
            task_id: "run-1",
            run_id: "run-1",
            event_type: "tool_use",
            timestamp: "2026-02-14T10:10:00Z",
            tool_name: "Read",
            tool_input_summary: "Read main.go"
          },
          {
            event_id: "evt-2",
            task_id: "run-1",
            run_id: "run-1",
            event_type: "tool_result",
            timestamp: "2026-02-14T10:10:01Z",
            tool_name: "Read",
            tool_success: true,
            content_preview: "Done"
          }
        ]}
      />
    );

    expect(screen.getByRole("complementary", { name: "Step details" })).toBeInTheDocument();
    expect(screen.getAllByText("Read main.go")).toHaveLength(2);
    expect(screen.getByText("Event timeline")).toBeInTheDocument();
  });
});

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RunGraph } from "./RunGraph";
import { StepDetailsDrawer } from "./StepDetailsDrawer";
import type { PipelineRun, StepDraft } from "../lib/types";

const steps: StepDraft[] = [
  { id: "analyze", name: "Analyze", prompt: "Analyze changes" },
  { id: "implement", name: "Implement", prompt: "Implement changes" }
];

function renderGraph(run: PipelineRun | null) {
  render(
    <div style={{ width: "1200px", height: "700px" }}>
      <RunGraph
        steps={steps}
        run={run}
        activitiesByStep={{}}
        selectedStepId={null}
        onSelectStep={() => {}}
      />
    </div>
  );
}

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
    // "Read main.go" appears in both Tool activity and Event timeline sections.
    expect(screen.getAllByText("Read main.go")).toHaveLength(2);
    expect(screen.getByText("Event timeline")).toBeInTheDocument();
  });
});

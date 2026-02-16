import { describe, expect, it } from "vitest";

import { mapActivitiesToSteps } from "./obs-mapping";
import type { AIActivityRecord, PipelineRun } from "./types";

function event(id: string, timestamp: string): AIActivityRecord {
  return {
    event_id: id,
    task_id: "run-1",
    run_id: "run-1",
    event_type: "tool_use",
    timestamp
  };
}

describe("mapActivitiesToSteps", () => {
  it("assigns events before first start to first non-skipped step", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 1,
      step_results: [
        {
          id: "sr-1",
          pipeline_run_id: "run-1",
          step_id: "step-a",
          step_index: 0,
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
          duration: 0,
          started_at: "2026-02-14T10:00:10Z"
        }
      ]
    };

    const mapped = mapActivitiesToSteps(run, [event("evt-1", "2026-02-14T10:00:00Z")], new Date("2026-02-14T10:10:00Z"));
    expect(mapped["step-a"]).toHaveLength(1);
  });

  it("assigns events after final window to last started step", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 1,
      step_results: [
        {
          id: "sr-1",
          pipeline_run_id: "run-1",
          step_id: "step-a",
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
          duration: 0,
          started_at: "2026-02-14T10:00:00Z",
          completed_at: "2026-02-14T10:01:00Z"
        },
        {
          id: "sr-2",
          pipeline_run_id: "run-1",
          step_id: "step-b",
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
          duration: 0,
          started_at: "2026-02-14T10:01:10Z"
        }
      ]
    };

    const mapped = mapActivitiesToSteps(run, [event("evt-1", "2026-02-14T11:20:00Z")], new Date("2026-02-14T11:21:00Z"));
    expect(mapped["step-b"]).toHaveLength(1);
  });

  it("does not create windows for skipped steps", () => {
    const run: PipelineRun = {
      id: "run-1",
      project_id: "project-1",
      name: "demo",
      status: 1,
      step_results: [
        {
          id: "sr-1",
          pipeline_run_id: "run-1",
          step_id: "step-skip",
          step_index: 0,
          status: 4,
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
        },
        {
          id: "sr-2",
          pipeline_run_id: "run-1",
          step_id: "step-live",
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
          duration: 0,
          started_at: "2026-02-14T10:02:00Z"
        }
      ]
    };

    const mapped = mapActivitiesToSteps(run, [event("evt-1", "2026-02-14T10:02:10Z")], new Date("2026-02-14T10:10:00Z"));
    expect(mapped["step-skip"]).toBeUndefined();
    expect(mapped["step-live"]).toHaveLength(1);
  });
});

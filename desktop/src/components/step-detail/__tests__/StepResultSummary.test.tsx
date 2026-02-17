import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StepResultSummary } from "../StepResultSummary";
import type { StepResult } from "../../../lib/types";

function makeResult(overrides?: Partial<StepResult>): StepResult {
  return {
    id: "sr-1",
    pipeline_run_id: "run-1",
    step_id: "step-1",
    step_index: 0,
    status: 2,
    commit_sha: "abc1234567890",
    commit_message: "Add feature",
    git_diff: "",
    files_changed: 3,
    insertions: 10,
    deletions: 2,
    input_tokens: 1500,
    output_tokens: 800,
    cache_read_tokens: 0,
    cache_create_tokens: 0,
    agent_output: "",
    duration: 5_000_000_000,
    ...overrides
  };
}

describe("StepResultSummary", () => {
  it("renders commit SHA truncated to 10 chars", () => {
    render(<StepResultSummary result={makeResult()} />);
    expect(screen.getByText("abc1234567")).toBeInTheDocument();
  });

  it("renders commit message", () => {
    render(<StepResultSummary result={makeResult()} />);
    expect(screen.getByText(/Add feature/)).toBeInTheDocument();
  });

  it("renders files changed with insertions and deletions", () => {
    render(<StepResultSummary result={makeResult()} />);
    expect(screen.getByText("3 (+10 -2)")).toBeInTheDocument();
  });

  it("renders duration in seconds", () => {
    render(<StepResultSummary result={makeResult()} />);
    expect(screen.getByText("5.0s")).toBeInTheDocument();
  });

  it("hides duration when zero", () => {
    render(<StepResultSummary result={makeResult({ duration: 0 })} />);
    expect(screen.queryByText("Duration")).not.toBeInTheDocument();
  });

  it("renders error message when present", () => {
    render(<StepResultSummary result={makeResult({ error_message: "Step timed out" })} />);
    expect(screen.getByText("Step timed out")).toBeInTheDocument();
  });

  it("shows cache stats when present", () => {
    render(<StepResultSummary result={makeResult({ cache_read_tokens: 500, cache_create_tokens: 100 })} />);
    expect(screen.getByText(/cache: 500 read, 100 create/)).toBeInTheDocument();
  });

  it("hides commit row when no commit_sha", () => {
    render(<StepResultSummary result={makeResult({ commit_sha: "" })} />);
    expect(screen.queryByText("Commit")).not.toBeInTheDocument();
  });
});

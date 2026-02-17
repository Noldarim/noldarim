import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { NoldarimGraphView } from "./NoldarimGraphView";
import { StepDetailsDrawer } from "./StepDetailsDrawer";
import { listPipelineRuns, getPipelineRun, getPipelineRunActivity } from "../lib/api";
import type { PipelineRun, PipelineRunsLoadedEvent, StepDraft } from "../lib/types";
import { useRunStore } from "../state/run-store";
import { useProjectGraphStore } from "../state/project-graph-store";

vi.mock("../lib/api", () => ({
  listPipelineRuns: vi.fn(),
  getPipelineRun: vi.fn(),
  getPipelineRunActivity: vi.fn()
}));

const steps: StepDraft[] = [
  { id: "analyze", name: "Analyze", prompt: "Analyze changes" },
  { id: "implement", name: "Implement", prompt: "Implement changes" }
];

const mockedListPipelineRuns = vi.mocked(listPipelineRuns);
const mockedGetPipelineRun = vi.mocked(getPipelineRun);
const mockedGetPipelineRunActivity = vi.mocked(getPipelineRunActivity);

function makeRunsPayload(projectId: string, runs: PipelineRun[]): PipelineRunsLoadedEvent {
  return {
    ProjectID: projectId,
    ProjectName: projectId,
    RepositoryPath: "/tmp/project",
    Runs: Object.fromEntries(runs.map((run) => [run.id, run]))
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

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
      <NoldarimGraphView projectId="proj-1" serverUrl="http://localhost:8080" selectedStep={null} onSelectStep={() => {}} onDeselectStep={() => {}} />
    </div>
  );
}

beforeEach(() => {
  mockedListPipelineRuns.mockResolvedValue(makeRunsPayload("proj-1", []));
  mockedGetPipelineRun.mockResolvedValue({
    id: "run-1",
    project_id: "proj-1",
    name: "Pipeline",
    status: 2,
    step_results: []
  });
  mockedGetPipelineRunActivity.mockResolvedValue({
    TaskID: "task-1",
    ProjectID: "proj-1",
    Activities: []
  });
  useRunStore.getState().reset();
  useProjectGraphStore.getState().reset();
});

afterEach(() => {
  vi.clearAllMocks();
  useRunStore.getState().reset();
  useProjectGraphStore.getState().reset();
});

describe("NoldarimGraphView", () => {
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

  it("ignores stale project-run responses when switching projects quickly", async () => {
    const first = deferred<PipelineRunsLoadedEvent>();
    const second = deferred<PipelineRunsLoadedEvent>();

    mockedListPipelineRuns.mockImplementation((_baseUrl, projectId) => {
      if (projectId === "proj-1") {
        return first.promise;
      }
      if (projectId === "proj-2") {
        return second.promise;
      }
      return Promise.resolve(makeRunsPayload(projectId, []));
    });

    const { rerender } = render(
      <div style={{ width: "1200px", height: "700px" }}>
        <NoldarimGraphView projectId="proj-1" serverUrl="http://localhost:8080" selectedStep={null} onSelectStep={() => {}} onDeselectStep={() => {}} />
      </div>
    );

    rerender(
      <div style={{ width: "1200px", height: "700px" }}>
        <NoldarimGraphView projectId="proj-2" serverUrl="http://localhost:8080" selectedStep={null} onSelectStep={() => {}} onDeselectStep={() => {}} />
      </div>
    );

    await act(async () => {
      second.resolve(
        makeRunsPayload("proj-2", [
          { id: "new-run", project_id: "proj-2", name: "new", status: 2, created_at: "2026-02-14T10:00:00Z" }
        ])
      );
      await second.promise;
    });

    await waitFor(() => {
      expect(useProjectGraphStore.getState().projectId).toBe("proj-2");
    });
    expect(useProjectGraphStore.getState().runs.map((run) => run.id)).toEqual(["new-run"]);

    await act(async () => {
      first.resolve(
        makeRunsPayload("proj-1", [
          { id: "old-run", project_id: "proj-1", name: "old", status: 2, created_at: "2026-02-14T09:00:00Z" }
        ])
      );
      await first.promise;
    });

    await waitFor(() => {
      expect(useProjectGraphStore.getState().runs.map((run) => run.id)).toEqual(["new-run"]);
    });
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

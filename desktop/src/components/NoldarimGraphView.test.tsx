// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import React, { useState } from "react";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { mergeAnimatedEdges, NoldarimGraphView } from "./NoldarimGraphView";
import { getCommits, getPipelineRun, getPipelineRunActivity, listPipelineRuns } from "../lib/api";
import { useRunStore } from "../state/run-store";
import { useProjectGraphStore } from "../state/project-graph-store";

vi.mock("../lib/api", () => ({
  listPipelineRuns: vi.fn(),
  getPipelineRun: vi.fn(),
  getPipelineRunActivity: vi.fn(),
  getCommits: vi.fn()
}));

const mockedListPipelineRuns = vi.mocked(listPipelineRuns);
const mockedGetPipelineRun = vi.mocked(getPipelineRun);
const mockedGetPipelineRunActivity = vi.mocked(getPipelineRunActivity);
const mockedGetCommits = vi.mocked(getCommits);

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function renderGraph(props?: { onSelectBaseCommit?: (sha: string) => void; onForkFromCommit?: (sha: string) => void }) {
  render(
    <div style={{ width: "1200px", height: "700px" }}>
      <NoldarimGraphView
        projectId="proj-1"
        serverUrl="http://localhost:8080"
        selectedBaseCommitSha={null}
        onSelectBaseCommit={props?.onSelectBaseCommit ?? (() => {})}
        onForkFromCommit={props?.onForkFromCommit ?? (() => {})}
      />
    </div>
  );
}

beforeEach(() => {
  mockedListPipelineRuns.mockResolvedValue({
    ProjectID: "proj-1",
    ProjectName: "proj-1",
    RepositoryPath: "/tmp/repo",
    Runs: {}
  });
  mockedGetPipelineRun.mockResolvedValue({
    id: "run-1",
    project_id: "proj-1",
    name: "Pipeline",
    status: 2,
    step_results: []
  });
  mockedGetPipelineRunActivity.mockResolvedValue({
    TaskID: "run-1",
    ProjectID: "proj-1",
    Activities: []
  });
  mockedGetCommits.mockResolvedValue({
    ProjectID: "proj-1",
    RepositoryPath: "/tmp/repo",
    Commits: [
      { Hash: "aaa111", Message: "a", Author: "bot", Parents: [] },
      { Hash: "bbb222", Message: "b", Author: "bot", Parents: [] },
      { Hash: "ccc333", Message: "c", Author: "bot", Parents: [] },
      { Hash: "ddd444", Message: "d", Author: "bot", Parents: [] }
    ]
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
  it("loads baseline commits when there are no runs", async () => {
    renderGraph();

    await screen.findByText("aaa111");
    expect(mockedGetCommits).toHaveBeenCalledWith(
      "http://localhost:8080",
      "proj-1",
      4,
      expect.any(Object)
    );
  });

  it("loads expanded commit context when runs exist", async () => {
    mockedListPipelineRuns.mockResolvedValue({
      ProjectID: "proj-1",
      ProjectName: "proj-1",
      RepositoryPath: "/tmp/repo",
      Runs: {
        "run-1": {
          id: "run-1",
          project_id: "proj-1",
          name: "Pipeline",
          status: 2,
          start_commit_sha: "bbb222",
          head_commit_sha: "aaa111",
          step_results: [
            {
              id: "sr-1",
              pipeline_run_id: "run-1",
              step_id: "s1",
              step_name: "Step 1",
              step_index: 0,
              status: 2,
              commit_sha: "aaa111",
              commit_message: "step 1",
              git_diff: "diff",
              files_changed: 1,
              insertions: 1,
              deletions: 0,
              input_tokens: 10,
              output_tokens: 10,
              cache_read_tokens: 0,
              cache_create_tokens: 0,
              agent_output: "ok",
              duration: 100_000_000
            }
          ],
          created_at: "2026-02-14T10:00:00Z"
        }
      }
    });

    renderGraph();

    await waitFor(() =>
      expect(mockedGetCommits).toHaveBeenCalledWith(
        "http://localhost:8080",
        "proj-1",
        200,
        expect.any(Object)
      )
    );

    expect(mockedGetPipelineRun).not.toHaveBeenCalled();
    expect(mockedGetPipelineRunActivity).not.toHaveBeenCalled();
  });

  it("selects base commit on commit click", async () => {
    const onSelectBaseCommit = vi.fn();
    renderGraph({ onSelectBaseCommit });

    const commitNode = await screen.findByText("aaa111");
    fireEvent.click(commitNode);

    expect(onSelectBaseCommit).toHaveBeenCalledWith("aaa111");
  });

  it("ignores stale run and commit responses after project switch", async () => {
    const listProj1 = deferred<Awaited<ReturnType<typeof listPipelineRuns>>>();
    const commitsProj1 = deferred<Awaited<ReturnType<typeof getCommits>>>();

    mockedListPipelineRuns.mockImplementation((_, projectId) => {
      if (projectId === "proj-1") {
        return listProj1.promise;
      }
      return Promise.resolve({
        ProjectID: "proj-2",
        ProjectName: "proj-2",
        RepositoryPath: "/tmp/repo-2",
        Runs: {}
      });
    });
    mockedGetCommits.mockImplementation((_, projectId) => {
      if (projectId === "proj-1") {
        return commitsProj1.promise;
      }
      return Promise.resolve({
        ProjectID: "proj-2",
        RepositoryPath: "/tmp/repo-2",
        Commits: [{ Hash: "new22222", Message: "new", Author: "bot", Parents: [] }]
      });
    });

    function Wrapper() {
      const [projectId, setProjectId] = useState("proj-1");
      return (
        <>
          <button type="button" onClick={() => setProjectId("proj-2")}>Switch Project</button>
          <div style={{ width: "1200px", height: "700px" }}>
            <NoldarimGraphView
              projectId={projectId}
              serverUrl="http://localhost:8080"
              selectedBaseCommitSha={null}
              onSelectBaseCommit={() => {}}
              onForkFromCommit={() => {}}
            />
          </div>
        </>
      );
    }

    render(<Wrapper />);
    fireEvent.click(screen.getByRole("button", { name: "Switch Project" }));

    await screen.findByText("new22222");
    listProj1.resolve({
      ProjectID: "proj-1",
      ProjectName: "proj-1",
      RepositoryPath: "/tmp/repo-1",
      Runs: {
        "run-old": {
          id: "run-old",
          project_id: "proj-1",
          name: "Old Run",
          status: 2
        }
      }
    });
    commitsProj1.resolve({
      ProjectID: "proj-1",
      RepositoryPath: "/tmp/repo-1",
      Commits: [{ Hash: "old11111", Message: "old", Author: "bot", Parents: [] }]
    });

    await waitFor(() => {
      expect(screen.queryByText("old11111")).not.toBeInTheDocument();
    });
  });

  it("fetches selected run details on demand when user selects a run node", async () => {
    mockedListPipelineRuns.mockResolvedValue({
      ProjectID: "proj-1",
      ProjectName: "proj-1",
      RepositoryPath: "/tmp/repo",
      Runs: {
        "run-1": {
          id: "run-1",
          project_id: "proj-1",
          name: "Pipeline",
          status: 2,
          start_commit_sha: "bbb222",
          head_commit_sha: "aaa111",
          step_results: [
            {
              id: "sr-1",
              pipeline_run_id: "run-1",
              step_id: "s1",
              step_name: "Step 1",
              step_index: 0,
              status: 2,
              commit_sha: "aaa111",
              commit_message: "step 1",
              git_diff: "diff",
              files_changed: 1,
              insertions: 1,
              deletions: 0,
              input_tokens: 10,
              output_tokens: 10,
              cache_read_tokens: 0,
              cache_create_tokens: 0,
              agent_output: "ok",
              duration: 100_000_000
            }
          ],
          created_at: "2026-02-14T10:00:00Z"
        }
      }
    });

    renderGraph();
    const runNodeTitle = await screen.findByText("Pipeline");
    expect(mockedGetPipelineRun).not.toHaveBeenCalled();
    expect(mockedGetPipelineRunActivity).not.toHaveBeenCalled();

    fireEvent.click(runNodeTitle);

    await waitFor(() => {
      expect(mockedGetPipelineRun).toHaveBeenCalledWith("http://localhost:8080", "run-1");
      expect(mockedGetPipelineRunActivity).toHaveBeenCalledWith("http://localhost:8080", "run-1");
    });
  });
});

describe("mergeAnimatedEdges", () => {
  it("deduplicates edges that are both active and exiting", () => {
    const edge = (id: string) => ({ id } as unknown as import("@xyflow/react").Edge);
    const merged = mergeAnimatedEdges(
      [edge("edge-1"), edge("edge-2")],
      [edge("edge-1"), edge("edge-3")]
    );

    expect(merged.map((edge) => edge.id)).toEqual(["edge-1", "edge-2", "edge-3"]);
  });
});

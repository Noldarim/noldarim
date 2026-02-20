import React, { useState } from "react";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { NoldarimGraphView } from "./NoldarimGraphView";
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
});

// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useFleetStore, type FleetRun } from "../../state/fleet-store";
import { FleetOverviewPanel } from "../FleetOverviewPanel";

function makeRun(overrides: Partial<FleetRun> = {}): FleetRun {
  return {
    runId: `run-${Math.random().toString(36).slice(2, 8)}`,
    projectId: "proj-1",
    projectName: "TestProject",
    name: "my-run",
    status: "running",
    stepCount: 3,
    completedSteps: 1,
    totalTokens: 5000,
    startedAt: new Date().toISOString(),
    ...overrides
  };
}

describe("FleetOverviewPanel", () => {
  afterEach(() => {
    useFleetStore.getState().reset();
  });

  it("renders nothing when no runs", () => {
    const { container } = render(<FleetOverviewPanel />);
    expect(container.querySelector(".fleet-panel")).toBeNull();
  });

  it("renders fleet rows when runs exist", () => {
    useFleetStore.getState().runsUpdated([
      makeRun({ runId: "r1", name: "Alpha Run" }),
      makeRun({ runId: "r2", name: "Beta Run", status: "completed" })
    ]);
    render(<FleetOverviewPanel />);
    expect(screen.getByText(/Fleet \(2\)/)).toBeDefined();
    expect(screen.getByText("Alpha Run")).toBeDefined();
    expect(screen.getByText("Beta Run")).toBeDefined();
  });

  it("calls onNavigateToRun when clicking a row", async () => {
    const navigate = vi.fn();
    useFleetStore.getState().runsUpdated([makeRun({ runId: "r1", name: "Click Me" })]);
    render(<FleetOverviewPanel onNavigateToRun={navigate} />);
    screen.getByText("Click Me").click();
    expect(navigate).toHaveBeenCalledWith("r1", "proj-1");
  });
});

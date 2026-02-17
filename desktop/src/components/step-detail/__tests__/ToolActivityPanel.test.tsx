import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ToolActivityPanel } from "../ToolActivityPanel";
import type { ToolNameGroup } from "../../../lib/obs-mapping";

describe("ToolActivityPanel", () => {
  it("renders empty state when no groups", () => {
    render(<ToolActivityPanel groups={[]} />);
    expect(screen.getByText("No tool events for this step yet.")).toBeInTheDocument();
  });

  it("renders tool name with count badge", () => {
    const groups: ToolNameGroup[] = [
      {
        toolName: "bash",
        calls: [
          { toolName: "bash", input: "ls -la", result: { success: true, output: "", error: "" } },
          { toolName: "bash", input: "go build", result: { success: true, output: "", error: "" } }
        ]
      }
    ];
    render(<ToolActivityPanel groups={groups} />);
    expect(screen.getByText("bash")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("renders individual call inputs", () => {
    const groups: ToolNameGroup[] = [
      {
        toolName: "Read",
        calls: [
          { toolName: "Read", input: "main.go" }
        ]
      }
    ];
    render(<ToolActivityPanel groups={groups} />);
    expect(screen.getByText("main.go")).toBeInTheDocument();
    expect(screen.getByText("Awaiting result")).toBeInTheDocument();
  });

  it("renders success and failure states", () => {
    const groups: ToolNameGroup[] = [
      {
        toolName: "bash",
        calls: [
          { toolName: "bash", input: "ok cmd", result: { success: true, output: "", error: "" } },
          { toolName: "bash", input: "bad cmd", result: { success: false, output: "", error: "exit 1" } }
        ]
      }
    ];
    render(<ToolActivityPanel groups={groups} />);
    expect(screen.getByText("Success")).toBeInTheDocument();
    expect(screen.getByText("Failed: exit 1")).toBeInTheDocument();
  });
});

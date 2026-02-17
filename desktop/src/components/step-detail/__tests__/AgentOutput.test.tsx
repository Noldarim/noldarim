import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AgentOutput } from "../AgentOutput";

describe("AgentOutput", () => {
  it("renders within a collapsible details element", () => {
    render(<AgentOutput output="Hello from agent" />);
    expect(screen.getByText("Agent output")).toBeInTheDocument();
    expect(screen.getByText("Hello from agent")).toBeInTheDocument();
  });

  it("renders output text in a pre element", () => {
    render(<AgentOutput output={"line 1\nline 2"} />);
    const pre = screen.getByText((_, el) => el?.tagName === "PRE" && el.textContent === "line 1\nline 2");
    expect(pre.tagName).toBe("PRE");
  });
});
